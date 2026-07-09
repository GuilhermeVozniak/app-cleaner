package tui

import (
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
