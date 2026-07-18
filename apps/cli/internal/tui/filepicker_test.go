package tui

import (
	"errors"
	"fmt"
	"os"
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

func TestFilePickerInitReturnsNilCmd(t *testing.T) {
	m := newTestFilePicker()
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init() = %v, want nil", cmd)
	}
}

func TestFilePickerEnterSetsDoneAndQuits(t *testing.T) {
	m := newTestFilePicker()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	fp := next.(FilePickerModel)
	if !fp.Done {
		t.Fatal("enter must set Done")
	}
	if cmd == nil {
		t.Fatal("enter must return tea.Quit")
	}
}

func TestFilePickerCtrlCSetsAbortedAndQuits(t *testing.T) {
	m := newTestFilePicker()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	fp := next.(FilePickerModel)
	if !fp.Aborted {
		t.Fatal("ctrl+c must set Aborted")
	}
	if cmd == nil {
		t.Fatal("ctrl+c must return tea.Quit")
	}
}

// TestFilePickerRightOnNonFileSelectionCategoryStaysInCategories guards the
// categories-pane '→' guard: entering the files pane must be refused for a
// category not present in CategoriesWithFiles, leaving the pane unchanged.
func TestFilePickerRightOnNonFileSelectionCategoryStaysInCategories(t *testing.T) {
	res := core.ScanResult{
		Category:  core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
		Items:     []core.CleanableItem{{Path: "/h/.Trash/a", Size: 1, Name: "a"}},
		TotalSize: 1,
	}
	m := NewFilePickerModel([]core.ScanResult{res}, map[core.CategoryID]bool{}, "/h", false)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	fp := next.(FilePickerModel)
	if fp.store.ActivePane() != "categories" {
		t.Fatalf("'→' on a non-file-selection category must stay in categories pane, got %s", fp.store.ActivePane())
	}
}

func TestFilePickerCategoriesPaneKeys(t *testing.T) {
	results := []core.ScanResult{
		{
			Category: core.Category{ID: "trash", Name: "Trash", SafetyLevel: core.SafetySafe},
			Items:    []core.CleanableItem{{Path: "/h/.Trash/a", Size: 1, Name: "a"}}, TotalSize: 1,
		},
		{
			Category: core.Category{ID: "downloads", Name: "Downloads", SafetyLevel: core.SafetySafe},
			Items:    []core.CleanableItem{{Path: "/h/Downloads/b", Size: 2, Name: "b"}}, TotalSize: 2,
		},
	}
	m := NewFilePickerModel(results, map[core.CategoryID]bool{}, "/h", false)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	fp := next.(FilePickerModel)
	if fp.store.CategoryCaret() != 1 {
		t.Fatalf("down must move the category caret to 1, got %d", fp.store.CategoryCaret())
	}

	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyUp})
	fp = next.(FilePickerModel)
	if fp.store.CategoryCaret() != 0 {
		t.Fatalf("up must move the category caret back to 0, got %d", fp.store.CategoryCaret())
	}

	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	fp = next.(FilePickerModel)
	if !fp.store.IsCategorySelected("trash") {
		t.Fatal("space on the categories pane must toggle the caret category")
	}

	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	fp = next.(FilePickerModel)
	if !fp.store.IsCategorySelected("trash") || !fp.store.IsCategorySelected("downloads") {
		t.Fatal("'a' on the categories pane must select every category")
	}

	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	fp = next.(FilePickerModel)
	if fp.store.IsCategorySelected("trash") || fp.store.IsCategorySelected("downloads") {
		t.Fatal("'i' after 'a' must invert every category back to deselected")
	}
}

