// Package output provides pure text and JSON formatting shared by every
// apps/cli command and the interactive TUI's non-interactive fallbacks.
// Nothing here touches a terminal, a file, or $HOME — every function takes
// its inputs as arguments so it stays table-test friendly. Colorizing text
// for a real terminal (lipgloss styles) is the caller's job; this package
// only produces the plain strings and JSON payloads to colorize/print.
package output

import (
	"encoding/json"
	"strings"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
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

// ScanJSONItem is one item entry in the `scan --json --verbose` output.
type ScanJSONItem struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// ScanJSONCategory is one category entry in the `scan --json` output. Items
// is present only when the scan was requested with --verbose — omitted
// entirely (not an empty array) otherwise, matching the original CLI's
// optional `items?` field (porting-notes.json §commands, scan.ts
// toJsonSummary).
type ScanJSONCategory struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Group       string         `json:"group"`
	SafetyLevel string         `json:"safetyLevel"`
	TotalSize   int64          `json:"totalSize"`
	ItemCount   int            `json:"itemCount"`
	Items       []ScanJSONItem `json:"items,omitempty"`
}

// ScanJSON is the `scan --json` payload shape.
type ScanJSON struct {
	TotalSize  int64              `json:"totalSize"`
	TotalItems int                `json:"totalItems"`
	Categories []ScanJSONCategory `json:"categories"`
}

// EncodeScanJSON builds the `scan --json` payload from an engine
// core.ScanSummary, reproducing the original CLI's toJsonSummary shape
// exactly (porting-notes.json §commands, mac-cleaner-cli
// src/commands/scan.ts). Categories with zero items are omitted; per-item
// paths/sizes are included only when verbose is true.
func EncodeScanJSON(summary core.ScanSummary, verbose bool) ScanJSON {
	out := ScanJSON{
		TotalSize:  summary.TotalSize,
		TotalItems: summary.TotalItems,
		Categories: []ScanJSONCategory{},
	}
	for _, r := range summary.Results {
		if len(r.Items) == 0 {
			continue
		}
		cat := ScanJSONCategory{
			ID:          string(r.Category.ID),
			Name:        r.Category.Name,
			Group:       string(r.Category.Group),
			SafetyLevel: string(r.Category.SafetyLevel),
			TotalSize:   r.TotalSize,
			ItemCount:   len(r.Items),
		}
		if verbose {
			cat.Items = make([]ScanJSONItem, len(r.Items))
			for i, it := range r.Items {
				cat.Items[i] = ScanJSONItem{Path: it.Path, Size: it.Size}
			}
		}
		out.Categories = append(out.Categories, cat)
	}
	return out
}

// MarshalScanJSON renders EncodeScanJSON's result with a 2-space indent,
// matching the original's JSON.stringify(obj, null, 2).
func MarshalScanJSON(summary core.ScanSummary, verbose bool) ([]byte, error) {
	return json.MarshalIndent(EncodeScanJSON(summary, verbose), "", "  ")
}

// CleanJSONResult is one category's result in the `clean --json` output.
type CleanJSONResult struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	CleanedItems int      `json:"cleanedItems"`
	FreedSpace   int64    `json:"freedSpace"`
	Errors       []string `json:"errors,omitempty"`
}

// CleanJSON is the `clean --json` payload shape. The original CLI
// (mac-cleaner-cli src/commands/clean.ts) has no documented --json flag in
// porting-notes.json; this shape is a deliberate extrapolation that mirrors
// EncodeScanJSON's id/name convention and reuses the engine's own
// totalFreedSpace/totalCleanedItems/totalErrors field names verbatim so the
// two JSON outputs read consistently for scripting (design spec §8.4).
type CleanJSON struct {
	Results           []CleanJSONResult `json:"results"`
	TotalFreedSpace   int64             `json:"totalFreedSpace"`
	TotalCleanedItems int               `json:"totalCleanedItems"`
	TotalErrors       int               `json:"totalErrors"`
}

// EncodeCleanJSON builds the `clean --json` payload from an engine
// core.CleanSummary. A per-category result is omitted only when it cleaned
// zero items AND reported zero errors (nothing happened for it).
func EncodeCleanJSON(summary core.CleanSummary) CleanJSON {
	out := CleanJSON{
		Results:           []CleanJSONResult{},
		TotalFreedSpace:   summary.TotalFreedSpace,
		TotalCleanedItems: summary.TotalCleanedItems,
		TotalErrors:       summary.TotalErrors,
	}
	for _, r := range summary.Results {
		if r.CleanedItems == 0 && len(r.Errors) == 0 {
			continue
		}
		out.Results = append(out.Results, CleanJSONResult{
			ID:           string(r.Category.ID),
			Name:         r.Category.Name,
			CleanedItems: r.CleanedItems,
			FreedSpace:   r.FreedSpace,
			Errors:       r.Errors,
		})
	}
	return out
}

// MarshalCleanJSON renders EncodeCleanJSON's result with a 2-space indent.
func MarshalCleanJSON(summary core.CleanSummary) ([]byte, error) {
	return json.MarshalIndent(EncodeCleanJSON(summary), "", "  ")
}
