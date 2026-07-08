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
