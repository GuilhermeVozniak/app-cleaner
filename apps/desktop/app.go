package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fda"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/grouping"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/loginitems"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/maintenance"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/scanners"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/spacelens"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/uninstall"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound bridge between the React frontend and the engine
// packages under internal/. It is the ONLY place (with main.go) that touches
// the Wails runtime.
type App struct {
	ctx context.Context

	mu           sync.Mutex
	scanCancel   context.CancelFunc
	cleanCancel  context.CancelFunc
	maintCancel  context.CancelFunc
	scanning     bool
	cleaning     bool
	maintaining  bool
	uninstalling bool

	lastScan     map[core.CategoryID]core.ScanResult
	lastApps     []uninstall.AppInfo
	cfg          config.Config
	backupMgr    *backup.Manager
	home         string
	iconCacheDir string
	stats        ActivityStats

	// runner/elevator back every maintenance call (RunMaintenance,
	// StartTMSnapshotsClear) and GetAppIcon. Injectable fields (rather than
	// constructing maintenance.ExecRunner{}/OsaElevator inline) let tests
	// substitute fakes so maintenance tests never shell out to the real
	// dscacheutil/purge/tmutil binaries.
	runner   maintenance.Runner
	elevator maintenance.Elevator

	// updateDmgPath is set by DownloadUpdate and consumed by InstallUpdate /
	// OpenDownloadedUpdate (selfupdate.go). Guarded by mu.
	updateDmgPath string
}

// CleanOptions is the options payload for StartClean.
type CleanOptions struct {
	DryRun bool `json:"dryRun"`
	Backup bool `json:"backup"`
}

func NewApp() *App {
	runner := maintenance.ExecRunner{}
	return &App{
		lastScan: make(map[core.CategoryID]core.ScanResult),
		runner:   runner,
		elevator: maintenance.OsaElevator{Runner: runner},
	}
}

// startup is wired to options.App.OnStartup in main.go. Not bound to JS
// (unexported).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}
	a.home = home
	a.cfg = config.Load(config.DefaultPath(home))
	a.backupMgr = backup.NewManager(home)
	a.stats = loadStats(statsPath(home))
	// Icon cache for GetAppIcon (uninstall.AppIcon stores converted PNGs here).
	a.iconCacheDir = filepath.Join(os.TempDir(), "appcleaner-icons")
	_ = os.MkdirAll(a.iconCacheDir, 0o755)
	// Backup retention cleanup on launch (spec §8).
	retention := a.cfg.BackupRetentionDays
	go func() {
		defer func() { _ = recover() }()
		a.backupMgr.CleanOld(retention)
	}()
}

// ---------------------------------------------------------------------------
// Categories & scanning
// ---------------------------------------------------------------------------

func (a *App) GetCategories() []core.Category {
	return core.CategoriesInOrder()
}

// resolveScanIDs maps a frontend category-id selection onto the ids to
// scan: empty input selects every registered category (in
// core.CategoriesInOrder()); a non-empty input is filtered down to known
// category ids, with unknown ids silently dropped. Pure function
// (unit-tested without the Wails runtime), mirroring resolveSelection.
func resolveScanIDs(raw []string) []string {
	if len(raw) == 0 {
		var ids []string
		for _, cat := range core.CategoriesInOrder() {
			ids = append(ids, string(cat.ID))
		}
		return ids
	}
	var ids []string
	for _, r := range raw {
		if _, ok := scanners.Get(core.CategoryID(r)); ok { // unknown ids silently skipped
			ids = append(ids, r)
		}
	}
	return ids
}

// StartScan starts scanning the given category ids (all 16 when empty) in a
// goroutine and returns immediately. Progress is streamed as scan:progress,
// completion as scan:done.
func (a *App) StartScan(rawIDs []string) error {
	a.mu.Lock()
	if a.scanning {
		a.mu.Unlock()
		return errors.New("a scan is already running")
	}
	var ids []core.CategoryID
	for _, raw := range resolveScanIDs(rawIDs) {
		ids = append(ids, core.CategoryID(raw))
	}
	if len(ids) == 0 {
		a.mu.Unlock()
		return errors.New("no valid category ids to scan")
	}
	a.scanning = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.scanCancel = cancel
	cfg := a.cfg
	a.mu.Unlock()

	go a.runScan(ctx, ids, cfg)
	return nil
}

