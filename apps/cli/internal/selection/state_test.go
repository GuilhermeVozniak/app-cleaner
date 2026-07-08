// Package selection tests. The first six Test* functions below are 1:1
// ports of mac-cleaner-cli's src/pickers/file-picker.test.ts (all 15 `it()`
// cases; each t.Run name is that test's exact TS description, and each
// carries a comment naming its ts describe block). The remaining Test*
// functions are State-machine-level tests covering caret/paging/expand
// mechanics that file-picker.test.ts does not itself exercise (it only
// tests the pure per-key helpers), required because the CLI TUI (Task 9)
// builds directly on State.
package selection

import (
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/grouping"
)

// ---- test helpers -----------------------------------------------------------

func itemsAt(paths ...string) []core.CleanableItem {
	items := make([]core.CleanableItem, len(paths))
	for i, p := range paths {
		items[i] = core.CleanableItem{Path: p, Size: 100, Name: p}
	}
	return items
}

func pathSet(paths ...string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return set
}

func mustLen(t *testing.T, label string, m map[string]bool, want int) {
	t.Helper()
	if len(m) != want {
		t.Fatalf("%s: got size %d, want %d (%v)", label, len(m), want, m)
	}
}

// ============================================================================
// Ported from file-picker.test.ts (15 cases, verbatim behavior)
// ============================================================================

// TS describe: "'a' key in files pane (select all)"
func TestSelectAllFilesInCategory(t *testing.T) {
	// TS it: "selects all files in category and marks category selected"
	t.Run("selects all files in category and marks category selected", func(t *testing.T) {
		paths := make([]string, 20)
		for i := range paths {
			paths[i] = "/test/dir/file" + string(rune('a'+i)) + ".txt"
		}
		selectedCategories := map[core.CategoryID]bool{}
		selectedFiles := map[core.CategoryID]map[string]bool{}

		newCategories, newFiles := SelectAllFilesInCategory(selectedCategories, selectedFiles, "large-files", paths)

		selected := newFiles["large-files"]
		mustLen(t, "selected", selected, 20)
		if !newCategories["large-files"] {
			t.Fatal("expected large-files category to be selected")
		}
		for _, p := range paths {
			if !selected[p] {
				t.Fatalf("expected %s to be selected", p)
			}
		}
	})

	// TS it: "deselects all files and unmarks category when pressed again"
	t.Run("deselects all files and unmarks category when pressed again", func(t *testing.T) {
		paths := make([]string, 15)
		for i := range paths {
			paths[i] = "/test/file" + string(rune('a'+i)) + ".txt"
		}
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{"large-files": pathSet(paths...)}

		newCategories, newFiles := SelectAllFilesInCategory(selectedCategories, selectedFiles, "large-files", paths)

		mustLen(t, "selected", newFiles["large-files"], 0)
		if newCategories["large-files"] {
			t.Fatal("expected large-files category to be deselected")
		}
	})
}

