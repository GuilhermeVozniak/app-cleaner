// Package output provides pure text and JSON formatting shared by every
// apps/cli command and the interactive TUI's non-interactive fallbacks.
// Nothing here touches a terminal, a file, or $HOME — every function takes
// its inputs as arguments so it stays table-test friendly. Colorizing text
// for a real terminal (lipgloss styles) is the caller's job; this package
// only produces the plain strings and JSON payloads to colorize/print.
package output

import (
	"strings"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// DryRunPrefix is the literal prefix the original CLI prints in cyan before
// any line describing an action a --dry-run run would have taken
// (mac-cleaner-cli src/commands/clean.ts / uninstall.ts:
// chalk.cyan('[DRY RUN] ...')). Coloring is the caller's responsibility.
const DryRunPrefix = "[DRY RUN]"

// Rule renders a horizontal rule of n "─" characters, matching the original
// CLI's '─'.repeat(n) (the 50/60/70-char rules in scan/clean/uninstall
// output). Coloring (chalk.dim in the original) is the caller's
// responsibility.
func Rule(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("─", n)
}

// ContractHome abbreviates path to "~" when it is under home, mirroring the
// original CLI's contractPath (mac-cleaner-cli src/utils/paths.ts). It is a
// byte-for-byte parity port: a bare prefix match, not a path-segment-aware
// check (so e.g. home="/Users/mac" would also contract "/Users/mac2/x" —
// the same behavior the original has). home is passed in explicitly (never
// read from os.UserHomeDir here) so callers and tests stay $HOME-safe.
func ContractHome(path, home string) string {
	if home == "" || !strings.HasPrefix(path, home) {
		return path
	}
	return "~" + strings.TrimPrefix(path, home)
}

// TruncateName truncates name to maxLength runes, preserving its extension
// and placing an ellipsis in the middle of the basename. Exact port of the
// original CLI's truncateFileName (mac-cleaner-cli src/utils/paths.ts;
// porting-notes.json §utils/paths) — used for the file picker's
// FILE_NAME_WIDTH=35 column and any other fixed-width name display.
func TruncateName(name string, maxLength int) string {
	runes := []rune(name)
	if len(runes) <= maxLength {
		return name
	}

	lastDot := strings.LastIndex(name, ".")
	var ext, base string
	if lastDot > 0 {
		ext = name[lastDot:]
		base = name[:lastDot]
	} else {
		ext = ""
		base = name
	}
	baseRunes := []rune(base)
	extRunes := []rune(ext)

	const ellipsis = "..."
	available := maxLength - len(extRunes) - len(ellipsis)

	if available <= 0 {
		cut := maxLength - len(ellipsis)
		if cut < 0 {
			cut = 0
		}
		if cut > len(runes) {
			cut = len(runes)
		}
		return string(runes[:cut]) + ellipsis
	}

	firstLen := (available + 1) / 2 // ceil(available/2)
	lastLen := available / 2        // floor(available/2)

	first := string(baseRunes[:firstLen])
	last := string(baseRunes[len(baseRunes)-lastLen:])
	return first + ellipsis + last + ext
}

// ErrnoBreakdown re-exports fsx.AggregateFailures so every apps/cli command
// formats errno breakdowns ("Failed to remove N items (32 EPERM, 8 EACCES)")
// through this package instead of reaching into the engine directly. The
// aggregation logic itself lives in fsx and is deliberately NOT
// reimplemented here — fsx already has full test coverage for it
// (TestAggregateFailures in packages/engine/fsx/remove_test.go).
func ErrnoBreakdown(failures []fsx.RemoveFailure) []string {
	return fsx.AggregateFailures(failures)
}