func (a *App) runScan(ctx context.Context, ids []core.CategoryID, cfg config.Config) {
	var summary core.ScanSummary
	defer func() {
		a.mu.Lock()
		a.scanning = false
		a.scanCancel = nil
		a.mu.Unlock()
		if r := recover(); r != nil {
			wruntime.EventsEmit(a.ctx, "scan:done", map[string]any{
				"summary":   summary,
				"cancelled": false,
				"error":     fmt.Sprintf("panic: %v", r),
			})
		}
	}()

	opts := scanners.Options{
		Roots:  scanners.DefaultRoots(),
		Cfg:    cfg,
		Runner: &scanners.ExecRunner{},
	}
	summary = scanners.RunScans(ctx, ids, opts, cfg.Concurrency,
		func(completed, total int, r core.ScanResult) {
			wruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{
				"completed":  completed,
				"total":      total,
				"categoryId": string(r.Category.ID),
				"totalSize":  r.TotalSize,
				"itemCount":  len(r.Items),
				"error":      r.Error,
			})
		})

	a.mu.Lock()
	for _, res := range summary.Results {
		a.lastScan[res.Category.ID] = res
	}
	a.stats.ScanRuns++
	a.persistStatsLocked()
	a.mu.Unlock()

	wruntime.EventsEmit(a.ctx, "scan:done", map[string]any{
		"summary":   summary,
		"cancelled": ctx.Err() != nil,
	})
}

func (a *App) CancelScan() {
	a.mu.Lock()
	if a.scanCancel != nil {
		a.scanCancel()
	}
	a.mu.Unlock()
}

func (a *App) GetScanResult(id string) core.ScanResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r, ok := a.lastScan[core.CategoryID(id)]; ok {
		return r
	}
	return core.ScanResult{Category: core.Categories[core.CategoryID(id)]}
}

// GroupItems returns the display rows for a category's last scan result,
// grouped per the CLI contract (defaultLimit 5, home-contracted paths).
func (a *App) GroupItems(id string, expand map[string]int) []grouping.DisplayRow {
	r := a.GetScanResult(id)
	return grouping.GroupItems(r.Items, a.home, expand, 5, false)
}

// GetHome returns the current user's home directory ("" if unresolvable).
// Frontend display helper: lets item rows contract $HOME to "~".
func (a *App) GetHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// ---------------------------------------------------------------------------
// Cleaning
// ---------------------------------------------------------------------------

// resolveSelection maps a frontend selection (categoryID -> selected item
// paths) onto the items stored from the last scan. Pure function (unit-tested
// without the Wails runtime). Unknown categories, empty path lists, and paths
// that do not match a scanned item are silently skipped. "Select all" is the
// caller passing every item path. Scan-result item order is preserved.
func resolveSelection(lastScan map[core.CategoryID]core.ScanResult, selection map[string][]string) map[core.CategoryID][]core.CleanableItem {
	out := make(map[core.CategoryID][]core.CleanableItem)
	for rawID, paths := range selection {
		if len(paths) == 0 {
			continue
		}
		id := core.CategoryID(rawID)
		result, ok := lastScan[id]
		if !ok {
			continue
		}
		want := make(map[string]bool, len(paths))
		for _, p := range paths {
			want[p] = true
		}
		var items []core.CleanableItem
		for _, it := range result.Items {
			if want[it.Path] {
				items = append(items, it)
			}
		}
		if len(items) > 0 {
			out[id] = items
		}
	}
	return out
}

