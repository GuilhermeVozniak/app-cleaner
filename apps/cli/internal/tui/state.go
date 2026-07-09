package tui

import (
	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/selection"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/grouping"
)

// pickerStore is a thin stateful wrapper over Task 2's pure selection.State.
// It exposes the id + []grouping.DisplayRow convenience surface
// FilePickerModel calls, delegating every mutation to selection.State (the
// canonical package — this file never reimplements selection logic). The
// `id` arguments always equal the caret/active category State already
// tracks; they are used only to resolve a caret row to its file path.
type pickerStore struct{ st *selection.State }

func newPickerStore(results []core.ScanResult, categoriesWithFiles map[core.CategoryID]bool) *pickerStore {
	return &pickerStore{st: selection.NewState(results, categoriesWithFiles)}
}

// pickerView is selection.CategoryPickerState plus an Active flag (whether id
// is the active files-pane category) the renderer needs.
type pickerView struct {
	Visible         bool
	Active          bool
	FileCaret       int
	DirExpandLimits map[string]int
}

func (p *pickerStore) ActivePane() selection.Pane  { return p.st.Pane() }
func (p *pickerStore) CategoryCaret() int          { return p.st.CategoryCaret() }
func (p *pickerStore) MoveCategoryCaret(delta int) { p.st.MoveCategoryCaret(delta) }

func (p *pickerStore) IsCategorySelected(id core.CategoryID) bool {
	return p.st.IsCategorySelected(id)
}

// ToggleCategory delegates to State.ToggleCategory, which acts on the caret
// category — always equal to id (the model passes the caret category).
func (p *pickerStore) ToggleCategory(id core.CategoryID) { p.st.ToggleCategory() }

func (p *pickerStore) SelectAllCategories() { p.st.SelectAllCategories() }
func (p *pickerStore) InvertCategories()    { p.st.InvertCategories() }

// EnterFiles drills into id's files pane (id is the caret category).
func (p *pickerStore) EnterFiles(id core.CategoryID, rows []grouping.DisplayRow) bool {
	return p.st.EnterFilesPane(rows)
}

func (p *pickerStore) Back()                                   { p.st.ExitFilesPane() }
func (p *pickerStore) ActiveCategory() (core.CategoryID, bool) { return p.st.ActiveCategory() }

func (p *pickerStore) PickerState(id core.CategoryID) pickerView {
	ps := p.st.PickerState(id)
	active := false
	if aid, ok := p.st.ActiveCategory(); ok && aid == id {
		active = true
	}
	return pickerView{
		Visible:         ps.Visible,
		Active:          active,
		FileCaret:       ps.FileCaret,
		DirExpandLimits: ps.DirExpandLimits,
	}
}

func (p *pickerStore) FileCaret(id core.CategoryID) int { return p.st.PickerState(id).FileCaret }

func (p *pickerStore) DirExpandLimits(id core.CategoryID) map[string]int {
	return p.st.PickerState(id).DirExpandLimits
}

func (p *pickerStore) MoveFileCaret(id core.CategoryID, delta int, rows []grouping.DisplayRow) {
	p.st.MoveFileCaret(delta, rows)
}

func (p *pickerStore) IsFileSelected(id core.CategoryID, path string) bool {
	return p.st.IsFileSelected(id, path)
}

// ToggleFile toggles the file under id's caret. State.ToggleFile takes a
// path, so resolve the caret row here first.
func (p *pickerStore) ToggleFile(id core.CategoryID, rows []grouping.DisplayRow) {
	caret := p.st.PickerState(id).FileCaret
	if caret < 0 || caret >= len(rows) || rows[caret].Type != "file" {
		return
	}
	p.st.ToggleFile(rows[caret].Path)
}

// SelectAllFiles / InvertFiles act on the active category's full item list
// (State ignores the visible-rows slice the model passes; "all" means all
// items in the category, matching the original CLI's files-pane a/i keys).
func (p *pickerStore) SelectAllFiles(id core.CategoryID, allPaths []string) { p.st.SelectAllFiles() }
func (p *pickerStore) InvertFiles(id core.CategoryID, allPaths []string)    { p.st.InvertFiles() }

func (p *pickerStore) ToggleDirectory(id core.CategoryID, rows []grouping.DisplayRow) {
	p.st.ToggleDirectoryAtCaret(rows)
}

// ExpandCurrentDir covers both 'm' and '→'-on-expand-hint: State's
// ExpandDirectoryAtCaret bumps the caret row's directory limit for any row
// carrying a DirectoryKey (header, file, or expand-hint).
func (p *pickerStore) ExpandCurrentDir(id core.CategoryID, rows []grouping.DisplayRow) {
	p.st.ExpandDirectoryAtCaret(rows)
}

// ExpandHintAtCaret covers the files-pane '→' key exclusively: unlike 'm'
// (ExpandCurrentDir), State.ExpandHintAtCaret bumps the directory limit ONLY
// when the caret sits on an expand-hint row, so it is a no-op from any other
// (directory-header/file) row — mirroring file-picker.ts's right-key handler.
func (p *pickerStore) ExpandHintAtCaret(id core.CategoryID, rows []grouping.DisplayRow) {
	p.st.ExpandHintAtCaret(rows)
}

func (p *pickerStore) CollapseCurrentDir(id core.CategoryID, rows []grouping.DisplayRow) {
	p.st.CollapseDirectoryAtCaret(rows)
}

func (p *pickerStore) Result() (map[core.CategoryID]bool, map[core.CategoryID]map[string]bool) {
	cats := p.st.SelectedCategories()
	files := map[core.CategoryID]map[string]bool{}
	for _, r := range p.st.Results() {
		if sel := p.st.SelectedFiles(r.Category.ID); len(sel) > 0 {
			files[r.Category.ID] = sel
		}
	}
	return cats, files
}
