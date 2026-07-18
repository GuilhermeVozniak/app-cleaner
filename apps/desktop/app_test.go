package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/maintenance"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/uninstall"
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

func TestGetBackupDetailsHeadless(t *testing.T) {
	home := t.TempDir()
	a := &App{home: home, backupMgr: backup.NewManager(home)}
	// Invalid (outside Root) -> empty, NON-NIL items, no panic.
	d := a.GetBackupDetails(filepath.Join(t.TempDir(), "nope"))
	if d.Items == nil || len(d.Items) != 0 {
		t.Fatalf("invalid path: Details = %+v, want empty non-nil Items", d)
	}
	// Valid session with manifest -> items pass through.
	p := filepath.Join(home, "Library", "Caches", "z.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := a.backupMgr.BackupItems(context.Background(), home, []core.CleanableItem{{Path: p, Size: 3, Name: "z.log"}}, nil)
	d = a.GetBackupDetails(out.SessionDir)
	if !d.FromManifest || len(d.Items) != 1 || d.Items[0].Name != "z.log" {
		t.Fatalf("Details = %+v, want 1 manifest item z.log", d)
	}
}

// ---------------------------------------------------------------------------
// NewApp / GetCategories — trivial headless-safe constructors/getters.
// ---------------------------------------------------------------------------

func TestNewApp_InitializesDefaults(t *testing.T) {
	a := NewApp()
	if a.lastScan == nil {
		t.Fatal("NewApp should initialize lastScan map")
	}
	if _, ok := a.runner.(maintenance.ExecRunner); !ok {
		t.Fatalf("expected runner to default to maintenance.ExecRunner, got %T", a.runner)
	}
	if _, ok := a.elevator.(maintenance.OsaElevator); !ok {
		t.Fatalf("expected elevator to default to maintenance.OsaElevator, got %T", a.elevator)
	}
}

func TestGetCategories_ReturnsAllRegisteredCategories(t *testing.T) {
	got := (&App{}).GetCategories()
	want := core.CategoriesInOrder()
	if len(got) != len(want) {
		t.Fatalf("GetCategories() returned %d categories, want %d", len(got), len(want))
	}
}

// ---------------------------------------------------------------------------
// StartScan/StartClean/StartUninstall/StartTMSnapshotsClear: only the
// synchronous, early-return error branches are headless-safe. The success
// branch of each spawns a goroutine (runScan/runClean/runUninstall/
// runTMClear) that calls wruntime.EventsEmit — with no real Wails frontend
// bound to the context, wails's getEvents() calls log.Fatalf (os.Exit(1)),
// which would kill the whole test binary. See the coverage report's
// exclusion list.
// ---------------------------------------------------------------------------

func TestStartScan_AlreadyRunningReturnsError(t *testing.T) {
	a := &App{scanning: true}
	if err := a.StartScan(nil); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected already-running error, got %v", err)
	}
}

func TestStartScan_NoValidIDsReturnsError(t *testing.T) {
	a := &App{}
	if err := a.StartScan([]string{"no-such-category"}); err == nil || !strings.Contains(err.Error(), "no valid category ids") {
		t.Fatalf("expected no-valid-ids error, got %v", err)
	}
}

func TestStartClean_AlreadyRunningReturnsError(t *testing.T) {
	a := &App{cleaning: true}
	if err := a.StartClean(nil, CleanOptions{}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected already-running error, got %v", err)
	}
}

func TestStartClean_NothingSelectedReturnsError(t *testing.T) {
	a := &App{lastScan: sampleScan()}
	err := a.StartClean(map[string][]string{"downloads": {"/no/such/path"}}, CleanOptions{})
	if err == nil || !strings.Contains(err.Error(), "nothing selected") {
		t.Fatalf("expected nothing-selected error, got %v", err)
	}
}

func TestStartUninstall_AlreadyRunningReturnsError(t *testing.T) {
	a := &App{uninstalling: true}
	if err := a.StartUninstall(nil, false); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected already-running error, got %v", err)
	}
}

