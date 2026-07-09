// Package selection is the pure, engine-agnostic file-picker selection state
// machine behind the CLI's dual-pane (categories/files) interactive picker.
// It has no I/O and no bubbletea dependency: the TUI layer
// (apps/cli/internal/tui) owns rendering and re-renders after each mutating
// call here. Ported from mac-cleaner-cli's src/pickers/file-picker.ts state
// transitions (see that file's createPrompt keypress handler) and its
// src/pickers/file-picker.test.ts pure-function test helpers.
package selection

import (
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/grouping"
)

// Constants from the original CLI (binding, see docs/superpowers/plans/2026-07-08-monorepo-cli-contract.md).
const (
	FilesPageSize    = 6  // files pane visible-row window size
	DirVisChildLimit = 5  // default per-directory visible file count
	ExpandIncrement  = 10 // "m" / expand-hint "right" bump amount
)

// Pane identifies which of the two picker panes has input focus.
type Pane string

const (
	PaneCategories Pane = "categories"
	PaneFiles      Pane = "files"
)

// CategoryPickerState is the per-category file-picker UI state that must
// persist across pane switches and across a category's file picker being
// hidden/shown again: caret position and per-directory expand overrides
// survive independently of selection state. DirExpandLimits maps a
// DisplayRow.DirectoryKey to the visible-row override for that directory
// (absent = DirVisChildLimit); pass it straight to grouping.GroupItems's
// expand parameter to regenerate rows after a mutation.
type CategoryPickerState struct {
	Visible         bool
	FileCaret       int
	DirExpandLimits map[string]int
}

// State is the selection state machine for one file-picker session (one
// scan's worth of categories). It is not safe for concurrent use.
type State struct {
	results                     []core.ScanResult
	categoriesWithFileSelection map[core.CategoryID]bool

	activePane    Pane
	categoryCaret int

	selectedCategories      map[core.CategoryID]bool
	selectedFilesByCategory map[core.CategoryID]map[string]bool

	activeCategory *core.CategoryID
	pickerStates   map[core.CategoryID]CategoryPickerState
}

// NewState builds a fresh State for results. categoriesWithFileSelection is
// the caller-computed set of category IDs whose file picker should be
// offered (mirrors the original's `-f` flag / category.SupportsFileSelection
// derivation in interactive.ts) — a nil or empty map means no category
// offers per-file selection, matching a category-only picker. results must
// be non-empty for CurrentCategory/EnterFilesPane to be meaningful; an empty
// results slice is otherwise safe (no-ops).
func NewState(results []core.ScanResult, categoriesWithFileSelection map[core.CategoryID]bool) *State {
	cwfs := make(map[core.CategoryID]bool, len(categoriesWithFileSelection))
	for id, v := range categoriesWithFileSelection {
		if v {
			cwfs[id] = true
		}
	}
	return &State{
		results:                     results,
		categoriesWithFileSelection: cwfs,
		activePane:                  PaneCategories,
		categoryCaret:               0,
		selectedCategories:          map[core.CategoryID]bool{},
		selectedFilesByCategory:     map[core.CategoryID]map[string]bool{},
		activeCategory:              nil,
		pickerStates:                map[core.CategoryID]CategoryPickerState{},
	}
}

// ---- pure, side-effect-free selection transitions -------------------------
//
// These five functions are 1:1 ports of the pure test helpers in
// file-picker.test.ts (the ts file factors them out of the createPrompt
// closure for the same reason: testing state transitions without an
// Inquirer prompt harness). Each takes the "current" maps and returns NEW
// maps; none mutates its arguments. State's methods below are thin wrappers
// that call these using the caret/active-category bookkeeping.

// SelectAllFilesInCategory mirrors file-picker.test.ts's
// `selectAllFilesInCategory` (based on file-picker.ts:438-471): the
// files-pane "a" key. If every path in allFiles is already selected for
// categoryID, all are deselected and the category is dropped; otherwise
// every path is selected and the category is added.
func SelectAllFilesInCategory(
	selectedCategories map[core.CategoryID]bool,
	selectedFilesByCategory map[core.CategoryID]map[string]bool,
	categoryID core.CategoryID,
	allFiles []string,
) (map[core.CategoryID]bool, map[core.CategoryID]map[string]bool) {
	current := selectedFilesByCategory[categoryID]
	allSelected := true
	for _, p := range allFiles {
		if !current[p] {
			allSelected = false
			break
		}
	}

	newSelected := copyStringSet(current)
	newCategories := copyBoolSet(selectedCategories)

	if allSelected {
		for _, p := range allFiles {
			delete(newSelected, p)
		}
		if len(newSelected) == 0 {
			delete(newCategories, categoryID)
		}
	} else {
		for _, p := range allFiles {
			newSelected[p] = true
		}
		newCategories[categoryID] = true
	}

	newFiles := copyFileMap(selectedFilesByCategory)
	newFiles[categoryID] = newSelected
	return newCategories, newFiles
}