// TS describe: "'i' key in files pane (invert)"
func TestInvertFilesInCategory(t *testing.T) {
	// TS it: "inverts selection across all files in category"
	t.Run("inverts selection across all files in category", func(t *testing.T) {
		paths := make([]string, 12)
		for i := range paths {
			paths[i] = "/test/file" + string(rune('a'+i)) + ".txt"
		}
		initiallySelected := paths[:5]
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{"large-files": pathSet(initiallySelected...)}

		newCategories, newFiles := InvertFilesInCategory(selectedCategories, selectedFiles, "large-files", paths)

		selected := newFiles["large-files"]
		mustLen(t, "selected", selected, 7)
		if !newCategories["large-files"] {
			t.Fatal("expected large-files category to remain selected")
		}
		for _, p := range paths[5:] {
			if !selected[p] {
				t.Fatalf("expected %s to be selected after invert", p)
			}
		}
		for _, p := range paths[:5] {
			if selected[p] {
				t.Fatalf("expected %s to be deselected after invert", p)
			}
		}
	})

	// TS it: "auto-selects category when inversion results in selected files"
	t.Run("auto-selects category when inversion results in selected files", func(t *testing.T) {
		paths := []string{"/test/file1.txt", "/test/file2.txt"}
		selectedCategories := map[core.CategoryID]bool{}
		selectedFiles := map[core.CategoryID]map[string]bool{}

		newCategories, newFiles := InvertFilesInCategory(selectedCategories, selectedFiles, "large-files", paths)

		if !newCategories["large-files"] {
			t.Fatal("expected large-files category to be selected")
		}
		mustLen(t, "selected", newFiles["large-files"], 2)
	})

	// TS it: "deselects category when inversion leaves no files selected"
	t.Run("deselects category when inversion leaves no files selected", func(t *testing.T) {
		paths := []string{"/test/file1.txt", "/test/file2.txt"}
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{"large-files": pathSet(paths...)}

		newCategories, newFiles := InvertFilesInCategory(selectedCategories, selectedFiles, "large-files", paths)

		if newCategories["large-files"] {
			t.Fatal("expected large-files category to be deselected")
		}
		mustLen(t, "selected", newFiles["large-files"], 0)
	})
}

// TS describe: "single-file toggle in files pane"
func TestToggleSingleFile(t *testing.T) {
	// TS it: "auto-selects parent category when selecting a file"
	t.Run("auto-selects parent category when selecting a file", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{}
		selectedFiles := map[core.CategoryID]map[string]bool{}

		newCategories, newFiles := ToggleSingleFile(selectedCategories, selectedFiles, "large-files", "/test/file1.txt")

		if !newCategories["large-files"] {
			t.Fatal("expected large-files category to be selected")
		}
		if !newFiles["large-files"]["/test/file1.txt"] {
			t.Fatal("expected /test/file1.txt to be selected")
		}
	})

	// TS it: "keeps category selected when other files remain selected"
	t.Run("keeps category selected when other files remain selected", func(t *testing.T) {
		file1, file2 := "/test/file1.txt", "/test/file2.txt"
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{"large-files": pathSet(file1, file2)}

		newCategories, newFiles := ToggleSingleFile(selectedCategories, selectedFiles, "large-files", file1)

		if !newCategories["large-files"] {
			t.Fatal("expected large-files category to remain selected")
		}
		mustLen(t, "selected", newFiles["large-files"], 1)
		if !newFiles["large-files"][file2] {
			t.Fatal("expected file2 to remain selected")
		}
	})

	// TS it: "deselects parent category when last file is deselected"
	t.Run("deselects parent category when last file is deselected", func(t *testing.T) {
		file1 := "/test/file1.txt"
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{"large-files": pathSet(file1)}

		newCategories, newFiles := ToggleSingleFile(selectedCategories, selectedFiles, "large-files", file1)

		if newCategories["large-files"] {
			t.Fatal("expected large-files category to be deselected")
		}
		mustLen(t, "selected", newFiles["large-files"], 0)
	})
}

// TS describe: "category deselection clears file selections"
func TestDeselectCategory(t *testing.T) {
	// TS it: "clears file selections when deselecting a category"
	t.Run("clears file selections when deselecting a category", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{
			"large-files": pathSet("/test/file1.txt", "/test/file2.txt"),
		}

		newCategories, newFiles := DeselectCategory(selectedCategories, selectedFiles, "large-files")

		if newCategories["large-files"] {
			t.Fatal("expected large-files category to be deselected")
		}
		mustLen(t, "selected", newFiles["large-files"], 0)
	})

	// TS it: "preserves file selections for other categories when deselecting one"
	t.Run("preserves file selections for other categories when deselecting one", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{"large-files": true, "downloads": true}
		selectedFiles := map[core.CategoryID]map[string]bool{
			"large-files": pathSet("/test1/file1.txt"),
			"downloads":   pathSet("/test2/file2.txt"),
		}

		newCategories, newFiles := DeselectCategory(selectedCategories, selectedFiles, "large-files")

		if newCategories["large-files"] {
			t.Fatal("expected large-files category to be deselected")
		}
		if !newCategories["downloads"] {
			t.Fatal("expected downloads category to remain selected")
		}
		mustLen(t, "large-files selected", newFiles["large-files"], 0)
		mustLen(t, "downloads selected", newFiles["downloads"], 1)
		if !newFiles["downloads"]["/test2/file2.txt"] {
			t.Fatal("expected downloads file2 to remain selected")
		}
	})
}

