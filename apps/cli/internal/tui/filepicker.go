package tui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GuilhermeVozniak/app-cleaner/apps/cli/internal/selection"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/grouping"
)

const (
	filesPageSize    = 6
	dirVisChildLimit = 5
	fileNameWidth    = FILE_NAME_WIDTH
)

// FILE_NAME_WIDTH is the contract's binding constant name; exported so
// Task 10 (or a future non-interactive fallback) can reuse the exact value.
const FILE_NAME_WIDTH = 35

// copiedMsg carries the result of an async pbcopy so the toast can clear
// itself after 2s without blocking the Update loop.
type copiedMsg struct {
	path string
	err  error
}
type clearToastMsg struct{}

// FilePickerModel is the dual-pane (categories left, files right) drill-down.
// Keymap — categories pane: space/a/i/right/enter; files pane:
// space/a/i/d/m/h/c/left|backspace/enter.
type FilePickerModel struct {
	Results             []core.ScanResult
	CategoriesWithFiles map[core.CategoryID]bool
	Home                string
	AbsolutePaths       bool
	Done                bool
	store               *pickerStore
	copyStatus          string
}

func NewFilePickerModel(results []core.ScanResult, categoriesWithFiles map[core.CategoryID]bool, home string, absolutePaths bool) FilePickerModel {
	return FilePickerModel{
		Results:             results,
		CategoriesWithFiles: categoriesWithFiles,
		Home:                home,
		AbsolutePaths:       absolutePaths,
		store:               newPickerStore(results, categoriesWithFiles),
	}
}

func (m FilePickerModel) Init() tea.Cmd { return nil }

func (m FilePickerModel) Result() (map[core.CategoryID]bool, map[core.CategoryID]map[string]bool) {
	return m.store.Result()
}

func (m FilePickerModel) currentResult() core.ScanResult {
	return m.Results[m.store.CategoryCaret()]
}

func (m FilePickerModel) rowsFor(id core.CategoryID) []grouping.DisplayRow {
	for _, r := range m.Results {
		if r.Category.ID == id {
			return grouping.GroupItems(r.Items, m.Home, m.store.DirExpandLimits(id), dirVisChildLimit, m.AbsolutePaths)
		}
	}
	return nil
}

func (m FilePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearToastMsg:
		m.copyStatus = ""
		return m, nil
	case copiedMsg:
		if msg.err != nil {
			m.copyStatus = fmt.Sprintf("Failed to copy: %v", msg.err)
		} else {
			m.copyStatus = fmt.Sprintf("Copied: %s", msg.path)
		}
		return m, clearToastAfter(2 * time.Second)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func clearToastAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return clearToastMsg{} })
}

func (m FilePickerModel) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "enter" {
		m.Done = true
		return m, tea.Quit
	}
	if key.String() == "ctrl+c" {
		m.Done = true
		return m, tea.Quit
	}

	if m.store.ActivePane() == selection.PaneCategories {
		switch key.String() {
		case "up", "k":
			m.store.MoveCategoryCaret(-1)
		case "down", "j":
			m.store.MoveCategoryCaret(1)
		case " ":
			m.store.ToggleCategory(m.currentResult().Category.ID)
		case "a":
			m.store.SelectAllCategories()
		case "i":
			m.store.InvertCategories()
		case "right":
			id := m.currentResult().Category.ID
			if m.CategoriesWithFiles[id] {
				m.store.EnterFiles(id, m.rowsFor(id))
			}
		}
		return m, nil
	}

	// PaneFiles
	id, ok := m.store.ActiveCategory()
	if !ok {
		m.store.Back()
		return m, nil
	}
	rows := m.rowsFor(id)
	switch key.String() {
	case "up", "k":
		m.store.MoveFileCaret(id, -1, rows)
	case "down", "j":
		m.store.MoveFileCaret(id, 1, rows)
	case " ":
		m.store.ToggleFile(id, rows)
	case "a":
		m.store.SelectAllFiles(id, pathsOf(rows))
	case "i":
		m.store.InvertFiles(id, pathsOf(rows))
	case "d":
		m.store.ToggleDirectory(id, rows)
	case "m", "right":
		m.store.ExpandCurrentDir(id, rows)
	case "h":
		m.store.CollapseCurrentDir(id, rows)
	case "c":
		caret := m.store.FileCaret(id)
		if caret >= 0 && caret < len(rows) && rows[caret].Path != "" {
			dir := filepath.Dir(rows[caret].Path)
			m.copyStatus = fmt.Sprintf("Copied: %s", dir)
			return m, copyToClipboard(dir)
		}
	case "left", "backspace":
		m.store.Back()
	}
	return m, nil
}

