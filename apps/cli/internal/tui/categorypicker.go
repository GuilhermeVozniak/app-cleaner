package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

const categoryPageSize = 15

// CategoryPickerModel is the flat checkbox list over scan results
// (porting-notes §commands' checkbox fallback UI; pageSize 15).
// Keymap: space toggle, a all, i invert, enter confirm.
type CategoryPickerModel struct {
	Results  []core.ScanResult
	Done     bool
	Aborted  bool
	caret    int
	selected map[core.CategoryID]bool
}

func NewCategoryPickerModel(results []core.ScanResult) CategoryPickerModel {
	return CategoryPickerModel{Results: results, selected: map[core.CategoryID]bool{}}
}

func (m CategoryPickerModel) Init() tea.Cmd { return nil }

func (m CategoryPickerModel) isSelected(i int) bool {
	if i < 0 || i >= len(m.Results) {
		return false
	}
	return m.selected[m.Results[i].Category.ID]
}

// Chosen returns selected category IDs in Results order.
func (m CategoryPickerModel) Chosen() []core.CategoryID {
	out := make([]core.CategoryID, 0, len(m.selected))
	for _, r := range m.Results {
		if m.selected[r.Category.ID] {
			out = append(out, r.Category.ID)
		}
	}
	return out
}

func (m CategoryPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.caret > 0 {
			m.caret--
		}
	case "down", "j":
		if m.caret < len(m.Results)-1 {
			m.caret++
		}
	case " ":
		if len(m.Results) > 0 {
			id := m.Results[m.caret].Category.ID
			if m.selected[id] {
				delete(m.selected, id)
			} else {
				m.selected[id] = true
			}
		}
	case "a":
		all := len(m.selected) == len(m.Results) && len(m.Results) > 0
		if all {
			m.selected = map[core.CategoryID]bool{}
		} else {
			m.selected = map[core.CategoryID]bool{}
			for _, r := range m.Results {
				m.selected[r.Category.ID] = true
			}
		}
	case "i":
		next := map[core.CategoryID]bool{}
		for _, r := range m.Results {
			if !m.selected[r.Category.ID] {
				next[r.Category.ID] = true
			}
		}
		m.selected = next
	case "enter", "right":
		m.Done = true
		return m, tea.Quit
	case "ctrl+c":
		m.Aborted = true
		return m, tea.Quit
	}
	return m, nil
}

func (m CategoryPickerModel) View() string {
	var b strings.Builder
	start, end := paginate(m.caret, len(m.Results), categoryPageSize)
	for i := start; i < end; i++ {
		r := m.Results[i]
		checkbox := styleDim.Render("◯")
		if m.isSelected(i) {
			checkbox = styleSafe.Render("◉")
		}
		caret := "  "
		if i == m.caret {
			caret = styleCaret.Render("> ")
		}
		name := padEnd(r.Category.Name, categoryNameWidth)
		size := padStart(core.FormatSize(r.TotalSize), 10)
		fmt.Fprintf(&b, "%s%s %s %s %s\n", caret, checkbox, safetyBadge(r.Category.SafetyLevel), name, styleSize.Render(size))
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("space: toggle | a: all | i: invert | enter: confirm"))
	return b.String()
}