// TS describe: "'d' key for directory selection"
func TestToggleDirectoryFiles(t *testing.T) {
	// TS it: "selects all files in the current directory only"
	t.Run("selects all files in the current directory only", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{}
		selectedFiles := map[core.CategoryID]map[string]bool{}
		dir1Files := []string{"/test/dir1/file1.txt", "/test/dir1/file2.txt"}

		newFiles := ToggleDirectoryFiles(selectedFiles, "large-files", dir1Files)

		selected := newFiles["large-files"]
		mustLen(t, "selected", selected, 2)
		if !selected["/test/dir1/file1.txt"] || !selected["/test/dir1/file2.txt"] {
			t.Fatal("expected both dir1 files to be selected")
		}
		if selected["/test/dir2/file3.txt"] {
			t.Fatal("expected dir2 file to remain unselected")
		}
		// Directory toggling never touches selectedCategories (only newFiles
		// is returned by ToggleDirectoryFiles) — the untouched input proves it.
		if len(selectedCategories) != 0 {
			t.Fatalf("expected selectedCategories to stay empty, got %v", selectedCategories)
		}
	})

	// TS it: "deselects all files in directory when toggled twice, keeping category membership"
	t.Run("deselects all files in directory when toggled twice, keeping category membership", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{
			"large-files": pathSet("/test/dir1/file1.txt", "/test/dir1/file2.txt"),
		}
		dir1Files := []string{"/test/dir1/file1.txt", "/test/dir1/file2.txt"}

		newFiles := ToggleDirectoryFiles(selectedFiles, "large-files", dir1Files)

		mustLen(t, "selected", newFiles["large-files"], 0)
		if !selectedCategories["large-files"] {
			t.Fatal("expected large-files category membership to be unchanged")
		}
	})

	// TS it: "only affects current directory, not others"
	t.Run("only affects current directory, not others", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{"large-files": true}
		selectedFiles := map[core.CategoryID]map[string]bool{
			"large-files": pathSet("/test/dir2/file3.txt", "/test/dir2/file4.txt"),
		}
		dir1Files := []string{"/test/dir1/file1.txt", "/test/dir1/file2.txt"}

		newFiles := ToggleDirectoryFiles(selectedFiles, "large-files", dir1Files)

		selected := newFiles["large-files"]
		mustLen(t, "selected", selected, 4)
		for _, p := range []string{
			"/test/dir1/file1.txt", "/test/dir1/file2.txt",
			"/test/dir2/file3.txt", "/test/dir2/file4.txt",
		} {
			if !selected[p] {
				t.Fatalf("expected %s to be selected", p)
			}
		}
		if !selectedCategories["large-files"] {
			t.Fatal("expected large-files category membership to be unchanged")
		}
	})
}

