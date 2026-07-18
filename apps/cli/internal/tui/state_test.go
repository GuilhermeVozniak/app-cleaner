package tui

import (
	"fmt"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/grouping"
)

func mkResult(id core.CategoryID, paths ...string) core.ScanResult {
	items := make([]core.CleanableItem, len(paths))
	var total int64
	for i, p := range paths {
		items[i] = core.CleanableItem{Path: p, Size: int64(100 * (i + 1)), Name: p}
		total += items[i].Size
	}
	return core.ScanResult{Category: core.Category{ID: id, Name: string(id)}, Items: items, TotalSize: total}
}

func TestPickerStoreToggleCategoryCouplesFiles(t *testing.T) {
	results := []core.ScanResult{mkResult("downloads", "/h/Downloads/a.zip", "/h/Downloads/b.zip")}
	p := newPickerStore(results, map[core.CategoryID]bool{"downloads": true})

	p.ToggleCategory("downloads")
	if !p.IsCategorySelected("downloads") {
		t.Fatal("toggle must select the category")
	}
	if !p.IsFileSelected("downloads", "/h/Downloads/a.zip") || !p.IsFileSelected("downloads", "/h/Downloads/b.zip") {
		t.Fatal("selecting a file-selection category must auto-select all its files")
	}
	p.ToggleCategory("downloads")
	if p.IsCategorySelected("downloads") || p.IsFileSelected("downloads", "/h/Downloads/a.zip") {
		t.Fatal("deselecting must clear the category and its file selections")
	}
}

func TestPickerStoreEnterFilesToggleBackAndResult(t *testing.T) {
	results := []core.ScanResult{mkResult("large-files", "/h/big.bin")}
	p := newPickerStore(results, map[core.CategoryID]bool{"large-files": true})
	rows := grouping.GroupItems(results[0].Items, "/h", nil, 5, false)

	if !p.EnterFiles("large-files", rows) {
		t.Fatal("EnterFiles must succeed for a file-selection category with rows")
	}
	if p.ActivePane() != "files" {
		t.Fatalf("ActivePane = %q, want files", p.ActivePane())
	}
	if id, ok := p.ActiveCategory(); !ok || id != "large-files" {
		t.Fatalf("ActiveCategory = %q,%v, want large-files,true", id, ok)
	}
	p.ToggleFile("large-files", rows) // caret sits on the sole file row
	cats, files := p.Result()
	if !cats["large-files"] || len(files["large-files"]) != 1 {
		t.Fatalf("expected coupled category + 1 selected file, got cats=%v files=%v", cats, files)
	}
	p.Back()
	if p.ActivePane() != "categories" {
		t.Fatalf("Back must return to categories, got %q", p.ActivePane())
	}
}

func TestPickerStoreDirectoryToggleScopedToCaretDir(t *testing.T) {
	// Sizes are 100*(i+1) by position, so DirA (200+300) outranks DirB (100)
	// and the caret lands on DirA's largest file after EnterFiles.
	results := []core.ScanResult{mkResult("large-files",
		"/h/DirB/three.bin", "/h/DirA/one.bin", "/h/DirA/two.bin")}
	p := newPickerStore(results, map[core.CategoryID]bool{"large-files": true})
	rows := grouping.GroupItems(results[0].Items, "/h", nil, 5, false)

	p.EnterFiles("large-files", rows)
	p.ToggleDirectory("large-files", rows)
	if !p.IsFileSelected("large-files", "/h/DirA/one.bin") || !p.IsFileSelected("large-files", "/h/DirA/two.bin") {
		t.Fatal("'d' must select every visible file in the caret's directory")
	}
	if p.IsFileSelected("large-files", "/h/DirB/three.bin") {
		t.Fatal("'d' must not touch a different directory")
	}
}

