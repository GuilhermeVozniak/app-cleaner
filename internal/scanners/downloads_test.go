package scanners

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestDownloadsScannerAgeBoundary(t *testing.T) {
	s, ok := Get("downloads")
	if !ok {
		t.Fatal("downloads scanner not registered")
	}
	if s.Category() != core.Categories["downloads"] {
		t.Fatalf("Category() = %+v, want core.Categories[downloads]", s.Category())
	}

	opts := testOptions(t) // config.Default() => DownloadsDaysOld = 30
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing ~/Downloads must yield empty result without error, got %+v", res)
	}

	dl := filepath.Join(opts.Roots.Home, "Downloads")
	oldFile := filepath.Join(dl, "old.zip")
	newFile := filepath.Join(dl, "new.zip")
	mkFile(t, oldFile, 1000)
	mkFile(t, newFile, 2000)
	now := time.Now()
	if err := os.Chtimes(oldFile, now, now.Add(-31*24*time.Hour)); err != nil { // 31 days old => included
		t.Fatal(err)
	}
	if err := os.Chtimes(newFile, now, now.Add(-29*24*time.Hour)); err != nil { // 29 days old => excluded
		t.Fatal(err)
	}

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	if len(res.Items) != 1 || res.Items[0].Name != "old.zip" || res.Items[0].Size != 1000 {
		t.Fatalf("items = %+v, want only old.zip (1000 bytes)", res.Items)
	}
	if res.TotalSize != 1000 {
		t.Fatalf("TotalSize = %d, want 1000", res.TotalSize)
	}
}

func TestDownloadsScannerEmptyExistingDir(t *testing.T) {
	s, ok := Get("downloads")
	if !ok {
		t.Fatal("downloads scanner not registered")
	}

	opts := testOptions(t)
	dl := filepath.Join(opts.Roots.Home, "Downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatal(err)
	}

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("existing but empty Downloads dir: got %+v, want 0 items, no error", res)
	}
}

func TestDownloadsScannerUsesConfiguredThreshold(t *testing.T) {
	s, _ := Get("downloads")
	opts := testOptions(t)
	opts.Cfg.DownloadsDaysOld = 10 // must flow through, not a hardcoded 30
	dl := filepath.Join(opts.Roots.Home, "Downloads")
	f := filepath.Join(dl, "two-weeks.zip")
	mkFile(t, f, 500)
	now := time.Now()
	if err := os.Chtimes(f, now, now.Add(-15*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	res := s.Scan(context.Background(), opts)
	if len(res.Items) != 1 || res.Items[0].Name != "two-weeks.zip" {
		t.Fatalf("15-day-old file with a 10-day threshold must be included, got %+v", res.Items)
	}
}