// InvertFilesInCategory mirrors file-picker.test.ts's
// `invertFilesInCategory` (based on file-picker.ts:510-544): the files-pane
// "i" key. Every path in allFiles flips selection state; the category is
// added if inversion leaves at least one file selected for it, and removed
// only if inversion leaves none selected for it.
func InvertFilesInCategory(
	selectedCategories map[core.CategoryID]bool,
	selectedFilesByCategory map[core.CategoryID]map[string]bool,
	categoryID core.CategoryID,
	allFiles []string,
) (map[core.CategoryID]bool, map[core.CategoryID]map[string]bool) {
	current := selectedFilesByCategory[categoryID]
	newSelected := copyStringSet(current)
	newCategories := copyBoolSet(selectedCategories)
	hasAnySelected := false

	for _, p := range allFiles {
		if newSelected[p] {
			delete(newSelected, p)
		} else {
			newSelected[p] = true
			hasAnySelected = true
		}
	}

	if hasAnySelected {
		newCategories[categoryID] = true
	} else if len(newSelected) == 0 {
		delete(newCategories, categoryID)
	}

	newFiles := copyFileMap(selectedFilesByCategory)
	newFiles[categoryID] = newSelected
	return newCategories, newFiles
}

// ToggleSingleFile mirrors file-picker.test.ts's `toggleSingleFile` (based
// on file-picker.ts:398-417): the files-pane "space" key on one file row.
// Selecting a file always selects its parent category; deselecting the last
// selected file for a category deselects the category.
func ToggleSingleFile(
	selectedCategories map[core.CategoryID]bool,
	selectedFilesByCategory map[core.CategoryID]map[string]bool,
	categoryID core.CategoryID,
	filePath string,
) (map[core.CategoryID]bool, map[core.CategoryID]map[string]bool) {
	current := selectedFilesByCategory[categoryID]
	newSelected := copyStringSet(current)
	newCategories := copyBoolSet(selectedCategories)

	if newSelected[filePath] {
		delete(newSelected, filePath)
		if len(newSelected) == 0 {
			delete(newCategories, categoryID)
		}
	} else {
		newSelected[filePath] = true
		newCategories[categoryID] = true
	}

	newFiles := copyFileMap(selectedFilesByCategory)
	newFiles[categoryID] = newSelected
	return newCategories, newFiles
}

// DeselectCategory mirrors file-picker.test.ts's `deselectCategory` (based
// on file-picker.ts:249-260): the categories-pane "space" key when the
// category under the caret is already selected. Clears every file
// selection recorded for categoryID; other categories are untouched.
func DeselectCategory(
	selectedCategories map[core.CategoryID]bool,
	selectedFilesByCategory map[core.CategoryID]map[string]bool,
	categoryID core.CategoryID,
) (map[core.CategoryID]bool, map[core.CategoryID]map[string]bool) {
	newCategories := copyBoolSet(selectedCategories)
	delete(newCategories, categoryID)

	newFiles := copyFileMap(selectedFilesByCategory)
	newFiles[categoryID] = map[string]bool{}

	return newCategories, newFiles
}

// ToggleDirectoryFiles mirrors file-picker.test.ts's `toggleDirectoryFiles`
// (based on file-picker.ts:472-509): the files-pane "d" key.
// filesInDirectory must be exactly the paths of the currently VISIBLE
// selectable rows sharing the caret's directoryKey — State.ToggleDirectoryAtCaret
// computes this from a DisplayRow stream, so files hidden behind an
// expand-hint are left untouched, exactly like the original. If every path
// in filesInDirectory is already selected they are all deselected,
// otherwise all selected. selectedCategories is deliberately never touched
// here — directory toggling does not change category membership in the
// original (confirmed by the "keeping category membership" test case).
func ToggleDirectoryFiles(
	selectedFilesByCategory map[core.CategoryID]map[string]bool,
	categoryID core.CategoryID,
	filesInDirectory []string,
) map[core.CategoryID]map[string]bool {
	current := selectedFilesByCategory[categoryID]
	allSelected := true
	for _, p := range filesInDirectory {
		if !current[p] {
			allSelected = false
			break
		}
	}

	newSelected := copyStringSet(current)
	if allSelected {
		for _, p := range filesInDirectory {
			delete(newSelected, p)
		}
	} else {
		for _, p := range filesInDirectory {
			newSelected[p] = true
		}
	}

	newFiles := copyFileMap(selectedFilesByCategory)
	newFiles[categoryID] = newSelected
	return newFiles
}

