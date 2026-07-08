package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/output"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/scanners"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	cleanCategories []string
	cleanDryRun     bool
	cleanYes        bool
	cleanBackup     bool
	cleanNoBackup   bool
	cleanJSON       bool
)

// neverBackup lists categories cleaned via their own external tool (brew
// cleanup / docker system prune) whose items are therefore never offered
// to the backup manager. This bridge-level policy lives in apps/desktop's
// app.go (splitNeverBackup/neverBackup) rather than packages/engine, so
// the CLI mirrors it verbatim here rather than importing a desktop-only
// file.
var neverBackup = map[core.CategoryID]bool{
	"homebrew": true,
	"docker":   true,
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean cleanable files from your Mac",
	RunE:  runClean,
}

func init() {
	cleanCmd.Flags().StringSliceVar(&cleanCategories, "categories", nil,
		"comma-separated category ids to clean (default: all 16)")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false,
		"show what would be cleaned without deleting or backing up anything")
	cleanCmd.Flags().BoolVar(&cleanYes, "yes", false, "skip the confirmation prompt")
	cleanCmd.Flags().BoolVar(&cleanBackup, "backup", false,
		"back up items before deleting (default: config's backupByDefault)")
	cleanCmd.Flags().BoolVar(&cleanNoBackup, "no-backup", false,
		"delete items directly without backing them up")
	cleanCmd.Flags().BoolVar(&cleanJSON, "json", false, "output machine-readable JSON")
	rootCmd.AddCommand(cleanCmd)
}

// resolveBackupFlag resolves the effective backup-before-delete decision
// from --backup/--no-backup (mutually exclusive; either wins when passed,
// honoring its explicit value even as --flag=false) or cfgDefault
// (config.Config.BackupByDefault) when neither flag was passed.
func resolveBackupFlag(cmd *cobra.Command, cfgDefault bool) (bool, error) {
	backupSet := cmd.Flags().Changed("backup")
	noBackupSet := cmd.Flags().Changed("no-backup")
	if backupSet && noBackupSet {
		return false, fmt.Errorf("--backup and --no-backup are mutually exclusive")
	}
	if backupSet {
		v, _ := cmd.Flags().GetBool("backup")
		return v, nil
	}
	if noBackupSet {
		v, _ := cmd.Flags().GetBool("no-backup")
		return !v, nil
	}
	return cfgDefault, nil
}

// selectAllFromScan takes every item of every non-empty scanned category —
// the non-interactive `clean` contract: --categories (or its absence,
// meaning all) always selects every item in the resolved categories, with
// no file-picker prompt (that only exists in the TUI's interactive mode).
func selectAllFromScan(summary core.ScanSummary) map[core.CategoryID][]core.CleanableItem {
	out := map[core.CategoryID][]core.CleanableItem{}
	for _, res := range summary.Results {
		if len(res.Items) == 0 {
			continue
		}
		out[res.Category.ID] = res.Items
	}
	return out
}

// splitNeverBackup partitions the resolved selection BEFORE the backup
// pass: neverBackup categories go straight to the clean path; everything
// else is eligible for backup.BackupItems. Mirrors app.go's function of
// the same name.
func splitNeverBackup(resolved map[core.CategoryID][]core.CleanableItem) (backupable, direct map[core.CategoryID][]core.CleanableItem) {
	backupable = map[core.CategoryID][]core.CleanableItem{}
	direct = map[core.CategoryID][]core.CleanableItem{}
	for id, items := range resolved {
		if neverBackup[id] {
			direct[id] = items
		} else {
			backupable[id] = items
		}
	}
	return backupable, direct
}

// splitByBackup partitions a backup.BackupOutcome back onto the resolved
// selection: items in movedPaths only need summary credit (already off
// disk); items in notBackedUp must still be permanently deleted by their
// scanner; items in neither (backup pass cancelled partway through) are
// left untouched and excluded entirely. Mirrors app.go's function of the
// same name.
func splitByBackup(resolved map[core.CategoryID][]core.CleanableItem, movedPaths, notBackedUp []string) (moved, remaining map[core.CategoryID][]core.CleanableItem) {
	movedSet := make(map[string]bool, len(movedPaths))
	for _, p := range movedPaths {
		movedSet[p] = true
	}
	skip := make(map[string]bool, len(notBackedUp))
	for _, p := range notBackedUp {
		skip[p] = true
	}
	moved = map[core.CategoryID][]core.CleanableItem{}
	remaining = map[core.CategoryID][]core.CleanableItem{}
	for id, items := range resolved {
		for _, it := range items {
			switch {
			case movedSet[it.Path]:
				moved[id] = append(moved[id], it)
			case skip[it.Path]:
				remaining[id] = append(remaining[id], it)
			}
		}
	}
	return moved, remaining
}

