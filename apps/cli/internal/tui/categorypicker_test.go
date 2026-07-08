package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func trashResult(size int64) core.ScanResult {
	return core.ScanResult{
		Category:  core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
		Items:     []core.CleanableItem{{Path: "/h/.Trash/a", Size: size, Name: "a"}},
		TotalSize: size,
	}
}

func TestCategoryPickerSpaceTogglesSelection(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1024)})

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	cp := next.(CategoryPickerModel)
	if !cp.isSelected(0) {
		t.Fatal("space must select the caret row")
	}
	view := cp.View()
	if !strings.Contains(view, "Trash") || !strings.Contains(view, "1.0 KB") {
		t.Fatalf("view must show name and size, got:\n%s", view)
	}
}

func TestCategoryPickerEnterMarksDoneAndCapturesChosen(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1024)})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	next, cmd := next.(CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	cp := next.(CategoryPickerModel)

	if !cp.Done {
		t.Fatal("enter must set Done")
	}
	if len(cp.Chosen()) != 1 || cp.Chosen()[0] != "trash" {
		t.Fatalf("Chosen() = %v, want [trash]", cp.Chosen())
	}
	if cmd == nil {
		t.Fatal("enter must return tea.Quit")
	}
}

func TestCategoryPickerAllAndInvert(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1), trashResult(2)})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	cp := next.(CategoryPickerModel)
	if !cp.isSelected(0) || !cp.isSelected(1) {
		t.Fatal("'a' must select all")
	}
	next, _ = cp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	cp = next.(CategoryPickerModel)
	if cp.isSelected(0) || cp.isSelected(1) {
		t.Fatal("'i' after all-selected must deselect all")
	}
}
