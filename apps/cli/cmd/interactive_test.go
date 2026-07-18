package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/tui"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fda"

	tea "github.com/charmbracelet/bubbletea"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. Several functions under test here (runInteractive,
// printCleanResults, runConfirm) print straight to os.Stdout rather than
// taking an io.Writer, so this is the only way to assert on their output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig
	return <-outCh
}

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

func TestNewInteractiveDepsWiresHomeConfigAndDefaultSeams(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default()
	cfg.Concurrency = 3

	deps := newInteractiveDeps(cfg, home)

	if deps.home != home {
		t.Fatalf("deps.home = %q, want %q", deps.home, home)
	}
	if deps.cfg.Concurrency != 3 {
		t.Fatalf("deps.cfg.Concurrency = %d, want 3", deps.cfg.Concurrency)
	}
	if deps.backupMgr == nil {
		t.Fatal("deps.backupMgr must be non-nil")
	}
	if deps.scan == nil {
		t.Fatal("deps.scan must be non-nil")
	}
	if reflect.ValueOf(deps.confirm).Pointer() != reflect.ValueOf(runConfirm).Pointer() {
		t.Fatal("deps.confirm must default to runConfirm")
	}
	if reflect.ValueOf(deps.printResults).Pointer() != reflect.ValueOf(printCleanResults).Pointer() {
		t.Fatal("deps.printResults must default to printCleanResults")
	}
}

func TestHideRiskyCategoriesFiltersOutRiskyByDefault(t *testing.T) {
	safe := core.ScanResult{Category: core.Category{ID: "trash", SafetyLevel: core.SafetySafe}, TotalSize: 10}
	risky := core.ScanResult{Category: core.Category{ID: "large-files", SafetyLevel: core.SafetyRisky}, TotalSize: 20}

	kept, hidden := hideRiskyCategories([]core.ScanResult{safe, risky}, false)
	if len(kept) != 1 || kept[0].Category.ID != "trash" {
		t.Fatalf("kept = %+v, want only trash", kept)
	}
	if len(hidden) != 1 || hidden[0].Category.ID != "large-files" {
		t.Fatalf("hidden = %+v, want only large-files", hidden)
	}
}

func TestHideRiskyCategoriesIncludesRiskyWhenRequested(t *testing.T) {
	safe := core.ScanResult{Category: core.Category{ID: "trash", SafetyLevel: core.SafetySafe}, TotalSize: 10}
	risky := core.ScanResult{Category: core.Category{ID: "large-files", SafetyLevel: core.SafetyRisky}, TotalSize: 20}

	kept, hidden := hideRiskyCategories([]core.ScanResult{safe, risky}, true)
	if len(kept) != 2 {
		t.Fatalf("kept = %+v, want both categories when includeRisky is set", kept)
	}
	if hidden != nil {
		t.Fatalf("hidden = %+v, want nil when includeRisky is set", hidden)
	}
}

// TestRunInteractiveHidesRiskyCategoriesAndReportsCount guards the
// risky-category-hiding notice: with a risky and a non-risky category in the
// scan results and --risky not set, the risky one must be excluded from the
// picker and its count/size reported.
func TestRunInteractiveHidesRiskyCategoriesAndReportsCount(t *testing.T) {
	deps := interactiveDeps{
		home: t.TempDir(),
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{
					{
						Category: core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
						Items:    []core.CleanableItem{{Path: "/x/a", Size: 10, Name: "a"}}, TotalSize: 10,
					},
					{
						Category: core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetyRisky},
						Items:    []core.CleanableItem{{Path: "/x/b", Size: 20, Name: "b"}}, TotalSize: 20,
					},
				},
				TotalSize: 30,
			}
		},
		confirm:      func(string, bool) bool { return false },
		printResults: func(core.CleanSummary, bool) {},
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			// Only "trash" is visible (risky was hidden), so space+enter
			// selects it and immediately proceeds to the decline-confirm exit.
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	var summary *core.CleanSummary
	var runErr error
	out := captureStdout(t, func() {
		summary, runErr = runInteractive(context.Background(), InteractiveOptions{}, deps)
	})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if summary != nil {
		t.Fatalf("declining the confirm must return nil, got %+v", summary)
	}
	if !strings.Contains(out, "Hiding 1 risky categories") {
		t.Fatalf("output = %q, want the hidden-risky-categories notice", out)
	}
	if strings.Contains(out, "Large Files") {
		t.Fatalf("output = %q, the risky category must never reach the picker view", out)
	}
}

