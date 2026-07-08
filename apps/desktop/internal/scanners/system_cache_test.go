package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestSystemCacheScanner(t *testing.T) {
	s, ok := Get("system-cache")
	if !ok {
		t.Fatal("system-cache scanner not registered")
	}
	if s.Category() != core.Categories["system-cache"] {
		t.Fatalf("Category() = %+v, want core.Categories[system-cache]", s.Category())
	}

	opts := testOptions(t)
	// Missing ~/Library/Caches => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 || res.TotalSize != 0 {
		t.Fatalf("missing dir must yield empty result without error, got %+v", res)
	}

	cache := filepath.Join(opts.Roots.Home, "Library", "Caches")
	mkFile(t, filepath.Join(cache, "com.example.app", "blob.bin"), 300)
	mkFile(t, filepath.Join(cache, "single.txt"), 200)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	if len(res.Items) != 2 || res.TotalSize != 500 {
		t.Fatalf("got %d items, total %d; want 2 items, total 500", len(res.Items), res.TotalSize)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if d := byName["com.example.app"]; !d.IsDirectory || d.Size != 300 {
		t.Fatalf("dir child = %+v, want IsDirectory with recursive size 300", d)
	}
	if f := byName["single.txt"]; f.IsDirectory || f.Size != 200 {
		t.Fatalf("file child = %+v, want plain file of size 200", f)
	}
}
