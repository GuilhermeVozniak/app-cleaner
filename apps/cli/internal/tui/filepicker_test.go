package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/output"
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

// TestFilePickerRightOnFileRowDoesNotExpand guards the files-pane keymap split:
// with the caret on a file row (the normal state after entering files), '→'
// must be a no-op (it only expands from an expand-hint row), whereas 'm'
// expands the caret's directory from any row.
func TestFilePickerRightOnFileRowDoesNotExpand(t *testing.T) {
	// 8 files in one directory (default limit 5) -> header + 5 files +
	// expand-hint; after entering, the caret lands on the first FILE row.
	items := make([]core.CleanableItem, 8)
	for i := range items {
		items[i] = core.CleanableItem{
			Path: fmt.Sprintf("/h/Downloads/f%02d.bin", i),
			Size: int64(8000 - i*100),
			Name: fmt.Sprintf("f%02d.bin", i),
		}
	}
	res := core.ScanResult{
		Category:  core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetyRisky, SupportsFileSelection: true},
		Items:     items,
		TotalSize: 40000,
	}
	m := NewFilePickerModel([]core.ScanResult{res}, map[core.CategoryID]bool{"large-files": true}, "/h", false)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter files pane
	fp := next.(FilePickerModel)
	if fp.store.ActivePane() != "files" {
		t.Fatalf("expected files pane, got %s", fp.store.ActivePane())
	}
	before := len(fp.store.DirExpandLimits("large-files"))

	// '→' with the caret on a file row must NOT change any expand limit.
	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRight})
	fp = next.(FilePickerModel)
	if got := len(fp.store.DirExpandLimits("large-files")); got != before {
		t.Fatalf("right on a file row must not change expand limits: before=%d after=%d", before, got)
	}

	// 'm' from the same file row DOES expand the caret's directory.
	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	fp = next.(FilePickerModel)
	if got := len(fp.store.DirExpandLimits("large-files")); got <= before {
		t.Fatalf("m must expand the caret's directory: before=%d after=%d", before, got)
	}
}

// TestFilePickerFileRowUsesTruncateName asserts the file-row rendering routes
// long names through output.TruncateName (the extension-preserving port),
// not a byte-cut that would drop the extension.
func TestFilePickerFileRowUsesTruncateName(t *testing.T) {
	longName := strings.Repeat("a", 44) + ".log" // 48 runes, exceeds FILE_NAME_WIDTH (35)
	res := core.ScanResult{
		Category:  core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetyRisky, SupportsFileSelection: true},
		Items:     []core.CleanableItem{{Path: "/h/Downloads/" + longName, Size: 5000, Name: longName}},
		TotalSize: 5000,
	}
	m := NewFilePickerModel([]core.ScanResult{res}, map[core.CategoryID]bool{"large-files": true}, "/h", false)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter files pane
	view := next.(FilePickerModel).View()

	want := output.TruncateName(longName, FILE_NAME_WIDTH)
	if !strings.Contains(view, want) {
		t.Fatalf("file row must render name via output.TruncateName (%q), got:\n%s", want, view)
	}
	if !strings.Contains(view, ".log") {
		t.Fatalf("output.TruncateName preserves the extension; a byte-cut would drop it, got:\n%s", view)
	}
}

// TestFilePickerInactiveCategoryPaginatesFromTop guards the inactive-block
// pagination fix: an inactive-but-visible category renders its file block from
// the TOP regardless of a deep remembered caret (the active pane stays
// centered on its caret).
func TestFilePickerInactiveCategoryPaginatesFromTop(t *testing.T) {
	// 8 single-file directories -> header,file,header,file,... (16 rows); files
	// sorted by size so directory order is file0..file7. Caret can walk deep.
	items := make([]core.CleanableItem, 8)
	for i := range items {
		items[i] = core.CleanableItem{
			Path: fmt.Sprintf("/h/d%d/file%d.bin", i, i),
			Size: int64(8000 - i*1000),
			Name: fmt.Sprintf("file%d.bin", i),
		}
	}
	res := core.ScanResult{
		Category:  core.Category{ID: "large-files", Name: "Large Files", SafetyLevel: core.SafetyRisky, SupportsFileSelection: true},
		Items:     items,
		TotalSize: 36000,
	}
	m := NewFilePickerModel([]core.ScanResult{res}, map[core.CategoryID]bool{"large-files": true}, "/h", false)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter files pane (active)
	fp := next.(FilePickerModel)
	for i := 0; i < 6; i++ { // drive the caret deep into the list
		next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyDown})
		fp = next.(FilePickerModel)
	}
	// Exit back to categories: the category stays Visible but is now inactive.
	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	fp = next.(FilePickerModel)
	if fp.store.ActivePane() != "categories" {
		t.Fatalf("expected categories pane after backspace, got %s", fp.store.ActivePane())
	}

	view := fp.View()
	if !strings.Contains(view, "file0.bin") {
		t.Fatalf("inactive category must render its first rows (file0.bin) from the top, got:\n%s", view)
	}
	if strings.Contains(view, "file7.bin") {
		t.Fatalf("inactive category must not center on the deep caret (file7.bin must be off-window), got:\n%s", view)
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
