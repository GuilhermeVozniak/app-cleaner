package scanners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestTrashScannerScanAndClean(t *testing.T) {
	s, ok := Get("trash")
	if !ok {
		t.Fatal("trash scanner not registered")
	}
	if s.Category() != core.Categories["trash"] {
		t.Fatalf("Category() = %+v, want core.Categories[trash]", s.Category())
	}

	opts := testOptions(t)
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing ~/.Trash must yield empty result without error, got %+v", res)
	}

	trash := filepath.Join(opts.Roots.Home, ".Trash")
	mkFile(t, filepath.Join(trash, "junk.txt"), 64)
	mkFile(t, filepath.Join(trash, "folder", "nested.txt"), 128)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 2 || res.TotalSize != 192 {
		t.Fatalf("scan = %+v; want 2 items totalling 192", res)
	}

	clean := s.Clean(context.Background(), res.Items, false, nil)
	if clean.CleanedItems != 2 || clean.FreedSpace != 192 || len(clean.Errors) != 0 {
		t.Fatalf("clean = %+v; want 2 cleaned, 192 freed, no errors", clean)
	}
	entries, err := os.ReadDir(trash)
	if err != nil || len(entries) != 0 {
		t.Fatalf("trash dir not emptied: entries=%v err=%v", entries, err)
	}
}

func TestTrashScannerEmptyExistingDir(t *testing.T) {
	s, ok := Get("trash")
	if !ok {
		t.Fatal("trash scanner not registered")
	}

	opts := testOptions(t)
	trash := filepath.Join(opts.Roots.Home, ".Trash")
	mkDir(t, trash)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 0 || res.TotalSize != 0 {
		t.Fatalf("existing but empty .Trash dir: got %+v, want 0 items / 0 size / no error", res)
	}
}