func TestStartUninstall_NoMatchingAppsReturnsError(t *testing.T) {
	a := &App{lastApps: []uninstall.AppInfo{{Path: "/Applications/Foo.app"}}}
	err := a.StartUninstall([]string{"/Applications/Bar.app"}, false)
	if err == nil || !strings.Contains(err.Error(), "no matching apps") {
		t.Fatalf("expected no-matching-apps error, got %v", err)
	}
}

func TestStartTMSnapshotsClear_AlreadyRunningReturnsError(t *testing.T) {
	a := &App{maintaining: true}
	if err := a.StartTMSnapshotsClear(); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected already-running error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Cancel* — headless-safe: only touch the mutex-guarded CancelFunc fields.
// ---------------------------------------------------------------------------

func TestCancelScan_CancelsActiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{scanCancel: cancel}
	a.CancelScan()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("CancelScan did not cancel scanCancel")
	}
}

func TestCancelScan_NoOpWhenNil(t *testing.T) {
	(&App{}).CancelScan() // must not panic
}

func TestCancelClean_CancelsActiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{cleanCancel: cancel}
	a.CancelClean()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("CancelClean did not cancel cleanCancel")
	}
}

func TestCancelMaintenance_CancelsActiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{maintCancel: cancel}
	a.CancelMaintenance()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("CancelMaintenance did not cancel maintCancel")
	}
}

// ---------------------------------------------------------------------------
// GetScanResult / GroupItems / GetHome — pure reads off App state.
// ---------------------------------------------------------------------------

func TestGetScanResult_ReturnsStoredResult(t *testing.T) {
	a := &App{lastScan: sampleScan()}
	r := a.GetScanResult("downloads")
	if r.TotalSize != 600 {
		t.Fatalf("expected stored scan result, got %+v", r)
	}
}

func TestGetScanResult_UnknownIDReturnsEmptyCategoryResult(t *testing.T) {
	a := &App{lastScan: map[core.CategoryID]core.ScanResult{}}
	r := a.GetScanResult("downloads")
	if len(r.Items) != 0 {
		t.Fatalf("expected empty result for uncached id, got %+v", r)
	}
}

func TestGroupItems_GroupsLastScanResult(t *testing.T) {
	a := &App{lastScan: sampleScan(), home: "/home/u"}
	rows := a.GroupItems("downloads", nil)
	if len(rows) == 0 {
		t.Fatal("expected at least one grouped row for downloads")
	}
}

func TestGetHome_ReturnsUserHomeDir(t *testing.T) {
	want, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir resolvable on this environment")
	}
	if got := (&App{}).GetHome(); got != want {
		t.Fatalf("GetHome() = %q, want %q", got, want)
	}
}

func TestGetHome_ReturnsEmptyStringOnError(t *testing.T) {
	t.Setenv("HOME", "")
	got := (&App{}).GetHome()
	if got != "" {
		t.Skipf("os.UserHomeDir() resolved a home dir via a non-HOME fallback on this platform (%q); error branch not exercised", got)
	}
}

// ---------------------------------------------------------------------------
// GetConfig / SaveConfig round trip against a temp home.
// ---------------------------------------------------------------------------

