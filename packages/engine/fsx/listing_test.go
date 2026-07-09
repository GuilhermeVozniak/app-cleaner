package fsx

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestGetSize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.txt"), 100)
	writeFile(t, filepath.Join(root, "big.bin"), 2000)
	mustMkdir(t, filepath.Join(root, "sub"))
	writeFile(t, filepath.Join(root, "sub", "a.txt"), 300)
	writeFile(t, filepath.Join(root, "sub", "b.txt"), 200)
	target := filepath.Join(root, "big.bin")
	mustSymlink(t, target, filepath.Join(root, "link"))
	// POSIX: a symlink's lstat size is the byte length of its target path.
	linkSize := int64(len(target))

	cases := []struct {
		name string
		path string
		want int64
	}{
		{"regular file", filepath.Join(root, "small.txt"), 100},
		{"directory is recursive logical sum", filepath.Join(root, "sub"), 500},
		{"symlink is link size, target never followed", filepath.Join(root, "link"), linkSize},
		{"whole tree counts the link, not its target", root, 100 + 2000 + 500 + linkSize},
		{"missing path is 0", filepath.Join(root, "nope"), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GetSize(c.path); got != c.want {
				t.Errorf("GetSize(%s) = %d, want %d", c.path, got, c.want)
			}
		})
	}
}

func TestGetSizeUnreadableDirIsZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits are ignored")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mustMkdir(t, locked)
	writeFile(t, filepath.Join(locked, "f"), 100)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if got := GetSize(locked); got != 0 {
		t.Errorf("GetSize(unreadable dir) = %d, want 0", got)
	}
}

func TestGetDirectoryItems(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "file.txt"), 100)
	mustMkdir(t, filepath.Join(root, "sub"))
	writeFile(t, filepath.Join(root, "sub", "inner.txt"), 400)
	target := filepath.Join(root, "file.txt")
	mustSymlink(t, target, filepath.Join(root, "link"))

	mtime := time.Date(2024, 6, 1, 10, 30, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(root, "file.txt"), mtime, mtime); err != nil {
		t.Fatal(err)
	}

	items := GetDirectoryItems(root)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(items), items)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range items {
		byName[it.Name] = it
	}

	f := byName["file.txt"]
	if f.Size != 100 || f.IsDirectory || f.Path != filepath.Join(root, "file.txt") {
		t.Errorf("file.txt item wrong: %+v", f)
	}
	if f.ModifiedAt == nil || !f.ModifiedAt.Equal(mtime) {
		t.Errorf("file.txt ModifiedAt = %v, want %v (lstat mtime)", f.ModifiedAt, mtime)
	}
	sub := byName["sub"]
	if sub.Size != 400 || !sub.IsDirectory {
		t.Errorf("sub item wrong (want fully-sized directory): %+v", sub)
	}
	link := byName["link"]
	if link.Size != int64(len(target)) || link.IsDirectory {
		t.Errorf("link item wrong (want lstat link size, not a directory): %+v", link)
	}

	if got := GetDirectoryItems(filepath.Join(root, "missing")); got == nil || len(got) != 0 {
		t.Errorf("missing dir: got %v, want empty non-nil slice", got)
	}
}

func TestGetItemsMinAgeDays(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	files := map[string]time.Time{
		"old.txt":        now.Add(-40 * 24 * time.Hour),
		"fresh.txt":      now.Add(-10 * 24 * time.Hour),
		"edge-under.txt": now.Add(-708 * time.Hour), // 29.5 days: fractional compare must reject
		"edge-over.txt":  now.Add(-732 * time.Hour), // 30.5 days: must pass
	}
	for name, mtime := range files {
		p := filepath.Join(dir, name)
		writeFile(t, p, 10)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	got := GetItems(dir, ItemFilter{MinAgeDays: 30})
	names := map[string]bool{}
	for _, it := range got {
		names[it.Name] = true
	}
	for _, want := range []string{"old.txt", "edge-over.txt"} {
		if !names[want] {
			t.Errorf("expected %s to pass the 30-day filter, got %v", want, names)
		}
	}
	for _, reject := range []string{"fresh.txt", "edge-under.txt"} {
		if names[reject] {
			t.Errorf("expected %s to be filtered out (age < 30 days, fractional-day compare)", reject)
		}
	}
}

func TestGetItemsMinSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "small.txt"), 100)
	writeFile(t, filepath.Join(dir, "big.bin"), 2000)
	mustMkdir(t, filepath.Join(dir, "bigdir"))
	writeFile(t, filepath.Join(dir, "bigdir", "content"), 1500)

	got := GetItems(dir, ItemFilter{MinSize: 1000})
	names := map[string]bool{}
	for _, it := range got {
		names[it.Name] = true
	}
	if len(got) != 2 || !names["big.bin"] || !names["bigdir"] {
		t.Errorf("MinSize filter: got %v, want exactly big.bin and bigdir (dirs sized recursively)", names)
	}
}

func TestGetItemsNoFilterReturnsAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), 1)
	writeFile(t, filepath.Join(dir, "b"), 2)
	if got := GetItems(dir, ItemFilter{}); len(got) != 2 {
		t.Errorf("no filter: got %d items, want 2", len(got))
	}
}