// splitByBackup partitions the resolved selection using a completed
// backup.BackupOutcome: items in movedPaths were actually renamed off disk by
// backup.BackupItems and only need summary credit; items in notBackedUp
// (non-$HOME item, cross-volume EXDEV, or any rename error) were left in
// place and must be permanently deleted by their scanner. An item in NEITHER
// list was never reached before the backup pass was cancelled (ctx
// cancellation) — it is untouched on disk and must be excluded entirely: not
// credited as cleaned/freed, and not handed to the scanner for deletion.
func splitByBackup(resolved map[core.CategoryID][]core.CleanableItem, movedPaths, notBackedUp []string) (moved, remaining map[core.CategoryID][]core.CleanableItem) {
	movedSet := make(map[string]bool, len(movedPaths))
	for _, p := range movedPaths {
		movedSet[p] = true
	}
	skip := make(map[string]bool, len(notBackedUp))
	for _, p := range notBackedUp {
		skip[p] = true
	}
	moved = make(map[core.CategoryID][]core.CleanableItem)
	remaining = make(map[core.CategoryID][]core.CleanableItem)
	for id, items := range resolved {
		for _, it := range items {
			switch {
			case movedSet[it.Path]:
				moved[id] = append(moved[id], it)
			case skip[it.Path]:
				remaining[id] = append(remaining[id], it)
			default:
				// Never reached before the backup pass was cancelled: leave
				// untouched — no credit, no deletion.
			}
		}
	}
	return moved, remaining
}

// neverBackup lists categories whose items are cleaned by their own external
// tool (brew cleanup / docker system prune) and are therefore NEVER backed up
// (spec §8): the "items" are not restorable file moves.
var neverBackup = map[core.CategoryID]bool{
	"homebrew": true,
	"docker":   true,
}

// splitNeverBackup partitions the resolved selection BEFORE the backup pass:
// neverBackup categories go straight to the clean path (direct); everything
// else is eligible for backup.BackupItems (backupable). Pure function.
func splitNeverBackup(resolved map[core.CategoryID][]core.CleanableItem) (backupable, direct map[core.CategoryID][]core.CleanableItem) {
	backupable = make(map[core.CategoryID][]core.CleanableItem)
	direct = make(map[core.CategoryID][]core.CleanableItem)
	for id, items := range resolved {
		if neverBackup[id] {
			direct[id] = items
		} else {
			backupable[id] = items
		}
	}
	return backupable, direct
}

// StartClean resolves the selection against the last scan and starts cleaning
// in a goroutine. With opts.Backup (and not dry-run) items are moved into a
// backup session first (backup:progress); items the backup refused
// (NotBackedUp) are then permanently deleted per category (clean:progress).
func (a *App) StartClean(selection map[string][]string, opts CleanOptions) error {
	a.mu.Lock()
	if a.cleaning {
		a.mu.Unlock()
		return errors.New("a clean is already running")
	}
	resolved := resolveSelection(a.lastScan, selection)
	if len(resolved) == 0 {
		a.mu.Unlock()
		return errors.New("nothing selected to clean")
	}
	a.cleaning = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.cleanCancel = cancel
	a.mu.Unlock()

	go a.runClean(ctx, resolved, opts)
	return nil
}