func pathsOf(rows []grouping.DisplayRow) []string {
	var out []string
	for _, r := range rows {
		if r.Type == "file" {
			out = append(out, r.Path)
		}
	}
	return out
}

// copyToClipboard shells out to pbcopy (macOS-only product; contract §Deps).
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("pbcopy")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return copiedMsg{path: text, err: err}
		}
		if err := cmd.Start(); err != nil {
			return copiedMsg{path: text, err: err}
		}
		_, _ = stdin.Write([]byte(text))
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			return copiedMsg{path: text, err: err}
		}
		return copiedMsg{path: text}
	}
}

func (m FilePickerModel) View() string {
	var b strings.Builder
	for i, r := range m.Results {
		isSelected := m.store.IsCategorySelected(r.Category.ID)
		checkbox := styleDim.Render("◯")
		if isSelected {
			checkbox = styleSafe.Render("◉")
		}
		caretMark := "  "
		if i == m.store.CategoryCaret() && m.store.ActivePane() == selection.PaneCategories {
			caretMark = styleCaret.Render("> ")
		}
		name := padEnd(r.Category.Name, categoryNameWidth)
		size := padStart(core.FormatSize(r.TotalSize), 10)
		fmt.Fprintf(&b, "%s%s %s %s\n", caretMark, checkbox, name, styleSize.Render(size))

		ps := m.store.PickerState(r.Category.ID)
		if ps.Visible || ps.Active {
			m.renderFiles(&b, r.Category.ID, ps.Active)
		}
	}
	b.WriteString("\n")
	if m.store.ActivePane() == selection.PaneCategories {
		b.WriteString(styleDim.Render("space: toggle | a: all | i: invert | →: see files | enter: confirm"))
	} else if m.copyStatus != "" {
		b.WriteString(styleToast.Render(m.copyStatus))
	} else {
		b.WriteString(styleDim.Render("space: toggle | a: all | d: select dir | i: invert | m: expand | h: collapse | c: copy path | ←/backspace: back | enter: confirm"))
	}
	return b.String()
}

func (m FilePickerModel) renderFiles(b *strings.Builder, id core.CategoryID, active bool) {
	rows := m.rowsFor(id)
	caret := m.store.FileCaret(id)
	start, end := paginate(caret, len(rows), filesPageSize)
	for j := start; j < end; j++ {
		row := rows[j]
		isCaretRow := active && j == caret
		caretMark := "  "
		if isCaretRow {
			caretMark = styleCaret.Render("> ")
		}
		var line string
		switch row.Type {
		case "directory-header":
			line = fmt.Sprintf("    %s%s (%d)", caretMark, row.DisplayName, row.TotalFilesInDir)
			if !active {
				line = styleDim.Render(line)
			}
		case "expand-hint":
			line = fmt.Sprintf("    %s+%d files", caretMark, row.HiddenCount)
		case "file":
			selectedMark := styleDim.Render("○")
			if m.store.IsFileSelected(id, row.Path) {
				selectedMark = styleSafe.Render("●")
			}
			name := padEnd(truncateFileName(row.DisplayName, fileNameWidth), fileNameWidth)
			size := padStart(core.FormatSize(row.Size), 10)
			line = fmt.Sprintf("    %s%s %s %s", caretMark, selectedMark, name, size)
			if !active {
				line = styleDim.Render(line)
			}
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
}

// truncateFileName middle-elides a filename longer than width, preserving
// the extension where possible (kept simple: hard cut with a trailing
// ellipsis, matching the original's fallback path for the common case).
func truncateFileName(name string, width int) string {
	if len(name) <= width {
		return name
	}
	if width < 4 {
		return name[:width]
	}
	return name[:width-3] + "..."
}