// TS describe: "cross-category isolation"
func TestCrossCategoryIsolation(t *testing.T) {
	// TS it: "does not affect other categories when selecting all files in one"
	t.Run("does not affect other categories when selecting all files in one", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{"downloads": true}
		selectedFiles := map[core.CategoryID]map[string]bool{"downloads": pathSet("/test2/file2.txt")}
		filesCategory1 := []string{"/test1/file1.txt"}

		newCategories, newFiles := SelectAllFilesInCategory(selectedCategories, selectedFiles, "large-files", filesCategory1)

		mustLen(t, "downloads selected", newFiles["downloads"], 1)
		if !newFiles["downloads"]["/test2/file2.txt"] {
			t.Fatal("expected downloads file2 to remain selected")
		}
		if !newCategories["downloads"] {
			t.Fatal("expected downloads category to remain selected")
		}
		mustLen(t, "large-files selected", newFiles["large-files"], 1)
		if !newCategories["large-files"] {
			t.Fatal("expected large-files category to be selected")
		}
	})

	// TS it: "does not affect other categories when deselecting one category"
	t.Run("does not affect other categories when deselecting one category", func(t *testing.T) {
		selectedCategories := map[core.CategoryID]bool{"large-files": true, "downloads": true, "trash": true}
		selectedFiles := map[core.CategoryID]map[string]bool{
			"large-files": pathSet("/test1/file1.txt"),
			"downloads":   pathSet("/test2/file2.txt"),
			"trash":       pathSet("/test3/file3.txt"),
		}

		newCategories, newFiles := DeselectCategory(selectedCategories, selectedFiles, "downloads")

		if newCategories["downloads"] {
			t.Fatal("expected downloads category to be deselected")
		}
		mustLen(t, "downloads selected", newFiles["downloads"], 0)
		if !newCategories["large-files"] || !newCategories["trash"] {
			t.Fatal("expected large-files and trash categories to remain selected")
		}
		mustLen(t, "large-files selected", newFiles["large-files"], 1)
		mustLen(t, "trash selected", newFiles["trash"], 1)
	})
}

// ============================================================================
// State-machine tests (caret, paging, expand, persistence) — not covered by
// file-picker.test.ts, required for the TUI (Task 9) to build on State.
// ============================================================================

func newTestState(withFileSelection bool) (*State, core.ScanResult) {
	result := core.ScanResult{
		Category: core.Category{ID: "large-files", Name: "Large Files", SupportsFileSelection: true},
		Items:    itemsAt("/test/file1.txt", "/test/file2.txt"),
	}
	other := core.ScanResult{
		Category: core.Category{ID: "downloads", Name: "Downloads"},
		Items:    itemsAt("/dl/a.zip"),
	}
	cwfs := map[core.CategoryID]bool{}
	if withFileSelection {
		cwfs["large-files"] = true
	}
	return NewState([]core.ScanResult{result, other}, cwfs), result
}

func TestToggleCategory(t *testing.T) {
	s, result := newTestState(true)

	s.ToggleCategory() // caret starts at 0 = "large-files"
	if !s.IsCategorySelected("large-files") {
		t.Fatal("expected large-files to be selected")
	}
	for _, it := range result.Items {
		if !s.IsFileSelected("large-files", it.Path) {
			t.Fatalf("expected %s to be auto-selected", it.Path)
		}
	}
	if !s.PickerState("large-files").Visible {
		t.Fatal("expected large-files picker to become visible")
	}

	s.ToggleCategory() // deselect
	if s.IsCategorySelected("large-files") {
		t.Fatal("expected large-files to be deselected")
	}
	mustLen(t, "large-files selected", s.SelectedFiles("large-files"), 0)
	if s.PickerState("large-files").Visible {
		t.Fatal("expected large-files picker to become hidden")
	}
}

func TestToggleCategoryWithoutFileSelectionDoesNotTouchFiles(t *testing.T) {
	s, _ := newTestState(true)
	s.MoveCategoryCaret(1) // "downloads" does not support file selection
	s.ToggleCategory()
	if !s.IsCategorySelected("downloads") {
		t.Fatal("expected downloads to be selected")
	}
	mustLen(t, "downloads selected files", s.SelectedFiles("downloads"), 0)
}

func TestSelectAllCategoriesTogglesEverythingIncludingFiles(t *testing.T) {
	s, result := newTestState(true)

	s.SelectAllCategories()
	if !s.IsCategorySelected("large-files") || !s.IsCategorySelected("downloads") {
		t.Fatal("expected both categories to be selected")
	}
	for _, it := range result.Items {
		if !s.IsFileSelected("large-files", it.Path) {
			t.Fatalf("expected %s to be auto-selected by select-all", it.Path)
		}
	}

	s.SelectAllCategories() // press again: full deselect
	if s.IsCategorySelected("large-files") || s.IsCategorySelected("downloads") {
		t.Fatal("expected both categories to be deselected")
	}
	mustLen(t, "large-files selected", s.SelectedFiles("large-files"), 0)
}

