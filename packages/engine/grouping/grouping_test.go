package grouping

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

const home = "/Users/tester"

func item(path string, size int64) core.CleanableItem {
	return core.CleanableItem{
		Path: path,
		Size: size,
		Name: path[strings.LastIndex(path, "/")+1:],
	}
}

func rowTypes(rows []DisplayRow) string {
	types := make([]string, len(rows))
	for i, r := range rows {
		types[i] = r.Type
	}
	return strings.Join(types, ",")
}

func TestGroupItems_GroupsSortedByLargestSingleFile(t *testing.T) {
	items := []core.CleanableItem{
		item(home+"/Downloads/small.zip", 100),
		item(home+"/Documents/huge.pdf", 10000),
		item(home+"/Movies/medium.mp4", 5000),
		item(home+"/Downloads/big.zip", 900),
		// Music: total 1500 (5×300), max 300.
		// By largest-single-file: Music (300) sorts below Downloads (900).
		// By total: Music (1500) would sort above Downloads (1000).
		// This fixture ensures we're testing largest-single-file, not total ordering.
		item(home+"/Music/song1.mp3", 300),
		item(home+"/Music/song2.mp3", 300),
		item(home+"/Music/song3.mp3", 300),
		item(home+"/Music/song4.mp3", 300),
		item(home+"/Music/song5.mp3", 300),
	}

	rows := GroupItems(items, home, nil, 5, false)

	wantTypes := "directory-header,file,directory-header,file,directory-header,file,file,directory-header,file,file,file,file,file"
	if got := rowTypes(rows); got != wantTypes {
		t.Fatalf("row types = %s, want %s", got, wantTypes)
	}
	// Directory order is driven by the largest SINGLE file in each dir
	// (Documents 10000 > Movies 5000 > Downloads 900 > Music 300), not by dir totals.
	// Music has total 1500 (more than Downloads' 1000), but max 300 (less than Downloads' 900),
	// so it sorts after Downloads under largest-single-file ordering.
	if rows[0].DirectoryKey != home+"/Documents" {
		t.Errorf("first group = %q, want ~/Documents", rows[0].DirectoryKey)
	}
	if rows[2].DirectoryKey != home+"/Movies" {
		t.Errorf("second group = %q, want ~/Movies", rows[2].DirectoryKey)
	}
	if rows[4].DirectoryKey != home+"/Downloads" {
		t.Errorf("third group = %q, want ~/Downloads", rows[4].DirectoryKey)
	}
	if rows[7].DirectoryKey != home+"/Music" {
		t.Errorf("fourth group = %q, want ~/Music", rows[7].DirectoryKey)
	}
	// Files inside a group are size-descending.
	if rows[5].Size != 900 || rows[6].Size != 100 {
		t.Errorf("Downloads files not size-desc: %d then %d", rows[5].Size, rows[6].Size)
	}
	// Music files all same size, should appear after Downloads.
	for i := 8; i < 13; i++ {
		if rows[i].Size != 300 {
			t.Errorf("Music file at row %d has size %d, want 300", i, rows[i].Size)
		}
	}
	// Header row shape.
	h := rows[0]
	if h.DisplayName != "~/Documents" || h.Selectable || h.TotalFilesInDir != 1 {
		t.Errorf("bad header row: %+v", h)
	}
	// File row shape.
	f := rows[1]
	if !f.Selectable || f.Name != "huge.pdf" || f.DisplayName != "huge.pdf" ||
		f.Path != home+"/Documents/huge.pdf" || f.Size != 10000 ||
		f.DirectoryKey != home+"/Documents" || f.TotalFilesInDir != 1 {
		t.Errorf("bad file row: %+v", f)
	}
}

