package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/tui"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fda"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/scanners"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// interactiveDeps is the test seam: production wires real engine calls
// (newInteractiveDeps); tests substitute fakes without touching $HOME or a
// TTY.
type interactiveDeps struct {
	scan         func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary
	home         string
	cfg          config.Config
	backupMgr    *backup.Manager
	confirm      func(message string, defaultYes bool) bool
	printResults func(summary core.CleanSummary, fdaUnknownOrDenied bool)
}

func newInteractiveDeps(cfg config.Config, home string) interactiveDeps {
	return interactiveDeps{
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			ids := make([]core.CategoryID, 0, len(core.CategoriesInOrder()))
			for _, c := range core.CategoriesInOrder() {
				ids = append(ids, c.ID)
			}
			return scanners.RunScans(ctx, ids, scanners.Options{Roots: scanners.DefaultRoots(), Cfg: cfg, Runner: &scanners.ExecRunner{}}, cfg.Concurrency, onResult)
		},
		home:         home,
		cfg:          cfg,
		backupMgr:    backup.NewManager(home),
		confirm:      runConfirm,
		printResults: printCleanResults,
	}
}

// InteractiveOptions bags the interactive-mode-only flags this file registers
// on rootCmd (Task 1's command). It is NOT a Task 1 type and is not part of
// any exported signature — runInteractive takes it so tests drive the flow
// without cobra.
type InteractiveOptions struct {
	IncludeRisky  bool
	NoProgress    bool
	AbsolutePaths bool
	FilePicker    bool // drill-down for ALL categories, not just SupportsFileSelection
}

// Interactive-mode flags, registered on rootCmd (declared by Task 1, same
// package) and read back inside the RunInteractive reassignment below.
var (
	interactiveRisky      bool
	interactiveNoProgress bool
	interactiveAbsPaths   bool
	interactiveFilePicker bool
)

func init() {
	rootCmd.Flags().BoolVar(&interactiveRisky, "risky", false, "include risky categories in the interactive picker")
	rootCmd.Flags().BoolVar(&interactiveNoProgress, "no-progress", false, "suppress live scan-progress output")
	rootCmd.Flags().BoolVar(&interactiveAbsPaths, "absolute-paths", false, "show absolute paths (no ~ contraction) in the file picker")
	rootCmd.Flags().BoolVar(&interactiveFilePicker, "file-picker", false, "enable per-file drill-down for every category, not just file-selection ones")

	// Reassign Task 1's RunInteractive stub var (rootCmd.RunE already
	// delegates to it) with the real flow. Ctrl-C context is set up here.
	RunInteractive = func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		opts := InteractiveOptions{
			IncludeRisky:  interactiveRisky,
			NoProgress:    interactiveNoProgress,
			AbsolutePaths: interactiveAbsPaths,
			FilePicker:    interactiveFilePicker,
		}
		home := scanners.DefaultRoots().Home
		cfg := config.Load(config.DefaultPath(home))
		_, err := runInteractive(ctx, opts, newInteractiveDeps(cfg, home))
		return err
	}
}

func runInteractive(ctx context.Context, opts InteractiveOptions, deps interactiveDeps) (*core.CleanSummary, error) {
	fmt.Println()
	fmt.Println("App Cleaner")
	fmt.Println(dimRule(50))
	fmt.Println()

	if granted := fda.Check(deps.home); granted != nil && !*granted {
		fmt.Printf("Full Disk Access not detected. Some items may fail with EPERM — grant access at %s\n\n", fda.SettingsURL)
	}

	fmt.Println("Scanning your Mac for cleanable files...")
	summary := deps.scan(ctx, func(completed, total int, r core.ScanResult) {
		fmt.Printf("\r[%d/%d] Scanning %s...", completed, total, r.Category.Name)
	})
	fmt.Println()

	if summary.TotalSize == 0 {
		fmt.Println("Your Mac is already clean! Nothing to remove.")
		return nil, nil
	}

	kept, hidden := hideRiskyCategories(nonEmpty(summary.Results), opts.IncludeRisky)
	if len(hidden) > 0 {
		var hiddenSize int64
		for _, r := range hidden {
			hiddenSize += r.TotalSize
		}
		fmt.Printf("Hiding %d risky categories (%s). Run with --risky to include them.\n\n", len(hidden), core.FormatSize(hiddenSize))
	}
	if len(kept) == 0 {
		fmt.Println("Nothing safe to clean!")
		return nil, nil
	}

	return finishInteractive(ctx, opts, deps, kept)
}