func (a *App) runClean(ctx context.Context, resolved map[core.CategoryID][]core.CleanableItem, opts CleanOptions) {
	var summary core.CleanSummary
	notBackedUp := []string{}
	defer func() {
		a.mu.Lock()
		a.cleaning = false
		a.cleanCancel = nil
		a.mu.Unlock()
		if r := recover(); r != nil {
			wruntime.EventsEmit(a.ctx, "clean:done", map[string]any{
				"summary":     summary,
				"notBackedUp": notBackedUp,
				"cancelled":   false,
				"error":       fmt.Sprintf("panic: %v", r),
			})
		}
	}()

	remaining := resolved
	movedByCat := map[core.CategoryID][]core.CleanableItem{}
	if opts.Backup && !opts.DryRun {
		// Spec §8: homebrew/docker items are cleaned via brew cleanup /
		// docker system prune and are NEVER backed up — route them straight
		// to the clean path; only the rest is offered to the backup manager.
		backupable, direct := splitNeverBackup(resolved)
		var all []core.CleanableItem
		for _, cat := range core.CategoriesInOrder() { // deterministic order
			all = append(all, backupable[cat.ID]...)
		}
		var outcome backup.BackupOutcome
		if len(all) > 0 { // no empty session when only brew/docker are selected
			outcome = a.backupMgr.BackupItems(ctx, a.home, all,
				func(current, total int, item core.CleanableItem) {
					wruntime.EventsEmit(a.ctx, "backup:progress", map[string]any{
						"current":  current,
						"total":    total,
						"itemName": item.Name,
					})
				})
		}
		notBackedUp = outcome.NotBackedUp
		if notBackedUp == nil {
			notBackedUp = []string{}
		}
		movedByCat, remaining = splitByBackup(backupable, outcome.Moved, outcome.NotBackedUp)
		for id, items := range direct {
			remaining[id] = items
		}
	}

	for _, cat := range core.CategoriesInOrder() { // stable category order
		items := remaining[cat.ID]
		moved := movedByCat[cat.ID]
		if len(items) == 0 && len(moved) == 0 {
			continue
		}
		sc, ok := scanners.Get(cat.ID)
		if !ok {
			continue
		}
		// Errors non-nil: a category whose items were ALL moved to backup skips
		// sc.Clean, and a nil slice would reach the frontend as "errors":null.
		res := core.CleanResult{Category: cat, Errors: []string{}}
		if len(items) > 0 {
			res = sc.Clean(ctx, items, opts.DryRun,
				func(current, total int, item core.CleanableItem) {
					wruntime.EventsEmit(a.ctx, "clean:progress", map[string]any{
						"current":    current,
						"total":      total,
						"categoryId": string(cat.ID),
						"itemName":   item.Name,
					})
				})
		}
		// Items moved into the backup session are off their original location:
		// credit them as cleaned/freed.
		for _, m := range moved {
			res.CleanedItems++
			res.FreedSpace += m.Size
		}
		summary.Results = append(summary.Results, res)
		summary.TotalFreedSpace += res.FreedSpace
		summary.TotalCleanedItems += res.CleanedItems
		summary.TotalErrors += len(res.Errors)
	}

	if !opts.DryRun {
		a.mu.Lock()
		a.stats = recordClean(a.stats, summary.TotalFreedSpace, summary.TotalCleanedItems, time.Now())
		a.persistStatsLocked()
		a.mu.Unlock()
	}

	// Contract: notBackedUp is a TOP-LEVEL sibling of summary in clean:done.
	wruntime.EventsEmit(a.ctx, "clean:done", map[string]any{
		"summary":     summary,
		"notBackedUp": notBackedUp,
		"cancelled":   ctx.Err() != nil,
	})
}