func TestGroupItems_DefaultLimitAndExpandOverride(t *testing.T) {
	dir := home + "/Library/Caches/big-app"
	var items []core.CleanableItem
	for i := 0; i < 8; i++ {
		items = append(items, item(fmt.Sprintf("%s/f%d.dat", dir, i), int64(800-i*100)))
	}

	// Default limit 5 -> header + 5 files + expand hint.
	rows := GroupItems(items, home, nil, 5, false)
	if len(rows) != 7 {
		t.Fatalf("expected 7 rows, got %d (%s)", len(rows), rowTypes(rows))
	}
	if rows[1].Size != 800 || rows[5].Size != 400 {
		t.Errorf("visible files must be the 5 largest: got %d..%d", rows[1].Size, rows[5].Size)
	}
	hint := rows[6]
	if hint.Type != "expand-hint" || hint.HiddenCount != 3 || hint.TotalFilesInDir != 8 ||
		hint.Selectable || hint.DirectoryKey != dir {
		t.Errorf("bad expand-hint row: %+v", hint)
	}

	// Expand override showing everything -> no hint row.
	rows = GroupItems(items, home, map[string]int{dir: 8}, 5, false)
	if len(rows) != 9 {
		t.Fatalf("expanded: expected 9 rows, got %d (%s)", len(rows), rowTypes(rows))
	}
	for _, r := range rows {
		if r.Type == "expand-hint" {
			t.Fatal("fully expanded group must not emit an expand-hint row")
		}
	}

	// Partial expand override (6 of 8 visible).
	rows = GroupItems(items, home, map[string]int{dir: 6}, 5, false)
	if len(rows) != 8 {
		t.Fatalf("partial expand: expected 8 rows, got %d (%s)", len(rows), rowTypes(rows))
	}
	if rows[7].Type != "expand-hint" || rows[7].HiddenCount != 2 {
		t.Errorf("partial expand hint wrong: %+v", rows[7])
	}
}

func TestGroupItems_AbsolutePathsSkipContractionAndTruncation(t *testing.T) {
	longDir := home + "/very-long-folder-name-here/another-long-name/third-level/fourth-level"
	rows := GroupItems([]core.CleanableItem{item(longDir+"/file.zip", 10)}, home, nil, 5, true)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].DisplayName != longDir {
		t.Errorf("absolutePaths header = %q, want raw %q", rows[0].DisplayName, longDir)
	}
	if strings.Contains(rows[0].DisplayName, "~") {
		t.Error("absolutePaths must not contract home to ~")
	}
}

func TestGroupItems_PreservesFullFileNames(t *testing.T) {
	longFileName := strings.Repeat("a", 100) + ".zip"
	rows := GroupItems([]core.CleanableItem{item(home+"/Downloads/"+longFileName, 1000)}, home, nil, 5, false)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// File rows are never truncated by the data layer; only directory paths are.
	if rows[1].DisplayName != longFileName {
		t.Errorf("file DisplayName = %q, want untruncated %q", rows[1].DisplayName, longFileName)
	}
}

func TestGroupItems_Empty(t *testing.T) {
	if rows := GroupItems(nil, home, nil, 5, false); len(rows) != 0 {
		t.Fatalf("expected no rows for no items, got %d", len(rows))
	}
}

func TestTruncateDirectoryPath(t *testing.T) {
	name48 := strings.Repeat("d", 48) // "~/" + 48 chars == exactly 50
	cases := []struct {
		label string
		path  string
		want  string
	}{
		{"home itself contracts to tilde", home, "~"},
		{"short path under home stays intact", home + "/Downloads", "~/Downloads"},
		{"short path outside home stays intact", "/tmp/foo", "/tmp/foo"},
		{"exactly max length stays intact", home + "/" + name48, "~/" + name48},
		{
			"long home path middle-elided keeping last two segments",
			home + "/very-long-folder-name-here/another-long-name/third-level/fourth-level/fifth-level",
			"~/.../fourth-level/fifth-level",
		},
		{
			"long non-home path middle-elided keeping root segment",
			"/Volumes/ExternalDrive/some-deeply/nested/folder-tree/media/movies",
			"/Volumes/.../media/movies",
		},
		{
			"single overlong segment hard-truncated",
			"/" + strings.Repeat("a", 60),
			"/" + strings.Repeat("a", 46) + "...",
		},
		{
			"elided form still too long hard-truncated",
			home + "/x/y/" + strings.Repeat("b", 30) + "/" + strings.Repeat("c", 30),
			"~/.../" + strings.Repeat("b", 30) + "/" + strings.Repeat("c", 10) + "...",
		},
	}
	for _, tc := range cases {
		if got := TruncateDirectoryPath(tc.path, home, 50); got != tc.want {
			t.Errorf("%s: TruncateDirectoryPath(%q) = %q, want %q", tc.label, tc.path, got, tc.want)
		}
	}
}