// TestRunInteractiveWarnsWhenFullDiskAccessDenied guards the FDA-denied hint:
// when the home probe returns a definite "denied" (EACCES/EPERM), runInteractive
// must print the Full Disk Access hint before scanning.
func TestRunInteractiveWarnsWhenFullDiskAccessDenied(t *testing.T) {
	home := t.TempDir()
	safariDir := filepath.Join(home, "Library", "Safari")
	if err := os.MkdirAll(safariDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(safariDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(safariDir, 0o755) })

	deps := interactiveDeps{
		home: home,
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{Results: []core.ScanResult{}, TotalSize: 0} // ends right after the FDA check
		},
	}

	out := captureStdout(t, func() {
		summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary != nil {
			t.Fatalf("expected nil summary (nothing to clean), got %+v", summary)
		}
	})
	if !strings.Contains(out, "Full Disk Access not detected") {
		t.Fatalf("output = %q, want the FDA hint when the probe returns denied", out)
	}
	if !strings.Contains(out, fda.SettingsURL) {
		t.Fatalf("output = %q, want the Settings deep link", out)
	}
}

// TestRunInteractiveCategoryPickerNoSelectionPrintsNothingToClean guards the
// "confirm without selecting anything" branch: pressing enter in the category
// picker with nothing checked must stop before confirm/clean, not treat an
// empty selection as "select everything".
func TestRunInteractiveCategoryPickerNoSelectionPrintsNothingToClean(t *testing.T) {
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
		confirm:      func(string, bool) bool { t.Fatal("confirm must not be reached with no selection"); return false },
		printResults: func(core.CleanSummary, bool) { t.Fatal("printResults must not be reached with no selection") },
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		if m, ok := model.(tui.CategoryPickerModel); ok {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm with nothing checked
			return next, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	var summary *core.CleanSummary
	var runErr error
	out := captureStdout(t, func() {
		summary, runErr = runInteractive(context.Background(), InteractiveOptions{}, deps)
	})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if summary != nil {
		t.Fatalf("expected nil summary, got %+v", summary)
	}
	if !strings.Contains(out, "No items selected. Nothing to clean.") {
		t.Fatalf("output = %q, want the nothing-to-clean message", out)
	}
}

// TestRunInteractiveFilePickerFlowCleansSelectedFile drives the full
// category-picker -> file-picker -> confirm -> clean path for a category that
// supports per-file selection, which none of the other runInteractive tests
// exercise (they all use file-selection-free fixtures).
func TestRunInteractiveFilePickerFlowCleansSelectedFile(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := interactiveDeps{
		home: home,
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{{
					Category:  core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetySafe, SupportsFileSelection: true},
					Items:     []core.CleanableItem{{Path: filePath, Size: 1, Name: "big.bin"}},
					TotalSize: 1,
				}},
				TotalSize: 1,
			}
		},
		confirm:      func(string, bool) bool { return true },
		printResults: func(core.CleanSummary, bool) {},
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		case tui.FilePickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter files pane
			next, _ = next.(tui.FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			next, _ = next.(tui.FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil || summary.TotalCleanedItems != 1 || summary.TotalFreedSpace != 1 {
		t.Fatalf("expected 1 item / 1 byte freed, got %+v", summary)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("the file selected in the file picker must have been cleaned")
	}
}

// TestRunInteractiveFilePickerAbortedCancelsClean guards ctrl+c in the file
// picker: it must cancel with a nil summary, same as ctrl+c in the category
// picker, and never reach confirm.
func TestRunInteractiveFilePickerAbortedCancelsClean(t *testing.T) {
	home := t.TempDir()
	deps := interactiveDeps{
		home: home,
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{{
					Category:  core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetySafe, SupportsFileSelection: true},
					Items:     []core.CleanableItem{{Path: "/x/big.bin", Size: 1, Name: "big.bin"}},
					TotalSize: 1,
				}},
				TotalSize: 1,
			}
		},
		confirm: func(string, bool) bool {
			t.Fatal("confirm must not be reached after a file-picker abort")
			return false
		},
		printResults: func(core.CleanSummary, bool) { t.Fatal("printResults must not be reached after a file-picker abort") },
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		case tui.FilePickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			return next, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	summary, err := runInteractive(context.Background(), InteractiveOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Fatalf("ctrl+c in the file picker must abort with a nil summary, got %+v", summary)
	}
}