func TestPickerStoreMoveCategoryCaretClampsToBounds(t *testing.T) {
	results := []core.ScanResult{mkResult("trash", "/h/.Trash/a"), mkResult("downloads", "/h/Downloads/b")}
	p := newPickerStore(results, nil)

	p.MoveCategoryCaret(-1) // already at 0: clamped no-op
	if p.CategoryCaret() != 0 {
		t.Fatalf("CategoryCaret() = %d, want 0 (clamped)", p.CategoryCaret())
	}
	p.MoveCategoryCaret(1)
	if p.CategoryCaret() != 1 {
		t.Fatalf("CategoryCaret() = %d, want 1", p.CategoryCaret())
	}
	p.MoveCategoryCaret(5) // past the end: clamped to the last row
	if p.CategoryCaret() != 1 {
		t.Fatalf("CategoryCaret() = %d, want 1 (clamped to last row)", p.CategoryCaret())
	}
}

func TestPickerStoreSelectAllCategoriesTogglesEveryCategory(t *testing.T) {
	results := []core.ScanResult{mkResult("trash", "/h/.Trash/a"), mkResult("downloads", "/h/Downloads/b")}
	p := newPickerStore(results, nil)

	p.SelectAllCategories()
	if !p.IsCategorySelected("trash") || !p.IsCategorySelected("downloads") {
		t.Fatal("SelectAllCategories must select every category")
	}
	p.SelectAllCategories() // pressed again: full deselect
	if p.IsCategorySelected("trash") || p.IsCategorySelected("downloads") {
		t.Fatal("SelectAllCategories a second time must deselect everything")
	}
}

func TestPickerStoreInvertCategoriesFlipsEachIndependently(t *testing.T) {
	results := []core.ScanResult{mkResult("trash", "/h/.Trash/a"), mkResult("downloads", "/h/Downloads/b")}
	p := newPickerStore(results, nil)
	p.ToggleCategory("trash") // caret starts at 0 = trash

	p.InvertCategories()
	if p.IsCategorySelected("trash") {
		t.Fatal("InvertCategories must deselect the previously-selected trash")
	}
	if !p.IsCategorySelected("downloads") {
		t.Fatal("InvertCategories must select the previously-unselected downloads")
	}
}

func TestPickerStoreSelectAllFilesAndInvertFiles(t *testing.T) {
	results := []core.ScanResult{mkResult("large-files", "/h/big1.bin", "/h/big2.bin")}
	p := newPickerStore(results, map[core.CategoryID]bool{"large-files": true})
	rows := grouping.GroupItems(results[0].Items, "/h", nil, 5, false)
	p.EnterFiles("large-files", rows)

	p.SelectAllFiles("large-files", nil)
	if !p.IsFileSelected("large-files", "/h/big1.bin") || !p.IsFileSelected("large-files", "/h/big2.bin") {
		t.Fatal("SelectAllFiles must select every file in the active category")
	}

	p.InvertFiles("large-files", nil)
	if p.IsFileSelected("large-files", "/h/big1.bin") || p.IsFileSelected("large-files", "/h/big2.bin") {
		t.Fatal("InvertFiles after SelectAllFiles must deselect everything")
	}
}

func TestPickerStoreCollapseCurrentDirResetsExpandLimit(t *testing.T) {
	// 8 files in one directory (default visible limit 5) so there is an
	// expand-hint row to expand past, and something for collapse to reset.
	paths := make([]string, 8)
	for i := range paths {
		paths[i] = fmt.Sprintf("/h/Dir/f%02d.bin", i)
	}
	results := []core.ScanResult{mkResult("large-files", paths...)}
	p := newPickerStore(results, map[core.CategoryID]bool{"large-files": true})
	rows := grouping.GroupItems(results[0].Items, "/h", nil, 5, false)
	p.EnterFiles("large-files", rows)

	p.ExpandCurrentDir("large-files", rows)
	expanded := p.DirExpandLimits("large-files")["/h/Dir"]
	if expanded <= 5 {
		t.Fatalf("expand limit after ExpandCurrentDir = %d, want > 5", expanded)
	}

	// Re-render rows with the expanded limit before collapsing, matching how
	// the model always recomputes rows from the current DirExpandLimits.
	rows = grouping.GroupItems(results[0].Items, "/h", p.DirExpandLimits("large-files"), 5, false)
	p.CollapseCurrentDir("large-files", rows)
	if got := p.DirExpandLimits("large-files")["/h/Dir"]; got != 5 {
		t.Fatalf("expand limit after CollapseCurrentDir = %d, want reset to 5", got)
	}
}
