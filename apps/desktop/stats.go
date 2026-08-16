package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// DiskUsage is the boot-volume capacity snapshot for the dashboard health card.
type DiskUsage struct {
	Total int64 `json:"total"`
	Free  int64 `json:"free"`
	Used  int64 `json:"used"`
}

// readDiskUsage stats the volume containing path. Zero-value on failure —
// the dashboard renders "unavailable" rather than erroring.
func readDiskUsage(path string) DiskUsage {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskUsage{}
	}
	bs := int64(st.Bsize)
	total := int64(st.Blocks) * bs
	free := int64(st.Bavail) * bs
	return DiskUsage{Total: total, Free: free, Used: total - free}
}

// ActivityStats is the lifetime tally shown on the Smart Care dashboard.
// Persisted as JSON next to config.json; all writes go through App.mu.
type ActivityStats struct {
	TotalCleanedBytes int64  `json:"totalCleanedBytes"`
	TotalCleanedItems int    `json:"totalCleanedItems"`
	CleanRuns         int    `json:"cleanRuns"`
	ScanRuns          int    `json:"scanRuns"`
	AppsUninstalled   int    `json:"appsUninstalled"`
	LastCleanAt       string `json:"lastCleanAt"` // RFC3339, "" = never
}

func statsPath(home string) string {
	return filepath.Join(home, "Library", "Application Support", "AppCleaner", "stats.json")
}

// loadStats mirrors config.Load's philosophy: never fails, corrupt = zero.
func loadStats(path string) ActivityStats {
	var s ActivityStats
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 100_000 {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

func saveStats(s ActivityStats, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// recordClean folds one finished clean run into s. now is injected for tests.
func recordClean(s ActivityStats, freedBytes int64, cleanedItems int, now time.Time) ActivityStats {
	s.TotalCleanedBytes += freedBytes
	s.TotalCleanedItems += cleanedItems
	s.CleanRuns++
	s.LastCleanAt = now.UTC().Format(time.RFC3339)
	return s
}