// FilePageWindow computes the [start, end) visible-row window for the files
// pane, mirroring file-picker.ts's inline pagination math (fileStart/fileEnd,
// ts lines ~656-668): centered on caret when active, clamped to
// [0, total-FilesPageSize]; from the top (start=0) when inactive.
func FilePageWindow(caret, total int, active bool) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if active {
		candidate := caret - FilesPageSize/2
		maxStart := total - FilesPageSize
		if candidate > maxStart {
			candidate = maxStart
		}
		if candidate < 0 {
			candidate = 0
		}
		start = candidate
	}
	end = start + FilesPageSize
	if end > total {
		end = total
	}
	return start, end
}

// ---- State: categories pane -------------------------------------------------

// Pane reports which pane currently has input focus.
func (s *State) Pane() Pane { return s.activePane }

// CategoryCaret reports the caret row index into Results().
func (s *State) CategoryCaret() int { return s.categoryCaret }

// Results returns the scan results this State was built from. The caller
// must not mutate the returned slice or its elements.
func (s *State) Results() []core.ScanResult { return s.results }

// CurrentCategory returns the category ID under the categories-pane caret.
// Returns "" if Results() is empty.
func (s *State) CurrentCategory() core.CategoryID {
	if len(s.results) == 0 {
		return ""
	}
	return s.results[s.categoryCaret].Category.ID
}

// ActiveCategory returns the category whose file picker is currently active
// (entered via EnterFilesPane), and whether one is active at all. Only one
// category can be active at a time; ExitFilesPane clears it.
func (s *State) ActiveCategory() (core.CategoryID, bool) {
	if s.activeCategory == nil {
		return "", false
	}
	return *s.activeCategory, true
}

// CanShowFiles reports whether id is in the categoriesWithFileSelection set
// passed to NewState.
func (s *State) CanShowFiles(id core.CategoryID) bool {
	return s.categoriesWithFileSelection[id]
}

// IsCategorySelected reports whether id is currently selected for cleaning.
func (s *State) IsCategorySelected(id core.CategoryID) bool {
	return s.selectedCategories[id]
}

// SelectedCategories returns a copy of the selected-category set.
func (s *State) SelectedCategories() map[core.CategoryID]bool {
	return copyBoolSet(s.selectedCategories)
}

// IsFileSelected reports whether path is selected within category id.
func (s *State) IsFileSelected(id core.CategoryID, path string) bool {
	return s.selectedFilesByCategory[id][path]
}

// SelectedFiles returns a copy of the selected-file-path set for category id
// (empty, non-nil map if none selected).
func (s *State) SelectedFiles(id core.CategoryID) map[string]bool {
	return copyStringSet(s.selectedFilesByCategory[id])
}

// PickerState returns a copy of the per-category picker UI state (caret,
// visibility, expand overrides), defaulting to the zero picker state
// (FileCaret 0, empty DirExpandLimits) if id has never been touched.
func (s *State) PickerState(id core.CategoryID) CategoryPickerState {
	return s.getPickerState(id)
}

// MoveCategoryCaret moves the categories-pane caret by delta, clamped to
// [0, len(Results())-1]. Mirrors the original's isUpKey/isDownKey handlers
// in the categories pane (Math.max/Math.min clamp, no selectable-skip is
// needed since every category row is always selectable).
func (s *State) MoveCategoryCaret(delta int) {
	if len(s.results) == 0 {
		return
	}
	next := s.categoryCaret + delta
	if next < 0 {
		next = 0
	}
	if next > len(s.results)-1 {
		next = len(s.results) - 1
	}
	s.categoryCaret = next
}