func (a *App) CancelClean() {
	a.mu.Lock()
	if a.cleanCancel != nil {
		a.cleanCancel()
	}
	a.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Uninstaller
// ---------------------------------------------------------------------------

func (a *App) ListApps() []uninstall.AppInfo {
	appDirs := []string{"/Applications", filepath.Join(a.home, "Applications")}
	apps := uninstall.ListApps(a.ctx, appDirs, a.home)
	a.mu.Lock()
	a.lastApps = apps
	a.mu.Unlock()
	return apps
}

// StartUninstall uninstalls the apps at the given bundle paths (matched
// against the last ListApps result) in a goroutine, streaming
// uninstall:progress / uninstall:done. Paths are the unique key — matching by
// display name would collapse two same-named apps installed in different
// locations (e.g. /Applications vs ~/Applications) into one entry.
func (a *App) StartUninstall(paths []string, dryRun bool) error {
	a.mu.Lock()
	if a.uninstalling {
		a.mu.Unlock()
		return errors.New("an uninstall is already running")
	}
	byPath := make(map[string]uninstall.AppInfo, len(a.lastApps))
	for _, info := range a.lastApps {
		byPath[info.Path] = info
	}
	var apps []uninstall.AppInfo
	for _, p := range paths {
		if info, ok := byPath[p]; ok {
			apps = append(apps, info)
		}
	}
	if len(apps) == 0 {
		a.mu.Unlock()
		return errors.New("no matching apps selected")
	}
	a.uninstalling = true
	a.mu.Unlock()

	go a.runUninstall(apps, dryRun)
	return nil
}

func (a *App) runUninstall(apps []uninstall.AppInfo, dryRun bool) {
	var sum uninstall.Summary
	defer func() {
		a.mu.Lock()
		a.uninstalling = false
		a.mu.Unlock()
		if r := recover(); r != nil {
			wruntime.EventsEmit(a.ctx, "uninstall:done", map[string]any{
				"uninstalled": sum.Uninstalled,
				"freedSpace":  sum.FreedSpace,
				"errors":      []string{},
				"cancelled":   false,
				"error":       fmt.Sprintf("panic: %v", r),
			})
		}
	}()

	sum = uninstall.Uninstall(a.ctx, apps, dryRun,
		func(current, total int, appName string) {
			wruntime.EventsEmit(a.ctx, "uninstall:progress", map[string]any{
				"current": current,
				"total":   total,
				"appName": appName,
			})
		})
	if sum.Errors == nil {
		sum.Errors = []string{}
	}
	if !dryRun && sum.Uninstalled > 0 {
		a.mu.Lock()
		a.stats.AppsUninstalled += sum.Uninstalled
		a.stats.TotalCleanedBytes += sum.FreedSpace
		a.persistStatsLocked()
		a.mu.Unlock()
	}
	wruntime.EventsEmit(a.ctx, "uninstall:done", map[string]any{
		"uninstalled": sum.Uninstalled,
		"freedSpace":  sum.FreedSpace,
		"errors":      sum.Errors,
		"cancelled":   a.ctx.Err() != nil,
	})
}

func (a *App) IsAppRunning(path string) bool {
	return uninstall.IsAppRunning(path)
}

// GetAppIcon lazily resolves an app's icon as base64 PNG ("" on any failure).
// Extraction (CFBundleIconFile), .icns → PNG conversion (/usr/bin/sips), and
// per-bundle-path caching live in uninstall.AppIcon; the cache dir is created
// in startup. maintenance.ExecRunner structurally satisfies uninstall.Runner
// (identical method set), so it is reused directly here — no adapter needed.
func (a *App) GetAppIcon(path string) string {
	return uninstall.AppIcon(a.ctx, maintenance.ExecRunner{}, path, a.iconCacheDir)
}

// ---------------------------------------------------------------------------
// Maintenance
// ---------------------------------------------------------------------------

// RunMaintenance runs the short synchronous tasks ("dns" | "purge"). Time
// Machine snapshot clearing is long-running and uses StartTMSnapshotsClear.
// a.runner/a.elevator are the ONE shared, tested Runner/Elevator used by
// every maintenance call and by GetAppIcon: maintenance.ExecRunner (real
// exec.CommandContext) and maintenance.OsaElevator, which enforces the 120s
// elevatedTimeout (internal/maintenance/elevate.go) for every
// admin-privileged call. Duplicating these locally previously bypassed that
// timeout; tests inject fakes via these fields instead of constructing them
// inline (see NewApp).
func (a *App) RunMaintenance(task string) maintenance.Result {
	switch task {
	case "dns":
		return maintenance.FlushDNS(a.ctx, a.elevator)
	case "purge":
		return maintenance.FreePurgeable(a.ctx, a.runner, a.elevator)
	default:
		return maintenance.Result{Success: false, Error: fmt.Sprintf("unknown maintenance task: %q", task)}
	}
}

func (a *App) StartTMSnapshotsClear() error {
	a.mu.Lock()
	if a.maintaining {
		a.mu.Unlock()
		return errors.New("a maintenance task is already running")
	}
	a.maintaining = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.maintCancel = cancel
	a.mu.Unlock()

	go a.runTMClear(ctx)
	return nil
}

func (a *App) runTMClear(ctx context.Context) {
	defer func() {
		a.mu.Lock()
		a.maintaining = false
		a.maintCancel = nil
		a.mu.Unlock()
		if r := recover(); r != nil {
			wruntime.EventsEmit(a.ctx, "maintenance:done", map[string]any{
				"result": maintenance.Result{Success: false, Error: fmt.Sprintf("panic: %v", r)},
			})
		}
	}()

	result := maintenance.ClearTMSnapshots(ctx, a.runner, a.elevator,
		func(done, total int, date, errMsg string) {
			wruntime.EventsEmit(a.ctx, "maintenance:progress", map[string]any{
				"done":  done,
				"total": total,
				"date":  date,
				"error": errMsg,
			})
		})
	wruntime.EventsEmit(a.ctx, "maintenance:done", map[string]any{"result": result})
}

func (a *App) CancelMaintenance() {
	a.mu.Lock()
	if a.maintCancel != nil {
		a.maintCancel()
	}
	a.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Config, backups, FDA, shell helpers
// ---------------------------------------------------------------------------

func (a *App) GetConfig() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) SaveConfig(c config.Config) error {
	path := config.DefaultPath(a.home)
	if err := config.Save(c, path); err != nil {
		return err
	}
	// Re-load so out-of-range fields snap back to defaults exactly as they
	// would on the next launch.
	loaded := config.Load(path)
	a.mu.Lock()
	a.cfg = loaded
	a.mu.Unlock()
	return nil
}

func (a *App) ListBackups() []backup.Info {
	return a.backupMgr.List()
}

// CleanOldBackups deletes backup sessions older than the configured retention
// window and returns how many were removed. The same sweep runs automatically
// in startup at every app launch.
func (a *App) CleanOldBackups() int {
	a.mu.Lock()
	retention := a.cfg.BackupRetentionDays
	a.mu.Unlock()
	return a.backupMgr.CleanOld(retention)
}

func (a *App) RestoreBackup(path string) backup.RestoreResult {
	return a.backupMgr.Restore(path, a.home)
}

// GetBackupDetails returns a backup session's contents for the Backups view.
// Containment errors yield an empty (non-nil) list — the UI cannot produce an
// invalid path, and the view's empty state covers the degenerate case.
func (a *App) GetBackupDetails(path string) backup.Details {
	d, err := a.backupMgr.Details(path, a.home)
	if err != nil {
		return backup.Details{Items: []backup.Item{}}
	}
	return d
}

func (a *App) DeleteBackup(path string) error {
	return a.backupMgr.Delete(path)
}

// ---------------------------------------------------------------------------
// Dashboard, Space Lens & Login Items
// ---------------------------------------------------------------------------

// persistStatsLocked writes a.stats to disk. Caller must hold a.mu. Write
// failures are ignored: stats are best-effort telemetry for the dashboard.
func (a *App) persistStatsLocked() {
	_ = saveStats(a.stats, statsPath(a.home))
}

// GetDiskUsage reports the boot volume's capacity for the health card.
func (a *App) GetDiskUsage() DiskUsage {
	return readDiskUsage("/")
}

// GetActivityStats returns the lifetime cleaning tally.
func (a *App) GetActivityStats() ActivityStats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stats
}

