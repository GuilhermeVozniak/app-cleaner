// Package output provides pure text and JSON formatting shared by every
// apps/cli command and the interactive TUI's non-interactive fallbacks.
// Nothing here touches a terminal, a file, or $HOME — every function takes
// its inputs as arguments so it stays table-test friendly. Colorizing text
// for a real terminal (lipgloss styles) is the caller's job; this package
// only produces the plain strings and JSON payloads to colorize/print.
package output

import "strings"

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
