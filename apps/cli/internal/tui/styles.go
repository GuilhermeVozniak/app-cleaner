package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

var (
	styleCaret    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSize     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleSafe     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleModerate = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleRisky    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	//nolint:unused // consumed by filepicker.go, added in the next commit (Task 9 Step 8).
	styleToast = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

const categoryNameWidth = 35

func safetyBadge(level core.SafetyLevel) string {
	switch level {
	case core.SafetySafe:
		return styleSafe.Render("●")
	case core.SafetyModerate:
		return styleModerate.Render("●")
	case core.SafetyRisky:
		return styleRisky.Render("●")
	default:
		return " "
	}
}

func padEnd(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padStart(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

// paginate returns a [start, end) window of size pageSize centered on caret,
// clamped to [0, total). Mirrors the original's centered-pagination math.
func paginate(caret, total, pageSize int) (start, end int) {
	if total <= pageSize {
		return 0, total
	}
	start = caret - pageSize/2
	if start < 0 {
		start = 0
	}
	if start > total-pageSize {
		start = total - pageSize
	}
	end = start + pageSize
	if end > total {
		end = total
	}
	return start, end
}