// ToggleCategory is the categories-pane "space" key (file-picker.ts:248-281).
// Deselecting clears the category's file selections (DeselectCategory) and
// hides its file picker; selecting auto-selects every file in the category
// (if it supports file selection) and shows its file picker.
func (s *State) ToggleCategory() {
	id := s.CurrentCategory()
	if s.selectedCategories[id] {
		newCategories, newFiles := DeselectCategory(s.selectedCategories, s.selectedFilesByCategory, id)
		s.selectedCategories = newCategories
		s.selectedFilesByCategory = newFiles
		if s.categoriesWithFileSelection[id] {
			s.setPickerVisible(id, false)
		}
		return
	}

	s.selectedCategories = copyBoolSet(s.selectedCategories)
	s.selectedCategories[id] = true
	if s.categoriesWithFileSelection[id] {
		s.setPickerVisible(id, true)
		allPaths := map[string]bool{}
		for _, it := range s.resultFor(id).Items {
			allPaths[it.Path] = true
		}
		s.selectedFilesByCategory = copyFileMap(s.selectedFilesByCategory)
		s.selectedFilesByCategory[id] = allPaths
	}
}

// SelectAllCategories is the categories-pane "a" key (file-picker.ts:282-310).
// If every category is already selected, everything is cleared (selections
// and all per-category picker state); otherwise every category is selected
// and every file-selection-capable category has all its files selected.
func (s *State) SelectAllCategories() {
	allSelected := true
	for _, r := range s.results {
		if !s.selectedCategories[r.Category.ID] {
			allSelected = false
			break
		}
	}

	if allSelected {
		s.selectedCategories = map[core.CategoryID]bool{}
		s.selectedFilesByCategory = map[core.CategoryID]map[string]bool{}
		s.pickerStates = map[core.CategoryID]CategoryPickerState{}
		return
	}

	newSelected := map[core.CategoryID]bool{}
	newFiles := map[core.CategoryID]map[string]bool{}
	for _, r := range s.results {
		id := r.Category.ID
		newSelected[id] = true
		if s.categoriesWithFileSelection[id] {
			paths := map[string]bool{}
			for _, it := range r.Items {
				paths[it.Path] = true
			}
			newFiles[id] = paths
			s.setPickerVisible(id, true)
		}
	}
	s.selectedCategories = newSelected
	s.selectedFilesByCategory = newFiles
}

// InvertCategories is the categories-pane "i" key (file-picker.ts:311-339).
// Every category flips selection state; a category with file selection gets
// all its files selected when it becomes selected, and its file selections
// cleared when it becomes deselected.
func (s *State) InvertCategories() {
	newSelected := copyBoolSet(s.selectedCategories)
	newFiles := copyFileMap(s.selectedFilesByCategory)

	for _, r := range s.results {
		id := r.Category.ID
		if newSelected[id] {
			delete(newSelected, id)
			if s.categoriesWithFileSelection[id] {
				delete(newFiles, id)
				s.setPickerVisible(id, false)
			}
		} else {
			newSelected[id] = true
			if s.categoriesWithFileSelection[id] {
				paths := map[string]bool{}
				for _, it := range r.Items {
					paths[it.Path] = true
				}
				newFiles[id] = paths
				s.setPickerVisible(id, true)
			}
		}
	}

	s.selectedCategories = newSelected
	s.selectedFilesByCategory = newFiles
}

// EnterFilesPane is the categories-pane "right"/"enter" drill-down
// (file-picker.ts:340-386). rows must be the current category's DisplayRow
// stream (grouping.GroupItems(currentCategoryItems, home,
// s.PickerState(id).DirExpandLimits, DirVisChildLimit, absolutePaths)) — the
// caller recomputes it whenever the category's expand limits change.
// Returns false (no state change) if the current category does not support
// file selection or rows is empty. On success the caret lands on the
// picker's previously-remembered FileCaret if it still points at a
// selectable row, otherwise the first selectable row (falling back to 0 if
// none is selectable), and ActivePane becomes PaneFiles.
func (s *State) EnterFilesPane(rows []grouping.DisplayRow) bool {
	id := s.CurrentCategory()
	if !s.categoriesWithFileSelection[id] || len(rows) == 0 {
		return false
	}

	state := s.getPickerState(id)
	caret := state.FileCaret
	if caret < 0 || caret >= len(rows) || !rows[caret].Selectable {
		caret = 0
		for caret < len(rows) && !rows[caret].Selectable {
			caret++
		}
		if caret >= len(rows) {
			caret = 0
		}
	}

	state.Visible = true
	state.FileCaret = caret
	s.pickerStates[id] = state

	idCopy := id
	s.activeCategory = &idCopy
	s.activePane = PaneFiles
	return true
}

