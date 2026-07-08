package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestTempFilesScanner(t *testing.T) {
	s, ok := Get("temp-files")
	if !ok {
		t.Fatal("temp-files scanner not registered")
	}
	if s.Category() != core.Categories["temp-files"] {
		t.Fatalf("Category() = %+v, want core.Categories[temp-files]", s.Category())
	}

	opts := testOptions(t)
	// Neither Roots.Tmp nor Roots.VarFolders exists yet => empty, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing roots must yield empty result without error, got %+v", res)
	}

	// /tmp children (a file and a 0-byte dir — both listed).
	mkFile(t, filepath.Join(opts.Roots.Tmp, "scratch.txt"), 100)
	mkDir(t, filepath.Join(opts.Roots.Tmp, "com.example.tmp"))
	// var/folders two-level layout: only children of an existing .../T dir count.
	mkFile(t, filepath.Join(opts.Roots.VarFolders, "ab", "c1", "T", "cache.db"), 250)
	mkDir(t, filepath.Join(opts.Roots.VarFolders, "ab", "c2"))          // no T dir => skipped
	mkFile(t, filepath.Join(opts.Roots.VarFolders, "zz", "c3", "T"), 0) // T is a FILE => skipped
	mkFile(t, filepath.Join(opts.Roots.VarFolders, "stray.txt"), 50)    // level-1 file => ReadDir fails, silently skipped

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	if len(res.Items) != 3 || res.TotalSize != 350 {
		t.Fatalf("got %d items total %d (%+v); want 3 items totalling 350", len(res.Items), res.TotalSize, res.Items)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if f := byName["scratch.txt"]; f.Size != 100 {
		t.Fatalf("scratch.txt = %+v, want size 100", f)
	}
	if d, ok := byName["com.example.tmp"]; !ok || !d.IsDirectory || d.Size != 0 {
		t.Fatalf("com.example.tmp = %+v ok=%v; 0-byte dir must still be listed", d, ok)
	}
	c := byName["cache.db"]
	if c.Size != 250 || c.Path != filepath.Join(opts.Roots.VarFolders, "ab", "c1", "T", "cache.db") {
		t.Fatalf("cache.db = %+v; want child of the T dir, size 250", c)
	}
}
