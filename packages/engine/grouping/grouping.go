// Package grouping turns a flat list of scanned items into the display-row
// structure the UI renders: items grouped by parent directory, directories
// with the largest single file first, with per-directory expand/collapse
// pagination. Mirrors the CLI contract in mac-cleaner-cli/src/utils/grouping.ts
// and truncateDirectoryPath in mac-cleaner-cli/src/utils/paths.ts.
package grouping

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// DisplayRow is one row of the grouped item list. The JSON shape is consumed
// verbatim by the frontend (lib/types.ts DisplayRow).
type DisplayRow struct {
	Type            string `json:"type"`                  // "directory-header" | "file" | "expand-hint"
	DirectoryKey    string `json:"directoryKey"`          // absolute dir path (grouping/expand key)
	DisplayName     string `json:"displayName"`           // header: truncated dir path; file: basename
	Path            string `json:"path,omitempty"`        // file rows only
	Size            int64  `json:"size,omitempty"`        // file rows only
	Name            string `json:"name,omitempty"`        // file rows only
	HiddenCount     int    `json:"hiddenCount,omitempty"` // expand-hint rows only
	TotalFilesInDir int    `json:"totalFilesInDir"`
	Selectable      bool   `json:"selectable"`
}

// GroupItems groups items by parent directory (filepath.Dir) and flattens the
// result into display rows:
//   - files inside a group are sorted by size descending (stable),
//   - groups are sorted by their largest single file descending (stable;
//     first-appearance order breaks ties, like the CLI's Map + stable sort),
//   - each group emits a directory-header row, then min(expand[dir] or
//     defaultLimit, len(files)) file rows, then an expand-hint row when files
//     remain hidden.
//
// home is used for "~" contraction in header display names; expand maps an
// absolute dir path to a per-directory visible-row override (nil is fine);
// defaultLimit <= 0 falls back to 5; absolutePaths=true renders raw absolute
// dir paths in headers (no contraction, no truncation).
func GroupItems(items []core.CleanableItem, home string, expand map[string]int, defaultLimit int, absolutePaths bool) []DisplayRow {
	if defaultLimit <= 0 {
		defaultLimit = 5
	}

	type dirGroup struct {
		dir     string
		files   []core.CleanableItem
		largest int64
	}

	// 1. Group by parent dir, preserving first-appearance order for stable ties.
	index := make(map[string]int)
	groups := []dirGroup{}
	for _, it := range items {
		dir := filepath.Dir(it.Path)
		i, ok := index[dir]
		if !ok {
			i = len(groups)
			index[dir] = i
			groups = append(groups, dirGroup{dir: dir})
		}
		groups[i].files = append(groups[i].files, it)
	}

	// 2. Files within each group: size descending.
	for i := range groups {
		files := groups[i].files
		sort.SliceStable(files, func(a, b int) bool { return files[a].Size > files[b].Size })
		groups[i].largest = files[0].Size // every group has >= 1 file by construction
	}

	// 3. Groups: largest single file descending.
	sort.SliceStable(groups, func(a, b int) bool { return groups[a].largest > groups[b].largest })

	// 4. Flatten into rows.
	rows := []DisplayRow{}
	for _, g := range groups {
		limit := defaultLimit
		if override, ok := expand[g.dir]; ok {
			limit = override
		}
		visible := limit
		if visible > len(g.files) {
			visible = len(g.files)
		}
		if visible < 0 {
			visible = 0
		}

		displayName := g.dir
		if !absolutePaths {
			displayName = TruncateDirectoryPath(g.dir, home, 50)
		}
		rows = append(rows, DisplayRow{
			Type:            "directory-header",
			DirectoryKey:    g.dir,
			DisplayName:     displayName,
			TotalFilesInDir: len(g.files),
			Selectable:      false,
		})
		for i := 0; i < visible; i++ {
			f := g.files[i]
			base := filepath.Base(f.Path)
			rows = append(rows, DisplayRow{
				Type:            "file",
				DirectoryKey:    g.dir,
				DisplayName:     base,
				Name:            base,
				Path:            f.Path,
				Size:            f.Size,
				TotalFilesInDir: len(g.files),
				Selectable:      true,
			})
		}
		if hidden := len(g.files) - visible; hidden > 0 {
			rows = append(rows, DisplayRow{
				Type:            "expand-hint",
				DirectoryKey:    g.dir,
				HiddenCount:     hidden,
				TotalFilesInDir: len(g.files),
				Selectable:      false,
			})
		}
	}
	return rows
}

// TruncateDirectoryPath renders a directory path for display: the home prefix
// contracts to "~", and paths longer than maxLen are middle-elided to
// "<first>/.../<last>/<two>" (keeping the last two segments), falling back to a
// hard "..."-suffixed cut when even the elided form (or a path with <= 2
// segments) exceeds maxLen. Ported from the CLI's truncateDirectoryPath.
func TruncateDirectoryPath(p, home string, maxLen int) string {
	display := p
	if home != "" {
		switch {
		case p == home:
			display = "~"
		case strings.HasPrefix(p, home+"/"):
			display = "~" + p[len(home):]
		}
	}
	if len(display) <= maxLen {
		return display
	}
	if maxLen < 4 {
		return display // degenerate maxLen: no room for "...", refuse to slice
	}

	parts := []string{}
	for _, part := range strings.Split(display, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) <= 2 {
		return display[:maxLen-3] + "..."
	}

	first := parts[0]
	if first != "~" {
		first = "/" + first
	}
	truncated := first + "/.../" + strings.Join(parts[len(parts)-2:], "/")
	if len(truncated) <= maxLen {
		return truncated
	}
	return truncated[:maxLen-3] + "..."
}