func nonEmpty(results []core.ScanResult) []core.ScanResult {
	out := make([]core.ScanResult, 0, len(results))
	for _, r := range results {
		if len(r.Items) > 0 {
			out = append(out, r)
		}
	}
	return out
}

func hideRiskyCategories(results []core.ScanResult, includeRisky bool) (kept, hidden []core.ScanResult) {
	if includeRisky {
		return results, nil
	}
	for _, r := range results {
		if r.Category.SafetyLevel == core.SafetyRisky {
			hidden = append(hidden, r)
		} else {
			kept = append(kept, r)
		}
	}
	return kept, hidden
}

func dimRule(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = '-'
	}
	return string(out)
}

func finishInteractive(ctx context.Context, opts InteractiveOptions, deps interactiveDeps, results []core.ScanResult) (*core.CleanSummary, error) {
	categoriesWithFiles := map[core.CategoryID]bool{}
	for _, r := range results {
		if r.Category.SupportsFileSelection || opts.FilePicker {
			categoriesWithFiles[r.Category.ID] = true
		}
	}

	cp := tui.NewCategoryPickerModel(results)
	finalCP, err := runTeaProgram(cp)
	if err != nil {
		return nil, err
	}
	chosen := finalCP.(tui.CategoryPickerModel).Chosen()
	if len(chosen) == 0 {
		fmt.Println("No items selected. Nothing to clean.")
		return nil, nil
	}

	selectedResults := filterResults(results, chosen)
	needsFilePicker := false
	for _, id := range chosen {
		if categoriesWithFiles[id] {
			needsFilePicker = true
			break
		}
	}

	selectedCategories := map[core.CategoryID]bool{}
	for _, id := range chosen {
		selectedCategories[id] = true
	}
	selectedFiles := map[core.CategoryID]map[string]bool{}

	if needsFilePicker {
		fp := tui.NewFilePickerModel(selectedResults, categoriesWithFiles, deps.home, opts.AbsolutePaths)
		finalFP, err := runTeaProgram(fp)
		if err != nil {
			return nil, err
		}
		selectedCategories, selectedFiles = finalFP.(tui.FilePickerModel).Result()
	}

	itemsByCategory := map[core.CategoryID][]core.CleanableItem{}
	var totalItems int
	var totalBytes int64
	for _, r := range selectedResults {
		if !selectedCategories[r.Category.ID] {
			continue
		}
		if categoriesWithFiles[r.Category.ID] {
			files := selectedFiles[r.Category.ID]
			if len(files) == 0 {
				continue // category flagged but no files chosen -> skip, per original parity
			}
			var kept []core.CleanableItem
			for _, it := range r.Items {
				if files[it.Path] {
					kept = append(kept, it)
				}
			}
			itemsByCategory[r.Category.ID] = kept
			for _, it := range kept {
				totalItems++
				totalBytes += it.Size
			}
		} else {
			itemsByCategory[r.Category.ID] = r.Items
			for _, it := range r.Items {
				totalItems++
				totalBytes += it.Size
			}
		}
	}
	if totalItems == 0 {
		fmt.Println("No items selected. Nothing to clean.")
		return nil, nil
	}

	fmt.Println()
	fmt.Printf("Items to delete: %d\n", totalItems)
	fmt.Printf("Space to free: %s\n\n", core.FormatSize(totalBytes))

	confirmFn := deps.confirm
	if confirmFn == nil {
		confirmFn = runConfirm
	}
	if !confirmFn("Proceed with cleaning?", true) {
		fmt.Println("Cleaning cancelled.")
		return nil, nil
	}

	return runInteractiveClean(ctx, deps, selectedResults, itemsByCategory)
}