// ExitFilesPane is the files-pane "left"/"backspace" key
// (file-picker.ts:611-614): returns focus to the categories pane. The
// active category's picker state (caret, expand limits) is left in
// pickerStates untouched, so re-entering restores it exactly.
func (s *State) ExitFilesPane() {
	s.activeCategory = nil
	s.activePane = PaneCategories
}

// ---- State: files pane -----------------------------------------------------

// ToggleFile is the files-pane "space" key on the active category. No-op if
// no category is active. Wraps ToggleSingleFile.
func (s *State) ToggleFile(path string) {
	id, ok := s.ActiveCategory()
	if !ok {
		return
	}
	s.selectedCategories, s.selectedFilesByCategory = ToggleSingleFile(
		s.selectedCategories, s.selectedFilesByCategory, id, path,
	)
}

// SelectAllFiles is the files-pane "a" key on the active category (uses
// every item in the category's ScanResult, not just currently-visible rows,
// matching the original's use of categoryResult.items). No-op if no category
// is active. Wraps SelectAllFilesInCategory.
func (s *State) SelectAllFiles() {
	id, ok := s.ActiveCategory()
	if !ok {
		return
	}
	allFiles := pathsOf(s.resultFor(id).Items)
	s.selectedCategories, s.selectedFilesByCategory = SelectAllFilesInCategory(
		s.selectedCategories, s.selectedFilesByCategory, id, allFiles,
	)
}

// InvertFiles is the files-pane "i" key on the active category (uses every
// item in the category's ScanResult, matching the original). No-op if no
// category is active. Wraps InvertFilesInCategory.
func (s *State) InvertFiles() {
	id, ok := s.ActiveCategory()
	if !ok {
		return
	}
	allFiles := pathsOf(s.resultFor(id).Items)
	s.selectedCategories, s.selectedFilesByCategory = InvertFilesInCategory(
		s.selectedCategories, s.selectedFilesByCategory, id, allFiles,
	)
}

// ToggleDirectoryAtCaret is the files-pane "d" key (file-picker.ts:472-509).
// rows must be the active category's current DisplayRow stream. Derives the
// caret row's directoryKey and toggles every VISIBLE selectable file row
// sharing it (files hidden behind an expand-hint are untouched). Returns
// false (no-op) if no category is active or the caret is not on a
// selectable row. Category membership is never touched, matching the
// original.
func (s *State) ToggleDirectoryAtCaret(rows []grouping.DisplayRow) bool {
	id, ok := s.ActiveCategory()
	if !ok {
		return false
	}
	caret := s.getPickerState(id).FileCaret
	if caret < 0 || caret >= len(rows) || !rows[caret].Selectable {
		return false
	}

	dirKey := rows[caret].DirectoryKey
	var visible []string
	for _, row := range rows {
		if row.Type == "file" && row.Selectable && row.DirectoryKey == dirKey {
			visible = append(visible, row.Path)
		}
	}

	s.selectedFilesByCategory = ToggleDirectoryFiles(s.selectedFilesByCategory, id, visible)
	return true
}

// MoveFileCaret moves the active category's file caret by delta (typically
// -1 for up, +1 for down), skipping over non-selectable rows (directory
// headers, expand-hints) exactly like the original's isUpKey/isDownKey
// handlers (file-picker.ts:392-414). No-op if no category is active, delta
// is 0, or the walk runs off either end of rows without landing on a
// selectable row.
func (s *State) MoveFileCaret(delta int, rows []grouping.DisplayRow) {
	id, ok := s.ActiveCategory()
	if !ok || delta == 0 {
		return
	}
	state := s.getPickerState(id)
	next := state.FileCaret + delta

	if delta > 0 {
		for next < len(rows) && !rows[next].Selectable {
			next++
		}
		if next >= len(rows) {
			return
		}
	} else {
		for next >= 0 && next < len(rows) && !rows[next].Selectable {
			next--
		}
		if next < 0 {
			return
		}
	}

	state.FileCaret = next
	s.pickerStates[id] = state
}

