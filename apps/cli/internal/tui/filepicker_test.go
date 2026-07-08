package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func largeFilesResult() core.ScanResult {
	return core.ScanResult{
		Category: core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetyRisky, SupportsFileSelection: true},
		Items: []core.CleanableItem{
			{Path: "/h/Downloads/big.bin", Size: 5000, Name: "big.bin"},
			{Path: "/h/Downloads/small.bin", Size: 100, Name: "small.bin"},
		},
		TotalSize: 5100,
	}
}

func newTestFilePicker() FilePickerModel {
	return NewFilePickerModel([]core.ScanResult{largeFilesResult()},
		map[core.CategoryID]bool{"large-files": true}, "/h", false)
}

func TestFilePickerEnterFilesOnRightArrow(t *testing.T) {
	m := newTestFilePicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	fp := next.(FilePickerModel)
	if fp.store.ActivePane() != "files" {
		t.Fatalf("right arrow on a file-selection category must enter files pane, got %s", fp.store.ActivePane())
	}
	view := fp.View()
	if !strings.Contains(view, "big.bin") {
		t.Fatalf("view must render file rows, got:\n%s", view)
	}
}

func TestFilePickerSpaceSelectsFileAndCouplesCategory(t *testing.T) {
	m := newTestFilePicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next, _ = next.(FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	fp := next.(FilePickerModel)

	cats, files := fp.Result()
	if !cats["large-files"] {
		t.Fatal("selecting a file must select its parent category")
	}
	if len(files["large-files"]) != 1 {
		t.Fatalf("expected exactly 1 selected file, got %d", len(files["large-files"]))
	}
}

func TestFilePickerBackspaceReturnsToCategories(t *testing.T) {
	m := newTestFilePicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next, _ = next.(FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	fp := next.(FilePickerModel)
	if fp.store.ActivePane() != "categories" {
		t.Fatalf("backspace must return to categories pane, got %s", fp.store.ActivePane())
	}
}

func TestFilePickerCopyShowsToastForTwoSeconds(t *testing.T) {
	m := newTestFilePicker()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next, _ = next.(FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	fp := next.(FilePickerModel)
	if fp.copyStatus == "" {
		t.Fatal("'c' must set a copy-status toast message")
	}
	if !strings.Contains(fp.View(), "Copied") {
		t.Fatalf("view must show the toast, got:\n%s", fp.View())
	}
	_ = cmd // enter's tea.Quit isn't relevant here; keeps var used
}