func filterResults(results []core.ScanResult, ids []core.CategoryID) []core.ScanResult {
	want := map[core.CategoryID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []core.ScanResult
	for _, r := range results {
		if want[r.Category.ID] {
			out = append(out, r)
		}
	}
	return out
}

// runInteractiveClean backs up (if configured) then permanently removes the
// selected items, category by category, printing progress; returns a partial
// summary on Ctrl-C (ctx canceled) rather than an error, matching the
// engine's cancel semantics (in-flight item finishes, remaining untouched).
// Named to avoid colliding with Task 5's cobra RunE `runClean` in package cmd.
func runInteractiveClean(ctx context.Context, deps interactiveDeps, results []core.ScanResult, itemsByCategory map[core.CategoryID][]core.CleanableItem) (*core.CleanSummary, error) {
	summary := core.CleanSummary{}
	for _, r := range results {
		items := itemsByCategory[r.Category.ID]
		if len(items) == 0 {
			continue
		}
		fmt.Printf("Cleaning %s...\n", r.Category.Name)

		toDelete := items
		if deps.cfg.BackupByDefault && deps.backupMgr != nil {
			outcome := deps.backupMgr.BackupItems(ctx, deps.home, items, func(current, total int, item core.CleanableItem) {
				fmt.Printf("\r  backing up %d/%d", current, total)
			})
			fmt.Println()
			toDelete = itemsFromPaths(items, outcome.NotBackedUp) // only permanently delete items the backup step could not move
		}

		out := fsx.RemoveItems(ctx, toDelete, false, func(current, total int, item core.CleanableItem) {
			fmt.Printf("\r  %d/%d", current, total)
		})
		fmt.Println()

		freedFromBackup := int64(0)
		backedUpCount := len(items) - len(toDelete)
		for _, it := range items {
			if !containsPath(toDelete, it.Path) {
				freedFromBackup += it.Size
			}
		}
		errs := fsx.AggregateFailures(out.Failures)
		summary.Results = append(summary.Results, core.CleanResult{
			Category:     r.Category,
			CleanedItems: out.Cleaned + backedUpCount,
			FreedSpace:   out.Freed + freedFromBackup,
			Errors:       errs,
		})
		summary.TotalCleanedItems += out.Cleaned + backedUpCount
		summary.TotalFreedSpace += out.Freed + freedFromBackup
		summary.TotalErrors += len(errs)

		if ctx.Err() != nil {
			break // Ctrl-C: stop starting new categories, return partial summary
		}
	}

	printResults := deps.printResults
	if printResults == nil {
		printResults = printCleanResults
	}
	granted := fda.Check(deps.home)
	fdaHint := summary.TotalErrors > 0 && (granted == nil || !*granted)
	printResults(summary, fdaHint)
	return &summary, nil
}

func itemsFromPaths(all []core.CleanableItem, paths []string) []core.CleanableItem {
	want := map[string]bool{}
	for _, p := range paths {
		want[p] = true
	}
	var out []core.CleanableItem
	for _, it := range all {
		if want[it.Path] {
			out = append(out, it)
		}
	}
	return out
}

func containsPath(items []core.CleanableItem, path string) bool {
	for _, it := range items {
		if it.Path == path {
			return true
		}
	}
	return false
}

// runConfirm is a minimal default-aware y/n prompt over stdin, matching
// the original's @inquirer/confirm default-yes for interactive mode.
func runConfirm(message string, defaultYes bool) bool {
	hint := "Y/n"
	if !defaultYes {
		hint = "y/N"
	}
	fmt.Printf("%s (%s) ", message, hint)
	var answer string
	_, _ = fmt.Scanln(&answer)
	switch answer {
	case "":
		return defaultYes
	case "y", "Y", "yes":
		return true
	default:
		return false
	}
}

func printCleanResults(summary core.CleanSummary, fdaHint bool) {
	fmt.Println()
	fmt.Println("Cleaning Complete!")
	fmt.Println(dimRule(50))
	for _, r := range summary.Results {
		if r.CleanedItems > 0 {
			fmt.Printf("  %-30s freed %s\n", r.Category.Name, core.FormatSize(r.FreedSpace))
		}
		for _, e := range r.Errors {
			fmt.Printf("  %-30s %s\n", r.Category.Name, e)
		}
	}
	fmt.Println(dimRule(50))
	fmt.Printf("Freed %s of disk space! Cleaned %d items\n", core.FormatSize(summary.TotalFreedSpace), summary.TotalCleanedItems)
	if summary.TotalErrors > 0 {
		fmt.Printf("Errors: %d\n", summary.TotalErrors)
	}
	if fdaHint {
		fmt.Printf("Some failures look permission-related — grant Full Disk Access: %s\n", fda.SettingsURL)
	}
}

// runTeaProgram runs a bubbletea program to completion and returns its
// final model. Extracted as a seam: interactive.go's picker steps call
// this instead of tea.NewProgram directly so integration tests can swap
// ProgramOptions (tea.WithInput/tea.WithoutRenderer) without a real TTY.
var runTeaProgram = func(model tea.Model) (tea.Model, error) {
	return tea.NewProgram(model).Run()
}