// ExpandDirectoryAtCaret is the files-pane "m" key (file-picker.ts:571-583):
// bumps the caret row's directory expand limit by ExpandIncrement (from
// DirVisChildLimit if not yet overridden). Works on any row type that
// carries a directoryKey (header, file, or expand-hint — the original does
// not restrict by type for "m"). Returns false if no category is active or
// the caret is out of range.
func (s *State) ExpandDirectoryAtCaret(rows []grouping.DisplayRow) bool {
	id, ok := s.ActiveCategory()
	if !ok {
		return false
	}
	state := s.getPickerState(id)
	caret := state.FileCaret
	if caret < 0 || caret >= len(rows) {
		return false
	}
	dirKey := rows[caret].DirectoryKey
	if dirKey == "" {
		return false
	}

	current, ok := state.DirExpandLimits[dirKey]
	if !ok {
		current = DirVisChildLimit
	}
	newLimits := copyIntMap(state.DirExpandLimits)
	newLimits[dirKey] = current + ExpandIncrement
	state.DirExpandLimits = newLimits
	s.pickerStates[id] = state
	return true
}

// CollapseDirectoryAtCaret is the files-pane "h" key (file-picker.ts:584-593):
// unconditionally resets the caret row's directory expand limit back to
// DirVisChildLimit (it is a reset, not a toggle — pressing it repeatedly has
// no further effect). Returns false if no category is active or the caret
// is out of range.
func (s *State) CollapseDirectoryAtCaret(rows []grouping.DisplayRow) bool {
	id, ok := s.ActiveCategory()
	if !ok {
		return false
	}
	state := s.getPickerState(id)
	caret := state.FileCaret
	if caret < 0 || caret >= len(rows) {
		return false
	}
	dirKey := rows[caret].DirectoryKey
	if dirKey == "" {
		return false
	}

	newLimits := copyIntMap(state.DirExpandLimits)
	newLimits[dirKey] = DirVisChildLimit
	state.DirExpandLimits = newLimits
	s.pickerStates[id] = state
	return true
}

// ExpandHintAtCaret is the files-pane "right" key (file-picker.ts:594-610),
// distinct from "m": it only fires when the caret sits on an expand-hint row
// (type "expand-hint"), and then applies the same +ExpandIncrement bump as
// "m". Returns false if no category is active, the caret is out of range,
// or the caret row is not an expand-hint.
func (s *State) ExpandHintAtCaret(rows []grouping.DisplayRow) bool {
	id, ok := s.ActiveCategory()
	if !ok {
		return false
	}
	state := s.getPickerState(id)
	caret := state.FileCaret
	if caret < 0 || caret >= len(rows) || rows[caret].Type != "expand-hint" {
		return false
	}
	dirKey := rows[caret].DirectoryKey

	current, ok := state.DirExpandLimits[dirKey]
	if !ok {
		current = DirVisChildLimit
	}
	newLimits := copyIntMap(state.DirExpandLimits)
	newLimits[dirKey] = current + ExpandIncrement
	state.DirExpandLimits = newLimits
	s.pickerStates[id] = state
	return true
}

// ---- internal helpers -------------------------------------------------------

func (s *State) resultFor(id core.CategoryID) core.ScanResult {
	for _, r := range s.results {
		if r.Category.ID == id {
			return r
		}
	}
	return core.ScanResult{}
}

func (s *State) getPickerState(id core.CategoryID) CategoryPickerState {
	existing, ok := s.pickerStates[id]
	if !ok {
		return CategoryPickerState{DirExpandLimits: map[string]int{}}
	}
	return CategoryPickerState{
		Visible:         existing.Visible,
		FileCaret:       existing.FileCaret,
		DirExpandLimits: copyIntMap(existing.DirExpandLimits),
	}
}

func (s *State) setPickerVisible(id core.CategoryID, visible bool) {
	state := s.getPickerState(id)
	state.Visible = visible
	s.pickerStates[id] = state
}

func pathsOf(items []core.CleanableItem) []string {
	paths := make([]string, len(items))
	for i, it := range items {
		paths[i] = it.Path
	}
	return paths
}

func copyBoolSet(src map[core.CategoryID]bool) map[core.CategoryID]bool {
	dst := make(map[core.CategoryID]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyFileMap(src map[core.CategoryID]map[string]bool) map[core.CategoryID]map[string]bool {
	dst := make(map[core.CategoryID]map[string]bool, len(src))
	for k, v := range src {
		dst[k] = copyStringSet(v)
	}
	return dst
}

func copyStringSet(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyIntMap(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
