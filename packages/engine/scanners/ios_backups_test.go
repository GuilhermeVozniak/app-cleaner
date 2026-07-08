package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestIOSBackupsScanner(t *testing.T) {
	s, ok := Get("ios-backups")
	if !ok {
		t.Fatal("ios-backups scanner not registered")
	}
	if s.Category() != core.Categories["ios-backups"] {
		t.Fatalf("Category() = %+v, want core.Categories[ios-backups]", s.Category())
	}

	opts := testOptions(t)
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing backup dir must yield empty result without error, got %+v", res)
	}

	backup := filepath.Join(opts.Roots.Home, "Library", "Application Support", "MobileSync", "Backup")
	mkFile(t, filepath.Join(backup, "00008030-001A2B3C4D5E6F78", "Manifest.db"), 512)
	mkFile(t, filepath.Join(backup, "short", "f"), 10) // dir name shorter than 8 chars

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 2 || res.TotalSize != 522 {
		t.Fatalf("scan = %+v; want 2 items totalling 522", res)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	b1, ok1 := byName["iOS Backup: 00008030..."] // first 8 chars of the UDID + "..."
	if !ok1 || !b1.IsDirectory || b1.Size != 512 {
		t.Fatalf("UDID item = %+v ok=%v; want dir of 512 named 'iOS Backup: 00008030...'", b1, ok1)
	}
	if b1.Path != filepath.Join(backup, "00008030-001A2B3C4D5E6F78") {
		t.Fatalf("Path = %q, want original backup dir path", b1.Path)
	}
	if b2, ok2 := byName["iOS Backup: short..."]; !ok2 || b2.Size != 10 { // whole name when < 8 chars
		t.Fatalf("short item = %+v ok=%v; want 'iOS Backup: short...' of size 10", b2, ok2)
	}
}
