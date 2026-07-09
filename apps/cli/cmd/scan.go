// Package cmd implements the app-cleaner CLI's cobra commands.
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/output"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/scanners"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	scanCategories []string
	scanJSON       bool
)

// scanRunner is the test seam over scanners.RunScans: cmd/scan_test.go and
// cmd/clean_test.go swap it for a fake so tests never touch a real scanner
// or the filesystem paths a real scan would walk.
var scanRunner = scanners.RunScans

// homeDirFn/configLoadFn are test seams: overridden in tests so scan/clean
// never resolve the real $HOME or read a real config.json (contract:
// t.TempDir() only, never the real $HOME).
var (
	homeDirFn    = os.UserHomeDir
	configLoadFn = func(home string) config.Config {
		return config.Load(config.DefaultPath(home))
	}
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan your Mac for cleanable files",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().StringSliceVar(&scanCategories, "categories", nil,
		"comma-separated category ids to scan (default: all 16)")
	scanCmd.Flags().BoolVar(&scanJSON, "json", false, "output machine-readable JSON")
	rootCmd.AddCommand(scanCmd)
}

// resolveCategories maps a --categories flag value onto the ids to scan
// (all 16, in stable core.CategoriesInOrder() order, when raw is empty)
// plus the subset of raw that scanners.Get does not recognize, so the
// caller can warn and skip them (mirrors apps/desktop/app.go's
// resolveScanIDs, extended to also report which ids were unknown instead
// of silently dropping them).
func resolveCategories(raw []string) (ids []core.CategoryID, unknown []string) {
	if len(raw) == 0 {
		for _, cat := range core.CategoriesInOrder() {
			ids = append(ids, cat.ID)
		}
		return ids, nil
	}
	for _, r := range raw {
		if _, ok := scanners.Get(core.CategoryID(r)); ok {
			ids = append(ids, core.CategoryID(r))
		} else {
			unknown = append(unknown, r)
		}
	}
	return ids, unknown
}

func warnUnknownCategories(cmd *cobra.Command, unknown []string) {
	for _, u := range unknown {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: unknown category %q, skipping\n", u)
	}
}

// stdoutIsTTY reports whether the process's real stdout is a terminal, as
// opposed to a pipe, file, or the buffer cmd.SetOut() installs in tests.
// Deliberately checks os.Stdout directly (not cmd.OutOrStdout()): cobra's
// output writer is redirectable for testability, but the
// spinner-vs-plain-lines decision must reflect the REAL terminal.
func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func defaultRoots(home string) scanners.Roots {
	return scanners.Roots{
		Home:         home,
		Tmp:          "/tmp",
		VarFolders:   "/private/var/folders",
		Applications: "/Applications",
	}
}

func runScan(cmd *cobra.Command, args []string) error {
	ids, unknown := resolveCategories(scanCategories)
	warnUnknownCategories(cmd, unknown)
	if len(ids) == 0 {
		return fmt.Errorf("no valid categories specified")
	}

	home, err := homeDirFn()
	if err != nil {
		home = "/"
	}
	cfg := configLoadFn(home)
	opts := scanners.Options{
		Roots:  defaultRoots(home),
		Cfg:    cfg,
		Runner: &scanners.ExecRunner{},
	}

	out := cmd.OutOrStdout()
	ctx := context.Background()

	var summary core.ScanSummary
	switch {
	case scanJSON:
		summary = scanRunner(ctx, ids, opts, cfg.Concurrency, nil)
	case stdoutIsTTY():
		summary = runScanWithSpinner(ctx, ids, opts, cfg)
	default:
		summary = scanRunner(ctx, ids, opts, cfg.Concurrency, func(completed, total int, r core.ScanResult) {
			_, _ = fmt.Fprintf(out, "Scanning %s... (%d/%d)\n", r.Category.Name, completed, total)
		})
	}

	if scanJSON {
		data, err := output.MarshalScanJSON(summary, false)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(out, string(data))
		return nil
	}
	printScanReport(out, summary)
	return nil
}

// --- TTY spinner (bubbles/spinner + bubbletea) ---------------------------

type scanProgressMsg struct {
	categoryName     string
	completed, total int
}

type scanDoneMsg struct{ summary core.ScanSummary }

type scanSpinnerModel struct {
	spinner spinner.Model
	label   string
	updates chan interface{}
	summary core.ScanSummary
}

func waitForScanUpdate(updates chan interface{}) tea.Cmd {
	return func() tea.Msg { return <-updates }
}

func (m scanSpinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForScanUpdate(m.updates))
}

func (m scanSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(v)
		return m, cmd
	case scanProgressMsg:
		m.label = fmt.Sprintf("Scanning %s... (%d/%d)", v.categoryName, v.completed, v.total)
		return m, waitForScanUpdate(m.updates)
	case scanDoneMsg:
		m.summary = v.summary
		return m, tea.Quit
	}
	return m, nil
}

func (m scanSpinnerModel) View() string {
	if m.label == "" {
		return ""
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.label)
}

// runScanWithSpinner runs scanRunner in a goroutine while a bubbletea
// program renders a spinner and the currently-scanning category name; used
// only when stdoutIsTTY() (runScan's gate). Falls back to a silent
// synchronous scan if the TUI program itself fails to start/run.
func runScanWithSpinner(ctx context.Context, ids []core.CategoryID, opts scanners.Options, cfg config.Config) core.ScanSummary {
	updates := make(chan interface{}, 1)
	go func() {
		summary := scanRunner(ctx, ids, opts, cfg.Concurrency, func(completed, total int, r core.ScanResult) {
			updates <- scanProgressMsg{categoryName: r.Category.Name, completed: completed, total: total}
		})
		updates <- scanDoneMsg{summary: summary}
	}()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	model := scanSpinnerModel{
		spinner: sp,
		label:   "Scanning your Mac for cleanable files...",
		updates: updates,
	}

	final, err := tea.NewProgram(model).Run()
	if err != nil {
		return scanRunner(ctx, ids, opts, cfg.Concurrency, nil)
	}
	return final.(scanSpinnerModel).summary
}

// --- human report ----------------------------------------------------------

func safetyDot(level core.SafetyLevel) string {
	color := lipgloss.Color("2") // green
	switch level {
	case core.SafetyModerate:
		color = lipgloss.Color("3") // yellow
	case core.SafetyRisky:
		color = lipgloss.Color("1") // red
	}
	return lipgloss.NewStyle().Foreground(color).Render("●")
}

func printScanReport(w io.Writer, summary core.ScanSummary) {
	_, _ = fmt.Fprintln(w, lipgloss.NewStyle().Bold(true).Render("Scan Results"))
	_, _ = fmt.Fprintln(w, output.Rule(60))
	for _, res := range summary.Results {
		if res.TotalSize == 0 {
			continue
		}
		_, _ = fmt.Fprintf(w, "  %s %-28s %10s (%d items)\n",
			safetyDot(res.Category.SafetyLevel), res.Category.Name, core.FormatSize(res.TotalSize), len(res.Items))
	}
	_, _ = fmt.Fprintln(w, output.Rule(60))
	_, _ = fmt.Fprintln(w, lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("Total: %s can be cleaned (%d items)", core.FormatSize(summary.TotalSize), summary.TotalItems),
	))
	_, _ = fmt.Fprintf(w, "Safety: %s safe  %s moderate  %s risky\n",
		safetyDot(core.SafetySafe), safetyDot(core.SafetyModerate), safetyDot(core.SafetyRisky))
}
