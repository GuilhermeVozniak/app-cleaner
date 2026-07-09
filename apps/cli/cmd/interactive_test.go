package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/tui"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunInteractiveNothingToCleanReturnsNil(t *testing.T) {
	deps := interactiveDeps{
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{Results: []core.ScanResult{}, TotalSize: 0}
		},
		home: t.TempDir(),
	}
	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Fatalf("expected nil summary when nothing to clean, got %+v", summary)
	}
}

func TestRunInteractiveDeclineConfirmReturnsNil(t *testing.T) {
	deps := interactiveDeps{
		home: t.TempDir(),
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{{
					Category:  core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
					Items:     []core.CleanableItem{{Path: "/x/a", Size: 100, Name: "a"}},
					TotalSize: 100,
				}},
				TotalSize: 100,
			}
		},
		confirm:      func(string, bool) bool { return false },
		printResults: func(core.CleanSummary, bool) {},
	}

	// Stub the picker to select everything without a real TTY. Note: this
	// diverges from this task's brief, which expected this test to "never
	// reach runTeaProgram" — but finishInteractive always drives the
	// category picker before consulting confirm, so exercising the decline
	// path genuinely requires getting through the picker first. See the
	// task report's concerns section.
	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		case tui.FilePickerModel:
			return m, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Fatal("declining the confirm must return nil, not clean")
	}
}

// TestRunInteractiveCategoryPickerCtrlCAborts guards the ctrl+c-is-not-confirm
// fix: pressing Ctrl-C in the category picker sets Aborted, so the interactive
// flow cancels with a nil summary and never reaches confirm or any cleaning.
func TestRunInteractiveCategoryPickerCtrlCAborts(t *testing.T) {
	deps := interactiveDeps{
		home: t.TempDir(),
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{{
					Category:  core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
					Items:     []core.CleanableItem{{Path: "/x/a", Size: 100, Name: "a"}},
					TotalSize: 100,
				}},
				TotalSize: 100,
			}
		},
		confirm:      func(string, bool) bool { t.Fatal("confirm must not be reached after ctrl+c"); return false },
		printResults: func(core.CleanSummary, bool) { t.Fatal("printResults must not be reached after ctrl+c") },
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			return next, nil
		case tui.FilePickerModel:
			return m, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Fatalf("ctrl+c in the category picker must abort with a nil summary, got %+v", summary)
	}
}

func TestRunInteractiveConfirmedCleansAndBacksUp(t *testing.T) {
	home := t.TempDir()
	trashDir := home + "/.Trash"
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := trashDir + "/a.txt"
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.BackupByDefault = true
	deps := interactiveDeps{
		home:      home,
		cfg:       cfg,
		backupMgr: backup.NewManager(home),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{{
					Category:  core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
					Items:     []core.CleanableItem{{Path: filePath, Size: 1, Name: "a.txt"}},
					TotalSize: 1,
				}},
				TotalSize: 1,
			}
		},
		confirm: func(string, bool) bool { return true },
	}
	var printed *core.CleanSummary
	deps.printResults = func(s core.CleanSummary, _ bool) { printed = &s }

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		case tui.FilePickerModel:
			return m, nil // no file-selection categories in this fixture (trash)
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil || summary.TotalFreedSpace != 1 || summary.TotalCleanedItems != 1 {
		t.Fatalf("expected 1 item / 1 byte freed, got %+v", summary)
	}
	if printed == nil {
		t.Fatal("printResults must be called")
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("backed-up file must have been moved out of its original path")
	}
}

// TestRunInteractiveCleanDispatchesThroughEachCategoryScanner is a regression
// guard for the Critical fix in runInteractiveClean: cleaning must dispatch
// through scanners.Get(category).Clean, never bypass straight to a raw
// fsx.RemoveItems call over the raw item paths. It mixes a category that IS
// registered in packages/engine/scanners ("trash") with one that is NOT
// ("not-a-real-category" — no scanner file registers that ID). If cleaning
// ever regressed to deleting every selected item directly via fsx regardless
// of scanner registration, the bogus category's real file would be deleted
// too; with the fix, scanners.Get's `ok` is false for it and the category is
// skipped entirely, leaving its file untouched while the registered
// category's file is genuinely cleaned through its own Scanner.Clean.
func TestRunInteractiveCleanDispatchesThroughEachCategoryScanner(t *testing.T) {
	home := t.TempDir()
	trashDir := home + "/.Trash"
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trashFile := trashDir + "/real.txt"
	if err := os.WriteFile(trashFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bogusFile := home + "/bogus.txt"
	if err := os.WriteFile(bogusFile, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := interactiveDeps{
		home: home,
		cfg:  config.Default(), // BackupByDefault false: exercises fsx.RemoveItems directly
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{
					{
						Category:  core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
						Items:     []core.CleanableItem{{Path: trashFile, Size: 1, Name: "real.txt"}},
						TotalSize: 1,
					},
					{
						Category:  core.Category{ID: "not-a-real-category", Name: "Bogus", SafetyLevel: core.SafetySafe},
						Items:     []core.CleanableItem{{Path: bogusFile, Size: 1, Name: "bogus.txt"}},
						TotalSize: 1,
					},
				},
				TotalSize: 2,
			}
		},
		confirm:      func(string, bool) bool { return true },
		printResults: func(core.CleanSummary, bool) {},
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		case tui.FilePickerModel:
			return m, nil // neither fixture category supports file selection
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil || summary.TotalCleanedItems != 1 || summary.TotalFreedSpace != 1 {
		t.Fatalf("expected only the registered trash item cleaned, got %+v", summary)
	}
	if _, err := os.Stat(trashFile); !os.IsNotExist(err) {
		t.Fatal("registered category's file should have been cleaned via its own scanner")
	}
	if _, err := os.Stat(bogusFile); err != nil {
		t.Fatalf("unregistered category's file must be left untouched (no scanner to authorize cleaning it), got stat err: %v", err)
	}
}
