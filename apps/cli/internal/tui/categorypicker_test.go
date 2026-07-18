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

// TestCategoryPickerPressingAllTwiceTogglesBackOff guards the "already all
// selected" branch of the 'a' key: pressing it a second time when every row
// is already selected must clear the selection back to none, rather than
// re-selecting (a no-op 'a' would defeat its purpose as a toggle).
func TestCategoryPickerPressingAllTwiceTogglesBackOff(t *testing.T) {
	downloadsResult := core.ScanResult{
		Category:  core.Category{ID: "downloads", Name: "Downloads", SafetyLevel: core.SafetySafe},
		Items:     []core.CleanableItem{{Path: "/h/Downloads/a", Size: 2, Name: "a"}},
		TotalSize: 2,
	}
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1), downloadsResult})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	cp := next.(CategoryPickerModel)
	if !cp.isSelected(0) || !cp.isSelected(1) {
		t.Fatal("first 'a' must select all")
	}
	next, _ = cp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	cp = next.(CategoryPickerModel)
	if cp.isSelected(0) || cp.isSelected(1) {
		t.Fatal("second 'a' (all already selected) must deselect all")
	}
}

func TestCategoryPickerInitReturnsNilCmd(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1)})
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init() = %v, want nil", cmd)
	}
}

func TestCategoryPickerIsSelectedOutOfRangeReturnsFalse(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1)})
	if m.isSelected(-1) {
		t.Fatal("isSelected(-1) must be false")
	}
	if m.isSelected(len(m.Results)) {
		t.Fatal("isSelected(len(Results)) must be false")
	}
}

func TestCategoryPickerUpDownMovesCaretWithinBounds(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1), trashResult(2), trashResult(3)})

	// "up" at the top is a clamped no-op.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	cp := next.(CategoryPickerModel)
	if cp.caret != 0 {
		t.Fatalf("caret after up-at-top = %d, want 0", cp.caret)
	}

	next, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})
	cp = next.(CategoryPickerModel)
	if cp.caret != 1 {
		t.Fatalf("caret after down = %d, want 1", cp.caret)
	}

	next, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(CategoryPickerModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	cp = next.(CategoryPickerModel)
	if cp.caret != 2 {
		t.Fatalf("caret after two more downs (clamped at last row) = %d, want 2", cp.caret)
	}

	next, _ = cp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	cp = next.(CategoryPickerModel)
	if cp.caret != 1 {
		t.Fatalf("caret after 'k' = %d, want 1", cp.caret)
	}
}

func TestCategoryPickerCtrlCSetsAbortedAndQuits(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1)})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	cp := next.(CategoryPickerModel)
	if !cp.Aborted {
		t.Fatal("ctrl+c must set Aborted")
	}
	if cmd == nil {
		t.Fatal("ctrl+c must return tea.Quit")
	}
}

// TestCategoryPickerNonKeyMsgIsANoOp guards the type-switch's `!ok` branch:
// a non-keyboard bubbletea message must leave the model untouched.
func TestCategoryPickerNonKeyMsgIsANoOp(t *testing.T) {
	m := NewCategoryPickerModel([]core.ScanResult{trashResult(1)})
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cp := next.(CategoryPickerModel)
	if cp.caret != 0 || cp.Done || cp.Aborted {
		t.Fatalf("non-key message must not mutate the model, got %+v", cp)
	}
	if cmd != nil {
		t.Fatalf("non-key message must return a nil cmd, got %v", cmd)
	}
}