func TestFilePickerFilesPaneSelectAllAndInvert(t *testing.T) {
	m := newTestFilePicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter files pane

	next, _ = next.(FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	fp := next.(FilePickerModel)
	_, files := fp.Result()
	if len(files["large-files"]) != 2 {
		t.Fatalf("'a' in the files pane must select every file, got %d selected", len(files["large-files"]))
	}

	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	fp = next.(FilePickerModel)
	_, files = fp.Result()
	if len(files["large-files"]) != 0 {
		t.Fatalf("'i' after 'a' must deselect every file, got %d selected", len(files["large-files"]))
	}
}

func TestFilePickerFilesPaneDToggleDirectory(t *testing.T) {
	m := newTestFilePicker() // both items share the /h/Downloads directory
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next, _ = next.(FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	fp := next.(FilePickerModel)
	_, files := fp.Result()
	if len(files["large-files"]) != 2 {
		t.Fatalf("'d' must select every file in the caret's directory, got %d selected", len(files["large-files"]))
	}
}

func TestFilePickerFilesPaneHCollapsesExpandedDirectory(t *testing.T) {
	items := make([]core.CleanableItem, 8) // exceeds the default 5-visible limit
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

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next, _ = next.(FilePickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	fp := next.(FilePickerModel)
	expanded := fp.store.DirExpandLimits("large-files")["/h/Downloads"]
	if expanded <= 5 {
		t.Fatalf("'m' must expand the directory limit past 5, got %d", expanded)
	}

	next, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	fp = next.(FilePickerModel)
	if got := fp.store.DirExpandLimits("large-files")["/h/Downloads"]; got != 5 {
		t.Fatalf("'h' must collapse the directory limit back to 5, got %d", got)
	}
}

func TestFilePickerUpdateCopiedMsgSetsToastAndSchedulesClear(t *testing.T) {
	m := newTestFilePicker()
	next, cmd := m.Update(copiedMsg{path: "/h/Downloads"})
	fp := next.(FilePickerModel)
	if !strings.Contains(fp.copyStatus, "Copied: /h/Downloads") {
		t.Fatalf("copyStatus = %q, want a Copied message", fp.copyStatus)
	}
	if cmd == nil {
		t.Fatal("a successful copy must schedule the toast-clear tick")
	}
}

func TestFilePickerUpdateCopiedMsgErrorSetsFailureToast(t *testing.T) {
	m := newTestFilePicker()
	next, _ := m.Update(copiedMsg{path: "/h/Downloads", err: errors.New("boom")})
	fp := next.(FilePickerModel)
	if !strings.Contains(fp.copyStatus, "Failed to copy") || !strings.Contains(fp.copyStatus, "boom") {
		t.Fatalf("copyStatus = %q, want a failure message mentioning the error", fp.copyStatus)
	}
}

func TestFilePickerUpdateClearToastMsgClearsStatus(t *testing.T) {
	m := newTestFilePicker()
	next, _ := m.Update(copiedMsg{path: "/h/Downloads"})
	next, _ = next.(FilePickerModel).Update(clearToastMsg{})
	fp := next.(FilePickerModel)
	if fp.copyStatus != "" {
		t.Fatalf("copyStatus after clearToastMsg = %q, want empty", fp.copyStatus)
	}
}

func TestClearToastAfterReturnsAClearToastMsg(t *testing.T) {
	cmd := clearToastAfter(0)
	if cmd == nil {
		t.Fatal("clearToastAfter must return a non-nil tea.Cmd")
	}
	msg := cmd()
	if _, ok := msg.(clearToastMsg); !ok {
		t.Fatalf("clearToastAfter's cmd produced %T, want clearToastMsg", msg)
	}
}

func TestPathsOfReturnsOnlyFileRowPaths(t *testing.T) {
	m := newTestFilePicker()
	fileRows := m.rowsFor("large-files")
	paths := pathsOf(fileRows)
	if len(paths) != 2 {
		t.Fatalf("pathsOf() = %v, want 2 file paths (directory-header rows excluded)", paths)
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, "/h/Downloads/") {
			t.Fatalf("pathsOf() returned a non-file path %q", p)
		}
	}
}

// TestCopyToClipboardReturnsCopiedMsg exercises the real pbcopy shell-out
// (macOS-only, matching this CLI's product scope): the happy path must
// produce a copiedMsg carrying the text and no error.
func TestCopyToClipboardReturnsCopiedMsg(t *testing.T) {
	if os.Getenv("APP_CLEANER_TEST_CLIPBOARD") == "" {
		t.Skip("set APP_CLEANER_TEST_CLIPBOARD=1 to run (writes to the real clipboard)")
	}
	cmd := copyToClipboard("app-cleaner coverage test")
	msg := cmd()
	cm, ok := msg.(copiedMsg)
	if !ok {
		t.Fatalf("copyToClipboard's cmd produced %T, want copiedMsg", msg)
	}
	if cm.path != "app-cleaner coverage test" {
		t.Fatalf("copiedMsg.path = %q, want the copied text", cm.path)
	}
	if cm.err != nil {
		t.Fatalf("copiedMsg.err = %v, want nil (pbcopy is expected to succeed on macOS)", cm.err)
	}
}
