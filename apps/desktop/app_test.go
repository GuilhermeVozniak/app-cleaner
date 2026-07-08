package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/maintenance"
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
		"downloads":        {},                             // empty selection -> skipped
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
	moved, remaining := splitByBackup(resolved,
		[]string{"/home/u/Downloads/a.zip", "/home/u/Downloads/c.zip"},
		[]string{"/home/u/Downloads/b.zip"})
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

// TestSplitByBackup_CancelledItemsExcludedEntirely covers a mid-batch cancel:
// an item that is in NEITHER outcome.Moved NOR outcome.NotBackedUp (the
// backup loop stopped before reaching it) must never be credited as
// cleaned/freed, and must never be handed to a scanner for deletion either —
// it was never touched on disk.
func TestSplitByBackup_CancelledItemsExcludedEntirely(t *testing.T) {
	resolved := resolveSelection(sampleScan(), map[string][]string{
		"downloads": {"/home/u/Downloads/a.zip", "/home/u/Downloads/b.zip", "/home/u/Downloads/c.zip"},
	})
	// Only a.zip was actually moved before cancellation; b.zip and c.zip were
	// never reached (not moved, not notBackedUp).
	moved, remaining := splitByBackup(resolved,
		[]string{"/home/u/Downloads/a.zip"},
		nil)
	if len(moved["downloads"]) != 1 || moved["downloads"][0].Path != "/home/u/Downloads/a.zip" {
		t.Fatalf("expected only a.zip credited as moved, got %#v", moved)
	}
	if len(remaining["downloads"]) != 0 {
		t.Fatalf("uncancelled/untouched items must not be handed to the scanner for deletion, got %#v", remaining)
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

// ---------------------------------------------------------------------------
// resolveScanIDs (StartScan's default-selection/filter branch, extracted as a
// pure function since StartScan itself needs a real Wails runtime context to
// run headless — see runScan's wruntime.EventsEmit calls).
// ---------------------------------------------------------------------------

func TestResolveScanIDs_EmptySelectsAllRegisteredCategories(t *testing.T) {
	got := resolveScanIDs(nil)
	want := core.CategoriesInOrder()
	if len(got) != len(want) {
		t.Fatalf("resolveScanIDs(nil) returned %d ids, want %d (all registered categories)", len(got), len(want))
	}
	for i, cat := range want {
		if got[i] != string(cat.ID) {
			t.Errorf("resolveScanIDs(nil)[%d] = %q, want %q (order must match core.CategoriesInOrder())", i, got[i], cat.ID)
		}
	}
}

func TestResolveScanIDs_FiltersToKnownIDsOnly(t *testing.T) {
	got := resolveScanIDs([]string{"trash", "no-such-category"})
	if len(got) != 1 || got[0] != "trash" {
		t.Fatalf("expected only the valid id to survive filtering, got %#v", got)
	}
}

// ---------------------------------------------------------------------------
// RunMaintenance dispatch (dns/purge), using fake Runner/Elevator injected
// via App's runner/elevator fields so the real dscacheutil/purge binaries
// are never executed.
// ---------------------------------------------------------------------------

type fakeMaintRunner struct {
	out   string
	err   error
	calls []string
}

func (f *fakeMaintRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.calls = append(f.calls, bin)
	return f.out, f.err
}

type fakeMaintElevator struct {
	out     string
	err     error
	scripts []string
}

func (f *fakeMaintElevator) RunElevated(ctx context.Context, script string) (string, error) {
	f.scripts = append(f.scripts, script)
	return f.out, f.err
}

func TestRunMaintenance_DNS_DispatchesToFlushDNS(t *testing.T) {
	e := &fakeMaintElevator{}
	app := &App{ctx: context.Background(), elevator: e}
	res := app.RunMaintenance("dns")
	if !res.Success || res.Message != "DNS cache flushed successfully" {
		t.Fatalf("result = %+v, want maintenance.FlushDNS's success result", res)
	}
	if len(e.scripts) != 1 {
		t.Fatalf("RunMaintenance(\"dns\") did not dispatch to FlushDNS's elevator, scripts = %#v", e.scripts)
	}
}

func TestRunMaintenance_DNS_PropagatesFailure(t *testing.T) {
	e := &fakeMaintElevator{err: errors.New("execution error: User canceled. (-128)")}
	app := &App{ctx: context.Background(), elevator: e}
	res := app.RunMaintenance("dns")
	if res.Success || !res.RequiresAdmin {
		t.Fatalf("result = %+v, want FlushDNS's failure/cancel result surfaced unchanged", res)
	}
}

func TestRunMaintenance_Purge_DispatchesToFreePurgeableSpace(t *testing.T) {
	r := &fakeMaintRunner{}
	e := &fakeMaintElevator{}
	app := &App{ctx: context.Background(), runner: r, elevator: e}
	res := app.RunMaintenance("purge")
	if !res.Success || res.Message != "Purgeable space freed successfully" {
		t.Fatalf("result = %+v, want maintenance.FreePurgeable's success result", res)
	}
	if len(r.calls) != 1 || r.calls[0] != "/usr/sbin/purge" {
		t.Fatalf("RunMaintenance(\"purge\") did not dispatch to FreePurgeable's runner, calls = %#v", r.calls)
	}
	if len(e.scripts) != 0 {
		t.Fatal("plain purge succeeded, must not have elevated")
	}
}

func TestRunMaintenance_Purge_ElevatesOnPermissionFailure(t *testing.T) {
	r := &fakeMaintRunner{err: errors.New("purge: Operation not permitted")}
	e := &fakeMaintElevator{}
	app := &App{ctx: context.Background(), runner: r, elevator: e}
	res := app.RunMaintenance("purge")
	if !res.Success || res.Message != "Purgeable space freed successfully" {
		t.Fatalf("result = %+v, want FreePurgeable's elevated-fallback success", res)
	}
	if len(e.scripts) != 1 || e.scripts[0] != "/usr/sbin/purge" {
		t.Fatalf("expected one elevated purge call, got %#v", e.scripts)
	}
}

// compile-time reassurance that App's fields satisfy the maintenance
// interfaces without an adapter (mirrors NewApp's defaults).
var (
	_ maintenance.Runner   = (*fakeMaintRunner)(nil)
	_ maintenance.Elevator = (*fakeMaintElevator)(nil)
)