func TestSaveConfig_ThenGetConfig_RoundTrips(t *testing.T) {
	home := t.TempDir()
	a := &App{home: home, cfg: config.Default()}
	newCfg := config.Default()
	newCfg.DownloadsDaysOld = 45
	newCfg.Concurrency = 8
	if err := a.SaveConfig(newCfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got := a.GetConfig()
	if got.DownloadsDaysOld != 45 || got.Concurrency != 8 {
		t.Fatalf("GetConfig() after SaveConfig = %+v, want DownloadsDaysOld=45 Concurrency=8", got)
	}
}

func TestSaveConfig_OutOfRangeSnapsToDefaultOnReload(t *testing.T) {
	home := t.TempDir()
	a := &App{home: home, cfg: config.Default()}
	bad := config.Default()
	bad.Concurrency = 999 // out of the 1..16 valid range
	if err := a.SaveConfig(bad); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got := a.GetConfig()
	if got.Concurrency != config.Default().Concurrency {
		t.Fatalf("expected out-of-range Concurrency to snap back to default on reload, got %d", got.Concurrency)
	}
}

// ---------------------------------------------------------------------------
// Backups: ListBackups / CleanOldBackups / RestoreBackup / DeleteBackup
// against a temp home.
// ---------------------------------------------------------------------------

func TestListBackups_CleanOldBackups_RestoreBackup_DeleteBackup(t *testing.T) {
	home := t.TempDir()
	mgr := backup.NewManager(home)
	a := &App{home: home, backupMgr: mgr, cfg: config.Default()}

	if got := a.ListBackups(); len(got) != 0 {
		t.Fatalf("expected no backups yet, got %#v", got)
	}
	if n := a.CleanOldBackups(); n != 0 {
		t.Fatalf("expected 0 removed from an empty/nonexistent backups dir, got %d", n)
	}

	p := filepath.Join(home, "Library", "Caches", "z.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := mgr.BackupItems(context.Background(), home, []core.CleanableItem{{Path: p, Size: 3, Name: "z.log"}}, nil)

	if backups := a.ListBackups(); len(backups) != 1 {
		t.Fatalf("expected 1 backup session, got %#v", backups)
	}

	res := a.RestoreBackup(outcome.SessionDir)
	if res.Restored != 1 || res.Failed != 0 {
		t.Fatalf("RestoreBackup = %+v, want 1 restored, 0 failed", res)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected z.log restored to %s: %v", p, err)
	}

	if err := a.DeleteBackup(outcome.SessionDir); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	if got := a.ListBackups(); len(got) != 0 {
		t.Fatalf("expected backup session removed, got %#v", got)
	}
}

// ---------------------------------------------------------------------------
// GetAppIcon / IsAppRunning / ListApps — headless-safe (real fs/process
// probes, no Wails runtime dependency).
// ---------------------------------------------------------------------------

func TestGetAppIcon_MissingBundleReturnsEmptyString(t *testing.T) {
	a := &App{ctx: context.Background(), iconCacheDir: t.TempDir()}
	got := a.GetAppIcon(filepath.Join(t.TempDir(), "NoSuchApp.app"))
	if got != "" {
		t.Fatalf("GetAppIcon(missing bundle) = %q, want \"\"", got)
	}
}

func TestIsAppRunning_FalseForNonRunningPath(t *testing.T) {
	a := &App{}
	if a.IsAppRunning(filepath.Join(t.TempDir(), "DefinitelyNotRunning.app")) {
		t.Fatal("expected IsAppRunning to be false for a bundle path with no running process")
	}
}

func TestListApps_PopulatesLastApps(t *testing.T) {
	a := &App{ctx: context.Background(), home: t.TempDir()}
	got := a.ListApps()
	if len(a.lastApps) != len(got) {
		t.Fatalf("lastApps (%d) should match ListApps() return (%d)", len(a.lastApps), len(got))
	}
}

// ---------------------------------------------------------------------------
// CheckFDA: diverges from the brief's suggested exclusion — fda.Check(home)
// only reads <home>/Library/Safari (no shell/clipboard/Wails-runtime
// dependency, unlike OpenFDASettings/RevealInFinder/CopyPath). A fresh temp
// home has no such dir, so the probe fails with ENOENT (not EPERM/EACCES)
// and Check deterministically returns nil (the tri-state "unknown" result).
// ---------------------------------------------------------------------------

func TestCheckFDA_UnknownWhenSafariDirMissing(t *testing.T) {
	a := &App{home: t.TempDir()}
	if got := a.CheckFDA(); got != nil {
		t.Fatalf("CheckFDA() = %v, want nil (unknown) for a home with no Library/Safari", *got)
	}
}