func TestInvertCategoriesFlipsEachIndependently(t *testing.T) {
	s, _ := newTestState(true)
	s.ToggleCategory() // select large-files only

	s.InvertCategories()
	if s.IsCategorySelected("large-files") {
		t.Fatal("expected large-files to be deselected by invert")
	}
	if !s.IsCategorySelected("downloads") {
		t.Fatal("expected downloads to be selected by invert")
	}
}

func TestEnterFilesPaneRejectsWhenUnsupportedOrEmpty(t *testing.T) {
	s, _ := newTestState(false) // no category supports file selection
	rows := []grouping.DisplayRow{{Type: "file", Path: "/test/file1.txt", Selectable: true}}
	if s.EnterFilesPane(rows) {
		t.Fatal("expected EnterFilesPane to reject a category without file selection")
	}
	if s.EnterFilesPane(nil) {
		t.Fatal("expected EnterFilesPane to reject empty rows")
	}
}

func TestEnterFilesPaneSkipsToFirstSelectableRow(t *testing.T) {
	s, _ := newTestState(true)
	rows := []grouping.DisplayRow{
		{Type: "directory-header", DirectoryKey: "/test", Selectable: false},
		{Type: "file", DirectoryKey: "/test", Path: "/test/file1.txt", Selectable: true},
	}

	if !s.EnterFilesPane(rows) {
		t.Fatal("expected EnterFilesPane to succeed")
	}
	if s.Pane() != PaneFiles {
		t.Fatalf("expected PaneFiles, got %v", s.Pane())
	}
	id, ok := s.ActiveCategory()
	if !ok || id != "large-files" {
		t.Fatalf("expected large-files to be active, got %v %v", id, ok)
	}
	if s.PickerState("large-files").FileCaret != 1 {
		t.Fatalf("expected caret to skip the header to row 1, got %d", s.PickerState("large-files").FileCaret)
	}
}

func TestExitFilesPanePreservesCaretAndExpandState(t *testing.T) {
	s, _ := newTestState(true)
	rows := []grouping.DisplayRow{
		{Type: "directory-header", DirectoryKey: "/test", Selectable: false},
		{Type: "file", DirectoryKey: "/test", Path: "/test/file1.txt", Selectable: true},
		{Type: "file", DirectoryKey: "/test", Path: "/test/file2.txt", Selectable: true},
	}
	s.EnterFilesPane(rows)
	s.MoveFileCaret(1, rows) // caret -> row 2 ("/test/file2.txt")
	s.ExpandDirectoryAtCaret(rows)

	s.ExitFilesPane()
	if s.Pane() != PaneCategories {
		t.Fatalf("expected PaneCategories after exit, got %v", s.Pane())
	}
	if _, ok := s.ActiveCategory(); ok {
		t.Fatal("expected no active category after ExitFilesPane")
	}

	// Re-entering must restore the remembered caret and expand override.
	if !s.EnterFilesPane(rows) {
		t.Fatal("expected re-entering files pane to succeed")
	}
	state := s.PickerState("large-files")
	if state.FileCaret != 2 {
		t.Fatalf("expected caret to persist at row 2, got %d", state.FileCaret)
	}
	if state.DirExpandLimits["/test"] != DirVisChildLimit+ExpandIncrement {
		t.Fatalf("expected expand override to persist, got %v", state.DirExpandLimits)
	}
}

