package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestBrowserCacheScanner(t *testing.T) {
	s, ok := Get("browser-cache")
	if !ok {
		t.Fatal("browser-cache scanner not registered")
	}
	if s.Category() != core.Categories["browser-cache"] {
		t.Fatalf("Category() = %+v, want core.Categories[browser-cache]", s.Category())
	}

	opts := testOptions(t)
	// No browser dirs exist => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("no browsers must yield empty result without error, got %+v", res)
	}

	home := opts.Roots.Home
	// Chrome: exists with content (whole tree is the item, recursive size).
	mkFile(t, filepath.Join(home, "Library", "Caches", "Google", "Chrome", "Default", "Cache", "data_0"), 4096)
	// Safari: exists but EMPTY — must still be listed (no size>0 gate).
	mkDir(t, filepath.Join(home, "Library", "Caches", "com.apple.Safari"))
	// Firefox and Arc: absent => omitted.

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items (%+v), want 2 (Chrome, Safari)", len(res.Items), res.Items)
	}
	chrome, safari := res.Items[0], res.Items[1] // fixed candidate order
	if chrome.Name != "Google Chrome Cache" || chrome.Size != 4096 || !chrome.IsDirectory {
		t.Fatalf("chrome = %+v; want 'Google Chrome Cache', size 4096, IsDirectory", chrome)
	}
	if chrome.Path != filepath.Join(home, "Library", "Caches", "Google", "Chrome") {
		t.Fatalf("chrome.Path = %q, want the whole Chrome caches tree", chrome.Path)
	}
	if safari.Name != "Safari Cache" || safari.Size != 0 || !safari.IsDirectory {
		t.Fatalf("safari = %+v; want 0-byte 'Safari Cache' dir still included", safari)
	}
	if chrome.ModifiedAt == nil || safari.ModifiedAt == nil {
		t.Fatal("browser items must carry ModifiedAt")
	}
	if res.TotalSize != 4096 {
		t.Fatalf("TotalSize = %d, want 4096", res.TotalSize)
	}
}
