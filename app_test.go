package main

import (
	"strings"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func sampleScan() map[core.CategoryID]core.ScanResult {
	return map[core.CategoryID]core.ScanResult{
		"downloads": {
			Category: core.Category{ID: "downloads", Name: "Old Downloads"},
			Items: []core.CleanableItem{
				{Path: "/home/u/Downloads/a.zip", Size: 100, Name: "a.zip"},
				{Path: "/home/u/Downloads/b.zip", Size: 200, Name: "b.zip"},
				{Path: "/home/u/Downloads/c.zip", Size: 300, Name: "c.zip"},
			},
			TotalSize: 600,
		},
		"trash": {
			Category: core.Category{ID: "trash", Name: "Trash"},
			Items: []core.CleanableItem{
				{Path: "/home/u/.Trash/junk", Size: 50, Name: "junk"},
			},
			TotalSize: 50,
		},
	}
}

func TestResolveSelection_MatchesPathsInScanOrder(t *testing.T) {
	sel := map[string][]string{
		"downloads": {
			"/home/u/Downloads/c.zip",
			"/home/u/Downloads/a.zip",
			"/home/u/Downloads/ghost.zip", // not in the scan -> silently dropped
		},
	}
	got := resolveSelection(sampleScan(), sel)
	items, ok := got["downloads"]
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 resolved downloads items, got %#v", got)
	}
	// Resolved items keep scan-result order regardless of selection order.
	if items[0].Path != "/home/u/Downloads/a.zip" || items[1].Path != "/home/u/Downloads/c.zip" {
		t.Errorf("wrong items/order: %#v", items)
	}
	if items[0].Size != 100 || items[1].Size != 300 {
		t.Errorf("sizes not carried over from scan results: %#v", items)
	}
}

func TestResolveSelection_SkipsUnknownCategoryAndEmptySelection(t *testing.T) {
	sel := map[string][]string{
		"no-such-category": {"/home/u/whatever"},
		"downloads":        {},                              // empty selection -> skipped
		"trash":            {"/home/u/.Trash/not-scanned"}, // no path matches -> skipped
	}
	if got := resolveSelection(sampleScan(), sel); len(got) != 0 {
		t.Fatalf("expected empty resolution, got %#v", got)
	}
}

func TestResolveSelection_AllPathsSelectsEverything(t *testing.T) {
	// "Select all" has no special token: the caller passes every item path.
	sel := map[string][]string{
		"downloads": {"/home/u/Downloads/a.zip", "/home/u/Downloads/b.zip", "/home/u/Downloads/c.zip"},
		"trash":     {"/home/u/.Trash/junk"},
	}
	got := resolveSelection(sampleScan(), sel)
	if len(got["downloads"]) != 3 || len(got["trash"]) != 1 {
		t.Fatalf("expected full selection resolved, got %#v", got)
	}
}

func TestSplitByBackup(t *testing.T) {
	resolved := resolveSelection(sampleScan(), map[string][]string{
		"downloads": {"/home/u/Downloads/a.zip", "/home/u/Downloads/b.zip", "/home/u/Downloads/c.zip"},
	})
	moved, remaining := splitByBackup(resolved, []string{"/home/u/Downloads/b.zip"})
	if len(moved["downloads"]) != 2 {
		t.Fatalf("expected 2 moved (backed-up) items, got %#v", moved)
	}
	if moved["downloads"][0].Path != "/home/u/Downloads/a.zip" ||
		moved["downloads"][1].Path != "/home/u/Downloads/c.zip" {
		t.Errorf("wrong moved items: %#v", moved["downloads"])
	}
	if len(remaining["downloads"]) != 1 || remaining["downloads"][0].Path != "/home/u/Downloads/b.zip" {
		t.Fatalf("expected only b.zip left for permanent delete, got %#v", remaining)
	}
}

func TestSplitNeverBackup_RoutesHomebrewAndDockerToCleanPath(t *testing.T) {
	resolved := map[core.CategoryID][]core.CleanableItem{
		"downloads": {{Path: "/home/u/Downloads/a.zip", Size: 100, Name: "a.zip"}},
		"homebrew":  {{Path: "/opt/homebrew/Caches", Size: 500, Name: "Homebrew cache"}},
		"docker":    {{Path: "/var/lib/docker", Size: 900, Name: "Docker data"}},
	}
	backupable, direct := splitNeverBackup(resolved)
	if len(backupable) != 1 || len(backupable["downloads"]) != 1 {
		t.Fatalf("only downloads should be eligible for backup, got %#v", backupable)
	}
	if len(direct) != 2 || len(direct["homebrew"]) != 1 || len(direct["docker"]) != 1 {
		t.Fatalf("homebrew and docker must go straight to the clean path, got %#v", direct)
	}
}

func TestRunMaintenance_UnknownTask(t *testing.T) {
	res := (&App{}).RunMaintenance("defrag")
	if res.Success {
		t.Fatal("unknown task must not succeed")
	}
	if !strings.Contains(res.Error, "unknown") {
		t.Errorf("error should name the unknown task, got %q", res.Error)
	}
}