func TestMoveFileCaretSkipsNonSelectableAndClampsAtEnds(t *testing.T) {
	s, _ := newTestState(true)
	rows := []grouping.DisplayRow{
		{Type: "directory-header", DirectoryKey: "/test", Selectable: false},             // 0
		{Type: "file", DirectoryKey: "/test", Path: "/test/file1.txt", Selectable: true}, // 1
		{Type: "expand-hint", DirectoryKey: "/test", Selectable: false},                  // 2
		{Type: "file", DirectoryKey: "/test", Path: "/test/file2.txt", Selectable: true}, // 3
	}
	s.EnterFilesPane(rows) // caret lands on row 1

	s.MoveFileCaret(1, rows) // skip row 2 (expand-hint), land on row 3
	if s.PickerState("large-files").FileCaret != 3 {
		t.Fatalf("expected caret 3, got %d", s.PickerState("large-files").FileCaret)
	}

	s.MoveFileCaret(1, rows) // no selectable row after 3: no-op
	if s.PickerState("large-files").FileCaret != 3 {
		t.Fatalf("expected caret to stay at 3 (no row beyond), got %d", s.PickerState("large-files").FileCaret)
	}

	s.MoveFileCaret(-1, rows) // skip row 2 again, land back on row 1
	if s.PickerState("large-files").FileCaret != 1 {
		t.Fatalf("expected caret 1, got %d", s.PickerState("large-files").FileCaret)
	}

	s.MoveFileCaret(-1, rows) // row 0 is a header, not selectable: no-op
	if s.PickerState("large-files").FileCaret != 1 {
		t.Fatalf("expected caret to stay at 1 (header not selectable), got %d", s.PickerState("large-files").FileCaret)
	}
}

func TestMoveCategoryCaretClamps(t *testing.T) {
	s, _ := newTestState(true)
	s.MoveCategoryCaret(-5)
	if s.CategoryCaret() != 0 {
		t.Fatalf("expected caret clamped to 0, got %d", s.CategoryCaret())
	}
	s.MoveCategoryCaret(5)
	if s.CategoryCaret() != len(s.Results())-1 {
		t.Fatalf("expected caret clamped to last row, got %d", s.CategoryCaret())
	}
}

func TestToggleDirectoryAtCaretOnlyTouchesVisibleRows(t *testing.T) {
	// This demonstrates the real-world nuance file-picker.test.ts's own
	// toggleDirectoryFiles helper glosses over: State.ToggleDirectoryAtCaret
	// derives filesInDirectory from the DisplayRow stream, so a file hidden
	// behind an expand-hint (not yet expanded) is left untouched.
	items := itemsAt("/test/dir1/a.txt", "/test/dir1/b.txt", "/test/dir1/c.txt")
	items[0].Size, items[1].Size, items[2].Size = 300, 200, 100
	result := core.ScanResult{
		Category: core.Category{ID: "large-files", SupportsFileSelection: true},
		Items:    items,
	}
	s := NewState([]core.ScanResult{result}, map[core.CategoryID]bool{"large-files": true})

	rows := grouping.GroupItems(items, "", nil, 2, true) // only 2 of 3 files visible
	if !s.EnterFilesPane(rows) {
		t.Fatal("expected EnterFilesPane to succeed")
	}

	if !s.ToggleDirectoryAtCaret(rows) {
		t.Fatal("expected ToggleDirectoryAtCaret to succeed")
	}
	sel := s.SelectedFiles("large-files")
	mustLen(t, "selected", sel, 2)
	if !sel["/test/dir1/a.txt"] || !sel["/test/dir1/b.txt"] {
		t.Fatal("expected the two visible files to be selected")
	}
	if sel["/test/dir1/c.txt"] {
		t.Fatal("expected the hidden (unexpanded) file to remain untouched")
	}
}

