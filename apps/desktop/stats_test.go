package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "stats.json")
	s := ActivityStats{TotalCleanedBytes: 1234, CleanRuns: 2, ScanRuns: 5, AppsUninstalled: 1, LastCleanAt: "2026-01-02T03:04:05Z"}
	if err := saveStats(s, path); err != nil {
		t.Fatal(err)
	}
	got := loadStats(path)
	if got != s {
		t.Fatalf("round trip mismatch: %+v != %+v", got, s)
	}
}

func TestLoadStatsMissingOrCorrupt(t *testing.T) {
	if got := loadStats(filepath.Join(t.TempDir(), "nope.json")); got != (ActivityStats{}) {
		t.Fatalf("missing file should load zero stats, got %+v", got)
	}
	p := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(p, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadStats(p); got != (ActivityStats{}) {
		t.Fatalf("corrupt file should load zero stats, got %+v", got)
	}
}

func TestRecordClean(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	s := recordClean(ActivityStats{TotalCleanedBytes: 100, CleanRuns: 1}, 50, 3, now)
	if s.TotalCleanedBytes != 150 || s.TotalCleanedItems != 3 || s.CleanRuns != 2 {
		t.Fatalf("unexpected: %+v", s)
	}
	if s.LastCleanAt != "2026-08-16T12:00:00Z" {
		t.Fatalf("LastCleanAt = %s", s.LastCleanAt)
	}
}

func TestReadDiskUsage(t *testing.T) {
	u := readDiskUsage(t.TempDir())
	if u.Total <= 0 || u.Free < 0 || u.Used < 0 || u.Used > u.Total {
		t.Fatalf("implausible usage: %+v", u)
	}
	if got := readDiskUsage("/definitely/not/a/path"); got != (DiskUsage{}) {
		t.Fatalf("missing path should be zero-value, got %+v", got)
	}
}