// BuildSpaceLens walks root (must be inside the home directory or /Volumes)
// and returns its size map, 2 levels deep. Synchronous: Wails runs each
// call on its own goroutine, so a long walk never blocks the UI thread —
// the frontend just awaits the promise.
func (a *App) BuildSpaceLens(root string) (spacelens.Node, error) {
	if root == "" {
		root = a.home
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return spacelens.Node{}, fmt.Errorf("invalid path: %w", err)
	}
	allowed := abs == a.home || strings.HasPrefix(abs, a.home+"/") || strings.HasPrefix(abs, "/Volumes/")
	if !allowed {
		return spacelens.Node{}, errors.New("path must be inside your home folder or /Volumes")
	}
	return spacelens.Build(a.ctx, abs, 2), nil
}

// ListLoginItems lists launchd agents/daemons from the standard locations.
func (a *App) ListLoginItems() []loginitems.Item {
	return loginitems.List(loginitems.StandardDirs(a.home))
}

func (a *App) CheckFDA() *bool {
	return fda.Check(a.home)
}

func (a *App) OpenFDASettings() {
	_ = exec.CommandContext(a.ctx, "/usr/bin/open", fda.SettingsURL).Run()
}

func (a *App) RevealInFinder(path string) {
	_ = exec.CommandContext(a.ctx, "/usr/bin/open", "-R", path).Run()
}

func (a *App) CopyPath(path string) {
	_ = wruntime.ClipboardSetText(a.ctx, path)
}