func TestExpandAndCollapseDirectoryAtCaret(t *testing.T) {
	s, _ := newTestState(true)
	rows := []grouping.DisplayRow{
		{Type: "directory-header", DirectoryKey: "/test", Selectable: false},
		{Type: "file", DirectoryKey: "/test", Path: "/test/file1.txt", Selectable: true},
	}
	s.EnterFilesPane(rows) // caret -> row 1

	if !s.ExpandDirectoryAtCaret(rows) {
		t.Fatal("expected ExpandDirectoryAtCaret to succeed")
	}
	if got := s.PickerState("large-files").DirExpandLimits["/test"]; got != DirVisChildLimit+ExpandIncrement {
		t.Fatalf("expected expand limit %d, got %d", DirVisChildLimit+ExpandIncrement, got)
	}

	if !s.ExpandDirectoryAtCaret(rows) { // expand again: cumulative
		t.Fatal("expected second ExpandDirectoryAtCaret to succeed")
	}
	if got := s.PickerState("large-files").DirExpandLimits["/test"]; got != DirVisChildLimit+2*ExpandIncrement {
		t.Fatalf("expected cumulative expand limit %d, got %d", DirVisChildLimit+2*ExpandIncrement, got)
	}

	if !s.CollapseDirectoryAtCaret(rows) {
		t.Fatal("expected CollapseDirectoryAtCaret to succeed")
	}
	if got := s.PickerState("large-files").DirExpandLimits["/test"]; got != DirVisChildLimit {
		t.Fatalf("expected collapse to reset to %d, got %d", DirVisChildLimit, got)
	}

	if !s.CollapseDirectoryAtCaret(rows) { // collapsing again is a no-op reset, not a toggle
		t.Fatal("expected repeated CollapseDirectoryAtCaret to still succeed")
	}
	if got := s.PickerState("large-files").DirExpandLimits["/test"]; got != DirVisChildLimit {
		t.Fatalf("expected collapse to remain at %d, got %d", DirVisChildLimit, got)
	}
}

func TestExpandHintAtCaretOnlyFiresOnExpandHintRows(t *testing.T) {
	s, _ := newTestState(true)
	fileRows := []grouping.DisplayRow{
		{Type: "file", DirectoryKey: "/test", Path: "/test/file1.txt", Selectable: true},
	}
	s.EnterFilesPane(fileRows)
	if s.ExpandHintAtCaret(fileRows) {
		t.Fatal("expected ExpandHintAtCaret to reject a plain file row")
	}

	hintRows := []grouping.DisplayRow{
		{Type: "expand-hint", DirectoryKey: "/test", HiddenCount: 3, Selectable: false},
	}
	// Force the caret onto the expand-hint row directly (EnterFilesPane would
	// skip it since it is not selectable; the "right" key case is reached
	// while already positioned there via directory-header navigation, so we
	// seed the picker state to simulate that.)
	s.pickerStates["large-files"] = CategoryPickerState{Visible: true, FileCaret: 0, DirExpandLimits: map[string]int{}}
	idCopy := core.CategoryID("large-files")
	s.activeCategory = &idCopy
	s.activePane = PaneFiles

	if !s.ExpandHintAtCaret(hintRows) {
		t.Fatal("expected ExpandHintAtCaret to succeed on an expand-hint row")
	}
	if got := s.PickerState("large-files").DirExpandLimits["/test"]; got != DirVisChildLimit+ExpandIncrement {
		t.Fatalf("expected expand limit %d, got %d", DirVisChildLimit+ExpandIncrement, got)
	}
}

func TestFilePageWindow(t *testing.T) {
	cases := []struct {
		name      string
		caret     int
		total     int
		active    bool
		wantStart int
		wantEnd   int
	}{
		{"inactive always starts at top", 10, 20, false, 0, FilesPageSize},
		{"fewer rows than a page", 1, 3, true, 0, 3},
		{"caret centered mid-list", 10, 20, true, 7, 13},
		{"caret near start clamps to 0", 1, 20, true, 0, FilesPageSize},
		{"caret near end clamps to total-pageSize", 19, 20, true, 14, 20},
		{"empty list", 0, 0, true, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := FilePageWindow(tc.caret, tc.total, tc.active)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("FilePageWindow(%d, %d, %v) = (%d, %d), want (%d, %d)",
					tc.caret, tc.total, tc.active, start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