// TestRunInteractiveFilePickerNoFilesChosenSkipsCategory guards the
// "category flagged for file selection but zero files chosen" branch: it
// must fall through to the same nothing-to-clean message as an empty
// category-picker selection, not a partial/zero-item clean.
func TestRunInteractiveFilePickerNoFilesChosenSkipsCategory(t *testing.T) {
	deps := interactiveDeps{
		home: t.TempDir(),
		cfg:  config.Default(),
		scan: func(ctx context.Context, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {
			return core.ScanSummary{
				Results: []core.ScanResult{{
					Category:  core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetySafe, SupportsFileSelection: true},
					Items:     []core.CleanableItem{{Path: "/x/big.bin", Size: 1, Name: "big.bin"}},
					TotalSize: 1,
				}},
				TotalSize: 1,
			}
		},
		confirm:      func(string, bool) bool { t.Fatal("confirm must not be reached when nothing is selected"); return false },
		printResults: func(core.CleanSummary, bool) { t.Fatal("printResults must not be reached when nothing is selected") },
	}

	old := runTeaProgram
	runTeaProgram = func(model tea.Model) (tea.Model, error) {
		switch m := model.(type) {
		case tui.CategoryPickerModel:
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			next, _ = next.(tui.CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		case tui.FilePickerModel:
			// Confirm immediately without entering the files pane or
			// selecting anything: Result() reports zero files chosen.
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return next, nil
		}
		return model, nil
	}
	t.Cleanup(func() { runTeaProgram = old })

	var summary *core.CleanSummary
	var runErr error
	out := captureStdout(t, func() {
		summary, runErr = runInteractive(context.Background(), InteractiveOptions{}, deps)
	})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if summary != nil {
		t.Fatalf("expected nil summary, got %+v", summary)
	}
	if !strings.Contains(out, "No items selected. Nothing to clean.") {
		t.Fatalf("output = %q, want the nothing-to-clean message", out)
	}
}

func TestRunConfirmReadsRealStdinAnswers(t *testing.T) {
	cases := []struct {
		name       string
		defaultYes bool
		input      string
		want       bool
	}{
		{"bare enter with default yes", true, "\n", true},
		{"bare enter with default no", false, "\n", false},
		{"y answers yes", false, "y\n", true},
		{"yes answers yes", false, "yes\n", true},
		{"n answers no even with default yes", true, "n\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.WriteString(c.input); err != nil {
				t.Fatal(err)
			}
			_ = w.Close()

			origStdin := os.Stdin
			os.Stdin = r
			t.Cleanup(func() { os.Stdin = origStdin })

			got := runConfirm("Proceed?", c.defaultYes)
			if got != c.want {
				t.Fatalf("runConfirm(input=%q, defaultYes=%v) = %v, want %v", c.input, c.defaultYes, got, c.want)
			}
		})
	}
}

func TestPrintCleanResultsShowsFreedSpaceErrorsAndFDAHint(t *testing.T) {
	summary := core.CleanSummary{
		Results: []core.CleanResult{
			{Category: core.Category{Name: "Trash"}, CleanedItems: 2, FreedSpace: 2048, Errors: []string{"EPERM: permission denied"}},
		},
		TotalFreedSpace:   2048,
		TotalCleanedItems: 2,
		TotalErrors:       1,
	}

	out := captureStdout(t, func() { printCleanResults(summary, true) })

	if !strings.Contains(out, "Cleaning Complete!") {
		t.Fatalf("output = %q, want the completion header", out)
	}
	if !strings.Contains(out, "Trash") || !strings.Contains(out, core.FormatSize(2048)) {
		t.Fatalf("output = %q, want a Trash line with the freed size", out)
	}
	if !strings.Contains(out, "EPERM: permission denied") {
		t.Fatalf("output = %q, want the per-category error line", out)
	}
	if !strings.Contains(out, "Errors: 1") {
		t.Fatalf("output = %q, want the errors total", out)
	}
	if !strings.Contains(out, fda.SettingsURL) {
		t.Fatalf("output = %q, want the FDA hint when fdaHint is true", out)
	}
}

func TestPrintCleanResultsNoErrorsOmitsErrorsAndFDALines(t *testing.T) {
	summary := core.CleanSummary{TotalFreedSpace: 100, TotalCleanedItems: 1}

	out := captureStdout(t, func() { printCleanResults(summary, false) })

	if strings.Contains(out, "Errors:") {
		t.Fatalf("output = %q, want no Errors line when TotalErrors is 0", out)
	}
	if strings.Contains(out, "Full Disk Access") {
		t.Fatalf("output = %q, want no FDA hint when fdaHint is false", out)
	}
}