func runClean(cmd *cobra.Command, args []string) error {
	ids, unknown := resolveCategories(cleanCategories)
	warnUnknownCategories(cmd, unknown)
	if len(ids) == 0 {
		return fmt.Errorf("no valid categories specified")
	}

	home, err := homeDirFn()
	if err != nil {
		home = "/"
	}
	cfg := configLoadFn(home)

	backupEnabled, err := resolveBackupFlag(cmd, cfg.BackupByDefault)
	if err != nil {
		return err
	}

	opts := scanners.Options{
		Roots:  defaultRoots(home),
		Cfg:    cfg,
		Runner: &scanners.ExecRunner{},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	out := cmd.OutOrStdout()
	if !cleanJSON {
		_, _ = fmt.Fprintln(out, "Scanning...")
	}
	scanSummary := scanRunner(ctx, ids, opts, cfg.Concurrency, nil)

	resolved := selectAllFromScan(scanSummary)
	if len(resolved) == 0 {
		if cleanJSON {
			data, err := output.MarshalCleanJSON(core.CleanSummary{})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(out, string(data))
			return nil
		}
		_, _ = fmt.Fprintln(out, "✓ Your Mac is already clean! Nothing to remove.")
		return nil
	}

	var totalItems int
	var totalSize int64
	for _, items := range resolved {
		for _, it := range items {
			totalItems++
			totalSize += it.Size
		}
	}

	if !cleanDryRun && !cleanYes {
		// Non-interactive clean's confirm defaults to NO (contract).
		if !promptConfirm(cmd.InOrStdin(), out, fmt.Sprintf("Delete %d items (%s)?", totalItems, core.FormatSize(totalSize)), false) {
			_, _ = fmt.Fprintln(out, "Cleaning cancelled.")
			return nil
		}
	}

	notBackedUp := []string{}
	movedByCat := map[core.CategoryID][]core.CleanableItem{}
	remaining := resolved
	if backupEnabled && !cleanDryRun {
		backupable, direct := splitNeverBackup(resolved)
		var all []core.CleanableItem
		for _, cat := range core.CategoriesInOrder() {
			all = append(all, backupable[cat.ID]...)
		}
		var outcome backup.BackupOutcome
		if len(all) > 0 {
			mgr := backup.NewManager(home)
			outcome = mgr.BackupItems(ctx, home, all, nil)
		}
		if outcome.NotBackedUp != nil {
			notBackedUp = outcome.NotBackedUp
		}
		movedByCat, remaining = splitByBackup(backupable, outcome.Moved, outcome.NotBackedUp)
		for id, items := range direct {
			remaining[id] = items
		}
	}

	var cleanSummary core.CleanSummary
	for _, cat := range core.CategoriesInOrder() {
		items := remaining[cat.ID]
		moved := movedByCat[cat.ID]
		if len(items) == 0 && len(moved) == 0 {
			continue
		}
		res, ok := cleanCategory(ctx, cat, items, moved)
		if !ok {
			continue
		}
		cleanSummary.Results = append(cleanSummary.Results, res)
		cleanSummary.TotalFreedSpace += res.FreedSpace
		cleanSummary.TotalCleanedItems += res.CleanedItems
		cleanSummary.TotalErrors += len(res.Errors)
	}

	if cleanJSON {
		data, err := output.MarshalCleanJSON(cleanSummary)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(out, string(data))
		return nil
	}
	printCleanReport(out, cleanSummary, notBackedUp, home, cleanDryRun)
	return nil
}

// cleanCategory runs one category's scanner.Clean over items (if any) and
// credits moved (backed-up) items as cleaned/freed without touching them
// again — mirrors app.go's runClean loop body.
func cleanCategory(ctx context.Context, cat core.Category, items, moved []core.CleanableItem) (core.CleanResult, bool) {
	sc, ok := scanners.Get(cat.ID)
	if !ok {
		return core.CleanResult{}, false
	}
	res := core.CleanResult{Category: cat, Errors: []string{}}
	if len(items) > 0 {
		res = sc.Clean(ctx, items, cleanDryRun, nil)
		if res.Errors == nil {
			res.Errors = []string{}
		}
	}
	for _, m := range moved {
		res.CleanedItems++
		res.FreedSpace += m.Size
	}
	return res, true
}

func printCleanReport(w io.Writer, summary core.CleanSummary, notBackedUp []string, home string, dryRun bool) {
	title := "✓ Cleaning Complete"
	if dryRun {
		title = output.DryRunPrefix + " Would clean the following:"
	}
	_, _ = fmt.Fprintln(w, lipgloss.NewStyle().Bold(true).Render(title))
	_, _ = fmt.Fprintln(w, output.Rule(50))
	for _, res := range summary.Results {
		if len(res.Errors) > 0 {
			_, _ = fmt.Fprintf(w, "  ✗ %s: %s\n", res.Category.Name, strings.Join(res.Errors, "; "))
			continue
		}
		_, _ = fmt.Fprintf(w, "  ✓ %s: %s freed (%d items)\n", res.Category.Name, core.FormatSize(res.FreedSpace), res.CleanedItems)
	}
	_, _ = fmt.Fprintln(w, output.Rule(50))
	verb := "Freed"
	if dryRun {
		verb = "Would free"
	}
	_, _ = fmt.Fprintf(w, "%s: %s\n", verb, core.FormatSize(summary.TotalFreedSpace))
	_, _ = fmt.Fprintf(w, "Cleaned %d items\n", summary.TotalCleanedItems)
	if summary.TotalErrors > 0 {
		_, _ = fmt.Fprintf(w, "Errors: %d\n", summary.TotalErrors)
	}
	if len(notBackedUp) > 0 {
		_, _ = fmt.Fprintf(w, "Warning: %d item(s) were not backed up before deletion:\n", len(notBackedUp))
		for _, p := range notBackedUp {
			_, _ = fmt.Fprintf(w, "  - %s\n", output.ContractHome(p, home))
		}
	}
}
