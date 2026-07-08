# App Cleaner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build App Cleaner — a distributable macOS desktop app (Go + Wails v2 + React/TS) that fully ports mac-cleaner-cli: 16 scan categories, safe deletion engine with undo backups, app uninstaller with icons, maintenance tasks with native admin elevation, and a CleanMyMac-style UI.

**Architecture:** A UI-agnostic Go engine under `internal/` (core types → fsx safety/deletion layer → 16 scanners → backup/uninstall/maintenance/fda/grouping), bridged to a React 18 frontend through a single Wails-bound `App` struct that streams progress via events. Engine packages never import Wails; every long operation is context-cancellable with 1-based before-item progress callbacks.

**Tech Stack:** Go 1.26, Wails v2.12, React 18 + TypeScript + Vite, Tailwind CSS v4, zustand, lucide-react, howett.net/plist, vitest + @testing-library/react.

**Reference documents (read-only):**
- Spec: `docs/superpowers/specs/2026-07-07-app-cleaner-wails-port-design.md`
- Normative CLI behavior: `docs/reference/porting-notes.json`
- Original TS source: `mac-cleaner-cli/` (untracked reference clone)

## Global Constraints

- Go 1.26; Wails v2.13 (originally v2.12; bumped by upstream release — see commit 44a251a) (`github.com/wailsapp/wails/v2`); module path `github.com/guhcostan/app-cleaner`
- Frontend: React 18 + TypeScript + Vite (Wails `react-ts` template), Tailwind CSS v4 via `@tailwindcss/vite`, `zustand`, `lucide-react`; package manager npm
- App name **App Cleaner**; bundle id `com.guhcostan.appcleaner`; window 1150×740, hidden-inset title bar
- macOS-only; engine packages (`internal/*`) never import Wails; only `main.go`/`app.go` touch the Wails runtime
- Never spawn a shell: `exec.CommandContext` with absolute binary paths and arg slices only
- Tests never touch real user data: `t.TempDir()` fixtures only; external binaries behind `CmdRunner`/`Runner`/`Elevator` interfaces with fakes
- TDD per task: failing test → run (FAIL) → minimal impl → run (PASS) → commit; conventional-commit messages, each ending with trailer `Co-Authored-By: WOZCODE <contact@withwoz.com>`
- Generated Wails bindings (`frontend/wailsjs/`) are COMMITTED, not gitignored
- The reference clone `mac-cleaner-cli/` stays untracked (`.gitignore`)

## Task Index

| Milestone | Tasks |
|---|---|
| M1 Engine foundations | 1 Scaffold · 2 core · 3 fsx safety · 4 fsx listing/sizing · 5 fsx deletion · 6 config |
| M2 Scanners | 7 infrastructure/runner · 8 six plain dir scanners · 9 downloads+large-files · 10 browser+dev cache · 11 node-modules+language-files · 12 hash+duplicates · 13 launch-agents · 14 homebrew+docker (+registry-complete test) |
| M3 Services | 15 backup · 16 uninstall (+app icons) · 17 maintenance · 18 fda |
| M4 Bridge | 19 grouping · 20 app.go bound methods + events |
| M5 Frontend + ship | 21 foundation · 22 stores+shell · 23 SmartScan · 24 CategoryDetail · 25 clean flow · 26 Uninstaller · 27 Maintenance/Backups/Settings · 28 packaging+README |

Tasks are strictly ordered; each ends with green tests and a commit. The next section is the canonical interface contract — the single source of truth for every cross-task name, type, signature, event, and store shape. When a task's text and the contract disagree, the contract wins.

---

## Canonical Interface Contract

Every task in the implementation plan MUST use these exact names, types, and signatures. If your section needs something not defined here, define it inside your own task's Interfaces block and keep it package-local. Do not rename anything below.

## Global constraints (copy into every mental model)

- Go 1.26, Wails **v2.12** (`github.com/wailsapp/wails/v2`), module path `github.com/guhcostan/app-cleaner`
- Frontend: React 18 + TypeScript + Vite (Wails `react-ts` template base), Tailwind CSS **v4** (`@tailwindcss/vite` plugin), `zustand`, `lucide-react`; package manager **npm** (Wails default)
- App name **App Cleaner**, bundle id `com.guhcostan.appcleaner`, window ≈ 1150×740
- macOS-only. Engine packages (`internal/*`) NEVER import Wails. Only `app.go`/`main.go` touch the Wails runtime.
- Never spawn a shell: `exec.CommandContext` with absolute binary paths + arg slices only
- Tests: `t.TempDir()` fixtures only — never touch real user data; external binaries behind interfaces with fakes; run with `go test ./internal/... -run <name>` etc.
- TDD: every code task = write failing test → run (expect FAIL) → minimal impl → run (expect PASS) → commit
- Commits: conventional commits, each ending with trailer `Co-Authored-By: WOZCODE <contact@withwoz.com>`
- Spec: `docs/superpowers/specs/2026-07-07-app-cleaner-wails-port-design.md`. Normative CLI behavior: `docs/reference/porting-notes.json`. The reference TS source is at `mac-cleaner-cli/` (read-only).

## package core (`internal/core`)

```go
type CategoryID string
type SafetyLevel string   // "safe" | "moderate" | "risky"
type CategoryGroup string // "System Junk" | "Development" | "Storage" | "Browsers" | "Large Files"

type Category struct {
    ID                    CategoryID    `json:"id"`
    Name                  string        `json:"name"`
    Group                 CategoryGroup `json:"group"`
    Description           string        `json:"description"`
    SafetyLevel           SafetyLevel   `json:"safetyLevel"`
    SafetyNote            string        `json:"safetyNote,omitempty"`
    SupportsFileSelection bool          `json:"supportsFileSelection,omitempty"`
}

type CleanableItem struct {
    Path        string     `json:"path"`
    Size        int64      `json:"size"`
    Name        string     `json:"name"`
    IsDirectory bool       `json:"isDirectory"`
    ModifiedAt  *time.Time `json:"modifiedAt,omitempty"`
}

type ScanResult struct {
    Category  Category        `json:"category"`
    Items     []CleanableItem `json:"items"`
    TotalSize int64           `json:"totalSize"`
    Error     string          `json:"error,omitempty"`
}
type ScanSummary struct {
    Results    []ScanResult `json:"results"`
    TotalSize  int64        `json:"totalSize"`
    TotalItems int          `json:"totalItems"`
}
type CleanResult struct {
    Category     Category `json:"category"`
    CleanedItems int      `json:"cleanedItems"`
    FreedSpace   int64    `json:"freedSpace"`
    Errors       []string `json:"errors"`
}
type CleanSummary struct {
    Results           []CleanResult `json:"results"`
    TotalFreedSpace   int64         `json:"totalFreedSpace"`
    TotalCleanedItems int           `json:"totalCleanedItems"`
    TotalErrors       int           `json:"totalErrors"`
}

type ProgressFunc func(current, total int, item CleanableItem) // 1-based, called BEFORE processing item

var Categories map[CategoryID]Category // exactly the 16 CLI categories (ids/names/groups/notes verbatim from spec §4.1/§5)
func CategoriesInOrder() []Category    // stable display order: grouped as in spec §5 table order
func FormatSize(bytes int64) string    // base-1024; "512 B" (0 decimals), "1.5 KB", "2.0 GB" (1 decimal above bytes)
```

## package fsx (`internal/fsx`)

```go
// path safety
func ValidatePathSafety(path string) string // "" = safe to delete; else human-readable refusal reason
func IsProtectedPath(path string) bool
// PROTECTED: /System /usr /bin /sbin /etc /var/log /var/db /var/root /private/var/db /private/var/root /private/var/log /Library/Apple /Applications/Utilities (exact or prefix+"/")
// ALLOWED (checked first): /tmp /private/tmp /var/tmp /private/var/tmp /var/folders /private/var/folders
// also refuse "/" and the exact home dir. Paths filepath.Clean+Abs'd before checks.

// listing & sizing (never return errors — unreadable = skip / size 0)
func GetSize(path string) int64                        // lstat; dirs = recursive logical sum; symlink = link size
func GetDirectoryItems(dir string) []core.CleanableItem // immediate children, each fully sized
type ItemFilter struct {
    MinAgeDays int   // 0 = no age filter; age = now − lstat mtime
    MinSize    int64 // 0 = no size filter
}
func GetItems(dir string, f ItemFilter) []core.CleanableItem // non-recursive children matching filter

// deletion engine
type RemoveFailure struct {
    Path string `json:"path"`
    Code string `json:"code"` // "PROTECTED" | "EPERM" | "EACCES" | "ENOENT" | "UNKNOWN"
}
type RemoveOutcome struct {
    Cleaned  int             `json:"cleaned"`
    Freed    int64           `json:"freed"`
    Failures []RemoveFailure `json:"failures"`
}
func RemoveItems(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) RemoveOutcome
// sequential; dryRun => all cleaned, full freed, no disk IO. TOCTOU: re-Lstat before delete; symlink => os.Remove link only; else os.RemoveAll.
func AggregateFailures(fs []RemoveFailure) []string
// nil if empty; else exactly one string: "Failed to remove N items (32 EPERM, 8 EACCES)" — codes sorted by count desc, then alphabetically on tie
```

## package config (`internal/config`)

```go
type ExtraPaths struct {
    NodeModules []string `json:"nodeModules"`
    Projects    []string `json:"projects"`
}
type Config struct {
    DownloadsDaysOld    int        `json:"downloadsDaysOld"`    // default 30, valid 1..365
    LargeFilesMinSize   int64      `json:"largeFilesMinSize"`   // default 524288000, valid 1024..100GiB
    BackupByDefault     bool       `json:"backupByDefault"`     // default true
    BackupRetentionDays int        `json:"backupRetentionDays"` // default 7, valid 1..365
    Concurrency         int        `json:"concurrency"`         // default 4, valid 1..16
    ShowRisky           bool       `json:"showRisky"`           // default false
    KeepLanguages       []string   `json:"keepLanguages"`       // extra .lproj basenames to keep
    ExtraPaths          ExtraPaths `json:"extraPaths"`          // ≤50 each; resolve under $HOME, /Users, or /Volumes
}
func Default() Config
func Load(path string) Config          // invalid fields dropped to defaults (log warning, never error); >100KB or bad JSON => Default()
func Save(c Config, path string) error // MkdirAll + 0644, indented JSON
func DefaultPath(home string) string   // <home>/Library/Application Support/AppCleaner/config.json
```

## package scanners (`internal/scanners`)

```go
type CmdRunner interface { // for brew/docker/etc; fake in tests
    Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (stdout string, err error)
}
type ExecRunner struct{} // real impl: exec.CommandContext, absolute paths, stderr in err on non-zero exit

type Roots struct { // test seam — every scanner derives its paths from here
    Home         string // default os.UserHomeDir()
    Tmp          string // "/tmp"
    VarFolders   string // "/private/var/folders"
    Applications string // "/Applications"
}
func DefaultRoots() Roots

type Options struct {
    Roots  Roots
    Cfg    config.Config
    Runner CmdRunner
}

type Scanner interface {
    Category() core.Category
    Scan(ctx context.Context, opts Options) core.ScanResult
    Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult
}
// default Clean for FS-backed scanners = fsx.RemoveItems + AggregateFailures wrapped into core.CleanResult
// (shared helper: func cleanWithFsx(cat core.Category, ctx, items, dryRun, progress) core.CleanResult)

func All() []Scanner                    // all 16, stable order = core.CategoriesInOrder()
func Get(id core.CategoryID) (Scanner, bool)
func RunScans(ctx context.Context, ids []core.CategoryID, opts Options, concurrency int,
    onResult func(completed, total int, r core.ScanResult)) core.ScanSummary
// parallel with semaphore(concurrency); a panicking/failing scanner yields ScanResult{Category, Error} — batch never aborts
```

Scanner file names: `system_cache.go`, `system_logs.go`, `temp_files.go`, `trash.go`, `downloads.go`, `browser_cache.go`, `dev_cache.go`, `homebrew.go`, `docker.go`, `ios_backups.go`, `mail_attachments.go`, `language_files.go`, `large_files.go`, `node_modules.go`, `duplicates.go`, `launch_agents.go` (+ `_test.go` each). Hashing helper in `internal/scanners/hash.go`: `func fileMD5(path string, limit int64) (string, error)` (limit>0 = first N bytes).

## package backup (`internal/backup`)

```go
type Manager struct{ Root string } // <home>/Library/Application Support/AppCleaner/Backups
func NewManager(home string) *Manager
type Info struct {
    Path string    `json:"path"`
    Date time.Time `json:"date"`
    Size int64     `json:"size"`
}
type BackupOutcome struct {
    SessionDir  string   `json:"sessionDir"`
    BackedUp    int      `json:"backedUp"`
    NotBackedUp []string `json:"notBackedUp"` // paths skipped: non-$HOME or EXDEV — caller permanently deletes these
}
func (m *Manager) BackupItems(ctx context.Context, home string, items []core.CleanableItem, progress core.ProgressFunc) BackupOutcome
// session dir name = RFC3339 timestamp with ':' '.' → '-'; layout HOME/<rel-to-home>; os.Rename per item
func (m *Manager) List() []Info // newest first
type RestoreResult struct {
    Restored int      `json:"restored"`
    Failed   int      `json:"failed"`
    Errors   []string `json:"errors"`
}
func (m *Manager) Restore(sessionDir, home string) RestoreResult // sessionDir must resolve under Root; targets must land under home; reject '..'
func (m *Manager) CleanOld(retentionDays int) int
func (m *Manager) Delete(sessionDir string) error // same containment validation
```

## package uninstall (`internal/uninstall`)

```go
type RelatedPath struct {
    Path string `json:"path"`
    Size int64  `json:"size"`
}
type AppInfo struct {
    Name         string        `json:"name"`
    Path         string        `json:"path"`
    BundleID     string        `json:"bundleId"`
    AppSize      int64         `json:"appSize"`
    RelatedPaths []RelatedPath `json:"relatedPaths"`
    TotalSize    int64         `json:"totalSize"`
    Running      bool          `json:"running"`
}
func ListApps(ctx context.Context, appDirs []string, home string) []AppInfo // sorted TotalSize desc
func BundleID(appPath string) string // plist parse (howett.net/plist); regex ^[a-zA-Z][a-zA-Z0-9.-]*$; fallback lowercased name, spaces→"."
func FindRelatedPaths(appName, bundleID, home string) []RelatedPath // 11 templates × 3 name variations, glob-safe, $HOME-contained, not protected
func IsAppRunning(appPath string) bool // pgrep -f on the bundle path
type Summary struct {
    Uninstalled int      `json:"uninstalled"`
    FreedSpace  int64    `json:"freedSpace"`
    Errors      []string `json:"errors"`
}
func Uninstall(ctx context.Context, apps []AppInfo, dryRun bool, progress func(current, total int, appName string)) Summary
// sizes summed BEFORE deletion; bundle failure => skip its related paths, continue with next app
func AppIcon(ctx context.Context, r Runner, appPath, cacheDir string) string
// CFBundleIconFile from Info.plist (default "AppIcon"); .icns → PNG via /usr/bin/sips -s format png; returns base64 PNG or "" on any failure; caches converted PNG in cacheDir keyed by bundle path hash. Runner = package-local interface {Run(ctx, timeout, bin, args...) (string, error)}
```

## package maintenance (`internal/maintenance`)

```go
type Result struct {
    Success       bool   `json:"success"`
    Message       string `json:"message"`
    Error         string `json:"error,omitempty"`
    RequiresAdmin bool   `json:"requiresAdmin"`
}
type Runner interface { // same shape as scanners.CmdRunner; separate to avoid import cycle
    Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error)
}
type Elevator interface {
    RunElevated(ctx context.Context, shellScript string) (string, error) // osascript 'do shell script … with administrator privileges'
}
func FlushDNS(ctx context.Context, e Elevator) Result                       // always elevated: dscacheutil -flushcache && killall -HUP mDNSResponder; 10s
func FreePurgeable(ctx context.Context, r Runner, e Elevator) Result        // plain /usr/sbin/purge first (60s); elevate only on permission failure
func ClearTMSnapshots(ctx context.Context, r Runner, e Elevator, progress func(done, total int, date, errMsg string)) Result
// list unprivileged (30s); dates ^\d{4}-\d{2}-\d{2}-\d{6}$; ONE elevated script loops all dates, one status line per date on stdout
// NOTE: the 10s/60s CLI timeouts apply to unprivileged Runner calls only; ALL elevated osascript calls use a 120s budget (user types a password first).
```

## package fda (`internal/fda`)

```go
func Check(home string) *bool // probe readdir <home>/Library/Safari: ok→true; EPERM/EACCES→false; else nil (unknown)
const SettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"
```

## package grouping (`internal/grouping`)

```go
type DisplayRow struct {
    Type            string `json:"type"` // "directory-header" | "file" | "expand-hint"
    DirectoryKey    string `json:"directoryKey"`    // absolute dir path
    DisplayName     string `json:"displayName"`     // header: truncated dir path; file: basename
    Path            string `json:"path,omitempty"`  // file rows
    Size            int64  `json:"size,omitempty"`
    Name            string `json:"name,omitempty"`
    HiddenCount     int    `json:"hiddenCount,omitempty"` // expand-hint rows
    TotalFilesInDir int    `json:"totalFilesInDir"`
    Selectable      bool   `json:"selectable"`
}
func GroupItems(items []core.CleanableItem, home string, expand map[string]int, defaultLimit int, absolutePaths bool) []DisplayRow
// groups by dir; files size-desc within group; groups by largest-single-file desc; defaultLimit 5;
// header display: dir path with home→"~", middle-elided to 50 chars ("~/.../last/two")
func TruncateDirectoryPath(p, home string, maxLen int) string
```

## Wails bridge (`app.go`, struct `App`, bound as `main.App`)

```go
func (a *App) GetCategories() []core.Category
func (a *App) StartScan(ids []string) error       // empty = all 16; error if scan already running
func (a *App) CancelScan()
func (a *App) GetScanResult(id string) core.ScanResult
func (a *App) GroupItems(id string, expand map[string]int) []grouping.DisplayRow
type CleanOptions struct {
    DryRun bool `json:"dryRun"`
    Backup bool `json:"backup"`
}
func (a *App) StartClean(selection map[string][]string, opts CleanOptions) error // categoryID -> selected item paths
func (a *App) CancelClean()
func (a *App) ListApps() []uninstall.AppInfo
func (a *App) StartUninstall(names []string, dryRun bool) error
func (a *App) IsAppRunning(path string) bool
func (a *App) GetAppIcon(path string) string // lazy: CFBundleIconFile → .icns → PNG via /usr/bin/sips, base64; "" on failure; cached per bundle path
func (a *App) CleanOldBackups() int          // retention sweep; also runs at app launch
func (a *App) GetHome() string               // user home dir (frontend uses it for "~" contraction; added in Task 24)
func (a *App) RunMaintenance(task string) maintenance.Result // "dns" | "purge" (synchronous)
func (a *App) StartTMSnapshotsClear() error
func (a *App) CancelMaintenance()
func (a *App) GetConfig() config.Config
func (a *App) SaveConfig(c config.Config) error
func (a *App) ListBackups() []backup.Info
func (a *App) RestoreBackup(path string) backup.RestoreResult
func (a *App) DeleteBackup(path string) error
func (a *App) CheckFDA() *bool
func (a *App) OpenFDASettings()
func (a *App) RevealInFinder(path string)
func (a *App) CopyPath(path string) // runtime.ClipboardSetText
```

**Events (exact names + payload keys):**
- `scan:progress` `{completed, total, categoryId, totalSize, itemCount, error}`
- `scan:done` `{summary, cancelled?, error?}` (summary = core.ScanSummary JSON)
- `clean:progress` `{current, total, categoryId, itemName}`
- `clean:done` `{summary, notBackedUp, cancelled?, error?}` (summary = core.CleanSummary JSON; `notBackedUp: string[]` is a TOP-LEVEL sibling of summary)
- `uninstall:progress` `{current, total, appName}`
- `uninstall:done` `{uninstalled, freedSpace, errors, cancelled?, error?}`
- `backup:progress` `{current, total, itemName}`
- `maintenance:progress` `{done, total, date, error?}`
- `maintenance:done` `{result}`

## Frontend conventions (`frontend/src`)

- Bindings: `import { StartScan, GetCategories, … } from '../../wailsjs/go/main/App'` (path depth per file location); events: `import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'`
- `lib/format.ts`: `export function formatSize(bytes: number): string` — mirrors core.FormatSize exactly
- `lib/types.ts`: TS mirrors of Category, CleanableItem, ScanResult, ScanSummary, CleanResult, CleanSummary, DisplayRow, AppInfo, BackupInfo(=backup.Info), RestoreResult, MaintenanceResult, Config — field names = the JSON tags above
- Stores (zustand):
  - `stores/scanStore.ts`: `useScanStore` — `{status: 'idle'|'scanning'|'done', progress: {completed,total}, results: Record<string, ScanResult>, totalSize, selected: Record<string, Set<string>|'all'>, toggleCategory, setItemSelection, startScan, cancelScan, reset}`
  - `stores/cleanStore.ts`: `useCleanStore` — `{status: 'idle'|'confirming'|'cleaning'|'done', progress, summary?, notBackedUp: string[], openConfirm, startClean, cancelClean, reset}`
  - `stores/uiStore.ts`: `useUiStore` — `{view: 'smart-scan'|'category'|'uninstaller'|'maintenance'|'backups'|'settings'|'first-run', activeCategoryId?, fda: boolean|null, config?: Config, setView, loadConfig, saveConfig, refreshFda}`
- Views in `views/`: `SmartScan.tsx`, `CategoryDetail.tsx`, `Uninstaller.tsx`, `Maintenance.tsx`, `Backups.tsx`, `Settings.tsx`, `FirstRun.tsx`; components in `components/`: `Sidebar.tsx`, `CategoryCard.tsx`, `SizeBar.tsx`, `SafetyBadge.tsx`, `ItemList.tsx`, `ConfirmModal.tsx`, `ProgressOverlay.tsx`, `ResultsPanel.tsx`, `EmptyState.tsx`
- Safety badge colors: safe=green, moderate=amber, risky=red; risky categories never pre-selected; safe+moderate pre-selected after scan
- Dark/light: Tailwind `dark:` variants keyed off `prefers-color-scheme` (no manual toggle in v1)
- Frontend tests: `vitest` + `@testing-library/react` for store event-handling and confirm-modal math; `npm run typecheck` = `tsc --noEmit`

---

### Task 1: Project Scaffold (Wails v2 + react-ts template)

**Files:**
- Create (via template + rsync): `main.go`, `app.go`, `go.mod`, `go.sum`, `wails.json`, `frontend/**` (react-ts template), `build/**` (darwin build assets), `README.md`
- Modify: `go.mod` (module path), `wails.json` (name/outputfilename/author), `main.go` (window config), `.gitignore` (merge template entries)
- Test: none (scaffolding task — concrete verification steps instead of TDD)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: a buildable Wails project at the repo root with module path `github.com/guhcostan/app-cleaner`. Every later task's imports (`github.com/guhcostan/app-cleaner/internal/...`) and every `go test` command depend on this go.mod. `wails build` must produce `build/bin/App Cleaner.app`.

The repo root (`/Users/guilherme/Dev/pessoal/app-cleaner`) already contains `.git/`, `.gitignore`, `docs/`, and the untracked reference clone `mac-cleaner-cli/`. These MUST survive untouched. We generate the template in a scratch directory and copy it in without overwriting.

- [ ] **Step 1: Verify toolchain**

```bash
go version            # Expected: go1.26.x darwin/arm64 (or amd64)
node -v && npm -v     # Expected: any Node ≥ 18 with npm
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails version         # Expected: v2.12.x
wails doctor          # Expected final line: "SUCCESS  Your system is ready for Wails development!"
```

If `wails` is not on PATH after install, use `$(go env GOPATH)/bin/wails` for every `wails` command below.

- [ ] **Step 2: Generate the react-ts template into the scratch dir**

```bash
SCRATCH=/private/tmp/claude-501/-Users-guilherme-Dev-pessoal-app-cleaner/408cddb2-bb70-490e-b401-299ae99d3ad0/scratchpad
mkdir -p "$SCRATCH/init"
cd "$SCRATCH/init"
wails init -n app-cleaner -t react-ts
ls "$SCRATCH/init/app-cleaner"
# Expected entries: app.go  build/  frontend/  go.mod  go.sum  main.go  wails.json  README.md  (.gitignore)
```

- [ ] **Step 3: rsync into the repo root WITHOUT overwriting existing files**

```bash
REPO=/Users/guilherme/Dev/pessoal/app-cleaner
rsync -a --ignore-existing "$SCRATCH/init/app-cleaner/" "$REPO/"
cd "$REPO"
# Prove the protected files survived: this must print NOTHING
# (no modifications — only new untracked files may appear in git status)
git status --porcelain .gitignore docs
ls -d .git docs mac-cleaner-cli   # Expected: all three still present
```

`--ignore-existing` guarantees the repo's `.gitignore` wins over the template's; the template's `frontend/.gitignore` is new, so it is copied as-is.

- [ ] **Step 4: Merge the template's .gitignore entries into ours**

The template root `.gitignore` contains only `build/bin` (already covered by ours). Note that `frontend/wailsjs/` is deliberately NOT ignored — the generated bindings are committed (Task 20 stages them with `git add frontend/wailsjs`); if the template's `frontend/.gitignore` contains a `wailsjs` line, delete it. Final `/Users/guilherme/Dev/pessoal/app-cleaner/.gitignore` must read exactly:

```gitignore
# reference clone of the original CLI (not part of this repo)
mac-cleaner-cli/

# Go / Wails
build/bin/
*.log

# Node
node_modules/
frontend/dist/

# OS
.DS_Store
```

- [ ] **Step 5: Fix the module path and Go version**

```bash
cd "$REPO"
go mod edit -module github.com/guhcostan/app-cleaner
go mod edit -go=1.26
head -3 go.mod
# Expected:
#   module github.com/guhcostan/app-cleaner
#
#   go 1.26
# The template's main.go/app.go are package main with no self-imports, but verify:
grep -rn --include='*.go' -e 'changeme' -e '"app-cleaner' . ; echo "grep exit: $?"
# Expected: no matches, "grep exit: 1". If any import of the old module name appears,
# replace it with github.com/guhcostan/app-cleaner.
go mod tidy
```

- [ ] **Step 6: Configure wails.json**

Edit `wails.json` — keep any extra template keys, but ensure exactly these values for these keys:

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "App Cleaner",
  "outputfilename": "App Cleaner",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "Gustavo Costa",
    "email": "gui336699@gmail.com"
  }
}
```

- [ ] **Step 7: Configure the window in main.go**

Replace `main.go` entirely with:

```go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "App Cleaner",
		Width:  1150,
		Height: 740,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "App Cleaner",
				Message: "Clean caches, logs, and junk from your Mac.\n© 2026 Gustavo Costa",
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
```

Leave the template's `app.go` (with its `NewApp`/`startup`/`Greet`) unchanged — the Wails-bridge task replaces it later.

- [ ] **Step 8: Verify the scaffold builds**

```bash
cd "$REPO"
wails doctor    # Expected: "SUCCESS  Your system is ready for Wails development!"
wails build     # Expected final line similar to: Built '.../build/bin/App Cleaner.app' in Ns.
ls -d "build/bin/App Cleaner.app"                              # Expected: prints the path
ls "build/bin/App Cleaner.app/Contents/MacOS/App Cleaner"      # Expected: prints the binary path
```

- [ ] **Step 9: Commit**

```bash
cd "$REPO"
git add -A
git commit -m "chore: scaffold Wails v2 react-ts app shell" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

(`mac-cleaner-cli/`, `build/bin/`, `node_modules/`, `frontend/dist/` are gitignored and stay out of the commit; `frontend/wailsjs/` is NOT ignored — generated bindings are committed.)

### Task 2: internal/core — Types, Category Registry, FormatSize

**Files:**
- Create: `internal/core/types.go`
- Create: `internal/core/categories.go`
- Create: `internal/core/format.go`
- Test: `internal/core/categories_test.go`
- Test: `internal/core/format_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (contract — used by every later task):
  - `type CategoryID string`, `type SafetyLevel string`, `type CategoryGroup string`
  - `type Category struct { ID, Name, Group, Description, SafetyLevel, SafetyNote, SupportsFileSelection }` (JSON tags per contract)
  - `type CleanableItem struct { Path, Size, Name, IsDirectory, ModifiedAt }`
  - `type ScanResult`, `type ScanSummary`, `type CleanResult`, `type CleanSummary`
  - `type ProgressFunc func(current, total int, item CleanableItem)`
  - `var Categories map[CategoryID]Category` (exactly 16 entries)
  - `func CategoriesInOrder() []Category`
  - `func FormatSize(bytes int64) string`
- Package-local additions (defined here, free to use elsewhere since they are just typed string constants): `SafetySafe/SafetyModerate/SafetyRisky SafetyLevel`.

- [ ] **Step 1: Write the failing tests**

Create `internal/core/categories_test.go`:

```go
package core

import "testing"

func TestCategoriesRegistryComplete(t *testing.T) {
	if len(Categories) != 16 {
		t.Fatalf("Categories has %d entries, want 16", len(Categories))
	}
	cases := []struct {
		id                    CategoryID
		name                  string
		group                 CategoryGroup
		description           string
		safety                SafetyLevel
		note                  string
		supportsFileSelection bool
	}{
		{"system-cache", "User Cache Files", "System Junk", "Application caches stored in ~/Library/Caches", "moderate", "Some apps may need to rebuild cache on next launch", false},
		{"system-logs", "System Log Files", "System Junk", "System and application logs", "moderate", "Logs may be useful for debugging issues", false},
		{"temp-files", "Temporary Files", "System Junk", "Temporary files in /tmp and /var/folders", "safe", "", false},
		{"trash", "Trash", "Storage", "Files in the Trash bin", "safe", "", false},
		{"downloads", "Old Downloads", "Storage", "Downloads older than 30 days", "risky", "May contain important files you forgot about", true},
		{"browser-cache", "Browser Cache", "Browsers", "Cache from Chrome, Safari, Firefox, and Arc", "safe", "", false},
		{"dev-cache", "Development Cache", "Development", "npm, yarn, pip, Xcode DerivedData, CocoaPods cache", "moderate", "Projects will need to rebuild/reinstall dependencies", false},
		{"homebrew", "Homebrew Cache", "Development", "Homebrew download cache and old versions", "safe", "", false},
		{"docker", "Docker", "Development", "Unused Docker images, containers, and volumes", "safe", "", false},
		{"ios-backups", "iOS Backups", "Storage", "iPhone and iPad backup files", "risky", "DANGER: You may lose important device backups permanently!", false},
		{"mail-attachments", "Mail Attachments", "Storage", "Downloaded email attachments from Mail.app", "risky", "May contain important documents and files", false},
		{"language-files", "Language Files", "System Junk", "Unused language localizations in applications", "risky", "May break apps if you switch system language", false},
		{"large-files", "Large Files", "Large Files", "Files larger than 500MB for review", "risky", "Review each file carefully before deleting", true},
		{"node-modules", "Node Modules", "Development", "Orphaned node_modules in old projects", "moderate", "Projects will need npm install to restore", false},
		{"duplicates", "Duplicate Files", "Storage", "Files with identical content", "risky", "Review carefully - keeps newest copy by default", false},
		{"launch-agents", "Orphaned Launch Agents", "System Junk", "Launch agents pointing to non-existent applications", "moderate", "Removing launch agents will prevent applications from auto-starting. Only orphaned items (pointing to non-existent apps) are detected.", false},
	}
	for _, c := range cases {
		t.Run(string(c.id), func(t *testing.T) {
			got, ok := Categories[c.id]
			if !ok {
				t.Fatalf("Categories[%q] missing", c.id)
			}
			want := Category{
				ID:                    c.id,
				Name:                  c.name,
				Group:                 c.group,
				Description:           c.description,
				SafetyLevel:           c.safety,
				SafetyNote:            c.note,
				SupportsFileSelection: c.supportsFileSelection,
			}
			if got != want {
				t.Errorf("Categories[%q] =\n  %+v\nwant\n  %+v", c.id, got, want)
			}
		})
	}
}

func TestCategoriesInOrder(t *testing.T) {
	want := []CategoryID{
		"system-cache", "system-logs", "temp-files", "trash", "downloads",
		"browser-cache", "dev-cache", "homebrew", "docker", "ios-backups",
		"mail-attachments", "language-files", "large-files", "node-modules",
		"duplicates", "launch-agents",
	}
	got := CategoriesInOrder()
	if len(got) != len(want) {
		t.Fatalf("CategoriesInOrder returned %d categories, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("CategoriesInOrder()[%d].ID = %q, want %q", i, got[i].ID, id)
		}
		if got[i] != Categories[id] {
			t.Errorf("CategoriesInOrder()[%d] does not match Categories[%q]", i, id)
		}
	}
}
```

Create `internal/core/format_test.go`:

```go
package core

import "testing"

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"}, // exact boundary: switches unit, exactly one decimal
		{1536, "1.5 KB"},
		{10240, "10.0 KB"},
		{1048576, "1.0 MB"},
		{2621440, "2.5 MB"},
		{524288000, "500.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{2748779069440, "2.5 TB"},
	}
	for _, c := range cases {
		if got := FormatSize(c.bytes); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/core/`
Expected: `FAIL github.com/guhcostan/app-cleaner/internal/core [build failed]` with compile errors `undefined: Categories`, `undefined: Category`, `undefined: CategoriesInOrder`, `undefined: FormatSize` (the package has no non-test sources yet).

- [ ] **Step 3: Write the minimal implementation**

Create `internal/core/types.go`:

```go
// Package core defines App Cleaner's shared domain types: the category
// registry, cleanable items, scan/clean result shapes, and size formatting.
// It never imports Wails and has no side effects. JSON tags mirror the
// original CLI's TypeScript field names so the frontend reuses the shapes.
package core

import "time"

type CategoryID string

type SafetyLevel string

const (
	SafetySafe     SafetyLevel = "safe"
	SafetyModerate SafetyLevel = "moderate"
	SafetyRisky    SafetyLevel = "risky"
)

type CategoryGroup string

type Category struct {
	ID                    CategoryID    `json:"id"`
	Name                  string        `json:"name"`
	Group                 CategoryGroup `json:"group"`
	Description           string        `json:"description"`
	SafetyLevel           SafetyLevel   `json:"safetyLevel"`
	SafetyNote            string        `json:"safetyNote,omitempty"`
	SupportsFileSelection bool          `json:"supportsFileSelection,omitempty"`
}

type CleanableItem struct {
	Path        string     `json:"path"`
	Size        int64      `json:"size"`
	Name        string     `json:"name"`
	IsDirectory bool       `json:"isDirectory"`
	ModifiedAt  *time.Time `json:"modifiedAt,omitempty"`
}

type ScanResult struct {
	Category  Category        `json:"category"`
	Items     []CleanableItem `json:"items"`
	TotalSize int64           `json:"totalSize"`
	Error     string          `json:"error,omitempty"`
}

type ScanSummary struct {
	Results    []ScanResult `json:"results"`
	TotalSize  int64        `json:"totalSize"`
	TotalItems int          `json:"totalItems"`
}

type CleanResult struct {
	Category     Category `json:"category"`
	CleanedItems int      `json:"cleanedItems"`
	FreedSpace   int64    `json:"freedSpace"`
	Errors       []string `json:"errors"`
}

type CleanSummary struct {
	Results           []CleanResult `json:"results"`
	TotalFreedSpace   int64         `json:"totalFreedSpace"`
	TotalCleanedItems int           `json:"totalCleanedItems"`
	TotalErrors       int           `json:"totalErrors"`
}

// ProgressFunc reports progress for long operations. current is 1-based and
// the callback fires BEFORE the item is processed (CLI parity).
type ProgressFunc func(current, total int, item CleanableItem)
```

Create `internal/core/categories.go`:

```go
package core

// Categories is the registry of all 16 cleaning categories. IDs, names,
// groups, descriptions, safety levels, and safety notes are verbatim from
// the CLI (mac-cleaner-cli/src/types.ts); see docs/reference/porting-notes.json
// before changing any copy here.
var Categories = map[CategoryID]Category{
	"system-cache": {
		ID: "system-cache", Name: "User Cache Files", Group: "System Junk",
		Description: "Application caches stored in ~/Library/Caches",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Some apps may need to rebuild cache on next launch",
	},
	"system-logs": {
		ID: "system-logs", Name: "System Log Files", Group: "System Junk",
		Description: "System and application logs",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Logs may be useful for debugging issues",
	},
	"temp-files": {
		ID: "temp-files", Name: "Temporary Files", Group: "System Junk",
		Description: "Temporary files in /tmp and /var/folders",
		SafetyLevel: SafetySafe,
	},
	"trash": {
		ID: "trash", Name: "Trash", Group: "Storage",
		Description: "Files in the Trash bin",
		SafetyLevel: SafetySafe,
	},
	"downloads": {
		ID: "downloads", Name: "Old Downloads", Group: "Storage",
		Description: "Downloads older than 30 days",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "May contain important files you forgot about",
		SupportsFileSelection: true,
	},
	"browser-cache": {
		ID: "browser-cache", Name: "Browser Cache", Group: "Browsers",
		Description: "Cache from Chrome, Safari, Firefox, and Arc",
		SafetyLevel: SafetySafe,
	},
	"dev-cache": {
		ID: "dev-cache", Name: "Development Cache", Group: "Development",
		Description: "npm, yarn, pip, Xcode DerivedData, CocoaPods cache",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Projects will need to rebuild/reinstall dependencies",
	},
	"homebrew": {
		ID: "homebrew", Name: "Homebrew Cache", Group: "Development",
		Description: "Homebrew download cache and old versions",
		SafetyLevel: SafetySafe,
	},
	"docker": {
		ID: "docker", Name: "Docker", Group: "Development",
		Description: "Unused Docker images, containers, and volumes",
		SafetyLevel: SafetySafe,
	},
	"ios-backups": {
		ID: "ios-backups", Name: "iOS Backups", Group: "Storage",
		Description: "iPhone and iPad backup files",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "DANGER: You may lose important device backups permanently!",
	},
	"mail-attachments": {
		ID: "mail-attachments", Name: "Mail Attachments", Group: "Storage",
		Description: "Downloaded email attachments from Mail.app",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "May contain important documents and files",
	},
	"language-files": {
		ID: "language-files", Name: "Language Files", Group: "System Junk",
		Description: "Unused language localizations in applications",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "May break apps if you switch system language",
	},
	"large-files": {
		ID: "large-files", Name: "Large Files", Group: "Large Files",
		Description: "Files larger than 500MB for review",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "Review each file carefully before deleting",
		SupportsFileSelection: true,
	},
	"node-modules": {
		ID: "node-modules", Name: "Node Modules", Group: "Development",
		Description: "Orphaned node_modules in old projects",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Projects will need npm install to restore",
	},
	"duplicates": {
		ID: "duplicates", Name: "Duplicate Files", Group: "Storage",
		Description: "Files with identical content",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "Review carefully - keeps newest copy by default",
	},
	"launch-agents": {
		ID: "launch-agents", Name: "Orphaned Launch Agents", Group: "System Junk",
		Description: "Launch agents pointing to non-existent applications",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Removing launch agents will prevent applications from auto-starting. Only orphaned items (pointing to non-existent apps) are detected.",
	},
}

// categoryOrder is the stable display order: the spec §5 table order, which
// is the CLI's declaration order. Grouping by CategoryGroup for display is a
// frontend concern.
var categoryOrder = []CategoryID{
	"system-cache", "system-logs", "temp-files", "trash", "downloads",
	"browser-cache", "dev-cache", "homebrew", "docker", "ios-backups",
	"mail-attachments", "language-files", "large-files", "node-modules",
	"duplicates", "launch-agents",
}

// CategoriesInOrder returns all 16 categories in stable display order.
func CategoriesInOrder() []Category {
	out := make([]Category, 0, len(categoryOrder))
	for _, id := range categoryOrder {
		out = append(out, Categories[id])
	}
	return out
}
```

Create `internal/core/format.go`:

```go
package core

import "fmt"

// FormatSize renders a byte count in base-1024 units, CLI-compatible:
// bytes get 0 decimals ("512 B"), every larger unit exactly one decimal
// ("1.5 KB", "2.0 GB"). Values >= 1024 TB stay in TB (clamped — the
// sensible fix recommended in porting-notes for the CLI's unguarded index).
func FormatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(bytes) / 1024
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/core/ -v`
Expected: PASS — `TestCategoriesRegistryComplete` (16 subtests), `TestCategoriesInOrder`, `TestFormatSize` all ok.

- [ ] **Step 5: Commit**

```bash
git add internal/core/types.go internal/core/categories.go internal/core/format.go internal/core/categories_test.go internal/core/format_test.go
git commit -m "feat(core): domain types, 16-category registry, FormatSize" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 3: internal/fsx — Path Safety

**Files:**
- Create: `internal/fsx/safety.go`
- Test: `internal/fsx/safety_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (contract):
  - `func ValidatePathSafety(path string) string` — `""` = safe to delete; else human-readable refusal reason (CLI-verbatim messages).
  - `func IsProtectedPath(path string) bool`
- Package-local (documented seam — verifiers and later fsx tests rely on it): unexported package var `homeDir string`, initialized once from `os.UserHomeDir()`. Same-package tests swap it via the `setTestHome(t, home)` helper defined in `safety_test.go`. Production code never mutates it. Helpers `normalizePath(path) string` and `matchesRoot(p, root string) bool` are package-local and reused by later fsx tasks.

Semantics (from the contract and `mac-cleaner-cli/src/utils/fs.ts`):
- Paths are cleaned + absolutized first (`filepath.Abs`, which implies `Clean`; mirror of Node `path.resolve()` — never `EvalSymlinks`).
- ALLOWED overrides checked FIRST: `/tmp /private/tmp /var/tmp /private/var/tmp /var/folders /private/var/folders` (exact or prefix+`/`) — never protected.
- PROTECTED (exact or prefix+`/`): `/System /usr /bin /sbin /etc /var/log /var/db /var/root /private/var/db /private/var/root /private/var/log /Library/Apple /Applications/Utilities`.
- `ValidatePathSafety` additionally refuses `/` and the exact home directory. Refusal messages echo the caller's ORIGINAL path string, not the cleaned one (CLI parity).
- Prefix matching uses `root+"/"` so `/usr2` never false-matches `/usr`.

- [ ] **Step 1: Write the failing test**

Create `internal/fsx/safety_test.go`:

```go
package fsx

import "testing"

// setTestHome points the package's home-directory seam at a fake home for
// the duration of one test. Restored automatically via t.Cleanup.
func setTestHome(t *testing.T, home string) {
	t.Helper()
	old := homeDir
	homeDir = home
	t.Cleanup(func() { homeDir = old })
}

func TestValidatePathSafety(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	cases := []struct {
		name string
		path string
		want string // "" = safe; otherwise the exact refusal message
	}{
		{"tmp allowed", "/tmp/foo", ""},
		{"var folders allowed", "/var/folders/x", ""},
		{"private var folders allowed", "/private/var/folders/zz/abc/T/foo", ""},
		{"var tmp allowed", "/private/var/tmp/x", ""},
		{"system refused", "/System/Library", "Refusing to delete protected system path: /System/Library"},
		{"usr prefix refused", "/usr/local", "Refusing to delete protected system path: /usr/local"},
		{"usr exact refused", "/usr", "Refusing to delete protected system path: /usr"},
		{"usr2 NOT refused (no false prefix match)", "/usr2", ""},
		{"usr2 subpath NOT refused", "/usr2/bin/thing", ""},
		{"var log refused", "/var/log/system.log", "Refusing to delete protected system path: /var/log/system.log"},
		{"library apple refused", "/Library/Apple/x", "Refusing to delete protected system path: /Library/Apple/x"},
		{"applications utilities refused", "/Applications/Utilities/Terminal.app", "Refusing to delete protected system path: /Applications/Utilities/Terminal.app"},
		{"root refused", "/", "Refusing to delete root directory"},
		{"home refused", "/Users/testuser", "Refusing to delete home directory"},
		{"home with trailing slash refused", "/Users/testuser/", "Refusing to delete home directory"},
		{"home subdir allowed", "/Users/testuser/Library/Caches/Foo", ""},
		{"dot segments cleaned: escapes tmp into protected", "/tmp/../System/Library", "Refusing to delete protected system path: /tmp/../System/Library"},
		{"dot segments cleaned: lands in tmp", "/usr/../tmp/foo", ""},
		{"dot segments cleaned: collapses to home", "/Users/testuser/Downloads/..", "Refusing to delete home directory"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidatePathSafety(c.path); got != c.want {
				t.Errorf("ValidatePathSafety(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

func TestIsProtectedPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/tmp", false},
		{"/tmp/anything", false},
		{"/private/var/folders/xy/z/T", false},
		{"/System", true},
		{"/usr/local/bin", true},
		{"/usr2/bin", false},
		{"/etc/hosts", true},
		{"/private/var/db/x", true},
		{"/Applications/Utilities", true},
		{"/Applications/Foo.app", false},
		{"/Users/testuser", false}, // home is NOT "protected"; ValidatePathSafety handles it separately
	}
	for _, c := range cases {
		if got := IsProtectedPath(c.path); got != c.want {
			t.Errorf("IsProtectedPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/fsx/`
Expected: `FAIL github.com/guhcostan/app-cleaner/internal/fsx [build failed]` — `undefined: homeDir`, `undefined: ValidatePathSafety`, `undefined: IsProtectedPath`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/fsx/safety.go`:

```go
// Package fsx is App Cleaner's safe filesystem layer: path-safety
// validation, logical sizing, directory listing, and the deletion engine.
// It never imports Wails, never spawns processes, and its walk/size/list
// functions never return errors (unreadable = skip / size 0).
package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowedPaths are always deletable, even though some sit under otherwise
// protected prefixes (e.g. /var/folders). Checked BEFORE protectedPaths.
var allowedPaths = []string{
	"/tmp",
	"/private/tmp",
	"/var/tmp",
	"/private/var/tmp",
	"/var/folders",
	"/private/var/folders",
}

// protectedPaths are never deletable (exact match or prefix + "/").
var protectedPaths = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/etc",
	"/var/log",
	"/var/db",
	"/var/root",
	"/private/var/db",
	"/private/var/root",
	"/private/var/log",
	"/Library/Apple",
	"/Applications/Utilities",
}

// homeDir is the current user's home directory, resolved once at package
// initialization. It is deliberately a package variable rather than an
// os.UserHomeDir() call at check time: same-package tests reassign it to a
// fake home (see setTestHome in safety_test.go). Production code never
// mutates it. Empty string (home unknown) disables only the exact-home check.
var homeDir = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}()

// normalizePath mirrors Node's path.resolve(): absolutize against the
// current working directory and collapse "."/".." segments. It must NOT
// follow symlinks (never use filepath.EvalSymlinks here).
func normalizePath(path string) string {
	abs, err := filepath.Abs(path) // Abs also Cleans
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

// matchesRoot reports whether p equals root or lives under root (root+"/"
// prefix — so "/usr2" never matches root "/usr").
func matchesRoot(p, root string) bool {
	return p == root || strings.HasPrefix(p, root+"/")
}

// IsProtectedPath reports whether path falls under a protected system root.
// Allowed-path overrides are checked first: anything under /tmp, /var/tmp,
// or /var/folders (and their /private twins) is never protected.
func IsProtectedPath(path string) bool {
	p := normalizePath(path)
	for _, a := range allowedPaths {
		if matchesRoot(p, a) {
			return false
		}
	}
	for _, pr := range protectedPaths {
		if matchesRoot(p, pr) {
			return true
		}
	}
	return false
}

// ValidatePathSafety returns "" when path is safe to delete, or a
// human-readable refusal reason. Checks run on the cleaned absolute path;
// messages echo the caller's original path string (CLI parity).
func ValidatePathSafety(path string) string {
	p := normalizePath(path)
	if IsProtectedPath(p) {
		return fmt.Sprintf("Refusing to delete protected system path: %s", path)
	}
	if p == "/" {
		return "Refusing to delete root directory"
	}
	if homeDir != "" && p == filepath.Clean(homeDir) {
		return "Refusing to delete home directory"
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/fsx/ -v`
Expected: PASS — all `TestValidatePathSafety` subtests and `TestIsProtectedPath` ok.

- [ ] **Step 5: Commit**

```bash
git add internal/fsx/safety.go internal/fsx/safety_test.go
git commit -m "feat(fsx): path-safety validation with protected/allowed roots" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 4: internal/fsx — Listing & Sizing

**Files:**
- Create: `internal/fsx/listing.go`
- Test: `internal/fsx/listing_test.go`

**Interfaces:**
- Consumes: `core.CleanableItem` (Task 2).
- Produces (contract):
  - `func GetSize(path string) int64` — lstat-based logical size; dirs = recursive sum; symlink = size of the link itself (never followed); unreadable/missing = 0.
  - `func GetDirectoryItems(dir string) []core.CleanableItem` — immediate children, each fully sized, `ModifiedAt` = lstat mtime.
  - `type ItemFilter struct { MinAgeDays int; MinSize int64 }` (0 = filter disabled; age = now − lstat mtime compared in fractional days).
  - `func GetItems(dir string, f ItemFilter) []core.CleanableItem` — non-recursive children matching the filter.
- Package-local helpers (reused by later fsx work): `sizeFromInfo(path, info) int64`, `dirSize(dir) int64`, `listChildren(dir, f) []core.CleanableItem`. Test helpers `writeFile`, `mustMkdir`, `mustSymlink` defined here are reused by Task 5's tests (same package).

Semantics (CLI parity, `mac-cleaner-cli/src/utils/fs.ts`): everything uses `os.Lstat`/`os.ReadDir` (lstat semantics — a symlink to a directory is NOT a directory and is never entered). Age filter runs before sizing (cheap first). Unreadable dirs yield an empty (non-nil) slice; per-entry errors skip the entry.

- [ ] **Step 1: Write the failing test**

Create `internal/fsx/listing_test.go`:

```go
package fsx

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestGetSize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.txt"), 100)
	writeFile(t, filepath.Join(root, "big.bin"), 2000)
	mustMkdir(t, filepath.Join(root, "sub"))
	writeFile(t, filepath.Join(root, "sub", "a.txt"), 300)
	writeFile(t, filepath.Join(root, "sub", "b.txt"), 200)
	target := filepath.Join(root, "big.bin")
	mustSymlink(t, target, filepath.Join(root, "link"))
	// POSIX: a symlink's lstat size is the byte length of its target path.
	linkSize := int64(len(target))

	cases := []struct {
		name string
		path string
		want int64
	}{
		{"regular file", filepath.Join(root, "small.txt"), 100},
		{"directory is recursive logical sum", filepath.Join(root, "sub"), 500},
		{"symlink is link size, target never followed", filepath.Join(root, "link"), linkSize},
		{"whole tree counts the link, not its target", root, 100 + 2000 + 500 + linkSize},
		{"missing path is 0", filepath.Join(root, "nope"), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GetSize(c.path); got != c.want {
				t.Errorf("GetSize(%s) = %d, want %d", c.path, got, c.want)
			}
		})
	}
}

func TestGetSizeUnreadableDirIsZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits are ignored")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mustMkdir(t, locked)
	writeFile(t, filepath.Join(locked, "f"), 100)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if got := GetSize(locked); got != 0 {
		t.Errorf("GetSize(unreadable dir) = %d, want 0", got)
	}
}

func TestGetDirectoryItems(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "file.txt"), 100)
	mustMkdir(t, filepath.Join(root, "sub"))
	writeFile(t, filepath.Join(root, "sub", "inner.txt"), 400)
	target := filepath.Join(root, "file.txt")
	mustSymlink(t, target, filepath.Join(root, "link"))

	mtime := time.Date(2024, 6, 1, 10, 30, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(root, "file.txt"), mtime, mtime); err != nil {
		t.Fatal(err)
	}

	items := GetDirectoryItems(root)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(items), items)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range items {
		byName[it.Name] = it
	}

	f := byName["file.txt"]
	if f.Size != 100 || f.IsDirectory || f.Path != filepath.Join(root, "file.txt") {
		t.Errorf("file.txt item wrong: %+v", f)
	}
	if f.ModifiedAt == nil || !f.ModifiedAt.Equal(mtime) {
		t.Errorf("file.txt ModifiedAt = %v, want %v (lstat mtime)", f.ModifiedAt, mtime)
	}
	sub := byName["sub"]
	if sub.Size != 400 || !sub.IsDirectory {
		t.Errorf("sub item wrong (want fully-sized directory): %+v", sub)
	}
	link := byName["link"]
	if link.Size != int64(len(target)) || link.IsDirectory {
		t.Errorf("link item wrong (want lstat link size, not a directory): %+v", link)
	}

	if got := GetDirectoryItems(filepath.Join(root, "missing")); got == nil || len(got) != 0 {
		t.Errorf("missing dir: got %v, want empty non-nil slice", got)
	}
}

func TestGetItemsMinAgeDays(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	files := map[string]time.Time{
		"old.txt":        now.Add(-40 * 24 * time.Hour),
		"fresh.txt":      now.Add(-10 * 24 * time.Hour),
		"edge-under.txt": now.Add(-708 * time.Hour), // 29.5 days: fractional compare must reject
		"edge-over.txt":  now.Add(-732 * time.Hour), // 30.5 days: must pass
	}
	for name, mtime := range files {
		p := filepath.Join(dir, name)
		writeFile(t, p, 10)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	got := GetItems(dir, ItemFilter{MinAgeDays: 30})
	names := map[string]bool{}
	for _, it := range got {
		names[it.Name] = true
	}
	for _, want := range []string{"old.txt", "edge-over.txt"} {
		if !names[want] {
			t.Errorf("expected %s to pass the 30-day filter, got %v", want, names)
		}
	}
	for _, reject := range []string{"fresh.txt", "edge-under.txt"} {
		if names[reject] {
			t.Errorf("expected %s to be filtered out (age < 30 days, fractional-day compare)", reject)
		}
	}
}

func TestGetItemsMinSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "small.txt"), 100)
	writeFile(t, filepath.Join(dir, "big.bin"), 2000)
	mustMkdir(t, filepath.Join(dir, "bigdir"))
	writeFile(t, filepath.Join(dir, "bigdir", "content"), 1500)

	got := GetItems(dir, ItemFilter{MinSize: 1000})
	names := map[string]bool{}
	for _, it := range got {
		names[it.Name] = true
	}
	if len(got) != 2 || !names["big.bin"] || !names["bigdir"] {
		t.Errorf("MinSize filter: got %v, want exactly big.bin and bigdir (dirs sized recursively)", names)
	}
}

func TestGetItemsNoFilterReturnsAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), 1)
	writeFile(t, filepath.Join(dir, "b"), 2)
	if got := GetItems(dir, ItemFilter{}); len(got) != 2 {
		t.Errorf("no filter: got %d items, want 2", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/fsx/`
Expected: `FAIL github.com/guhcostan/app-cleaner/internal/fsx [build failed]` — `undefined: GetSize`, `undefined: GetDirectoryItems`, `undefined: ItemFilter`, `undefined: GetItems`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/fsx/listing.go`:

```go
package fsx

import (
	"os"
	"path/filepath"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// GetSize returns the logical size in bytes of the file, directory tree, or
// symlink at path. Symlinks report the size of the link itself and are never
// followed. Unreadable or missing paths report 0 — sizing never fails.
func GetSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	return sizeFromInfo(path, info)
}

// sizeFromInfo sizes an already-lstat'ed path. The symlink check must come
// before IsDir: a symlink to a directory is a link, not a directory.
func sizeFromInfo(path string, info os.FileInfo) int64 {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return info.Size()
	case info.IsDir():
		return dirSize(path)
	default:
		return info.Size()
	}
}

// dirSize recursively sums logical sizes under dir. Unreadable directories
// contribute 0; unreadable entries are skipped.
func dirSize(dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		total += sizeFromInfo(full, info)
	}
	return total
}

// ItemFilter filters GetItems results. Zero values disable each filter.
type ItemFilter struct {
	MinAgeDays int   // age = now − lstat mtime, compared in fractional days
	MinSize    int64 // minimum fully-computed size in bytes
}

// GetDirectoryItems lists dir's immediate children, each fully sized
// (directories recursively). Unreadable dir = empty slice, never an error.
func GetDirectoryItems(dir string) []core.CleanableItem {
	return listChildren(dir, ItemFilter{})
}

// GetItems lists dir's immediate children matching f (non-recursive).
func GetItems(dir string, f ItemFilter) []core.CleanableItem {
	return listChildren(dir, f)
}

func listChildren(dir string, f ItemFilter) []core.CleanableItem {
	items := []core.CleanableItem{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}
	now := time.Now()
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		if f.MinAgeDays > 0 {
			ageDays := now.Sub(info.ModTime()).Hours() / 24
			if ageDays < float64(f.MinAgeDays) {
				continue
			}
		}
		size := sizeFromInfo(full, info)
		if f.MinSize > 0 && size < f.MinSize {
			continue
		}
		mt := info.ModTime()
		items = append(items, core.CleanableItem{
			Path:        full,
			Size:        size,
			Name:        e.Name(),
			IsDirectory: info.IsDir(),
			ModifiedAt:  &mt,
		})
	}
	return items
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/fsx/ -v`
Expected: PASS — Task 3's safety tests plus `TestGetSize`, `TestGetSizeUnreadableDirIsZero`, `TestGetDirectoryItems`, `TestGetItemsMinAgeDays`, `TestGetItemsMinSize`, `TestGetItemsNoFilterReturnsAll` all ok.

- [ ] **Step 5: Commit**

```bash
git add internal/fsx/listing.go internal/fsx/listing_test.go
git commit -m "feat(fsx): lstat-based sizing, directory listing, item filters" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 5: internal/fsx — Deletion Engine

**Files:**
- Create: `internal/fsx/remove.go`
- Test: `internal/fsx/remove_test.go`

**Interfaces:**
- Consumes: `core.CleanableItem`, `core.ProgressFunc` (Task 2); `ValidatePathSafety` (Task 3).
- Produces (contract):
  - `type RemoveFailure struct { Path string; Code string }` (JSON tags `path`, `code`) — Code is one of `"PROTECTED" | "EPERM" | "EACCES" | "ENOENT" | "UNKNOWN"`.
  - `type RemoveOutcome struct { Cleaned int; Freed int64; Failures []RemoveFailure }` (JSON tags `cleaned`, `freed`, `failures`).
  - `func RemoveItems(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) RemoveOutcome`
  - `func AggregateFailures(fs []RemoveFailure) []string` — nil if empty; else exactly one string `"Failed to remove N items (32 EPERM, 8 EACCES)"`, codes sorted by count desc, then alphabetically on tie. The word is always "items" (contract-literal, even for N=1).
- Package-local helpers: `removeOne(path) string` ("" = success, else failure code) and `errnoCode(err) string`. Test file reuses `writeFile` from `listing_test.go` (same package).

Semantics (contract + `mac-cleaner-cli/src/utils/fs.ts`):
- Sequential; `progress` fires BEFORE each item, 1-based; nil progress allowed.
- `dryRun` short-circuits per item immediately — all items counted cleaned, full `item.Size` credited, zero disk IO, and (CLI parity) even safety validation is skipped.
- Per real item: `ValidatePathSafety` → code `PROTECTED`; fresh `os.Lstat` (TOCTOU guard) → `ENOENT` if the item vanished; symlink → `os.Remove` the link only (target must survive); anything else → `os.RemoveAll`.
- Freed space credits the scan-time `item.Size` — never re-measured at delete time.
- Errno mapping via `errors.As(&syscall.Errno)`: EPERM/EACCES/ENOENT verbatim, everything else `UNKNOWN`.
- Context cancellation stops between items: the in-flight item finishes, later items are untouched.

- [ ] **Step 1: Write the failing test**

Create `internal/fsx/remove_test.go`:

```go
package fsx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func mkItem(path string, size int64) core.CleanableItem {
	return core.CleanableItem{Path: path, Size: size, Name: filepath.Base(path)}
}

func TestRemoveItemsDryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, 10)
	writeFile(t, b, 20)
	items := []core.CleanableItem{
		mkItem(a, 10),
		mkItem(b, 20),
		// dry-run skips even safety validation (CLI parity): counted as cleaned
		mkItem("/System/Library/CoreServices", 999),
	}
	var calls [][2]int
	out := RemoveItems(context.Background(), items, true, func(cur, total int, item core.CleanableItem) {
		calls = append(calls, [2]int{cur, total})
	})
	if out.Cleaned != 3 || out.Freed != 1029 || len(out.Failures) != 0 {
		t.Errorf("dry-run outcome = %+v, want Cleaned=3 Freed=1029 no failures", out)
	}
	for _, p := range []string{a, b} {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("dry-run touched disk: %s is gone", p)
		}
	}
	wantCalls := [][2]int{{1, 3}, {2, 3}, {3, 3}}
	if len(calls) != len(wantCalls) {
		t.Fatalf("progress called %d times, want %d", len(calls), len(wantCalls))
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Errorf("progress call %d = %v, want %v (1-based)", i, calls[i], wantCalls[i])
		}
	}
}

func TestRemoveItemsProgressCalledBeforeDeletion(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeFile(t, a, 10)
	RemoveItems(context.Background(), []core.CleanableItem{mkItem(a, 10)}, false,
		func(cur, total int, item core.CleanableItem) {
			if _, err := os.Lstat(item.Path); err != nil {
				t.Errorf("progress fired after %s was already deleted", item.Path)
			}
		})
	if _, err := os.Lstat(a); !os.IsNotExist(err) {
		t.Errorf("item %s was not deleted", a)
	}
}

func TestRemoveItemsSymlinkRemovesLinkOnly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link")
	writeFile(t, target, 100)
	mustSymlink(t, target, link)
	out := RemoveItems(context.Background(), []core.CleanableItem{mkItem(link, 5)}, false, nil)
	if out.Cleaned != 1 || out.Freed != 5 {
		t.Fatalf("outcome = %+v, want Cleaned=1 Freed=5", out)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("symlink still exists")
	}
	if _, err := os.Lstat(target); err != nil {
		t.Errorf("symlink target was deleted — links must never be followed: %v", err)
	}
}

func TestRemoveItemsDirectoryUsesScanTimeSize(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	mustMkdir(t, filepath.Join(victim, "nested"))
	writeFile(t, filepath.Join(victim, "nested", "f.txt"), 100)
	out := RemoveItems(context.Background(), []core.CleanableItem{mkItem(victim, 12345)}, false, nil)
	if out.Cleaned != 1 || out.Freed != 12345 {
		t.Errorf("outcome = %+v, want Cleaned=1 Freed=12345 (scan-time size, never re-measured)", out)
	}
	if _, err := os.Lstat(victim); !os.IsNotExist(err) {
		t.Errorf("directory %s was not removed recursively", victim)
	}
}

func TestRemoveItemsProtectedPath(t *testing.T) {
	out := RemoveItems(context.Background(),
		[]core.CleanableItem{mkItem("/System/Library/CoreServices", 1)}, false, nil)
	if out.Cleaned != 0 || out.Freed != 0 {
		t.Errorf("outcome = %+v, want nothing cleaned", out)
	}
	if len(out.Failures) != 1 || out.Failures[0].Code != "PROTECTED" {
		t.Fatalf("failures = %+v, want one PROTECTED failure", out.Failures)
	}
	if out.Failures[0].Path != "/System/Library/CoreServices" {
		t.Errorf("failure path = %q, want the item path", out.Failures[0].Path)
	}
}

func TestRemoveItemsMissingPathENOENT(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.txt")
	out := RemoveItems(context.Background(), []core.CleanableItem{mkItem(missing, 10)}, false, nil)
	if out.Cleaned != 0 || len(out.Failures) != 1 {
		t.Fatalf("outcome = %+v, want one failure", out)
	}
	if out.Failures[0].Code != "ENOENT" {
		t.Errorf("code = %q, want ENOENT (re-Lstat found the item gone)", out.Failures[0].Code)
	}
}

func TestRemoveItemsCancellationStopsBetweenItems(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 3)
	items := make([]core.CleanableItem, 3)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		writeFile(t, paths[i], 10)
		items[i] = mkItem(paths[i], 10)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := RemoveItems(ctx, items, false, func(cur, total int, item core.CleanableItem) {
		if cur == 2 {
			cancel() // cancel while item 2 is in flight
		}
	})
	if out.Cleaned != 2 {
		t.Errorf("Cleaned = %d, want 2 (in-flight item finishes, later items stop)", out.Cleaned)
	}
	for i := 0; i < 2; i++ {
		if _, err := os.Lstat(paths[i]); !os.IsNotExist(err) {
			t.Errorf("item %d should have been deleted before cancellation", i+1)
		}
	}
	if _, err := os.Lstat(paths[2]); err != nil {
		t.Errorf("item 3 must survive cancellation, got lstat error %v", err)
	}
}

func TestErrnoCode(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&os.PathError{Op: "remove", Path: "/x", Err: syscall.EPERM}, "EPERM"},
		{&os.PathError{Op: "remove", Path: "/x", Err: syscall.EACCES}, "EACCES"},
		{&os.PathError{Op: "lstat", Path: "/x", Err: syscall.ENOENT}, "ENOENT"},
		{&os.PathError{Op: "remove", Path: "/x", Err: syscall.EROFS}, "UNKNOWN"},
		{errors.New("boom"), "UNKNOWN"},
	}
	for _, c := range cases {
		if got := errnoCode(c.err); got != c.want {
			t.Errorf("errnoCode(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestAggregateFailures(t *testing.T) {
	if got := AggregateFailures(nil); got != nil {
		t.Errorf("AggregateFailures(nil) = %v, want nil", got)
	}
	if got := AggregateFailures([]RemoveFailure{}); got != nil {
		t.Errorf("AggregateFailures(empty) = %v, want nil", got)
	}
	mk := func(code string, n int) []RemoveFailure {
		fs := make([]RemoveFailure, n)
		for i := range fs {
			fs[i] = RemoveFailure{Path: fmt.Sprintf("/x/%s-%d", code, i), Code: code}
		}
		return fs
	}
	cases := []struct {
		name     string
		failures []RemoveFailure
		want     string
	}{
		{"single code", mk("ENOENT", 1), "Failed to remove 1 items (1 ENOENT)"},
		{"count desc", append(mk("EACCES", 8), mk("EPERM", 32)...), "Failed to remove 40 items (32 EPERM, 8 EACCES)"},
		{"tie broken alphabetically", append(append(mk("EPERM", 3), mk("EACCES", 3)...), mk("ENOENT", 1)...), "Failed to remove 7 items (3 EACCES, 3 EPERM, 1 ENOENT)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AggregateFailures(c.failures)
			if len(got) != 1 || got[0] != c.want {
				t.Errorf("AggregateFailures = %v, want [%q]", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/fsx/`
Expected: `FAIL github.com/guhcostan/app-cleaner/internal/fsx [build failed]` — `undefined: RemoveItems`, `undefined: RemoveFailure`, `undefined: AggregateFailures`, `undefined: errnoCode`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/fsx/remove.go`:

```go
package fsx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// RemoveFailure records one item that could not be deleted.
// Code is one of "PROTECTED", "EPERM", "EACCES", "ENOENT", "UNKNOWN" —
// callers key on these strings (e.g. the UI's Full Disk Access hint).
type RemoveFailure struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// RemoveOutcome summarizes one RemoveItems batch.
type RemoveOutcome struct {
	Cleaned  int             `json:"cleaned"`
	Freed    int64           `json:"freed"`
	Failures []RemoveFailure `json:"failures"`
}

// RemoveItems permanently deletes items sequentially. progress (may be nil)
// fires BEFORE each item with a 1-based index. dryRun counts every item as
// cleaned with its full scan-time size and performs zero disk IO. Freed
// space always credits the scan-time item.Size — never re-measured.
// Context cancellation stops between items: the in-flight item completes,
// remaining items are left untouched.
func RemoveItems(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) RemoveOutcome {
	var out RemoveOutcome
	total := len(items)
	for i, item := range items {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress(i+1, total, item)
		}
		if dryRun {
			out.Cleaned++
			out.Freed += item.Size
			continue
		}
		if code := removeOne(item.Path); code != "" {
			out.Failures = append(out.Failures, RemoveFailure{Path: item.Path, Code: code})
			continue
		}
		out.Cleaned++
		out.Freed += item.Size
	}
	return out
}

// removeOne deletes a single path. Returns "" on success or a failure code.
func removeOne(path string) string {
	if reason := ValidatePathSafety(path); reason != "" {
		return "PROTECTED"
	}
	// TOCTOU guard: re-Lstat immediately before deleting — the path could
	// have vanished or been swapped for a symlink since scan time.
	info, err := os.Lstat(path)
	if err != nil {
		return errnoCode(err) // ENOENT when the item is already gone
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// Delete the link itself; never follow it.
		if err := os.Remove(path); err != nil {
			return errnoCode(err)
		}
		return ""
	}
	if err := os.RemoveAll(path); err != nil {
		return errnoCode(err)
	}
	return ""
}

// errnoCode maps an error to the CLI's errno string contract.
func errnoCode(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EPERM:
			return "EPERM"
		case syscall.EACCES:
			return "EACCES"
		case syscall.ENOENT:
			return "ENOENT"
		}
	}
	return "UNKNOWN"
}

// AggregateFailures collapses failures into the CLI's single summary line:
// "Failed to remove N items (32 EPERM, 8 EACCES)" — codes ordered by count
// desc, ties broken alphabetically. Returns nil for no failures.
func AggregateFailures(fs []RemoveFailure) []string {
	if len(fs) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, f := range fs {
		counts[f.Code]++
	}
	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool {
		if counts[codes[i]] != counts[codes[j]] {
			return counts[codes[i]] > counts[codes[j]]
		}
		return codes[i] < codes[j]
	})
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = fmt.Sprintf("%d %s", counts[c], c)
	}
	return []string{fmt.Sprintf("Failed to remove %d items (%s)", len(fs), strings.Join(parts, ", "))}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/fsx/ -v`
Expected: PASS — all Task 3/4 tests plus `TestRemoveItemsDryRunTouchesNothing`, `TestRemoveItemsProgressCalledBeforeDeletion`, `TestRemoveItemsSymlinkRemovesLinkOnly`, `TestRemoveItemsDirectoryUsesScanTimeSize`, `TestRemoveItemsProtectedPath`, `TestRemoveItemsMissingPathENOENT`, `TestRemoveItemsCancellationStopsBetweenItems`, `TestErrnoCode`, `TestAggregateFailures` ok.

- [ ] **Step 5: Commit**

```bash
git add internal/fsx/remove.go internal/fsx/remove_test.go
git commit -m "feat(fsx): sequential deletion engine with errno mapping and failure aggregation" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 6: internal/config — Load / Validate / Save

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing (standalone; scanners and app.go consume it later).
- Produces (contract):
  - `type ExtraPaths struct { NodeModules []string; Projects []string }` (JSON tags `nodeModules`, `projects`)
  - `type Config struct { DownloadsDaysOld int; LargeFilesMinSize int64; BackupByDefault bool; BackupRetentionDays int; Concurrency int; ShowRisky bool; KeepLanguages []string; ExtraPaths ExtraPaths }` (JSON tags per contract)
  - `func Default() Config` — 30 / 524288000 / true / 7 / 4 / false / `[]string{}` / empty ExtraPaths
  - `func Load(path string) Config` — never errors: missing file, file >100KB, or bad JSON ⇒ `Default()`; individually invalid fields dropped to their default with a warning via the standard `log` package
  - `func Save(c Config, path string) error` — `os.MkdirAll` the parent, indented JSON, mode 0644
  - `func DefaultPath(home string) string` — `<home>/Library/Application Support/AppCleaner/config.json`
- Package-local (documented seam): unexported package var `homeDir string` (same pattern as fsx — initialized from `os.UserHomeDir()`, reassigned by same-package tests via `setTestHome`), plus helpers `loadInt`, `loadInt64`, `loadBool`, `sanitizeExtraPaths`, `underAllowedRoot` and constants `maxConfigBytes = 100*1024`, `maxExtraPaths = 50`.

Validation rules (contract + `mac-cleaner-cli/src/utils/config.ts`):
- Bounds: `downloadsDaysOld` int 1..365; `largeFilesMinSize` int 1024..107374182400 (100 GiB); `backupRetentionDays` int 1..365; `concurrency` int 1..16. Non-integers (30.5) and wrong types are invalid. Invalid ⇒ keep default + `log` warning.
- Booleans: must be JSON booleans; wrong type ⇒ default (no JS-style truthiness coercion — deliberate simplification, noted).
- `keepLanguages`: must be a JSON string array; wrong type or JSON `null` ⇒ default `[]string{}`.
- `extraPaths.nodeModules` / `.projects`, each independently: keep only string elements → truncate to first 50 (CLI `.slice(0,50)` order: cap BEFORE per-entry validation) → expand leading `~/` (and bare `~`) against `homeDir` → `filepath.Abs` → resolved path must equal or live under `homeDir`, `/Users`, or `/Volumes` (prefix+`/` match) else dropped with warning. Load stores the RESOLVED absolute paths (scanners consume them as-is).

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// setTestHome points the package's home-directory seam at a fake home for
// the duration of one test. Restored automatically via t.Cleanup.
func setTestHome(t *testing.T, home string) {
	t.Helper()
	old := homeDir
	homeDir = home
	t.Cleanup(func() { homeDir = old })
}

func loadFrom(t *testing.T, content string) Config {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(p)
}

func TestDefault(t *testing.T) {
	want := Config{
		DownloadsDaysOld:    30,
		LargeFilesMinSize:   524288000,
		BackupByDefault:     true,
		BackupRetentionDays: 7,
		Concurrency:         4,
		ShowRisky:           false,
		KeepLanguages:       []string{},
		ExtraPaths:          ExtraPaths{NodeModules: []string{}, Projects: []string{}},
	}
	if got := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Default() = %+v, want %+v", got, want)
	}
}

func TestDefaultPath(t *testing.T) {
	got := DefaultPath("/Users/x")
	want := "/Users/x/Library/Application Support/AppCleaner/config.json"
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestLoadValidationBounds(t *testing.T) {
	cases := []struct {
		name string
		json string
		mut  func(c *Config) // applies the expected surviving change to Default()
	}{
		{"downloadsDaysOld below min dropped", `{"downloadsDaysOld":0}`, func(c *Config) {}},
		{"downloadsDaysOld above max dropped", `{"downloadsDaysOld":366}`, func(c *Config) {}},
		{"downloadsDaysOld min kept", `{"downloadsDaysOld":1}`, func(c *Config) { c.DownloadsDaysOld = 1 }},
		{"downloadsDaysOld max kept", `{"downloadsDaysOld":365}`, func(c *Config) { c.DownloadsDaysOld = 365 }},
		{"downloadsDaysOld non-integer dropped", `{"downloadsDaysOld":30.5}`, func(c *Config) {}},
		{"downloadsDaysOld wrong type dropped", `{"downloadsDaysOld":"abc"}`, func(c *Config) {}},
		{"largeFilesMinSize below 1KB dropped", `{"largeFilesMinSize":1023}`, func(c *Config) {}},
		{"largeFilesMinSize 1KB kept", `{"largeFilesMinSize":1024}`, func(c *Config) { c.LargeFilesMinSize = 1024 }},
		{"largeFilesMinSize 100GiB kept", `{"largeFilesMinSize":107374182400}`, func(c *Config) { c.LargeFilesMinSize = 107374182400 }},
		{"largeFilesMinSize above 100GiB dropped", `{"largeFilesMinSize":107374182401}`, func(c *Config) {}},
		{"backupRetentionDays zero dropped", `{"backupRetentionDays":0}`, func(c *Config) {}},
		{"backupRetentionDays above max dropped", `{"backupRetentionDays":366}`, func(c *Config) {}},
		{"backupRetentionDays kept", `{"backupRetentionDays":14}`, func(c *Config) { c.BackupRetentionDays = 14 }},
		{"concurrency zero dropped", `{"concurrency":0}`, func(c *Config) {}},
		{"concurrency 17 dropped", `{"concurrency":17}`, func(c *Config) {}},
		{"concurrency 1 kept", `{"concurrency":1}`, func(c *Config) { c.Concurrency = 1 }},
		{"concurrency 16 kept", `{"concurrency":16}`, func(c *Config) { c.Concurrency = 16 }},
		{"backupByDefault false kept", `{"backupByDefault":false}`, func(c *Config) { c.BackupByDefault = false }},
		{"backupByDefault wrong type dropped", `{"backupByDefault":"yes"}`, func(c *Config) {}},
		{"showRisky true kept", `{"showRisky":true}`, func(c *Config) { c.ShowRisky = true }},
		{"showRisky wrong type dropped", `{"showRisky":1}`, func(c *Config) {}},
		{"keepLanguages kept", `{"keepLanguages":["pt-BR","fr"]}`, func(c *Config) { c.KeepLanguages = []string{"pt-BR", "fr"} }},
		{"keepLanguages wrong type dropped", `{"keepLanguages":"pt"}`, func(c *Config) {}},
		{"keepLanguages null treated as absent", `{"keepLanguages":null}`, func(c *Config) {}},
		{"unknown fields ignored", `{"someFutureField":123}`, func(c *Config) {}},
		{"valid fields survive alongside invalid ones", `{"downloadsDaysOld":60,"concurrency":99}`, func(c *Config) { c.DownloadsDaysOld = 60 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := Default()
			c.mut(&want)
			if got := loadFrom(t, c.json); !reflect.DeepEqual(got, want) {
				t.Errorf("Load(%s) = %+v, want %+v", c.json, got, want)
			}
		})
	}
}

func TestLoadDegenerateInputs(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		got := Load(filepath.Join(t.TempDir(), "nope.json"))
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("missing file: got %+v, want defaults", got)
		}
	})
	t.Run("bad JSON", func(t *testing.T) {
		got := loadFrom(t, `{not json at all`)
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("bad JSON: got %+v, want defaults", got)
		}
	})
	t.Run("over 100KB uses defaults even when valid", func(t *testing.T) {
		pad := strings.Repeat("x", 100*1024)
		got := loadFrom(t, fmt.Sprintf(`{"downloadsDaysOld":90,"pad":"%s"}`, pad))
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("oversized config must be ignored entirely, got %+v", got)
		}
	})
}

func TestLoadWarnsOnInvalidField(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	loadFrom(t, `{"downloadsDaysOld":9999}`)
	if !strings.Contains(buf.String(), "downloadsDaysOld") {
		t.Errorf("expected a log warning naming downloadsDaysOld, got: %q", buf.String())
	}
}

func TestLoadExtraPaths(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	cases := []struct {
		name string
		json string
		want []string // expected NodeModules after load (resolved absolute paths)
	}{
		{"tilde expansion", `{"extraPaths":{"nodeModules":["~/Projects"],"projects":[]}}`, []string{"/Users/testuser/Projects"}},
		{"bare tilde resolves to home and is kept", `{"extraPaths":{"nodeModules":["~"],"projects":[]}}`, []string{"/Users/testuser"}},
		{"absolute under /Users kept", `{"extraPaths":{"nodeModules":["/Users/other/code"],"projects":[]}}`, []string{"/Users/other/code"}},
		{"absolute under /Volumes kept", `{"extraPaths":{"nodeModules":["/Volumes/Ext/repos"],"projects":[]}}`, []string{"/Volumes/Ext/repos"}},
		{"outside allowed roots dropped", `{"extraPaths":{"nodeModules":["/etc/nginx","/tmp/work","/opt/stuff"],"projects":[]}}`, []string{}},
		{"traversal out of home dropped", `{"extraPaths":{"nodeModules":["~/../../etc"],"projects":[]}}`, []string{}},
		{"non-strings filtered", `{"extraPaths":{"nodeModules":[42,true,null,"~/ok"],"projects":[]}}`, []string{"/Users/testuser/ok"}},
		{"dot segments cleaned", `{"extraPaths":{"nodeModules":["/Users/other/../other2/code"],"projects":[]}}`, []string{"/Users/other2/code"}},
		{"wrong container type keeps defaults", `{"extraPaths":"nope"}`, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := loadFrom(t, c.json)
			if !reflect.DeepEqual(got.ExtraPaths.NodeModules, c.want) {
				t.Errorf("NodeModules = %v, want %v", got.ExtraPaths.NodeModules, c.want)
			}
			if len(got.ExtraPaths.Projects) != 0 {
				t.Errorf("Projects = %v, want empty", got.ExtraPaths.Projects)
			}
		})
	}
}

func TestLoadExtraPathsProjectsValidatedIndependently(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	got := loadFrom(t, `{"extraPaths":{"nodeModules":[],"projects":["~/work","/etc/bad"]}}`)
	want := []string{"/Users/testuser/work"}
	if !reflect.DeepEqual(got.ExtraPaths.Projects, want) {
		t.Errorf("Projects = %v, want %v", got.ExtraPaths.Projects, want)
	}
	if len(got.ExtraPaths.NodeModules) != 0 {
		t.Errorf("NodeModules = %v, want empty", got.ExtraPaths.NodeModules)
	}
}

func TestLoadExtraPathsCap50(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	entries := make([]string, 60)
	for i := range entries {
		entries[i] = fmt.Sprintf("%q", fmt.Sprintf("~/p%02d", i))
	}
	got := loadFrom(t, fmt.Sprintf(`{"extraPaths":{"nodeModules":[%s],"projects":[]}}`, strings.Join(entries, ",")))
	if len(got.ExtraPaths.NodeModules) != 50 {
		t.Fatalf("got %d paths, want capped at 50", len(got.ExtraPaths.NodeModules))
	}
	if got.ExtraPaths.NodeModules[0] != "/Users/testuser/p00" || got.ExtraPaths.NodeModules[49] != "/Users/testuser/p49" {
		t.Errorf("cap kept wrong entries: first=%s last=%s", got.ExtraPaths.NodeModules[0], got.ExtraPaths.NodeModules[49])
	}
}

func TestSaveCreatesDirsAndRoundTrips(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Library", "Application Support", "AppCleaner", "config.json")
	c := Default()
	c.DownloadsDaysOld = 60
	c.ShowRisky = true
	if err := Save(c, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "  \"downloadsDaysOld\": 60") {
		t.Errorf("expected 2-space-indented JSON, got:\n%s", data)
	}
	if got := Load(p); !reflect.DeepEqual(got, c) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, c)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/config/`
Expected: `FAIL github.com/guhcostan/app-cleaner/internal/config [build failed]` — `undefined: homeDir`, `undefined: Config`, `undefined: Default`, `undefined: DefaultPath`, `undefined: Load`, `undefined: ExtraPaths`, `undefined: Save`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/config/config.go`:

```go
// Package config loads, validates, and saves App Cleaner's JSON
// configuration. Load never fails: unusable files fall back to Default()
// and individually invalid fields are dropped to their defaults with a
// warning on the standard log package. It never imports Wails.
package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxConfigBytes = 100 * 1024
	maxExtraPaths  = 50
)

// homeDir is the current user's home directory, resolved once at package
// initialization. Package variable (not a call site) so same-package tests
// can point it at a fake home via setTestHome; production never mutates it.
var homeDir = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}()

type ExtraPaths struct {
	NodeModules []string `json:"nodeModules"`
	Projects    []string `json:"projects"`
}

type Config struct {
	DownloadsDaysOld    int        `json:"downloadsDaysOld"`    // default 30, valid 1..365
	LargeFilesMinSize   int64      `json:"largeFilesMinSize"`   // default 524288000, valid 1024..100GiB
	BackupByDefault     bool       `json:"backupByDefault"`     // default true
	BackupRetentionDays int        `json:"backupRetentionDays"` // default 7, valid 1..365
	Concurrency         int        `json:"concurrency"`         // default 4, valid 1..16
	ShowRisky           bool       `json:"showRisky"`           // default false
	KeepLanguages       []string   `json:"keepLanguages"`       // extra .lproj basenames to keep
	ExtraPaths          ExtraPaths `json:"extraPaths"`          // ≤50 each; resolved under $HOME, /Users, or /Volumes
}

// Default returns the hard defaults.
func Default() Config {
	return Config{
		DownloadsDaysOld:    30,
		LargeFilesMinSize:   524288000,
		BackupByDefault:     true,
		BackupRetentionDays: 7,
		Concurrency:         4,
		ShowRisky:           false,
		KeepLanguages:       []string{},
		ExtraPaths:          ExtraPaths{NodeModules: []string{}, Projects: []string{}},
	}
}

// DefaultPath returns the canonical config location for a home directory.
func DefaultPath(home string) string {
	return filepath.Join(home, "Library", "Application Support", "AppCleaner", "config.json")
}

// Load reads path and overlays its valid fields onto Default(). It never
// returns an error: a missing/unreadable file, a file over 100KB, or
// syntactically invalid JSON all yield Default(); an individually invalid
// field keeps its default and logs a warning.
func Load(path string) Config {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if len(data) > maxConfigBytes {
		log.Printf("config: %s is larger than 100KB, using defaults", path)
		return cfg
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("config: invalid JSON in %s, using defaults: %v", path, err)
		return cfg
	}
	loadInt(raw, "downloadsDaysOld", 1, 365, &cfg.DownloadsDaysOld)
	loadInt64(raw, "largeFilesMinSize", 1024, 100*1024*1024*1024, &cfg.LargeFilesMinSize)
	loadBool(raw, "backupByDefault", &cfg.BackupByDefault)
	loadInt(raw, "backupRetentionDays", 1, 365, &cfg.BackupRetentionDays)
	loadInt(raw, "concurrency", 1, 16, &cfg.Concurrency)
	loadBool(raw, "showRisky", &cfg.ShowRisky)
	if v, ok := raw["keepLanguages"]; ok {
		var langs []string
		if err := json.Unmarshal(v, &langs); err != nil {
			log.Printf("config: invalid keepLanguages, keeping default: %v", err)
		} else if langs != nil { // JSON null decodes to nil: treat as absent
			cfg.KeepLanguages = langs
		}
	}
	if v, ok := raw["extraPaths"]; ok {
		var ep struct {
			NodeModules []any `json:"nodeModules"`
			Projects    []any `json:"projects"`
		}
		if err := json.Unmarshal(v, &ep); err != nil {
			log.Printf("config: invalid extraPaths, keeping default: %v", err)
		} else {
			cfg.ExtraPaths.NodeModules = sanitizeExtraPaths(ep.NodeModules)
			cfg.ExtraPaths.Projects = sanitizeExtraPaths(ep.Projects)
		}
	}
	return cfg
}

func loadInt(raw map[string]json.RawMessage, key string, min, max int, dst *int) {
	v, ok := raw[key]
	if !ok {
		return
	}
	var n int
	if err := json.Unmarshal(v, &n); err != nil || n < min || n > max {
		log.Printf("config: invalid %s %s (valid %d..%d), keeping default %d", key, string(v), min, max, *dst)
		return
	}
	*dst = n
}

func loadInt64(raw map[string]json.RawMessage, key string, min, max int64, dst *int64) {
	v, ok := raw[key]
	if !ok {
		return
	}
	var n int64
	if err := json.Unmarshal(v, &n); err != nil || n < min || n > max {
		log.Printf("config: invalid %s %s (valid %d..%d), keeping default %d", key, string(v), min, max, *dst)
		return
	}
	*dst = n
}

func loadBool(raw map[string]json.RawMessage, key string, dst *bool) {
	v, ok := raw[key]
	if !ok {
		return
	}
	var b bool
	if err := json.Unmarshal(v, &b); err != nil {
		log.Printf("config: invalid %s %s (want true/false), keeping default %v", key, string(v), *dst)
		return
	}
	*dst = b
}

// sanitizeExtraPaths applies the CLI's extraPaths rules: keep only strings,
// truncate to the first 50 BEFORE validation (CLI .slice(0,50) order),
// expand "~"/"~/" against homeDir, resolve to an absolute clean path, and
// require the result under homeDir, /Users, or /Volumes. Returns resolved
// absolute paths; never nil.
func sanitizeExtraPaths(vals []any) []string {
	strs := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok {
			strs = append(strs, s)
		}
	}
	if len(strs) > maxExtraPaths {
		log.Printf("config: extraPaths list truncated to %d entries", maxExtraPaths)
		strs = strs[:maxExtraPaths]
	}
	out := []string{}
	for _, s := range strs {
		p := s
		if p == "~" {
			p = homeDir
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(homeDir, p[2:])
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			log.Printf("config: skipping unresolvable path: %s", s)
			continue
		}
		if !underAllowedRoot(abs) {
			log.Printf("config: skipping path outside allowed directories: %s", s)
			continue
		}
		out = append(out, abs)
	}
	return out
}

func underAllowedRoot(abs string) bool {
	for _, root := range []string{homeDir, "/Users", "/Volumes"} {
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if abs == root || strings.HasPrefix(abs, root+"/") {
			return true
		}
	}
	return false
}

// Save writes c to path as indented JSON, creating parent directories.
func Save(c Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner && go test ./internal/config/ -v`
Expected: PASS — `TestDefault`, `TestDefaultPath`, `TestLoadValidationBounds` (all subtests), `TestLoadDegenerateInputs`, `TestLoadWarnsOnInvalidField`, `TestLoadExtraPaths`, `TestLoadExtraPathsProjectsValidatedIndependently`, `TestLoadExtraPathsCap50`, `TestSaveCreatesDirsAndRoundTrips` ok.

Also run the full engine suite to confirm nothing regressed: `go test ./internal/...` / Expected: `ok` for `internal/core`, `internal/fsx`, `internal/config`.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): load/validate/save with bounds and extraPaths sanitization" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 7: Scanner Infrastructure (interface, exec runner, parallel scan runner)

**Files:**
- Create: `internal/scanners/exec.go`
- Create: `internal/scanners/scanners.go`
- Create: `internal/scanners/runner.go`
- Test: `internal/scanners/exec_test.go`
- Test: `internal/scanners/testhelpers_test.go`
- Test: `internal/scanners/scanners_test.go`
- Test: `internal/scanners/runner_test.go`

**Interfaces:**
- Consumes (from earlier tasks / contract):
  - `core.Category`, `core.CategoryID`, `core.CleanableItem`, `core.ScanResult`, `core.ScanSummary`, `core.CleanResult`, `core.ProgressFunc`, `core.Categories map[core.CategoryID]core.Category`, `func core.CategoriesInOrder() []core.Category`
  - `func fsx.RemoveItems(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) fsx.RemoveOutcome` (fields `Cleaned int`, `Freed int64`, `Failures []fsx.RemoveFailure`)
  - `func fsx.AggregateFailures(fs []fsx.RemoveFailure) []string`
  - `config.Config`, `func config.Default() config.Config`
- Produces (contract names — later tasks and `app.go` rely on these exactly):
  - `type CmdRunner interface { Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (stdout string, err error) }`
  - `type ExecRunner struct{}` (real `CmdRunner`)
  - `type Roots struct { Home, Tmp, VarFolders, Applications string }`, `func DefaultRoots() Roots`
  - `type Options struct { Roots Roots; Cfg config.Config; Runner CmdRunner }`
  - `type Scanner interface { Category() core.Category; Scan(ctx context.Context, opts Options) core.ScanResult; Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult }`
  - `func All() []Scanner`, `func Get(id core.CategoryID) (Scanner, bool)`
  - `func RunScans(ctx context.Context, ids []core.CategoryID, opts Options, concurrency int, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary`
- Produces (package-local, used by Tasks 8–12):
  - `func register(s Scanner)` — called from each scanner file's `init()`; the registry grows as scanner tasks land. `All()` returns the registered subset in `core.CategoriesInOrder()` order; the final scanner task asserts `len(All()) == 16`.
  - `func newScanResult(cat core.Category, items []core.CleanableItem) core.ScanResult` — sums `TotalSize`, guarantees non-nil `Items`.
  - `func cleanWithFsx(cat core.Category, ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult` — shared Clean for all FS-backed scanners; coalesces `AggregateFailures`'s nil to `[]string{}` for stable JSON.
  - Test helpers (same package, `_test.go`): `mkFile(t, path, size)`, `mkDir(t, path)`, `testRoots(t) Roots`, `testOptions(t) Options`, `registerFake(t, s Scanner)`, `type fakeScanner`.

- [ ] **Step 1: Write the failing test for CmdRunner/ExecRunner**

Create `internal/scanners/exec_test.go`. Note: this is the ONE place real binaries run in tests (`/bin/echo`, `/bin/cat`, `/bin/sleep` — always present on macOS, no shell involved). Everything else in the suite fakes `CmdRunner`.

```go
package scanners

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecRunnerCapturesStdout(t *testing.T) {
	var r ExecRunner
	out, err := r.Run(context.Background(), 5*time.Second, "/bin/echo", "hello", "world")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(out) != "hello world" {
		t.Fatalf("stdout = %q, want %q", out, "hello world")
	}
}

func TestExecRunnerNonZeroExitCarriesStderr(t *testing.T) {
	var r ExecRunner
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")
	_, err := r.Run(context.Background(), 5*time.Second, "/bin/cat", missing)
	if err == nil {
		t.Fatal("want error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "No such file or directory") {
		t.Fatalf("error %q does not carry stderr text", err)
	}
}

func TestExecRunnerRejectsRelativeBinaryPath(t *testing.T) {
	var r ExecRunner
	_, err := r.Run(context.Background(), 5*time.Second, "echo", "hi")
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("err = %v, want absolute-path refusal", err)
	}
}

func TestExecRunnerEnforcesTimeout(t *testing.T) {
	var r ExecRunner
	start := time.Now()
	_, err := r.Run(context.Background(), 150*time.Millisecond, "/bin/sleep", "5")
	if err == nil {
		t.Fatal("want timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout not enforced, took %s", elapsed)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout message", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestExecRunner -v`
Expected: FAIL — `[build failed]` with `undefined: ExecRunner` (package has no non-test file yet, so the build error is the failing state).

- [ ] **Step 3: Write minimal implementation**

Create `internal/scanners/exec.go`:

```go
package scanners

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CmdRunner abstracts external binaries (brew, docker, ...) so scanner and
// maintenance-style code can be tested with fakes. Implementations must never
// spawn a shell: bin must be an absolute path, args a plain argv slice.
type CmdRunner interface {
	Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (stdout string, err error)
}

// ExecRunner is the real CmdRunner: exec.CommandContext with a per-call
// timeout. Non-zero exit returns an error carrying the trimmed stderr text.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	if !filepath.IsAbs(bin) {
		return "", fmt.Errorf("binary path must be absolute, got %q", bin)
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			return stdout.String(), fmt.Errorf("%s timed out after %s", filepath.Base(bin), timeout)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s failed: %s", filepath.Base(bin), detail)
	}
	return stdout.String(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run TestExecRunner -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Write the failing tests for Roots, registry, newScanResult, cleanWithFsx**

Create `internal/scanners/testhelpers_test.go` (helpers reused by Tasks 8–10):

```go
package scanners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/config"
)

// mkFile creates a file at path (creating parent dirs) filled with size bytes.
func mkFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkDir creates a directory (with parents).
func mkDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// testRoots returns Roots pointing at fresh temp dirs. Only Home exists on
// disk; Tmp/VarFolders/Applications are created lazily by tests that need them.
func testRoots(t *testing.T) Roots {
	t.Helper()
	base := t.TempDir()
	r := Roots{
		Home:         filepath.Join(base, "home"),
		Tmp:          filepath.Join(base, "tmp"),
		VarFolders:   filepath.Join(base, "varfolders"),
		Applications: filepath.Join(base, "apps"),
	}
	mkDir(t, r.Home)
	return r
}

// testOptions returns Options wired to temp roots and the default config.
func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{Roots: testRoots(t), Cfg: config.Default()}
}

// registerFake registers a scanner for the duration of one test. Fake
// category ids must not collide with the 16 real ones (use "fake-*").
func registerFake(t *testing.T, s Scanner) {
	t.Helper()
	id := s.Category().ID
	if _, exists := registry[id]; exists {
		t.Fatalf("scanner %s already registered", id)
	}
	registry[id] = s
	t.Cleanup(func() { delete(registry, id) })
}
```

Create `internal/scanners/scanners_test.go`:

```go
package scanners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestDefaultRoots(t *testing.T) {
	r := DefaultRoots()
	if r.Home == "" {
		t.Fatal("Home must not be empty")
	}
	if r.Tmp != "/tmp" || r.VarFolders != "/private/var/folders" || r.Applications != "/Applications" {
		t.Fatalf("unexpected roots: %+v", r)
	}
}

func TestGetUnknownCategory(t *testing.T) {
	if _, ok := Get("definitely-not-a-category"); ok {
		t.Fatal("Get must return ok=false for unknown ids")
	}
}

func TestAllReturnsRegisteredScannersInDisplayOrder(t *testing.T) {
	// Stays valid as scanner tasks land: All() must equal the registered
	// subset of core.CategoriesInOrder(), in that order.
	var want []core.CategoryID
	for _, c := range core.CategoriesInOrder() {
		if _, ok := registry[c.ID]; ok {
			want = append(want, c.ID)
		}
	}
	var got []core.CategoryID
	for _, s := range All() {
		got = append(got, s.Category().ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("All() ids = %v, want %v", got, want)
	}
}

func TestNewScanResult(t *testing.T) {
	cat := core.Categories["trash"]
	r := newScanResult(cat, nil)
	if r.Items == nil || len(r.Items) != 0 || r.TotalSize != 0 || r.Error != "" {
		t.Fatalf("empty result malformed: %+v", r)
	}
	r = newScanResult(cat, []core.CleanableItem{{Size: 7}, {Size: 5}})
	if r.TotalSize != 12 || r.Category.ID != "trash" {
		t.Fatalf("result = %+v, want TotalSize 12 for trash", r)
	}
}

func TestCleanWithFsxRealDelete(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	mkFile(t, f1, 10)
	items := []core.CleanableItem{
		{Path: f1, Size: 10, Name: "a.txt"},
		{Path: "/System/never-delete-me", Size: 5, Name: "never"}, // PROTECTED, no disk IO
	}
	var progressed []string
	res := cleanWithFsx(core.Categories["trash"], context.Background(), items, false,
		func(current, total int, it core.CleanableItem) {
			progressed = append(progressed, fmt.Sprintf("%d/%d %s", current, total, it.Name))
		})
	if res.Category.ID != "trash" {
		t.Fatalf("Category = %+v, want trash", res.Category)
	}
	if res.CleanedItems != 1 || res.FreedSpace != 10 {
		t.Fatalf("CleanedItems=%d FreedSpace=%d, want 1 and 10", res.CleanedItems, res.FreedSpace)
	}
	wantErrs := []string{"Failed to remove 1 items (1 PROTECTED)"}
	if !reflect.DeepEqual(res.Errors, wantErrs) {
		t.Fatalf("Errors = %v, want %v", res.Errors, wantErrs)
	}
	if _, err := os.Lstat(f1); !os.IsNotExist(err) {
		t.Fatal("a.txt should have been deleted")
	}
	if !reflect.DeepEqual(progressed, []string{"1/2 a.txt", "2/2 never"}) {
		t.Fatalf("progress calls = %v, want before-each-item 1-based calls", progressed)
	}
}

func TestCleanWithFsxDryRun(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	mkFile(t, f1, 10)
	items := []core.CleanableItem{
		{Path: f1, Size: 10, Name: "a.txt"},
		{Path: "/System/never-delete-me", Size: 5, Name: "never"},
	}
	res := cleanWithFsx(core.Categories["trash"], context.Background(), items, true, nil)
	if res.CleanedItems != 2 || res.FreedSpace != 15 || len(res.Errors) != 0 {
		t.Fatalf("dry run = %+v, want all items credited, no errors", res)
	}
	if _, err := os.Lstat(f1); err != nil {
		t.Fatal("dry run must not touch disk")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestDefaultRoots|TestGetUnknownCategory|TestAllReturns|TestNewScanResult|TestCleanWithFsx' -v`
Expected: FAIL — `[build failed]` with `undefined: Roots`, `undefined: Options`, `undefined: registry`, etc.

- [ ] **Step 7: Write minimal implementation**

Create `internal/scanners/scanners.go`:

```go
// Package scanners implements the 16 cleaning-category scanners and the
// parallel scan runner. Engine-only: never imports Wails.
package scanners

import (
	"context"
	"os"

	"github.com/guhcostan/app-cleaner/internal/config"
	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// Roots is the filesystem test seam: every scanner derives its paths from
// these roots instead of hardcoding absolute paths.
type Roots struct {
	Home         string // default os.UserHomeDir()
	Tmp          string // "/tmp"
	VarFolders   string // "/private/var/folders"
	Applications string // "/Applications"
}

// DefaultRoots returns the real macOS roots.
func DefaultRoots() Roots {
	home, _ := os.UserHomeDir()
	return Roots{
		Home:         home,
		Tmp:          "/tmp",
		VarFolders:   "/private/var/folders",
		Applications: "/Applications",
	}
}

// Options carries everything a scanner may need. The same value is passed to
// every scanner in a batch run.
type Options struct {
	Roots  Roots
	Cfg    config.Config
	Runner CmdRunner
}

// Scanner is implemented once per category.
type Scanner interface {
	Category() core.Category
	Scan(ctx context.Context, opts Options) core.ScanResult
	Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult
}

// registry maps category id -> singleton scanner. Populated by register()
// from each scanner file's init(); grows as scanner tasks land. The final
// scanner task asserts len(All()) == 16.
var registry = map[core.CategoryID]Scanner{}

func register(s Scanner) { registry[s.Category().ID] = s }

// Get returns the scanner registered for id.
func Get(id core.CategoryID) (Scanner, bool) {
	s, ok := registry[id]
	return s, ok
}

// All returns every registered scanner in stable display order
// (core.CategoriesInOrder()).
func All() []Scanner {
	var out []Scanner
	for _, c := range core.CategoriesInOrder() {
		if s, ok := registry[c.ID]; ok {
			out = append(out, s)
		}
	}
	return out
}

// newScanResult assembles a ScanResult with TotalSize = sum of item sizes.
// Items is never nil so the frontend always receives a JSON array.
func newScanResult(cat core.Category, items []core.CleanableItem) core.ScanResult {
	if items == nil {
		items = []core.CleanableItem{}
	}
	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: cat, Items: items, TotalSize: total}
}

// cleanWithFsx is the shared Clean implementation for every filesystem-backed
// scanner: sequential fsx.RemoveItems + one aggregated error string.
func cleanWithFsx(cat core.Category, ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	out := fsx.RemoveItems(ctx, items, dryRun, progress)
	errs := fsx.AggregateFailures(out.Failures)
	if errs == nil {
		errs = []string{}
	}
	return core.CleanResult{
		Category:     cat,
		CleanedItems: out.Cleaned,
		FreedSpace:   out.Freed,
		Errors:       errs,
	}
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run 'TestDefaultRoots|TestGetUnknownCategory|TestAllReturns|TestNewScanResult|TestCleanWithFsx' -v`
Expected: PASS (6 tests).

- [ ] **Step 9: Write the failing test for RunScans**

Create `internal/scanners/runner_test.go`. Two fake scanners — one returns items, one panics — prove isolation, totals, and progress ordering; a third set proves the semaphore.

```go
package scanners

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

type fakeScanner struct {
	cat  core.Category
	scan func(ctx context.Context, opts Options) core.ScanResult
}

func (f fakeScanner) Category() core.Category { return f.cat }

func (f fakeScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	return f.scan(ctx, opts)
}

func (f fakeScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(f.cat, ctx, items, dryRun, progress)
}

func TestRunScansTotalsPanicIsolationAndProgress(t *testing.T) {
	goodCat := core.Category{ID: "fake-good", Name: "Fake Good"}
	badCat := core.Category{ID: "fake-bad", Name: "Fake Bad"}
	registerFake(t, fakeScanner{cat: goodCat, scan: func(ctx context.Context, opts Options) core.ScanResult {
		return core.ScanResult{Category: goodCat, Items: []core.CleanableItem{
			{Path: "/x/a", Size: 100, Name: "a"},
			{Path: "/x/b", Size: 50, Name: "b"},
		}, TotalSize: 150}
	}})
	registerFake(t, fakeScanner{cat: badCat, scan: func(ctx context.Context, opts Options) core.ScanResult {
		panic("boom")
	}})

	type call struct{ completed, total int }
	var calls []call // safe: RunScans invokes onResult serially under its lock
	sum := RunScans(context.Background(), []core.CategoryID{"fake-good", "fake-bad"}, Options{}, 2,
		func(completed, total int, r core.ScanResult) {
			calls = append(calls, call{completed, total})
		})

	if len(sum.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(sum.Results))
	}
	if sum.Results[0].Category.ID != "fake-good" || sum.Results[1].Category.ID != "fake-bad" {
		t.Fatalf("results not in input id order: %s, %s",
			sum.Results[0].Category.ID, sum.Results[1].Category.ID)
	}
	if !strings.Contains(sum.Results[1].Error, "boom") {
		t.Fatalf("panicking scanner Error = %q, want it to mention the panic value", sum.Results[1].Error)
	}
	if sum.Results[1].Category != badCat {
		t.Fatalf("panicking scanner must keep its Category, got %+v", sum.Results[1].Category)
	}
	if sum.TotalSize != 150 || sum.TotalItems != 2 {
		t.Fatalf("TotalSize=%d TotalItems=%d, want 150 and 2", sum.TotalSize, sum.TotalItems)
	}
	if len(calls) != 2 {
		t.Fatalf("onResult called %d times, want 2", len(calls))
	}
	for i, c := range calls {
		if c.completed != i+1 || c.total != 2 {
			t.Fatalf("progress call %d = %+v, want completed=%d total=2 (monotonic)", i, c, i+1)
		}
	}
}

func TestRunScansUnknownCategory(t *testing.T) {
	sum := RunScans(context.Background(), []core.CategoryID{"no-such"}, Options{}, 4, nil)
	if len(sum.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(sum.Results))
	}
	r := sum.Results[0]
	if r.Category.ID != "no-such" || !strings.Contains(r.Error, "unknown scanner category") {
		t.Fatalf("result = %+v, want Category.ID preserved and unknown-category error", r)
	}
}

func TestRunScansHonorsConcurrencyLimit(t *testing.T) {
	var active, maxActive int32
	mk := func(id string) fakeScanner {
		cat := core.Category{ID: core.CategoryID(id)}
		return fakeScanner{cat: cat, scan: func(ctx context.Context, opts Options) core.ScanResult {
			n := atomic.AddInt32(&active, 1)
			for {
				m := atomic.LoadInt32(&maxActive)
				if n <= m || atomic.CompareAndSwapInt32(&maxActive, m, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			return core.ScanResult{Category: cat}
		}}
	}
	ids := []core.CategoryID{"fake-c1", "fake-c2", "fake-c3"}
	for _, id := range ids {
		registerFake(t, mk(string(id)))
	}
	RunScans(context.Background(), ids, Options{}, 1, nil)
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("max concurrent scanners = %d, want 1", got)
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestRunScans -v`
Expected: FAIL — `[build failed]` with `undefined: RunScans`.

- [ ] **Step 11: Write minimal implementation**

Create `internal/scanners/runner.go`:

```go
package scanners

import (
	"context"
	"fmt"
	"sync"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// RunScans runs the scanners for ids in parallel, bounded by a semaphore of
// size concurrency (clamped to 1..len(ids)). Results[i] always corresponds to
// ids[i]. A scanner that panics — or an id with no registered scanner —
// yields ScanResult{Category, Error}; the batch never aborts. onResult (may
// be nil) is invoked after each scanner finishes, serially under the runner's
// lock, with completed = 1..total strictly increasing.
func RunScans(ctx context.Context, ids []core.CategoryID, opts Options, concurrency int,
	onResult func(completed, total int, r core.ScanResult)) core.ScanSummary {

	total := len(ids)
	if total == 0 {
		return core.ScanSummary{Results: []core.ScanResult{}}
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > total {
		concurrency = total
	}

	results := make([]core.ScanResult, total)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	for i, id := range ids {
		wg.Add(1)
		go func(i int, id core.CategoryID) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := runOne(ctx, id, opts)

			mu.Lock()
			defer mu.Unlock()
			results[i] = r
			completed++
			if onResult != nil {
				onResult(completed, total, r)
			}
		}(i, id)
	}
	wg.Wait()

	summary := core.ScanSummary{Results: results}
	for _, r := range results {
		summary.TotalSize += r.TotalSize
		summary.TotalItems += len(r.Items)
	}
	return summary
}

// runOne executes a single scanner with panic isolation.
func runOne(ctx context.Context, id core.CategoryID, opts Options) (r core.ScanResult) {
	s, ok := Get(id)
	if !ok {
		return core.ScanResult{
			Category: core.Category{ID: id},
			Items:    []core.CleanableItem{},
			Error:    fmt.Sprintf("unknown scanner category: %s", id),
		}
	}
	defer func() {
		if rec := recover(); rec != nil {
			r = core.ScanResult{
				Category: s.Category(),
				Items:    []core.CleanableItem{},
				Error:    fmt.Sprintf("scanner panicked: %v", rec),
			}
		}
	}()
	return s.Scan(ctx, opts)
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `go test ./internal/scanners/ -race -v`
Expected: PASS — all tests in the package (exec, scanners, runner), race-clean.

- [ ] **Step 13: Commit**

```
git add internal/scanners/exec.go internal/scanners/scanners.go internal/scanners/runner.go internal/scanners/exec_test.go internal/scanners/testhelpers_test.go internal/scanners/scanners_test.go internal/scanners/runner_test.go
git commit -m "feat(scanners): scanner interface, exec runner, and parallel scan runner" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 8: Six Plain Directory Scanners (system_cache, system_logs, trash, ios_backups, mail_attachments, temp_files)

**Files:**
- Create: `internal/scanners/system_cache.go`
- Create: `internal/scanners/system_logs.go`
- Create: `internal/scanners/trash.go`
- Create: `internal/scanners/ios_backups.go`
- Create: `internal/scanners/mail_attachments.go`
- Create: `internal/scanners/temp_files.go`
- Test: `internal/scanners/system_cache_test.go`
- Test: `internal/scanners/system_logs_test.go`
- Test: `internal/scanners/trash_test.go`
- Test: `internal/scanners/ios_backups_test.go`
- Test: `internal/scanners/mail_attachments_test.go`
- Test: `internal/scanners/temp_files_test.go`

**Interfaces:**
- Consumes:
  - Task 7: `Scanner`, `Options`, `Roots`, `register(s Scanner)`, `newScanResult(cat core.Category, items []core.CleanableItem) core.ScanResult`, `cleanWithFsx(...)`, `Get(id core.CategoryID) (Scanner, bool)`; test helpers `mkFile`, `mkDir`, `testOptions`
  - `func fsx.GetDirectoryItems(dir string) []core.CleanableItem` — immediate children, each fully sized (dirs recursive), `[]` on any error (missing dir ⇒ empty, no error)
  - `core.Categories` — the six entries `"system-cache"`, `"system-logs"`, `"trash"`, `"ios-backups"`, `"mail-attachments"`, `"temp-files"`
- Produces: six scanners self-registered via `init()` (no new exported API). Paths scanned:
  - system-cache: `Roots.Home + "/Library/Caches"`
  - system-logs: `Roots.Home + "/Library/Logs"` ONLY (`/var/log` deliberately dropped, spec §10.1)
  - trash: `Roots.Home + "/.Trash"`
  - ios-backups: `Roots.Home + "/Library/Application Support/MobileSync/Backup"`, item name `"iOS Backup: " + first 8 chars of dir name + "..."` (whole name if shorter than 8)
  - mail-attachments: `Roots.Home + "/Library/Containers/com.apple.mail/Data/Library/Mail Downloads"`
  - temp-files: children of `Roots.Tmp`, plus for every `Roots.VarFolders/<d1>/<d2>/T` that exists (and is a dir), ITS children; permission/read errors at any level silently skipped

- [ ] **Step 1: Write the failing tests for system_cache, system_logs, trash**

Create `internal/scanners/system_cache_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestSystemCacheScanner(t *testing.T) {
	s, ok := Get("system-cache")
	if !ok {
		t.Fatal("system-cache scanner not registered")
	}
	if s.Category() != core.Categories["system-cache"] {
		t.Fatalf("Category() = %+v, want core.Categories[system-cache]", s.Category())
	}

	opts := testOptions(t)
	// Missing ~/Library/Caches => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 || res.TotalSize != 0 {
		t.Fatalf("missing dir must yield empty result without error, got %+v", res)
	}

	cache := filepath.Join(opts.Roots.Home, "Library", "Caches")
	mkFile(t, filepath.Join(cache, "com.example.app", "blob.bin"), 300)
	mkFile(t, filepath.Join(cache, "single.txt"), 200)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	if len(res.Items) != 2 || res.TotalSize != 500 {
		t.Fatalf("got %d items, total %d; want 2 items, total 500", len(res.Items), res.TotalSize)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if d := byName["com.example.app"]; !d.IsDirectory || d.Size != 300 {
		t.Fatalf("dir child = %+v, want IsDirectory with recursive size 300", d)
	}
	if f := byName["single.txt"]; f.IsDirectory || f.Size != 200 {
		t.Fatalf("file child = %+v, want plain file of size 200", f)
	}
}
```

Create `internal/scanners/system_logs_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestSystemLogsScanner(t *testing.T) {
	s, ok := Get("system-logs")
	if !ok {
		t.Fatal("system-logs scanner not registered")
	}
	if s.Category() != core.Categories["system-logs"] {
		t.Fatalf("Category() = %+v, want core.Categories[system-logs]", s.Category())
	}

	opts := testOptions(t)
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing dir must yield empty result without error, got %+v", res)
	}

	// Only ~/Library/Logs is scanned — /var/log was deliberately dropped
	// (spec §10.1), so a Roots-relative fixture fully specifies the scanner.
	logs := filepath.Join(opts.Roots.Home, "Library", "Logs")
	mkFile(t, filepath.Join(logs, "app.log"), 150)
	mkFile(t, filepath.Join(logs, "CrashReporter", "dump.crash"), 350)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 2 || res.TotalSize != 500 {
		t.Fatalf("got %+v; want 2 items totalling 500", res)
	}
}
```

Create `internal/scanners/trash_test.go` (also proves the shared `cleanWithFsx` Clean wiring end-to-end once):

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestSystemCacheScanner|TestSystemLogsScanner|TestTrashScanner' -v`
Expected: FAIL — each test dies at `t.Fatal("<id> scanner not registered")` (tests compile; `Get` returns ok=false).

- [ ] **Step 3: Write minimal implementation**

Create `internal/scanners/system_cache.go`:

```go
package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

type systemCacheScanner struct{}

func init() { register(systemCacheScanner{}) }

func (systemCacheScanner) Category() core.Category { return core.Categories["system-cache"] }

func (s systemCacheScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Caches")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s systemCacheScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

Create `internal/scanners/system_logs.go`:

```go
package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// systemLogsScanner scans ~/Library/Logs ONLY. The CLI also listed /var/log,
// but its own safety layer made every /var/log item undeletable (always
// PROTECTED), so the port drops /var/log from scanning (spec §10.1).
type systemLogsScanner struct{}

func init() { register(systemLogsScanner{}) }

func (systemLogsScanner) Category() core.Category { return core.Categories["system-logs"] }

func (s systemLogsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Logs")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s systemLogsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

Create `internal/scanners/trash.go`:

```go
package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

type trashScanner struct{}

func init() { register(trashScanner{}) }

func (trashScanner) Category() core.Category { return core.Categories["trash"] }

func (s trashScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, ".Trash")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s trashScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run 'TestSystemCacheScanner|TestSystemLogsScanner|TestTrashScanner' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Write the failing tests for ios_backups and mail_attachments**

Create `internal/scanners/ios_backups_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
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
```

Create `internal/scanners/mail_attachments_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestMailAttachmentsScanner(t *testing.T) {
	s, ok := Get("mail-attachments")
	if !ok {
		t.Fatal("mail-attachments scanner not registered")
	}
	if s.Category() != core.Categories["mail-attachments"] {
		t.Fatalf("Category() = %+v, want core.Categories[mail-attachments]", s.Category())
	}

	opts := testOptions(t)
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing dir must yield empty result without error, got %+v", res)
	}

	mail := filepath.Join(opts.Roots.Home, "Library", "Containers", "com.apple.mail",
		"Data", "Library", "Mail Downloads")
	mkFile(t, filepath.Join(mail, "9F1A2B3C-0000-4444-8888-ABCDEF012345", "invoice.pdf"), 700)
	mkFile(t, filepath.Join(mail, "photo.jpg"), 300)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 2 || res.TotalSize != 1000 {
		t.Fatalf("scan = %+v; want 2 items totalling 1000", res)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if d := byName["9F1A2B3C-0000-4444-8888-ABCDEF012345"]; !d.IsDirectory || d.Size != 700 {
		t.Fatalf("attachment folder = %+v; want dir of size 700, name kept as-is", d)
	}
	if f := byName["photo.jpg"]; f.IsDirectory || f.Size != 300 {
		t.Fatalf("file item = %+v; want file of size 300", f)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestIOSBackupsScanner|TestMailAttachmentsScanner' -v`
Expected: FAIL — `t.Fatal("ios-backups scanner not registered")` / `t.Fatal("mail-attachments scanner not registered")`.

- [ ] **Step 7: Write minimal implementation**

Create `internal/scanners/ios_backups.go`:

```go
package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

type iosBackupsScanner struct{}

func init() { register(iosBackupsScanner{}) }

func (iosBackupsScanner) Category() core.Category { return core.Categories["ios-backups"] }

func (s iosBackupsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Application Support", "MobileSync", "Backup")
	items := fsx.GetDirectoryItems(dir)
	for i := range items {
		// CLI parity: name = "iOS Backup: " + dirName.substring(0,8) + "..."
		// (whole name when shorter than 8 chars; UDIDs are ASCII hex).
		udid := items[i].Name
		if len(udid) > 8 {
			udid = udid[:8]
		}
		items[i].Name = "iOS Backup: " + udid + "..."
	}
	return newScanResult(s.Category(), items)
}

func (s iosBackupsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

Create `internal/scanners/mail_attachments.go`:

```go
package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

type mailAttachmentsScanner struct{}

func init() { register(mailAttachmentsScanner{}) }

func (mailAttachmentsScanner) Category() core.Category {
	return core.Categories["mail-attachments"]
}

func (s mailAttachmentsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Containers", "com.apple.mail",
		"Data", "Library", "Mail Downloads")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s mailAttachmentsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run 'TestIOSBackupsScanner|TestMailAttachmentsScanner' -v`
Expected: PASS (2 tests).

- [ ] **Step 9: Write the failing test for temp_files**

Create `internal/scanners/temp_files_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestTempFilesScanner(t *testing.T) {
	s, ok := Get("temp-files")
	if !ok {
		t.Fatal("temp-files scanner not registered")
	}
	if s.Category() != core.Categories["temp-files"] {
		t.Fatalf("Category() = %+v, want core.Categories[temp-files]", s.Category())
	}

	opts := testOptions(t)
	// Neither Roots.Tmp nor Roots.VarFolders exists yet => empty, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing roots must yield empty result without error, got %+v", res)
	}

	// /tmp children (a file and a 0-byte dir — both listed).
	mkFile(t, filepath.Join(opts.Roots.Tmp, "scratch.txt"), 100)
	mkDir(t, filepath.Join(opts.Roots.Tmp, "com.example.tmp"))
	// var/folders two-level layout: only children of an existing .../T dir count.
	mkFile(t, filepath.Join(opts.Roots.VarFolders, "ab", "c1", "T", "cache.db"), 250)
	mkDir(t, filepath.Join(opts.Roots.VarFolders, "ab", "c2"))          // no T dir => skipped
	mkFile(t, filepath.Join(opts.Roots.VarFolders, "zz", "c3", "T"), 0) // T is a FILE => skipped
	mkFile(t, filepath.Join(opts.Roots.VarFolders, "stray.txt"), 50)    // level-1 file => ReadDir fails, silently skipped

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	if len(res.Items) != 3 || res.TotalSize != 350 {
		t.Fatalf("got %d items total %d (%+v); want 3 items totalling 350", len(res.Items), res.TotalSize, res.Items)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if f := byName["scratch.txt"]; f.Size != 100 {
		t.Fatalf("scratch.txt = %+v, want size 100", f)
	}
	if d, ok := byName["com.example.tmp"]; !ok || !d.IsDirectory || d.Size != 0 {
		t.Fatalf("com.example.tmp = %+v ok=%v; 0-byte dir must still be listed", d, ok)
	}
	c := byName["cache.db"]
	if c.Size != 250 || c.Path != filepath.Join(opts.Roots.VarFolders, "ab", "c1", "T", "cache.db") {
		t.Fatalf("cache.db = %+v; want child of the T dir, size 250", c)
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestTempFilesScanner -v`
Expected: FAIL — `t.Fatal("temp-files scanner not registered")`.

- [ ] **Step 11: Write minimal implementation**

Create `internal/scanners/temp_files.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// tempFilesScanner lists children of Roots.Tmp plus, for every
// Roots.VarFolders/<d1>/<d2>/T directory that exists, the children of that T
// dir. Read/permission errors at any level are silently skipped (on the real
// system many /var/folders entries belong to other users — expected).
type tempFilesScanner struct{}

func init() { register(tempFilesScanner{}) }

func (tempFilesScanner) Category() core.Category { return core.Categories["temp-files"] }

func (s tempFilesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	items := fsx.GetDirectoryItems(opts.Roots.Tmp)
	if level1, err := os.ReadDir(opts.Roots.VarFolders); err == nil {
		for _, d1 := range level1 {
			l2 := filepath.Join(opts.Roots.VarFolders, d1.Name())
			level2, err := os.ReadDir(l2)
			if err != nil {
				continue // non-dir or unreadable entry: silently skipped
			}
			for _, d2 := range level2 {
				tDir := filepath.Join(l2, d2.Name(), "T")
				if fi, err := os.Stat(tDir); err == nil && fi.IsDir() {
					items = append(items, fsx.GetDirectoryItems(tDir)...)
				}
			}
		}
	}
	return newScanResult(s.Category(), items)
}

func (s tempFilesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `go test ./internal/scanners/ -v`
Expected: PASS — whole package, including the Task 7 `TestAllReturnsRegisteredScannersInDisplayOrder` (now covering 6 registered scanners in display order).

- [ ] **Step 13: Commit**

```
git add internal/scanners/system_cache.go internal/scanners/system_cache_test.go internal/scanners/system_logs.go internal/scanners/system_logs_test.go internal/scanners/trash.go internal/scanners/trash_test.go internal/scanners/ios_backups.go internal/scanners/ios_backups_test.go internal/scanners/mail_attachments.go internal/scanners/mail_attachments_test.go internal/scanners/temp_files.go internal/scanners/temp_files_test.go
git commit -m "feat(scanners): six directory-listing scanners" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 9: Downloads and Large Files Scanners

**Files:**
- Create: `internal/scanners/downloads.go`
- Create: `internal/scanners/large_files.go`
- Test: `internal/scanners/downloads_test.go`
- Test: `internal/scanners/large_files_test.go`

**Interfaces:**
- Consumes:
  - Task 7: `Scanner`, `Options`, `register`, `newScanResult`, `cleanWithFsx`, `Get`; test helpers `mkFile`, `testOptions`
  - `func fsx.GetItems(dir string, f fsx.ItemFilter) []core.CleanableItem` with `type fsx.ItemFilter struct { MinAgeDays int; MinSize int64 }` — non-recursive children matching filter; age = now − lstat mtime; missing dir ⇒ empty
  - `config.Config` fields: `DownloadsDaysOld int` (default 30), `LargeFilesMinSize int64` (default 524288000)
  - `core.Categories["downloads"]`, `core.Categories["large-files"]`
- Produces:
  - Registered `downloads` scanner: `Roots.Home + "/Downloads"` via `fsx.GetItems{MinAgeDays: opts.Cfg.DownloadsDaysOld}`
  - Registered `large-files` scanner: walks `Roots.Home + "/Downloads"` and `Roots.Home + "/Documents"`, maxDepth 3 (root = depth 0; dirs at depth ≤ 3 are read, so files 4 components below a root are found), skips names starting with `.`, regular files only (symlinks/sockets skipped, never followed), `size >= Cfg.LargeFilesMinSize`, sorted size desc, `Name` = basename
  - Package-local: `func findLargeFiles(dir string, depth int, minSize int64) []core.CleanableItem`

- [ ] **Step 1: Write the failing test for downloads**

Create `internal/scanners/downloads_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestDownloadsScanner -v`
Expected: FAIL — `t.Fatal("downloads scanner not registered")`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/scanners/downloads.go`:

```go
package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// downloadsScanner lists immediate children of ~/Downloads whose age
// (now − lstat mtime) is >= Cfg.DownloadsDaysOld days. Non-recursive; child
// directories are sized recursively by fsx. No dot-file exclusion (CLI parity).
type downloadsScanner struct{}

func init() { register(downloadsScanner{}) }

func (downloadsScanner) Category() core.Category { return core.Categories["downloads"] }

func (s downloadsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Downloads")
	items := fsx.GetItems(dir, fsx.ItemFilter{MinAgeDays: opts.Cfg.DownloadsDaysOld})
	return newScanResult(s.Category(), items)
}

func (s downloadsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run TestDownloadsScanner -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Write the failing test for large_files**

Create `internal/scanners/large_files_test.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestLargeFilesScanner(t *testing.T) {
	s, ok := Get("large-files")
	if !ok {
		t.Fatal("large-files scanner not registered")
	}
	if s.Category() != core.Categories["large-files"] {
		t.Fatalf("Category() = %+v, want core.Categories[large-files]", s.Category())
	}

	opts := testOptions(t)
	// Missing ~/Downloads and ~/Documents => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing roots must yield empty result without error, got %+v", res)
	}

	opts.Cfg.LargeFilesMinSize = 1000 // keep fixtures tiny
	dl := filepath.Join(opts.Roots.Home, "Downloads")
	docs := filepath.Join(opts.Roots.Home, "Documents")

	mkFile(t, filepath.Join(dl, "big.bin"), 1500)                             // in
	mkFile(t, filepath.Join(dl, "exact.bin"), 1000)                           // in: size >= min is inclusive
	mkFile(t, filepath.Join(dl, "small.bin"), 999)                            // out: below threshold
	mkFile(t, filepath.Join(dl, ".hidden.bin"), 5000)                         // out: dot file skipped
	mkFile(t, filepath.Join(dl, ".hiddendir", "inside.bin"), 5000)            // out: dot dir never descended
	mkFile(t, filepath.Join(dl, "d1", "d2", "d3", "deep.bin"), 1200)          // in: file at depth 4 (dir d3 at depth 3 is read)
	mkFile(t, filepath.Join(dl, "d1", "d2", "d3", "d4", "toodeep.bin"), 9000) // out: dir d4 at depth 4 not descended
	mkFile(t, filepath.Join(docs, "doc.bin"), 3000)                           // in: second root
	if err := os.Symlink(filepath.Join(dl, "big.bin"), filepath.Join(dl, "link.bin")); err != nil {
		t.Fatal(err) // out: symlinks are not regular files, never followed
	}

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	var names []string
	for _, it := range res.Items {
		names = append(names, it.Name)
	}
	want := []string{"doc.bin", "big.bin", "deep.bin", "exact.bin"} // size desc: 3000, 1500, 1200, 1000
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v (sorted size desc)", names, want)
	}
	for _, it := range res.Items {
		if it.IsDirectory {
			t.Fatalf("%s flagged as directory; large-files returns regular files only", it.Name)
		}
		if it.ModifiedAt == nil {
			t.Fatalf("%s has nil ModifiedAt", it.Name)
		}
		if filepath.Base(it.Path) != it.Name {
			t.Fatalf("Name %q must be the basename of Path %q", it.Name, it.Path)
		}
	}
	if res.TotalSize != 3000+1500+1200+1000 {
		t.Fatalf("TotalSize = %d, want %d", res.TotalSize, 3000+1500+1200+1000)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestLargeFilesScanner -v`
Expected: FAIL — `t.Fatal("large-files scanner not registered")`.

- [ ] **Step 7: Write minimal implementation**

Create `internal/scanners/large_files.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// largeFilesScanner walks ~/Downloads and ~/Documents looking for regular
// files >= Cfg.LargeFilesMinSize. CLI-parity depth semantics: the root call
// is depth 0 and the guard is depth > 3, so directories at depth 0..3 are
// read and files up to 4 path components below a root are found. Entries
// whose name starts with '.' are skipped (files and dirs). Symlinks are
// neither regular files nor directories under lstat semantics, so they are
// skipped entirely and never followed.
type largeFilesScanner struct{}

func init() { register(largeFilesScanner{}) }

func (largeFilesScanner) Category() core.Category { return core.Categories["large-files"] }

func (s largeFilesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	var items []core.CleanableItem
	for _, root := range []string{
		filepath.Join(opts.Roots.Home, "Downloads"),
		filepath.Join(opts.Roots.Home, "Documents"),
	} {
		items = append(items, findLargeFiles(root, 0, opts.Cfg.LargeFilesMinSize)...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	return newScanResult(s.Category(), items)
}

// findLargeFiles reads dir (at the given depth relative to the walk root) and
// returns matching regular files. Unreadable dirs and per-entry stat errors
// are silently skipped.
func findLargeFiles(dir string, depth int, minSize int64) []core.CleanableItem {
	if depth > 3 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []core.CleanableItem
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(dir, name)
		if e.IsDir() { // DirEntry.Type() is lstat-based: a symlink to a dir is NOT IsDir
			out = append(out, findLargeFiles(p, depth+1, minSize)...)
			continue
		}
		if !e.Type().IsRegular() { // symlinks, sockets, pipes: skipped
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() < minSize {
			continue
		}
		mt := info.ModTime()
		out = append(out, core.CleanableItem{
			Path:        p,
			Size:        info.Size(),
			Name:        name,
			IsDirectory: false,
			ModifiedAt:  &mt,
		})
	}
	return out
}

func (s largeFilesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/scanners/ -v`
Expected: PASS — whole package (8 scanners now registered).

- [ ] **Step 9: Commit**

```
git add internal/scanners/downloads.go internal/scanners/downloads_test.go internal/scanners/large_files.go internal/scanners/large_files_test.go
git commit -m "feat(scanners): downloads age filter and large-files depth-bounded walk" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 10: Browser Cache and Dev Cache Scanners

**Files:**
- Create: `internal/scanners/browser_cache.go`
- Create: `internal/scanners/dev_cache.go`
- Test: `internal/scanners/browser_cache_test.go`
- Test: `internal/scanners/dev_cache_test.go`

**Interfaces:**
- Consumes:
  - Task 7: `Scanner`, `Options`, `register`, `newScanResult`, `cleanWithFsx`, `Get`; test helpers `mkFile`, `mkDir`, `testOptions`
  - `func fsx.GetSize(path string) int64` (recursive logical size, unreadable ⇒ 0), `func fsx.GetDirectoryItems(dir string) []core.CleanableItem`
  - `core.Categories["browser-cache"]`, `core.Categories["dev-cache"]`
- Produces:
  - Registered `browser-cache` scanner — 4 fixed candidates relative to `Roots.Home`, in this order: Google Chrome → `Library/Caches/Google/Chrome`; Safari → `Library/Caches/com.apple.Safari`; Firefox → `Library/Caches/Firefox/Profiles`; Arc → `Library/Caches/company.thebrowser.Browser`. One item per EXISTING dir (0-byte dirs included — no size gate), `Name` = `"<Browser> Cache"`, `IsDirectory` true.
  - Registered `dev-cache` scanner — part 1: 7 fixed paths, each included only if it exists AND `fsx.GetSize > 0` (`npm cache` → `.npm/_cacache`; `Yarn cache` → `Library/Caches/Yarn`; `pnpm store` → `Library/pnpm/store`; `pip cache` → `.cache/pip`; `CocoaPods cache` → `Library/Caches/CocoaPods`; `Gradle cache` → `.gradle/caches`; `Cargo cache` → `.cargo/registry`); part 2: `Library/Developer/Xcode/DerivedData` → one item PER CHILD named `"Xcode: <child>"` (no size gate); part 3: `Library/Developer/Xcode/Archives` as ONE item `"Xcode Archives"` if it exists and size > 0.
  - Package-local tables: `browserCandidates`, `devCachePaths` (name + home-relative path pairs).

- [ ] **Step 1: Write the failing test for browser_cache**

Create `internal/scanners/browser_cache_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestBrowserCacheScanner -v`
Expected: FAIL — `t.Fatal("browser-cache scanner not registered")`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/scanners/browser_cache.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// browserCandidates are the four fixed browser cache locations, relative to
// Roots.Home, checked in this exact order. Each existing directory becomes
// ONE CleanableItem (even at 0 bytes — CLI parity: no size gate).
var browserCandidates = []struct {
	name string // display name; item Name = name + " Cache"
	rel  string // path relative to home
}{
	{"Google Chrome", "Library/Caches/Google/Chrome"},
	{"Safari", "Library/Caches/com.apple.Safari"},
	{"Firefox", "Library/Caches/Firefox/Profiles"},
	{"Arc", "Library/Caches/company.thebrowser.Browser"},
}

type browserCacheScanner struct{}

func init() { register(browserCacheScanner{}) }

func (browserCacheScanner) Category() core.Category { return core.Categories["browser-cache"] }

func (s browserCacheScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	var items []core.CleanableItem
	for _, b := range browserCandidates {
		p := filepath.Join(opts.Roots.Home, b.rel)
		fi, err := os.Stat(p)
		if err != nil || !fi.IsDir() {
			continue // missing/unreadable browser silently omitted
		}
		mt := fi.ModTime()
		items = append(items, core.CleanableItem{
			Path:        p,
			Size:        fsx.GetSize(p),
			Name:        b.name + " Cache",
			IsDirectory: true,
			ModifiedAt:  &mt,
		})
	}
	return newScanResult(s.Category(), items)
}

func (s browserCacheScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run TestBrowserCacheScanner -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for dev_cache**

Create `internal/scanners/dev_cache_test.go`:

```go
package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func devCacheItemsByName(t *testing.T, s Scanner, opts Options) map[string]core.CleanableItem {
	t.Helper()
	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if len(byName) != len(res.Items) {
		t.Fatalf("duplicate item names in %+v", res.Items)
	}
	return byName
}

func TestDevCacheScannerFixedPathsSizeGate(t *testing.T) {
	s, ok := Get("dev-cache")
	if !ok {
		t.Fatal("dev-cache scanner not registered")
	}
	if s.Category() != core.Categories["dev-cache"] {
		t.Fatalf("Category() = %+v, want core.Categories[dev-cache]", s.Category())
	}

	opts := testOptions(t)
	// Nothing exists => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("empty home must yield empty result without error, got %+v", res)
	}

	home := opts.Roots.Home
	mkFile(t, filepath.Join(home, ".npm", "_cacache", "content-v2", "blob"), 2048) // exists, size>0 => in
	mkDir(t, filepath.Join(home, "Library", "Caches", "Yarn"))                     // exists, size 0  => OUT (gate)
	mkFile(t, filepath.Join(home, ".cargo", "registry", "cache", "pkg.crate"), 512)
	// pnpm, pip, CocoaPods, Gradle: absent => out.

	byName := devCacheItemsByName(t, s, opts)
	if len(byName) != 2 {
		t.Fatalf("got %d items (%v), want npm + cargo only", len(byName), byName)
	}
	npm := byName["npm cache"]
	if npm.Size != 2048 || !npm.IsDirectory || npm.Path != filepath.Join(home, ".npm", "_cacache") {
		t.Fatalf("npm cache = %+v; want dir item of 2048 at ~/.npm/_cacache", npm)
	}
	if _, hasYarn := byName["Yarn cache"]; hasYarn {
		t.Fatal("0-byte Yarn cache must be excluded by the size>0 gate")
	}
	if cargo := byName["Cargo cache"]; cargo.Size != 512 {
		t.Fatalf("Cargo cache = %+v; want size 512", cargo)
	}
}

func TestDevCacheScannerXcodeDerivedDataPerChild(t *testing.T) {
	s, _ := Get("dev-cache")
	opts := testOptions(t)
	home := opts.Roots.Home
	dd := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	mkFile(t, filepath.Join(dd, "MyApp-abcdefgh", "Build", "x.o"), 700)
	mkDir(t, filepath.Join(dd, "Empty-00000000")) // 0-byte child: NO size gate for DerivedData
	mkDir(t, filepath.Join(home, "Library", "Developer", "Xcode", "Archives")) // empty Archives => out

	byName := devCacheItemsByName(t, s, opts)
	if len(byName) != 2 {
		t.Fatalf("got %d items (%v), want the two DerivedData children", len(byName), byName)
	}
	my := byName["Xcode: MyApp-abcdefgh"]
	if my.Size != 700 || !my.IsDirectory || my.Path != filepath.Join(dd, "MyApp-abcdefgh") {
		t.Fatalf("DerivedData child = %+v; want 'Xcode: MyApp-abcdefgh', size 700", my)
	}
	if empty, ok := byName["Xcode: Empty-00000000"]; !ok || empty.Size != 0 {
		t.Fatalf("0-byte DerivedData child = %+v ok=%v; must still be listed", empty, ok)
	}
	if _, hasArchives := byName["Xcode Archives"]; hasArchives {
		t.Fatal("empty Archives dir must be excluded by the size>0 gate")
	}
}

func TestDevCacheScannerXcodeArchivesSingleItem(t *testing.T) {
	s, _ := Get("dev-cache")
	opts := testOptions(t)
	home := opts.Roots.Home
	ar := filepath.Join(home, "Library", "Developer", "Xcode", "Archives")
	mkFile(t, filepath.Join(ar, "2026-01-01", "App.xcarchive", "Info.plist"), 900)

	byName := devCacheItemsByName(t, s, opts)
	if len(byName) != 1 {
		t.Fatalf("got %d items (%v), want exactly one 'Xcode Archives'", len(byName), byName)
	}
	arch := byName["Xcode Archives"]
	if arch.Size != 900 || !arch.IsDirectory || arch.Path != ar {
		t.Fatalf("Archives = %+v; want ONE dir item of 900 at %s", arch, ar)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run TestDevCacheScanner -v`
Expected: FAIL — `t.Fatal("dev-cache scanner not registered")`.

- [ ] **Step 7: Write minimal implementation**

Create `internal/scanners/dev_cache.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// devCachePaths are the fixed single-item developer caches (part 1), relative
// to Roots.Home, in this exact order. Each is included only if it exists AND
// its recursive size is > 0 (CLI parity).
var devCachePaths = []struct {
	name string // item Name, verbatim
	rel  string // path relative to home
}{
	{"npm cache", ".npm/_cacache"},
	{"Yarn cache", "Library/Caches/Yarn"},
	{"pnpm store", "Library/pnpm/store"},
	{"pip cache", ".cache/pip"},
	{"CocoaPods cache", "Library/Caches/CocoaPods"},
	{"Gradle cache", ".gradle/caches"},
	{"Cargo cache", ".cargo/registry"},
}

type devCacheScanner struct{}

func init() { register(devCacheScanner{}) }

func (devCacheScanner) Category() core.Category { return core.Categories["dev-cache"] }

func (s devCacheScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	home := opts.Roots.Home
	var items []core.CleanableItem

	// Part 1: fixed paths, exists AND size > 0.
	for _, c := range devCachePaths {
		p := filepath.Join(home, c.rel)
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		size := fsx.GetSize(p)
		if size <= 0 {
			continue
		}
		mt := fi.ModTime()
		items = append(items, core.CleanableItem{
			Path:        p,
			Size:        size,
			Name:        c.name,
			IsDirectory: true,
			ModifiedAt:  &mt,
		})
	}

	// Part 2: Xcode DerivedData — one item per child (per-project folder),
	// name rewritten to "Xcode: <child>". Deliberately NO size>0 gate.
	dd := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	for _, it := range fsx.GetDirectoryItems(dd) {
		it.Name = "Xcode: " + it.Name
		items = append(items, it)
	}

	// Part 3: Xcode Archives as ONE item, if it exists and size > 0.
	ar := filepath.Join(home, "Library", "Developer", "Xcode", "Archives")
	if fi, err := os.Stat(ar); err == nil {
		if size := fsx.GetSize(ar); size > 0 {
			mt := fi.ModTime()
			items = append(items, core.CleanableItem{
				Path:        ar,
				Size:        size,
				Name:        "Xcode Archives",
				IsDirectory: true,
				ModifiedAt:  &mt,
			})
		}
	}

	return newScanResult(s.Category(), items)
}

func (s devCacheScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/scanners/ -race`
Expected: PASS — whole package, race-clean; 10 scanners now registered (`system-cache`, `system-logs`, `temp-files`, `trash`, `downloads`, `browser-cache`, `dev-cache`, `ios-backups`, `mail-attachments`, `large-files`), with the remaining 6 (`homebrew`, `docker`, `language-files`, `node-modules`, `duplicates`, `launch-agents`) arriving in later tasks.

- [ ] **Step 9: Commit**

```
git add internal/scanners/browser_cache.go internal/scanners/browser_cache_test.go internal/scanners/dev_cache.go internal/scanners/dev_cache_test.go
git commit -m "feat(scanners): browser-cache candidates and dev-cache size-gated paths" -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```
### Task 11: node-modules + language-files scanners

**Files:**
- Create: `internal/scanners/node_modules.go`
- Create: `internal/scanners/language_files.go`
- Modify: `go.mod` / `go.sum` (via `go get howett.net/plist@v1.0.1` — needed by the default AppleLanguages resolver; Task 13 reuses it)
- Test: `internal/scanners/node_modules_test.go`
- Test: `internal/scanners/language_files_test.go`

**Interfaces:**
- Consumes (from the contract / earlier tasks):
  - `core.Category`, `core.CleanableItem`, `core.ScanResult`, `core.CleanResult`, `core.ProgressFunc`, `core.Categories map[core.CategoryID]core.Category` (core task)
  - `fsx.GetSize(path string) int64` (fsx task)
  - `config.Config` (fields `DownloadsDaysOld int`, `KeepLanguages []string`, `ExtraPaths struct{ NodeModules, Projects []string }`), `config.Default() config.Config` (config task)
  - Scanner framework (scanners-framework task): `type Options struct { Roots Roots; Cfg config.Config; Runner CmdRunner }`, `type Roots struct { Home, Tmp, VarFolders, Applications string }`, `type Scanner interface { Category() core.Category; Scan(ctx context.Context, opts Options) core.ScanResult; Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult }`, the shared helper `cleanWithFsx(cat core.Category, ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult`, and the registry hook `register(s Scanner)` (called from each scanner file's `init()`, same pattern as Tasks 8–10)
- Produces (package-local; each scanner file's `init()` calls `register(...)` with one long-lived instance per category — that is how these scanners reach `All()`):
  - `func newNodeModulesScanner() *nodeModulesScanner` — satisfies `Scanner`
  - `func newLanguageFilesScanner() *languageFilesScanner` — satisfies `Scanner`; struct field `preferred func(home string) []string` is the test seam for system preferred-language tags
  - `func keepLanguageSet(tags, cfgKeep []string) map[string]bool` (used only inside this file, exported here for its unit test)
  - `howett.net/plist v1.0.1` present in `go.mod`

**Behavior being locked in (from porting-notes → scanners → node-modules.ts / language-files.ts and spec §5):**
- node-modules roots: the 6 fixed dirs under Home (`Projects`, `Developer`, `Code`, `dev`, `workspace`, `repos`) plus `Cfg.ExtraPaths.NodeModules` plus `Cfg.ExtraPaths.Projects`; nonexistent roots skipped silently. Walk with root at depth 0, guard `depth > 4` (so dirs at depth 0..4 are read). Skip non-directories and dot-names. A dir named exactly `node_modules`: with sibling `package.json` → included only if `(now − stat(parentDir).mtime) ≥ daysOld` days (fractional compare), `Name = "<parentBase> (<floor(days)>d old)"`; without `package.json` → included at any age but only if recursive size > 0, `Name = "<parentBase> (orphaned)"`. Never recurse INTO `node_modules`. Items sorted size desc. **daysOld resolution:** porting-notes ties node-modules to the *same* `options.daysOld ?? 30` knob that feeds the downloads scanner; the GUI exposes that knob as `Cfg.DownloadsDaysOld` (default 30), so this scanner reads `Cfg.DownloadsDaysOld` and falls back to 30 when ≤ 0. Deliberate improvement over the CLI (documented in porting-notes as a choice): only a *missing* `package.json` marks a project orphaned; other stat errors skip the entry instead of mislabeling it.
- language-files: readdir `Roots.Applications` for top-level entries ending `.app`; inside `<app>/Contents/Resources`, every `*.lproj` whose basename (minus `.lproj`) is NOT in the keep-set becomes an item `{Path: <lproj dir>, Size: recursive, Name: "<app>.app: <lang>.lproj", IsDirectory: true, ModifiedAt: stat mtime}`. Keep-set = for each preferred-language tag: the tag verbatim, the tag with `-`→`_`, and its base language; union `{"en","Base"}` and `Cfg.KeepLanguages` verbatim. Case-sensitive. Per-app / per-lproj errors silently skipped. The preferred-tag source is injectable; the default reads `AppleLanguages` from `<home>/Library/Preferences/.GlobalPreferences.plist` via `howett.net/plist` (binary-safe, no subprocess).

- [ ] **Step 1: Write the failing node-modules test**

Create `internal/scanners/node_modules_test.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/config"
)

// nmMkFile creates a file (and its parent dirs) with the given content.
func nmMkFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNodeModulesScanner(t *testing.T) {
	home := t.TempDir()
	projects := filepath.Join(home, "Projects")

	// Stale project (package.json present, project dir mtime 40 days ago) → "(40d old)".
	stale := filepath.Join(projects, "stale-app")
	nmMkFile(t, filepath.Join(stale, "package.json"), "{}")
	nmMkFile(t, filepath.Join(stale, "node_modules", "left-pad", "index.js"), "module.exports = 1;")
	// A nested node_modules must never be reached (no recursion INTO node_modules).
	nmMkFile(t, filepath.Join(stale, "node_modules", "left-pad", "node_modules", "nested.js"), "nested")
	fortyDaysAgo := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(stale, fortyDaysAgo, fortyDaysAgo); err != nil {
		t.Fatal(err)
	}

	// Fresh project (mtime = now) → excluded by the 30-day default.
	fresh := filepath.Join(projects, "fresh-app")
	nmMkFile(t, filepath.Join(fresh, "package.json"), "{}")
	nmMkFile(t, filepath.Join(fresh, "node_modules", "a.js"), "a")

	// No package.json + non-empty node_modules → orphaned, included at any age.
	orphan := filepath.Join(projects, "orphan-app")
	nmMkFile(t, filepath.Join(orphan, "node_modules", "b.js"), "bb")

	// No package.json + EMPTY node_modules (recursive size 0) → excluded.
	if err := os.MkdirAll(filepath.Join(projects, "empty-app", "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Dot-directories are never walked.
	nmMkFile(t, filepath.Join(projects, ".hidden", "node_modules", "c.js"), "c")

	// Depth: dirs are read at depth 0..4, so projects/a/b/c/d/node_modules IS found
	// (d is read at depth 4) but projects/a/b/c/d/deep-app/node_modules is NOT
	// (walking into deep-app would be depth 5 > 4).
	nmMkFile(t, filepath.Join(projects, "a", "b", "c", "d", "node_modules", "ok.js"), "ok")
	nmMkFile(t, filepath.Join(projects, "a", "b", "c", "d", "deep-app", "node_modules", "deep.js"), "deep")

	// Extra config root outside the six fixed dirs; nonexistent extras skipped silently.
	extra := t.TempDir()
	nmMkFile(t, filepath.Join(extra, "legacy", "node_modules", "d.js"), "dddd")

	cfg := config.Default()
	cfg.ExtraPaths.NodeModules = []string{extra, filepath.Join(home, "does-not-exist")}
	opts := Options{Roots: Roots{Home: home}, Cfg: cfg}

	res := newNodeModulesScanner().Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %q", res.Error)
	}

	got := map[string]bool{}
	for _, it := range res.Items {
		got[it.Name] = true
		if !it.IsDirectory {
			t.Errorf("%s: IsDirectory = false, want true", it.Name)
		}
		if filepath.Base(it.Path) != "node_modules" {
			t.Errorf("%s: path %q does not point at a node_modules dir", it.Name, it.Path)
		}
		if it.ModifiedAt == nil {
			t.Errorf("%s: ModifiedAt is nil", it.Name)
		}
	}
	want := []string{
		"stale-app (40d old)",
		"orphan-app (orphaned)",
		"legacy (orphaned)",
		"d (orphaned)",
	}
	if len(res.Items) != len(want) {
		t.Fatalf("got %d items %v, want %d %v", len(res.Items), got, len(want), want)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing expected item %q (got %v)", w, got)
		}
	}

	// Sorted by size descending; TotalSize = sum of item sizes.
	var sum int64
	for i, it := range res.Items {
		sum += it.Size
		if i > 0 && res.Items[i-1].Size < it.Size {
			t.Errorf("items not sorted by size desc at index %d", i)
		}
	}
	if res.TotalSize != sum {
		t.Errorf("TotalSize = %d, want %d", res.TotalSize, sum)
	}
}

func TestNodeModulesScannerAgeFromConfig(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "Code", "app")
	nmMkFile(t, filepath.Join(proj, "package.json"), "{}")
	nmMkFile(t, filepath.Join(proj, "node_modules", "x.js"), "x")
	fortyDaysAgo := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(proj, fortyDaysAgo, fortyDaysAgo); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default() // DownloadsDaysOld = 30
	res := newNodeModulesScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}, Cfg: cfg})
	if len(res.Items) != 1 || res.Items[0].Name != "app (40d old)" {
		t.Fatalf("with 30d threshold: got %+v, want one item named 'app (40d old)'", res.Items)
	}

	// Same knob as downloads (CLI: options.daysOld fed both scanners).
	cfg.DownloadsDaysOld = 60
	res = newNodeModulesScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}, Cfg: cfg})
	if len(res.Items) != 0 {
		t.Fatalf("with 60d threshold: got %+v, want no items", res.Items)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestNodeModules' -v`
Expected: `FAIL github.com/guhcostan/app-cleaner/internal/scanners [build failed]` with `undefined: newNodeModulesScanner` (a build failure of the test package is the failing state in Go TDD).

- [ ] **Step 3: Write the node-modules implementation**

Create `internal/scanners/node_modules.go`:

```go
package scanners

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// nodeModulesMaxDepth: root call is depth 0 and the guard is `depth > max`,
// so directories at depth 0..4 are read (CLI parity: node-modules.ts maxDepth 4).
const nodeModulesMaxDepth = 4

// The CLI's six fixed search roots, relative to the home directory.
var nodeModulesSearchDirs = []string{"Projects", "Developer", "Code", "dev", "workspace", "repos"}

type nodeModulesScanner struct{}

func newNodeModulesScanner() *nodeModulesScanner { return &nodeModulesScanner{} }

func init() { register(newNodeModulesScanner()) }

func (s *nodeModulesScanner) Category() core.Category {
	return core.Categories["node-modules"]
}

func (s *nodeModulesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	// Staleness knob: the CLI fed the SAME options.daysOld (?? 30) to downloads
	// and node-modules; the GUI exposes it as Cfg.DownloadsDaysOld.
	daysOld := opts.Cfg.DownloadsDaysOld
	if daysOld <= 0 {
		daysOld = 30
	}

	roots := make([]string, 0, len(nodeModulesSearchDirs))
	for _, d := range nodeModulesSearchDirs {
		roots = append(roots, filepath.Join(opts.Roots.Home, d))
	}
	roots = append(roots, opts.Cfg.ExtraPaths.NodeModules...)
	roots = append(roots, opts.Cfg.ExtraPaths.Projects...)

	var items []core.CleanableItem
	now := time.Now()
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue // nonexistent roots skipped silently
		}
		items = append(items, findNodeModules(ctx, root, float64(daysOld), now, 0)...)
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].Size > items[j].Size })

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func findNodeModules(ctx context.Context, dir string, daysOld float64, now time.Time, depth int) []core.CleanableItem {
	if depth > nodeModulesMaxDepth || ctx.Err() != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // permission errors etc. silently skipped
	}
	var items []core.CleanableItem
	for _, e := range entries {
		// DirEntry.IsDir is lstat-based: symlinks are skipped, matching Node dirents.
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.Name() != "node_modules" {
			items = append(items, findNodeModules(ctx, full, daysOld, now, depth+1)...)
			continue
		}

		// Found a node_modules dir — never recurse into it.
		nmInfo, err := os.Stat(full)
		if err != nil {
			continue
		}
		mod := nmInfo.ModTime()

		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			// Real project: include only if the PROJECT dir is stale.
			parentInfo, err := os.Stat(dir)
			if err != nil {
				continue
			}
			days := now.Sub(parentInfo.ModTime()).Hours() / 24
			if days >= daysOld {
				items = append(items, core.CleanableItem{
					Path:        full,
					Size:        fsx.GetSize(full), // included even if 0 (CLI parity)
					Name:        fmt.Sprintf("%s (%dd old)", filepath.Base(dir), int(math.Floor(days))),
					IsDirectory: true,
					ModifiedAt:  &mod,
				})
			}
		} else {
			// Orphaned (no package.json): any age, but only if it holds anything.
			size := fsx.GetSize(full)
			if size > 0 {
				items = append(items, core.CleanableItem{
					Path:        full,
					Size:        size,
					Name:        filepath.Base(dir) + " (orphaned)",
					IsDirectory: true,
					ModifiedAt:  &mod,
				})
			}
		}
	}
	return items
}

func (s *nodeModulesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run 'TestNodeModules' -v`
Expected: PASS (`--- PASS: TestNodeModulesScanner`, `--- PASS: TestNodeModulesScannerAgeFromConfig`).

- [ ] **Step 5: Write the failing language-files test**

Create `internal/scanners/language_files_test.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/config"
)

func lfMkFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKeepLanguageSet(t *testing.T) {
	keep := keepLanguageSet([]string{"pt-BR", "zh_Hans"}, []string{"ja"})
	for _, want := range []string{"en", "Base", "pt-BR", "pt_BR", "pt", "zh_Hans", "zh", "ja"} {
		if !keep[want] {
			t.Errorf("keep-set missing %q (got %v)", want, keep)
		}
	}
	if keep["fr"] || keep["EN"] {
		t.Errorf("keep-set unexpectedly contains fr/EN: %v", keep)
	}
}

func TestLanguageFilesScanner(t *testing.T) {
	apps := t.TempDir()
	res := filepath.Join(apps, "Slack.app", "Contents", "Resources")
	for _, lang := range []string{"en", "Base", "fr", "de", "pt", "pt_BR", "pt-BR", "EN"} {
		lfMkFile(t, filepath.Join(res, lang+".lproj", "Localizable.strings"), "strings for "+lang)
	}
	// Non-.lproj resource entries are ignored.
	lfMkFile(t, filepath.Join(res, "AppIcon.icns"), "icon")
	// App bundle without Contents/Resources → silently skipped.
	if err := os.MkdirAll(filepath.Join(apps, "Bare.app", "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Entries not ending in .app are ignored entirely.
	lfMkFile(t, filepath.Join(apps, "NotAnApp", "Contents", "Resources", "it.lproj", "x.strings"), "x")

	s := newLanguageFilesScanner()
	// Test seam: inject the system preferred-language tags.
	s.preferred = func(home string) []string { return []string{"pt-BR"} }

	cfg := config.Default()
	cfg.KeepLanguages = []string{"de"}

	result := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir(), Applications: apps}, Cfg: cfg})
	if result.Error != "" {
		t.Fatalf("unexpected scan error: %q", result.Error)
	}

	// keep-set = {en, Base} ∪ expand(pt-BR)={pt-BR, pt_BR, pt} ∪ config {de}.
	// Case-sensitive: EN.lproj is NOT kept. Expected leftovers: fr, EN.
	got := map[string]string{} // name → path
	for _, it := range result.Items {
		got[it.Name] = it.Path
		if !it.IsDirectory {
			t.Errorf("%s: IsDirectory = false, want true", it.Name)
		}
		if it.Size <= 0 {
			t.Errorf("%s: Size = %d, want > 0", it.Name, it.Size)
		}
		if it.ModifiedAt == nil {
			t.Errorf("%s: ModifiedAt is nil", it.Name)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got items %v, want exactly fr.lproj and EN.lproj", got)
	}
	if p := got["Slack.app: fr.lproj"]; p != filepath.Join(res, "fr.lproj") {
		t.Errorf("fr item path = %q, want %q", p, filepath.Join(res, "fr.lproj"))
	}
	if _, ok := got["Slack.app: EN.lproj"]; !ok {
		t.Errorf("case-sensitive keep failed: EN.lproj should be listed (got %v)", got)
	}

	var sum int64
	for _, it := range result.Items {
		sum += it.Size
	}
	if result.TotalSize != sum {
		t.Errorf("TotalSize = %d, want %d", result.TotalSize, sum)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestLanguageFiles|TestKeepLanguageSet' -v`
Expected: `FAIL … [build failed]` with `undefined: keepLanguageSet` / `undefined: newLanguageFilesScanner`.

- [ ] **Step 7: Add the plist dependency and write the language-files implementation**

Run: `go get howett.net/plist@v1.0.1`
Expected output: `go: added howett.net/plist v1.0.1` (go.mod and go.sum updated).

Create `internal/scanners/language_files.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

type languageFilesScanner struct {
	// preferred returns the user's system preferred-language tags
	// (e.g. ["en-US", "pt-BR"]). Injectable in tests; the default reads
	// AppleLanguages from the user's GlobalPreferences plist.
	preferred func(home string) []string
}

func newLanguageFilesScanner() *languageFilesScanner {
	return &languageFilesScanner{preferred: readAppleLanguages}
}

func init() { register(newLanguageFilesScanner()) }

// readAppleLanguages parses <home>/Library/Preferences/.GlobalPreferences.plist
// (binary or XML) and returns the AppleLanguages array; nil on any error.
// No subprocess is spawned (engine rule: no shells, minimal exec surface).
func readAppleLanguages(home string) []string {
	data, err := os.ReadFile(filepath.Join(home, "Library", "Preferences", ".GlobalPreferences.plist"))
	if err != nil {
		return nil
	}
	var prefs struct {
		AppleLanguages []string `plist:"AppleLanguages"`
	}
	if _, err := plist.Unmarshal(data, &prefs); err != nil {
		return nil
	}
	return prefs.AppleLanguages
}

// keepLanguageSet expands each preferred tag into {verbatim, '-'→'_' variant,
// base language} and unions {"en", "Base"} plus the config keep-list (verbatim).
// Matching is case-sensitive (spec §5 / §10.6).
func keepLanguageSet(tags, cfgKeep []string) map[string]bool {
	keep := map[string]bool{"en": true, "Base": true}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		keep[tag] = true
		keep[strings.ReplaceAll(tag, "-", "_")] = true
		base := tag
		if i := strings.IndexAny(tag, "-_"); i > 0 {
			base = tag[:i]
		}
		keep[base] = true
	}
	for _, k := range cfgKeep {
		if k != "" {
			keep[k] = true
		}
	}
	return keep
}

func (s *languageFilesScanner) Category() core.Category {
	return core.Categories["language-files"]
}

func (s *languageFilesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	var tags []string
	if s.preferred != nil {
		tags = s.preferred(opts.Roots.Home)
	}
	keep := keepLanguageSet(tags, opts.Cfg.KeepLanguages)

	apps, err := os.ReadDir(opts.Roots.Applications)
	if err != nil {
		return core.ScanResult{Category: s.Category()} // unreadable → empty, silent (CLI parity)
	}

	var items []core.CleanableItem
	for _, app := range apps {
		if ctx.Err() != nil {
			break
		}
		if !strings.HasSuffix(app.Name(), ".app") {
			continue // top-level *.app bundles only
		}
		resources := filepath.Join(opts.Roots.Applications, app.Name(), "Contents", "Resources")
		entries, err := os.ReadDir(resources)
		if err != nil {
			continue // per-app errors silently skipped
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".lproj") {
				continue
			}
			lang := strings.TrimSuffix(e.Name(), ".lproj")
			if keep[lang] {
				continue
			}
			full := filepath.Join(resources, e.Name())
			info, err := os.Stat(full)
			if err != nil {
				continue // per-lproj errors silently skipped
			}
			mod := info.ModTime()
			items = append(items, core.CleanableItem{
				Path:        full,
				Size:        fsx.GetSize(full),
				Name:        app.Name() + ": " + e.Name(), // e.g. "Slack.app: fr.lproj"
				IsDirectory: true,
				ModifiedAt:  &mod,
			})
		}
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func (s *languageFilesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/scanners/ -run 'TestNodeModules|TestLanguageFiles|TestKeepLanguageSet' -v`
Expected: PASS (all four tests).
Then run the whole engine: `go test ./internal/...`  /  Expected: `ok` for every package (no regressions).

- [ ] **Step 9: Commit**

```
git add internal/scanners/node_modules.go internal/scanners/node_modules_test.go internal/scanners/language_files.go internal/scanners/language_files_test.go go.mod go.sum
git commit -m "feat(scanners): add node-modules and language-files scanners

node-modules: 6 fixed roots + config extra paths, depth<=4 walk, stale
(>= downloadsDaysOld, CLI's shared daysOld knob) vs orphaned detection,
never recurses into node_modules. language-files: /Applications *.app
.lproj sweep with configurable, case-sensitive keep-set (system
AppleLanguages + en + Base + config), plist-based resolver. Both
scanners self-register into the package registry via init().

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

---

### Task 12: MD5 hashing helper + duplicates scanner

**Files:**
- Create: `internal/scanners/hash.go`
- Create: `internal/scanners/duplicates.go`
- Test: `internal/scanners/hash_test.go`
- Test: `internal/scanners/duplicates_test.go`

**Interfaces:**
- Consumes: `core.*` types + `core.Categories` (core task); `Options` / `Roots` / `Scanner` / `register(s Scanner)` / `cleanWithFsx(cat core.Category, ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult` (scanners framework).
- Produces:
  - `func fileMD5(path string, limit int64) (string, error)` — **contract name**: streaming MD5 hex digest; `limit > 0` hashes only the first `limit` bytes; files shorter than `limit` hash their full content.
  - `func newDuplicatesScanner() *duplicatesScanner` — satisfies `Scanner`; `duplicates.go` registers one long-lived instance from `init()` via `register(...)`.

**Behavior being locked in (porting-notes → duplicates.ts, spec §5 + §10.7):** roots `<Home>/Downloads`, `<Home>/Documents`, `<Home>/Desktop`; recursive walk, root depth 0, guard `depth > 5`; dot-entries skipped; regular files only (symlinks skipped); minimum size fixed at 1 MiB (1048576 — the CLI default; the GUI config's `LargeFilesMinSize` belongs to large-files only). Phase 1 groups by exact byte size; phase 2, for size groups ≥ 2, regroups by partial hash (first 1 MiB — the pre-filter the CLI defined but never wired up, added deliberately per spec §10.7) and then by full MD5 within surviving partial groups ≥ 2; per-file hash errors drop that file. Phase 3: each final hash group ≥ 2 sorts by `ModifiedAt` descending (stable — first-encountered wins ties), KEEPS the newest, and lists each older copy as `"<base> (dup of <newestBase>)"`. All items sorted size desc.

- [ ] **Step 1: Write the failing hash test**

Create `internal/scanners/hash_test.go`:

```go
package scanners

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileMD5(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	full, err := fileMD5(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if full != "5eb63bbbe01eeed093cb22bb8f5acdc3" { // md5("hello world")
		t.Errorf("full hash = %q, want 5eb63bbbe01eeed093cb22bb8f5acdc3", full)
	}

	partial, err := fileMD5(p, 5)
	if err != nil {
		t.Fatal(err)
	}
	if partial != "5d41402abc4b2a76b9719d911017c592" { // md5("hello")
		t.Errorf("partial hash = %q, want 5d41402abc4b2a76b9719d911017c592", partial)
	}

	// limit beyond EOF hashes the whole file — identical to the full hash.
	beyond, err := fileMD5(p, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if beyond != full {
		t.Errorf("limit-beyond-EOF hash = %q, want %q", beyond, full)
	}

	if _, err := fileMD5(filepath.Join(dir, "missing"), 0); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestFileMD5' -v`
Expected: `FAIL … [build failed]` with `undefined: fileMD5`.

- [ ] **Step 3: Write the hash implementation**

Create `internal/scanners/hash.go`:

```go
package scanners

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

// fileMD5 returns the hex MD5 digest of a file's content, streamed (never
// loads the file into memory). limit > 0 hashes only the first limit bytes;
// files shorter than limit simply hash their full content.
func fileMD5(path string, limit int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	var r io.Reader = f
	if limit > 0 {
		r = io.LimitReader(f, limit)
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run 'TestFileMD5' -v`  /  Expected: PASS.

- [ ] **Step 5: Write the failing duplicates test**

Create `internal/scanners/duplicates_test.go`:

```go
package scanners

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const dupMiB = 1 << 20

// dupWrite creates a file (and parents) with the given content and mtime.
func dupWrite(t *testing.T, path string, content []byte, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicatesScanner(t *testing.T) {
	home := t.TempDir()
	now := time.Now()

	// Trio of identical 2 MiB files across all three roots, distinct mtimes:
	// the NEWEST (newest.bin) is kept, the two older copies are listed.
	trio := bytes.Repeat([]byte{0xAB}, 2*dupMiB)
	dupWrite(t, filepath.Join(home, "Downloads", "old.bin"), trio, now.Add(-48*time.Hour))
	dupWrite(t, filepath.Join(home, "Documents", "middle.bin"), trio, now.Add(-24*time.Hour))
	dupWrite(t, filepath.Join(home, "Desktop", "newest.bin"), trio, now)

	// Same size, different content from byte 0 → NOT duplicates.
	dupWrite(t, filepath.Join(home, "Downloads", "diff-a.bin"), bytes.Repeat([]byte{0x01}, dupMiB+1), now)
	dupWrite(t, filepath.Join(home, "Downloads", "diff-b.bin"), bytes.Repeat([]byte{0x02}, dupMiB+1), now)

	// Partial-hash collision: same size, same first MiB, different tail
	// → survives the partial pre-filter but the full hash differs → NOT duplicates.
	head := bytes.Repeat([]byte{0x0F}, dupMiB)
	collA := append(append([]byte{}, head...), []byte("tail-one")...)
	collB := append(append([]byte{}, head...), []byte("tail-two")...)
	dupWrite(t, filepath.Join(home, "Documents", "coll-a.bin"), collA, now)
	dupWrite(t, filepath.Join(home, "Documents", "coll-b.bin"), collB, now)

	// Identical but below the 1 MiB floor → ignored.
	small := bytes.Repeat([]byte{0x33}, 1024)
	dupWrite(t, filepath.Join(home, "Downloads", "small-a.bin"), small, now)
	dupWrite(t, filepath.Join(home, "Downloads", "small-b.bin"), small, now)

	// Identical pair inside a dot-directory → never walked (would otherwise
	// join the trio's 2 MiB size group and break the expected count).
	dupWrite(t, filepath.Join(home, "Downloads", ".cache", "h-a.bin"), trio, now)
	dupWrite(t, filepath.Join(home, "Downloads", ".cache", "h-b.bin"), trio, now)

	res := newDuplicatesScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %q", res.Error)
	}

	got := map[string]bool{}
	for _, it := range res.Items {
		got[it.Name] = true
		if it.IsDirectory {
			t.Errorf("%s: IsDirectory = true, want false", it.Name)
		}
		if filepath.Base(it.Path) == "newest.bin" {
			t.Errorf("newest copy was listed for deletion: %q", it.Path)
		}
		if it.Size != int64(2*dupMiB) {
			t.Errorf("%s: Size = %d, want %d", it.Name, it.Size, 2*dupMiB)
		}
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items (%v), want exactly the 2 older trio copies", len(res.Items), got)
	}
	for _, w := range []string{"old.bin (dup of newest.bin)", "middle.bin (dup of newest.bin)"} {
		if !got[w] {
			t.Errorf("missing expected item %q (got %v)", w, got)
		}
	}
	if res.TotalSize != int64(4*dupMiB) {
		t.Errorf("TotalSize = %d, want %d", res.TotalSize, 4*dupMiB)
	}

	// Sorted by size descending.
	for i := 1; i < len(res.Items); i++ {
		if res.Items[i-1].Size < res.Items[i].Size {
			t.Errorf("items not sorted by size desc at index %d", i)
		}
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestDuplicates' -v`
Expected: `FAIL … [build failed]` with `undefined: newDuplicatesScanner`.

- [ ] **Step 7: Write the duplicates implementation**

Create `internal/scanners/duplicates.go`:

```go
package scanners

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

const (
	// Root call is depth 0, guard is `depth > max`: dirs at depth 0..5 are read.
	duplicatesMaxDepth = 5
	// Fixed candidate floor (CLI: MIN_FILE_SIZE = 1 MiB). Deliberately NOT tied
	// to Cfg.LargeFilesMinSize — that knob belongs to the large-files scanner.
	duplicatesMinSize = 1 << 20
	// Partial-hash prefix length (spec §10.7 pre-filter).
	duplicatesHashLimit = 1 << 20
)

type dupFileInfo struct {
	path       string
	size       int64
	modifiedAt time.Time
}

type duplicatesScanner struct{}

func newDuplicatesScanner() *duplicatesScanner { return &duplicatesScanner{} }

func init() { register(newDuplicatesScanner()) }

func (s *duplicatesScanner) Category() core.Category {
	return core.Categories["duplicates"]
}

func (s *duplicatesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	roots := []string{
		filepath.Join(opts.Roots.Home, "Downloads"),
		filepath.Join(opts.Roots.Home, "Documents"),
		filepath.Join(opts.Roots.Home, "Desktop"),
	}

	// Phase 1: group candidate files by exact byte size.
	bySize := map[int64][]dupFileInfo{}
	for _, root := range roots {
		collectDupCandidates(ctx, root, 0, bySize)
	}

	var items []core.CleanableItem
	for _, group := range bySize {
		if len(group) < 2 {
			continue
		}
		// Phase 2a: cheap pre-filter — regroup by hash of the first 1 MiB.
		byPartial := map[string][]dupFileInfo{}
		for _, f := range group {
			h, err := fileMD5(f.path, duplicatesHashLimit)
			if err != nil {
				continue // unreadable → drop this file, keep going
			}
			byPartial[h] = append(byPartial[h], f)
		}
		for _, pg := range byPartial {
			if len(pg) < 2 {
				continue
			}
			// Phase 2b: confirm with the full-content hash. (Files exactly
			// limit-sized hash identically in both passes — handled naturally.)
			byFull := map[string][]dupFileInfo{}
			for _, f := range pg {
				h, err := fileMD5(f.path, 0)
				if err != nil {
					continue
				}
				byFull[h] = append(byFull[h], f)
			}
			// Phase 3: newest kept, older copies become items.
			for _, dg := range byFull {
				if len(dg) < 2 {
					continue
				}
				sort.SliceStable(dg, func(i, j int) bool {
					return dg[i].modifiedAt.After(dg[j].modifiedAt)
				})
				newest := filepath.Base(dg[0].path)
				for _, f := range dg[1:] {
					mod := f.modifiedAt
					items = append(items, core.CleanableItem{
						Path:        f.path,
						Size:        f.size,
						Name:        filepath.Base(f.path) + " (dup of " + newest + ")",
						IsDirectory: false,
						ModifiedAt:  &mod,
					})
				}
			}
		}
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].Size > items[j].Size })

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func collectDupCandidates(ctx context.Context, dir string, depth int, bySize map[int64][]dupFileInfo) {
	if depth > duplicatesMaxDepth || ctx.Err() != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // missing root / unreadable dir → silent skip
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
			collectDupCandidates(ctx, full, depth+1, bySize)
		case e.Type().IsRegular(): // lstat-based: symlinks are skipped
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.Size() < duplicatesMinSize {
				continue
			}
			bySize[info.Size()] = append(bySize[info.Size()], dupFileInfo{
				path:       full,
				size:       info.Size(),
				modifiedAt: info.ModTime(),
			})
		}
	}
}

func (s *duplicatesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/scanners/ -run 'TestFileMD5|TestDuplicates' -v`  /  Expected: PASS.
Then: `go test ./internal/...`  /  Expected: all packages `ok`.

- [ ] **Step 9: Commit**

```
git add internal/scanners/hash.go internal/scanners/hash_test.go internal/scanners/duplicates.go internal/scanners/duplicates_test.go
git commit -m "feat(scanners): add md5 hashing helper and duplicates scanner

fileMD5 streams full or first-N-bytes digests. Duplicates: Downloads/
Documents/Desktop, depth<=5, files >=1MiB; size grouping -> partial-hash
pre-filter (first 1MiB, the optimization the CLI never wired up) -> full
MD5 -> keep-newest, older copies listed as '<base> (dup of <newest>)'.
The scanner self-registers via init().

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

---

### Task 13: launch-agents scanner (real plist parsing)

**Files:**
- Create: `internal/scanners/launch_agents.go`
- Test: `internal/scanners/launch_agents_test.go`

**Interfaces:**
- Consumes: `core.*` + `core.Categories`; `Options` / `Roots` / `Scanner` / `register(s Scanner)` / `cleanWithFsx(...)` (scanners framework); `howett.net/plist` (already in go.mod from Task 11 — if executing this task standalone, run `go get howett.net/plist@v1.0.1` first; the command is idempotent).
- Produces:
  - `func newLaunchAgentsScanner() *launchAgentsScanner` — satisfies `Scanner`; `launch_agents.go` registers one long-lived instance from `init()` via `register(...)`.
  - `func launchAgentProgram(plistPath string) string` (package-local): extracts `Program`, else `ProgramArguments[0]`, from an XML **or** binary plist; `""` on any parse/read failure or when no program key exists.
  - `var systemBinaryPrefixes []string` (package-local).

**Behavior being locked in (porting-notes → launch-agents.ts, spec §5 + §10.8):** scan `<Home>/Library/LaunchAgents` only. Missing dir → empty result, **no** error; any other readdir failure → `ScanResult.Error = "Failed to read LaunchAgents directory: <err>"`. Only `*.plist` files. Program path = `Program` (string) with precedence over `ProgramArguments[0]`; trimmed. Skip silently when: no program found, path not absolute, or path starts with any of `/usr/bin/`, `/bin/`, `/sbin/`, `/usr/sbin/`, `/usr/local/bin/`, `/opt/homebrew/bin/`. Orphaned = the program path does not exist → item `{Path: plist file, Size: plist file size, Name: "<plistfile> → <programPath> (missing)", IsDirectory: false, ModifiedAt: plist mtime}` (Unicode arrow `" → "`). Per-plist errors skip silently. Clean = `cleanWithFsx` (deletes the plist file only; the agent stays loaded until logout — CLI parity).

- [ ] **Step 1: Write the failing test**

Create `internal/scanners/launch_agents_test.go`:

```go
package scanners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// laHome creates a temp home with an existing Library/LaunchAgents dir.
func laHome(t *testing.T) (home, agents string) {
	t.Helper()
	home = t.TempDir()
	agents = filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, agents
}

// laWriteXML writes an XML plist with a Program key.
func laWriteXML(t *testing.T, path, program string) {
	t.Helper()
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>test.label</string>
	<key>Program</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, program)
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// laWriteBinary writes a BINARY plist (bplist00) encoded with the same library
// the scanner uses — proves binary-plist coverage the CLI's regex never had.
func laWriteBinary(t *testing.T, path string, dict map[string]interface{}) {
	t.Helper()
	data, err := plist.Marshal(dict, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchAgentsScanner(t *testing.T) {
	home, agents := laHome(t)
	targets := t.TempDir()
	missing := filepath.Join(targets, "gone-binary")
	existing := filepath.Join(targets, "real-binary")
	if err := os.WriteFile(existing, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. XML plist, Program → missing path: ORPHANED.
	laWriteXML(t, filepath.Join(agents, "com.test.xml-orphan.plist"), missing)
	// 2. Binary plist, ProgramArguments[0] → missing path: ORPHANED.
	laWriteBinary(t, filepath.Join(agents, "com.test.bin-orphan.plist"), map[string]interface{}{
		"Label":            "com.test.bin-orphan",
		"ProgramArguments": []string{missing, "--flag"},
	})
	// 3. Existing target → skipped.
	laWriteXML(t, filepath.Join(agents, "com.test.alive.plist"), existing)
	// 4. System-binary prefix → skipped even though the path doesn't exist.
	laWriteXML(t, filepath.Join(agents, "com.test.system.plist"), "/usr/bin/no-such-tool-xyz")
	// 5. Program takes precedence over ProgramArguments[0]:
	//    Program exists → skipped, even though ProgramArguments[0] is missing.
	laWriteBinary(t, filepath.Join(agents, "com.test.precedence.plist"), map[string]interface{}{
		"Program":          existing,
		"ProgramArguments": []string{missing},
	})
	// 6. Relative program path → skipped.
	laWriteXML(t, filepath.Join(agents, "com.test.relative.plist"), "relative/bin/tool")
	// 7. Non-.plist files → ignored.
	if err := os.WriteFile(filepath.Join(agents, "README.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %q", res.Error)
	}

	got := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		got[it.Name] = it
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items (%v), want exactly the two orphans", len(res.Items), got)
	}

	wantXML := fmt.Sprintf("com.test.xml-orphan.plist → %s (missing)", missing)
	xmlItem, ok := got[wantXML]
	if !ok {
		t.Fatalf("missing XML orphan item %q in %v", wantXML, got)
	}
	if xmlItem.Path != filepath.Join(agents, "com.test.xml-orphan.plist") {
		t.Errorf("XML orphan Path = %q, want the plist file path", xmlItem.Path)
	}
	fi, err := os.Stat(xmlItem.Path)
	if err != nil {
		t.Fatal(err)
	}
	if xmlItem.Size != fi.Size() {
		t.Errorf("Size = %d, want plist file size %d", xmlItem.Size, fi.Size())
	}
	if xmlItem.IsDirectory {
		t.Error("IsDirectory = true, want false")
	}
	if xmlItem.ModifiedAt == nil {
		t.Error("ModifiedAt is nil, want plist mtime")
	}

	wantBin := fmt.Sprintf("com.test.bin-orphan.plist → %s (missing)", missing)
	if _, ok := got[wantBin]; !ok {
		t.Errorf("missing binary-plist orphan item %q (got %v)", wantBin, got)
	}
}

func TestLaunchAgentsScannerMissingAndUnreadableDir(t *testing.T) {
	// Missing ~/Library/LaunchAgents → empty result, NO error.
	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}})
	if res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing dir: got error=%q items=%d, want clean empty result", res.Error, len(res.Items))
	}

	// Path exists but is not a directory → readdir failure → ScanResult.Error.
	home := t.TempDir()
	lib := filepath.Join(home, "Library")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "LaunchAgents"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if !strings.HasPrefix(res.Error, "Failed to read LaunchAgents directory: ") {
		t.Fatalf("Error = %q, want prefix 'Failed to read LaunchAgents directory: '", res.Error)
	}
}

func TestLaunchAgentsClean(t *testing.T) {
	home, agents := laHome(t)
	missing := filepath.Join(t.TempDir(), "gone")
	laWriteXML(t, filepath.Join(agents, "com.test.rm.plist"), missing)

	s := newLaunchAgentsScanner()
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if len(res.Items) != 1 {
		t.Fatalf("scan found %d items, want 1", len(res.Items))
	}

	noop := func(current, total int, item core.CleanableItem) {}
	cr := s.Clean(context.Background(), res.Items, false, noop)
	if cr.CleanedItems != 1 || len(cr.Errors) != 0 {
		t.Fatalf("CleanResult = %+v, want 1 cleaned / no errors", cr)
	}
	if cr.FreedSpace != res.Items[0].Size {
		t.Errorf("FreedSpace = %d, want %d", cr.FreedSpace, res.Items[0].Size)
	}
	if _, err := os.Stat(res.Items[0].Path); !os.IsNotExist(err) {
		t.Errorf("plist still exists after clean (stat err = %v)", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestLaunchAgents' -v`
Expected: `FAIL … [build failed]` with `undefined: newLaunchAgentsScanner`.

- [ ] **Step 3: Write the implementation**

(If go.mod does not yet contain the plist library, run `go get howett.net/plist@v1.0.1` — idempotent, already added in Task 11.)

Create `internal/scanners/launch_agents.go`:

```go
package scanners

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// Program paths under these prefixes are assumed managed by the system or a
// package manager — never reported as orphaned (CLI SYSTEM_BINARY_PREFIXES).
var systemBinaryPrefixes = []string{
	"/usr/bin/", "/bin/", "/sbin/", "/usr/sbin/", "/usr/local/bin/", "/opt/homebrew/bin/",
}

type launchAgentsScanner struct{}

func newLaunchAgentsScanner() *launchAgentsScanner { return &launchAgentsScanner{} }

func init() { register(newLaunchAgentsScanner()) }

func (s *launchAgentsScanner) Category() core.Category {
	return core.Categories["launch-agents"]
}

// launchAgentProgram extracts the target program path from a LaunchAgent
// plist (XML or binary — spec §10.8 replaces the CLI's regex-on-text):
// Program (string) wins; otherwise ProgramArguments[0]. "" = none found
// or unparseable (caller skips silently, CLI parity).
func launchAgentProgram(plistPath string) string {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return ""
	}
	var payload struct {
		Program          string   `plist:"Program"`
		ProgramArguments []string `plist:"ProgramArguments"`
	}
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if p := strings.TrimSpace(payload.Program); p != "" {
		return p
	}
	if len(payload.ProgramArguments) > 0 {
		return strings.TrimSpace(payload.ProgramArguments[0])
	}
	return ""
}

func hasSystemBinaryPrefix(p string) bool {
	for _, prefix := range systemBinaryPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func (s *launchAgentsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "LaunchAgents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return core.ScanResult{Category: s.Category()} // no LaunchAgents dir → nothing to report
		}
		return core.ScanResult{
			Category: s.Category(),
			Error:    fmt.Sprintf("Failed to read LaunchAgents directory: %v", err),
		}
	}

	var items []core.CleanableItem
	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		plistPath := filepath.Join(dir, e.Name())

		program := launchAgentProgram(plistPath)
		if program == "" || !filepath.IsAbs(program) || hasSystemBinaryPrefix(program) {
			continue
		}
		if _, err := os.Stat(program); err == nil {
			continue // target exists → not orphaned
		}

		info, err := e.Info()
		if err != nil {
			continue // per-plist errors skip silently
		}
		mod := info.ModTime()
		items = append(items, core.CleanableItem{
			Path:        plistPath,
			Size:        info.Size(),
			Name:        fmt.Sprintf("%s → %s (missing)", e.Name(), program),
			IsDirectory: false,
			ModifiedAt:  &mod,
		})
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func (s *launchAgentsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	// Deletes the .plist only — deliberately no `launchctl bootout` (CLI parity);
	// the agent stays loaded until logout/reboot.
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scanners/ -run 'TestLaunchAgents' -v`  /  Expected: PASS (all three tests).
Then: `go test ./internal/...`  /  Expected: all packages `ok`.

- [ ] **Step 5: Commit**

```
git add internal/scanners/launch_agents.go internal/scanners/launch_agents_test.go
git commit -m "feat(scanners): add orphaned launch-agents scanner with plist parsing

Parses ~/Library/LaunchAgents/*.plist with howett.net/plist (XML AND
binary — fixes the CLI's regex-on-text gap). Program wins over
ProgramArguments[0]; relative paths and system-binary prefixes exempt;
orphan = target path missing. Clean deletes the plist file only. The
scanner self-registers via init().

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

---

### Task 14: homebrew + docker command-backed scanners

**Files:**
- Create: `internal/scanners/homebrew.go`
- Create: `internal/scanners/docker.go`
- Test: `internal/scanners/homebrew_test.go` (also defines the shared `scriptedRunner` fake + `mkExec` helper used by `docker_test.go`)
- Test: `internal/scanners/docker_test.go` (also carries `TestAllScannersRegistered`, the final registry-completeness check — this is the last scanner task)

**Interfaces:**
- Consumes: `core.*` + `core.Categories`; `fsx.GetSize`; scanners framework `CmdRunner interface { Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (stdout string, err error) }`, `Options`, `Roots`, `Scanner`, `register(s Scanner)`, `All() []Scanner`, `cleanWithFsx(...)`; `core.CategoriesInOrder()` (registry-completeness test).
- Produces (package-local; each file's `init()` calls `register(...)` with one long-lived instance per category):
  - `func newHomebrewScanner() *homebrewScanner` — stateful: caches `runner`, `brewPath` from Scan for Clean (porting-notes: keep one long-lived instance per category).
  - `func newDockerScanner() *dockerScanner` — stateful: caches `runner`, `dockerPath`.
  - `func findExecutable(candidates []string) string` — first candidate passing an X_OK `syscall.Access` check; `""` if none. $PATH is NEVER consulted (hijack prevention).
  - `func parseDockerSize(s string) int64` — regex `([\d.]+)\s*([kKMGT]?B)` with **SI/decimal** multipliers (kB=1e3, MB=1e6, GB=1e9, TB=1e12; spec §10.5 fixes the CLI's 1024 bug).
  - `func externalDryRun(cat core.Category, items []core.CleanableItem, progress core.ProgressFunc) core.CleanResult` — dry-run short-circuit shared by both command-backed scanners.
  - Test-only (in `homebrew_test.go`): `scriptedRunner` (records `scriptedCall{Timeout, Bin, Args}`, replays `scriptedResponse{Stdout, Err}` in order), `mkExec(t, dir, name) string`.
  - Test-only (in `docker_test.go`): `TestAllScannersRegistered` — final registry-completeness gate: `len(All()) == 16` and the `All()` category ids match `core.CategoriesInOrder()` exactly.

**Behavior being locked in (porting-notes → homebrew.ts / docker.ts, spec §5 + §10.5 + §10.14):**
- homebrew: brew allowlist `/opt/homebrew/bin/brew`, `/usr/local/bin/brew` (macOS-only port drops the CLI's linuxbrew path); none executable → empty result, zero commands run. Scan: `Run(ctx, 30s, brew, "--cache")`; trimmed stdout must equal or live under (`prefix+"/"`) one of `<Home>/Library/Caches/Homebrew`, `/opt/homebrew/Caches`, `/usr/local/Caches` — otherwise log a warning and return empty. Valid + exists + recursive size > 0 → exactly ONE item `{Path: cachePath, Name: "Homebrew Download Cache", IsDirectory: true}`. Clean: dryRun → all cleaned + precomputed sizes; else re-resolve `brew --cache` (30s) and if any selected item's Path EXACTLY equals the current cache root → `Run(ctx, 60s, brew, "cleanup", "--prune=all")` with freed = precomputed sum, failure → single error `"Homebrew cleanup failed: <msg>"` with 0 cleaned/0 freed; otherwise (subpaths only / no brew / `--cache` failed) → fall back to `cleanWithFsx` (direct deletion).
- docker: allowlist `/usr/local/bin/docker`, `/opt/homebrew/bin/docker`, `/Applications/Docker.app/Contents/Resources/bin/docker`; none, or any scan-time command error (daemon down) → silently empty. Scan: `Run(ctx, 30s, docker, "system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}")`; per line split on `\t`, lowercased+trimmed type must be `images` | `containers` | `build cache` — `local volumes` rows are EXCLUDED (spec §10.14: `prune -af` without `--volumes` never frees them) and anything else is dropped (injection guard). Reclaimable parsed via `parseDockerSize`; item only when > 0: `{Path: "docker:"+lowercased type with spaces→'-', Name: "Docker <original trimmed type text>", IsDirectory: false}`. Clean: dryRun → all; else `Run(ctx, 60s, docker, "system", "prune", "-af")` — NEVER `--volumes`; freed = precomputed sum; failure → `"Docker cleanup failed: <msg>"`, 0 cleaned/0 freed; missing binary → error `"Docker binary not found in safe locations"`.
- registration: like every scanner file, `homebrew.go` and `docker.go` self-register a long-lived instance from `init()`. This is the LAST scanner task — after it all 16 scanner files exist — so `docker_test.go` closes with a registry-completeness test: `len(All()) == 16` and the `All()` order matches `core.CategoriesInOrder()` ids exactly.

- [ ] **Step 1: Write the failing homebrew test (includes the shared fake runner)**

Create `internal/scanners/homebrew_test.go`:

```go
package scanners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// scriptedRunner is a CmdRunner fake: records every call, replays scripted
// responses in order, and errors on unexpected extra calls.
type scriptedCall struct {
	Timeout time.Duration
	Bin     string
	Args    []string
}

type scriptedResponse struct {
	Stdout string
	Err    error
}

type scriptedRunner struct {
	calls     []scriptedCall
	responses []scriptedResponse
}

func (r *scriptedRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	r.calls = append(r.calls, scriptedCall{Timeout: timeout, Bin: bin, Args: args})
	if len(r.responses) == 0 {
		return "", fmt.Errorf("scriptedRunner: unexpected call: %s %v", bin, args)
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	return resp.Stdout, resp.Err
}

// mkExec drops an executable (0755) fake binary and returns its path.
func mkExec(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// hbFixture creates a fake brew binary and a populated allowed cache dir.
func hbFixture(t *testing.T) (brew, home, cache string) {
	t.Helper()
	brew = mkExec(t, t.TempDir(), "brew")
	home = t.TempDir()
	cache = filepath.Join(home, "Library", "Caches", "Homebrew")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "pkg.tar.gz"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	return brew, home, cache
}

func TestFindExecutableOrdering(t *testing.T) {
	dir := t.TempDir()
	second := mkExec(t, dir, "second")
	third := mkExec(t, dir, "third")
	nonExec := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(nonExec, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// First existing+executable candidate wins; missing and non-executable skipped.
	if got := findExecutable([]string{filepath.Join(dir, "missing"), nonExec, second, third}); got != second {
		t.Errorf("findExecutable = %q, want %q", got, second)
	}
	if got := findExecutable([]string{filepath.Join(dir, "nope")}); got != "" {
		t.Errorf("findExecutable = %q, want \"\"", got)
	}
}

func TestHomebrewScan(t *testing.T) {
	brew, home, cache := hbFixture(t)
	r := &scriptedRunner{responses: []scriptedResponse{{Stdout: cache + "\n"}}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}

	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})
	if res.Error != "" || len(res.Items) != 1 {
		t.Fatalf("result = %+v, want exactly one item and no error", res)
	}
	it := res.Items[0]
	if it.Name != "Homebrew Download Cache" || it.Path != cache || !it.IsDirectory || it.Size <= 0 {
		t.Errorf("unexpected item %+v", it)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %+v, want exactly one brew --cache call", r.calls)
	}
	c := r.calls[0]
	if c.Bin != brew || !reflect.DeepEqual(c.Args, []string{"--cache"}) || c.Timeout != 30*time.Second {
		t.Errorf("--cache call = %+v, want [--cache] @ 30s on %s", c, brew)
	}
}

func TestHomebrewScanEmptyCases(t *testing.T) {
	// No brew binary in the allowlist → empty result, no commands run.
	r := &scriptedRunner{}
	s := newHomebrewScanner()
	s.candidates = []string{filepath.Join(t.TempDir(), "missing-brew")}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" || len(r.calls) != 0 {
		t.Fatalf("no-binary case: items=%v error=%q calls=%v, want all empty", res.Items, res.Error, r.calls)
	}

	// Cache path outside the allowlisted roots → warn + empty.
	brew := mkExec(t, t.TempDir(), "brew")
	evil := t.TempDir()
	if err := os.WriteFile(filepath.Join(evil, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = &scriptedRunner{responses: []scriptedResponse{{Stdout: evil + "\n"}}}
	s = newHomebrewScanner()
	s.candidates = []string{brew}
	res = s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 {
		t.Fatalf("out-of-allowlist cache: items = %+v, want none", res.Items)
	}

	// brew --cache failing → empty (scan errors are swallowed).
	r = &scriptedRunner{responses: []scriptedResponse{{Err: errors.New("brew broke")}}}
	s = newHomebrewScanner()
	s.candidates = []string{brew}
	res = s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" {
		t.Fatalf("command-failure case: %+v, want silent empty", res)
	}
}

func TestHomebrewCleanCacheRootUsesBrewCleanup(t *testing.T) {
	brew, home, cache := hbFixture(t)
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: cache + "\n"}, // scan: brew --cache
		{Stdout: cache + "\n"}, // clean: brew --cache (re-resolved)
		{Stdout: "cleaned"},    // clean: brew cleanup --prune=all
	}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if len(cr.Errors) != 0 || cr.CleanedItems != 1 || cr.FreedSpace != res.Items[0].Size {
		t.Fatalf("CleanResult = %+v, want 1 cleaned / freed %d", cr, res.Items[0].Size)
	}
	if len(r.calls) != 3 {
		t.Fatalf("calls = %+v, want 3 (scan --cache, clean --cache, cleanup)", r.calls)
	}
	cleanup := r.calls[2]
	if cleanup.Bin != brew || !reflect.DeepEqual(cleanup.Args, []string{"cleanup", "--prune=all"}) || cleanup.Timeout != 60*time.Second {
		t.Errorf("cleanup call = %+v, want [cleanup --prune=all] @ 60s", cleanup)
	}
	// brew owns the deletion — the scanner must NOT rm the cache itself.
	if _, err := os.Stat(filepath.Join(cache, "pkg.tar.gz")); err != nil {
		t.Errorf("brew-cleanup path deleted files directly: %v", err)
	}
}

func TestHomebrewCleanSubpathFallsBackToDirectDelete(t *testing.T) {
	brew, home, cache := hbFixture(t)
	sub := filepath.Join(cache, "old-download.tar.gz")
	if err := os.WriteFile(sub, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: cache + "\n"}, // scan: brew --cache
		{Stdout: cache + "\n"}, // clean: brew --cache — root NOT among selected items
	}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}
	s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})

	items := []core.CleanableItem{{Path: sub, Size: 10, Name: "old-download.tar.gz"}}
	noop := func(current, total int, item core.CleanableItem) {}
	cr := s.Clean(context.Background(), items, false, noop)
	if cr.CleanedItems != 1 || cr.FreedSpace != 10 || len(cr.Errors) != 0 {
		t.Fatalf("CleanResult = %+v, want direct delete of the subpath", cr)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls = %+v, want 2 (no cleanup invocation)", r.calls)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Errorf("subpath not deleted (stat err = %v)", err)
	}
	// The cache root itself must survive.
	if _, err := os.Stat(cache); err != nil {
		t.Errorf("cache root should still exist: %v", err)
	}
}

func TestHomebrewCleanupFailure(t *testing.T) {
	brew, home, cache := hbFixture(t)
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: cache + "\n"},          // scan
		{Stdout: cache + "\n"},          // clean --cache
		{Err: errors.New("boom")},       // cleanup fails
	}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if cr.CleanedItems != 0 || cr.FreedSpace != 0 {
		t.Errorf("CleanResult = %+v, want 0 cleaned / 0 freed on failure", cr)
	}
	if len(cr.Errors) != 1 || cr.Errors[0] != "Homebrew cleanup failed: boom" {
		t.Errorf("Errors = %v, want ['Homebrew cleanup failed: boom']", cr.Errors)
	}
}

func TestHomebrewCleanDryRun(t *testing.T) {
	r := &scriptedRunner{}
	s := newHomebrewScanner() // no Scan needed: dry-run short-circuits
	items := []core.CleanableItem{{Path: "/anything", Size: 42, Name: "Homebrew Download Cache"}}
	cr := s.Clean(context.Background(), items, true, nil)
	if cr.CleanedItems != 1 || cr.FreedSpace != 42 || len(cr.Errors) != 0 {
		t.Fatalf("dry-run CleanResult = %+v, want full success without commands", cr)
	}
	if len(r.calls) != 0 {
		t.Fatalf("dry run must not execute commands: %v", r.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestHomebrew|TestFindExecutable' -v`
Expected: `FAIL … [build failed]` with `undefined: findExecutable` / `undefined: newHomebrewScanner`.

- [ ] **Step 3: Write the homebrew implementation**

Create `internal/scanners/homebrew.go`:

```go
package scanners

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// Brew binary allowlist, priority order. $PATH is never consulted (hijack
// prevention). macOS-only port: the CLI's linuxbrew path is dropped.
var defaultBrewCandidates = []string{
	"/opt/homebrew/bin/brew", // Apple Silicon
	"/usr/local/bin/brew",    // Intel
}

// findExecutable returns the first candidate that passes an X_OK access
// check, or "" when none does. Shared by the homebrew and docker scanners.
func findExecutable(candidates []string) string {
	for _, p := range candidates {
		if syscall.Access(p, 0x1) == nil { // 0x1 = X_OK
			return p
		}
	}
	return ""
}

// externalDryRun is the dry-run short-circuit shared by the command-backed
// scanners (homebrew, docker): every item counts as cleaned, full size
// credited, no commands executed, no disk IO.
func externalDryRun(cat core.Category, items []core.CleanableItem, progress core.ProgressFunc) core.CleanResult {
	var freed int64
	for i, it := range items {
		if progress != nil {
			progress(i+1, len(items), it)
		}
		freed += it.Size
	}
	return core.CleanResult{Category: cat, CleanedItems: len(items), FreedSpace: freed}
}

type homebrewScanner struct {
	candidates []string  // injectable in tests
	runner     CmdRunner // cached from the last Scan, reused by Clean
	brewPath   string    // cached from the last Scan
}

func newHomebrewScanner() *homebrewScanner {
	return &homebrewScanner{candidates: defaultBrewCandidates}
}

func init() { register(newHomebrewScanner()) }

func (s *homebrewScanner) Category() core.Category {
	return core.Categories["homebrew"]
}

// brewCacheAllowedRoots lists where `brew --cache` output may point; anything
// else is rejected (command-output allowlist validation, CLI parity).
func brewCacheAllowedRoots(home string) []string {
	return []string{
		filepath.Join(home, "Library", "Caches", "Homebrew"),
		"/opt/homebrew/Caches",
		"/usr/local/Caches",
	}
}

func validBrewCachePath(p, home string) bool {
	resolved := filepath.Clean(p)
	if !filepath.IsAbs(resolved) {
		return false
	}
	for _, root := range brewCacheAllowedRoots(home) {
		if resolved == root || strings.HasPrefix(resolved, root+"/") {
			return true
		}
	}
	return false
}

func (s *homebrewScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	s.runner = opts.Runner
	s.brewPath = findExecutable(s.candidates)
	if s.brewPath == "" {
		return core.ScanResult{Category: s.Category()} // Homebrew not installed
	}

	out, err := opts.Runner.Run(ctx, 30*time.Second, s.brewPath, "--cache")
	if err != nil {
		return core.ScanResult{Category: s.Category()} // scan errors are swallowed
	}
	cachePath := strings.TrimSpace(out)
	if !validBrewCachePath(cachePath, opts.Roots.Home) {
		log.Printf("Unexpected Homebrew cache location: %s", cachePath)
		return core.ScanResult{Category: s.Category()}
	}

	info, err := os.Stat(cachePath)
	if err != nil {
		return core.ScanResult{Category: s.Category()} // cache dir does not exist
	}
	size := fsx.GetSize(cachePath)
	if size <= 0 {
		return core.ScanResult{Category: s.Category()}
	}
	mod := info.ModTime()
	item := core.CleanableItem{
		Path:        cachePath,
		Size:        size,
		Name:        "Homebrew Download Cache",
		IsDirectory: true,
		ModifiedAt:  &mod,
	}
	return core.ScanResult{Category: s.Category(), Items: []core.CleanableItem{item}, TotalSize: size}
}

func (s *homebrewScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	if dryRun {
		return externalDryRun(s.Category(), items, progress)
	}
	if s.brewPath == "" {
		s.brewPath = findExecutable(s.candidates)
	}
	if s.brewPath == "" || s.runner == nil {
		// No brew (or Clean before Scan) → plain filesystem deletion.
		return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
	}

	// Re-resolve the CURRENT cache root, exactly like the CLI's clean() does.
	cacheRoot := ""
	if out, err := s.runner.Run(ctx, 30*time.Second, s.brewPath, "--cache"); err == nil {
		cacheRoot = strings.TrimSpace(out)
	}
	selected := false
	if cacheRoot != "" {
		for _, it := range items {
			if it.Path == cacheRoot {
				selected = true
				break
			}
		}
	}
	if !selected {
		// Cache root not among the selection → direct deletion of what was picked.
		return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
	}

	var freed int64
	for i, it := range items {
		if progress != nil {
			progress(i+1, len(items), it)
		}
		freed += it.Size // freed space = PRE-computed scan sizes, never re-measured
	}
	if _, err := s.runner.Run(ctx, 60*time.Second, s.brewPath, "cleanup", "--prune=all"); err != nil {
		return core.CleanResult{
			Category: s.Category(),
			Errors:   []string{"Homebrew cleanup failed: " + err.Error()},
		}
	}
	return core.CleanResult{Category: s.Category(), CleanedItems: len(items), FreedSpace: freed}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanners/ -run 'TestHomebrew|TestFindExecutable' -v`  /  Expected: PASS (all six tests).

- [ ] **Step 5: Write the failing docker test (plus the final registry-completeness test)**

Create `internal/scanners/docker_test.go`:

```go
package scanners

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestDefaultCandidateAllowlists(t *testing.T) {
	wantBrew := []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"}
	if !reflect.DeepEqual(defaultBrewCandidates, wantBrew) {
		t.Errorf("defaultBrewCandidates = %v, want %v", defaultBrewCandidates, wantBrew)
	}
	wantDocker := []string{
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
		"/Applications/Docker.app/Contents/Resources/bin/docker",
	}
	if !reflect.DeepEqual(defaultDockerCandidates, wantDocker) {
		t.Errorf("defaultDockerCandidates = %v, want %v", defaultDockerCandidates, wantDocker)
	}
}

func TestParseDockerSize(t *testing.T) {
	cases := map[string]int64{
		"1.5GB":         1500000000, // SI/decimal — spec §10.5 fixes the CLI's 1024 bug
		"250.5MB (62%)":  250500000,
		"2kB":            2000,
		"2KB":            2000,
		"1TB":            1000000000000,
		"512B":           512,
		"0B":             0,
		"N/A":            0,
	}
	for in, want := range cases {
		if got := parseDockerSize(in); got != want {
			t.Errorf("parseDockerSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestDockerScanParsesDf(t *testing.T) {
	docker := mkExec(t, t.TempDir(), "docker")
	df := "Images\t3.2GB\t1.5GB (46%)\n" +
		"Containers\t400MB\t0B (0%)\n" + // zero reclaimable → skipped
		"Local Volumes\t120GB\t120GB (100%)\n" + // EXCLUDED: prune -af never frees volumes
		"Build Cache\t250.5kB\t250.5kB\n" +
		"Evil Injection\t9GB\t9GB\n" // unknown type → dropped (injection guard)
	r := &scriptedRunner{responses: []scriptedResponse{{Stdout: df}}}
	s := newDockerScanner()
	s.candidates = []string{docker}

	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if res.Error != "" || len(res.Items) != 2 {
		t.Fatalf("result = %+v, want images + build cache only", res)
	}

	byPath := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byPath[it.Path] = it
		if it.IsDirectory {
			t.Errorf("%s: IsDirectory = true, want false", it.Path)
		}
	}
	img := byPath["docker:images"]
	if img.Size != 1500000000 || img.Name != "Docker Images" {
		t.Errorf("images item = %+v, want size 1500000000 / name 'Docker Images'", img)
	}
	bc := byPath["docker:build-cache"]
	if bc.Size != 250500 || bc.Name != "Docker Build Cache" {
		t.Errorf("build-cache item = %+v, want size 250500 / name 'Docker Build Cache'", bc)
	}
	if res.TotalSize != 1500250500 {
		t.Errorf("TotalSize = %d, want 1500250500", res.TotalSize)
	}

	if len(r.calls) != 1 {
		t.Fatalf("calls = %+v, want one df call", r.calls)
	}
	wantArgs := []string{"system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}"}
	if r.calls[0].Bin != docker || !reflect.DeepEqual(r.calls[0].Args, wantArgs) || r.calls[0].Timeout != 30*time.Second {
		t.Errorf("df call = %+v, want args %v @ 30s", r.calls[0], wantArgs)
	}
}

func TestDockerScanSilentlyEmpty(t *testing.T) {
	// No binary in the allowlist → empty, zero commands.
	r := &scriptedRunner{}
	s := newDockerScanner()
	s.candidates = []string{filepath.Join(t.TempDir(), "missing-docker")}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" || len(r.calls) != 0 {
		t.Fatalf("no-binary case: %+v / calls %v, want silent empty", res, r.calls)
	}

	// Daemon down (command error) → empty, still NO ScanResult.Error.
	docker := mkExec(t, t.TempDir(), "docker")
	r = &scriptedRunner{responses: []scriptedResponse{{Err: errors.New("Cannot connect to the Docker daemon")}}}
	s = newDockerScanner()
	s.candidates = []string{docker}
	res = s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" {
		t.Fatalf("daemon-down case: %+v, want silent empty", res)
	}
}

func TestDockerCleanPrune(t *testing.T) {
	docker := mkExec(t, t.TempDir(), "docker")
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: "Images\t3.2GB\t1.5GB (46%)\n"}, // scan df
		{Stdout: "Total reclaimed space: 1.5GB"}, // prune
	}}
	s := newDockerScanner()
	s.candidates = []string{docker}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if cr.CleanedItems != 1 || cr.FreedSpace != 1500000000 || len(cr.Errors) != 0 {
		t.Fatalf("CleanResult = %+v, want 1 cleaned / freed 1500000000", cr)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls = %+v, want df + prune", r.calls)
	}
	prune := r.calls[1]
	wantArgs := []string{"system", "prune", "-af"} // NEVER --volumes
	if prune.Bin != docker || !reflect.DeepEqual(prune.Args, wantArgs) || prune.Timeout != 60*time.Second {
		t.Errorf("prune call = %+v, want args %v @ 60s", prune, wantArgs)
	}
	for _, a := range prune.Args {
		if a == "--volumes" {
			t.Fatal("prune must never be called with --volumes")
		}
	}
}

func TestDockerCleanFailureAndDryRun(t *testing.T) {
	docker := mkExec(t, t.TempDir(), "docker")
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: "Images\t3.2GB\t1.5GB (46%)\n"},
		{Err: errors.New("daemon down")},
	}}
	s := newDockerScanner()
	s.candidates = []string{docker}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if cr.CleanedItems != 0 || cr.FreedSpace != 0 {
		t.Errorf("CleanResult = %+v, want 0 cleaned / 0 freed on failure", cr)
	}
	if len(cr.Errors) != 1 || cr.Errors[0] != "Docker cleanup failed: daemon down" {
		t.Errorf("Errors = %v, want ['Docker cleanup failed: daemon down']", cr.Errors)
	}

	// Dry run: no commands, everything credited.
	r2 := &scriptedRunner{}
	s2 := newDockerScanner()
	cr = s2.Clean(context.Background(), []core.CleanableItem{{Path: "docker:images", Size: 7, Name: "Docker Images"}}, true, nil)
	if cr.CleanedItems != 1 || cr.FreedSpace != 7 || len(cr.Errors) != 0 {
		t.Fatalf("dry-run CleanResult = %+v, want full success", cr)
	}
	if len(r2.calls) != 0 {
		t.Fatalf("dry run must not execute commands: %v", r2.calls)
	}
}

// TestAllScannersRegistered is the final gate of the scanner tasks: homebrew
// and docker are the last two scanner files, so once their init() has run the
// package registry must be complete — all 16 categories, in display order.
func TestAllScannersRegistered(t *testing.T) {
	all := All()
	if len(all) != 16 {
		t.Fatalf("len(All()) = %d, want 16 registered scanners", len(all))
	}
	cats := core.CategoriesInOrder()
	if len(cats) != len(all) {
		t.Fatalf("core.CategoriesInOrder() returned %d categories, All() returned %d scanners", len(cats), len(all))
	}
	for i := range all {
		if got, want := all[i].Category().ID, cats[i].ID; got != want {
			t.Errorf("All()[%d].Category().ID = %q, want %q — All() must follow core.CategoriesInOrder()", i, got, want)
		}
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/scanners/ -run 'TestDocker|TestParseDockerSize|TestDefaultCandidate|TestAllScannersRegistered' -v`
Expected: `FAIL … [build failed]` with `undefined: newDockerScanner` / `undefined: parseDockerSize` / `undefined: defaultDockerCandidates` (the build failure also keeps `TestAllScannersRegistered` failing — the registry cannot be complete without `docker.go`).

- [ ] **Step 7: Write the docker implementation**

Create `internal/scanners/docker.go`:

```go
package scanners

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// Docker binary allowlist, priority order. $PATH is never consulted.
var defaultDockerCandidates = []string{
	"/usr/local/bin/docker",
	"/opt/homebrew/bin/docker",
	"/Applications/Docker.app/Contents/Resources/bin/docker",
}

// Row types accepted from `docker system df` (output allowlist / injection
// guard). "local volumes" is deliberately EXCLUDED: `docker system prune -af`
// without --volumes never frees it (spec §10.14).
var validDockerTypes = map[string]bool{
	"images":      true,
	"containers":  true,
	"build cache": true,
}

var dockerSizeRe = regexp.MustCompile(`([\d.]+)\s*([kKMGT]?B)`)

// parseDockerSize converts docker's human-readable sizes ("1.5GB",
// "346.2MB (100%)") to bytes using SI/decimal multipliers — docker prints
// decimal units (kB=1e3, MB=1e6, GB=1e9, TB=1e12); spec §10.5 fixes the
// CLI's 1024-based parsing bug. Unparseable input → 0.
func parseDockerSize(s string) int64 {
	m := dockerSizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	var mult float64
	switch strings.ToUpper(m[2]) {
	case "B":
		mult = 1
	case "KB":
		mult = 1e3
	case "MB":
		mult = 1e6
	case "GB":
		mult = 1e9
	case "TB":
		mult = 1e12
	default:
		mult = 1
	}
	return int64(value * mult)
}

type dockerScanner struct {
	candidates []string  // injectable in tests
	runner     CmdRunner // cached from the last Scan, reused by Clean
	dockerPath string    // cached from the last Scan
}

func newDockerScanner() *dockerScanner {
	return &dockerScanner{candidates: defaultDockerCandidates}
}

func init() { register(newDockerScanner()) }

func (s *dockerScanner) Category() core.Category {
	return core.Categories["docker"]
}

func (s *dockerScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	s.runner = opts.Runner
	s.dockerPath = findExecutable(s.candidates)
	if s.dockerPath == "" {
		return core.ScanResult{Category: s.Category()} // Docker not installed
	}

	out, err := opts.Runner.Run(ctx, 30*time.Second, s.dockerPath,
		"system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}")
	if err != nil {
		return core.ScanResult{Category: s.Category()} // daemon down etc. → silently empty
	}

	var items []core.CleanableItem
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(fields[0]))
		if !validDockerTypes[typ] {
			continue // drops "local volumes" and any unexpected output
		}
		size := parseDockerSize(fields[2])
		if size <= 0 {
			continue
		}
		items = append(items, core.CleanableItem{
			Path:        "docker:" + strings.ReplaceAll(typ, " ", "-"), // virtual path, e.g. docker:build-cache
			Size:        size,
			Name:        "Docker " + strings.TrimSpace(fields[0]), // original type text, e.g. "Docker Build Cache"
			IsDirectory: false,
		})
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func (s *dockerScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	if dryRun {
		return externalDryRun(s.Category(), items, progress)
	}
	if s.dockerPath == "" {
		s.dockerPath = findExecutable(s.candidates)
	}
	if s.dockerPath == "" || s.runner == nil {
		// Items have virtual docker:* paths — the filesystem cleaner must NEVER
		// receive them, so a missing binary is a hard per-category error.
		return core.CleanResult{
			Category: s.Category(),
			Errors:   []string{"Docker binary not found in safe locations"},
		}
	}

	var freed int64
	for i, it := range items {
		if progress != nil {
			progress(i+1, len(items), it)
		}
		freed += it.Size // pre-computed scan sizes
	}
	// prune -af is all-or-nothing (it cannot prune a single df row) and is
	// INTENTIONALLY invoked without --volumes to prevent data loss.
	if _, err := s.runner.Run(ctx, 60*time.Second, s.dockerPath, "system", "prune", "-af"); err != nil {
		return core.CleanResult{
			Category: s.Category(),
			Errors:   []string{"Docker cleanup failed: " + err.Error()},
		}
	}
	return core.CleanResult{Category: s.Category(), CleanedItems: len(items), FreedSpace: freed}
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/scanners/ -run 'TestHomebrew|TestFindExecutable|TestDocker|TestParseDockerSize|TestDefaultCandidate|TestAllScannersRegistered' -v`  /  Expected: PASS (all tests — `TestAllScannersRegistered` proves all 16 scanners are registered and `All()` follows `core.CategoriesInOrder()`).
Then: `go test ./internal/...`  /  Expected: all packages `ok`.

- [ ] **Step 9: Commit**

```
git add internal/scanners/homebrew.go internal/scanners/homebrew_test.go internal/scanners/docker.go internal/scanners/docker_test.go
git commit -m "feat(scanners): add homebrew and docker command-backed scanners

Both locate binaries via fixed X_OK allowlists (never PATH) and run
through the injected CmdRunner. Homebrew validates 'brew --cache' output
against allowed roots and cleans via 'brew cleanup --prune=all' when the
cache root is selected (fsx fallback otherwise). Docker parses
'system df' with SI units, excludes 'local volumes', and prunes with
'system prune -af' (never --volumes). Freed space = pre-scan sizes.
Both self-register via init(). With the registry now complete,
TestAllScannersRegistered asserts All() returns 16 scanners ordered
per core.CategoriesInOrder().

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```
### Task 15: Backup / Undo Engine (`internal/backup`)

**Files:**
- Create: `internal/backup/backup.go`
- Test: `internal/backup/backup_test.go`

**Interfaces:**
- Consumes (from the `internal/core` task): `core.CleanableItem`, `core.ProgressFunc func(current, total int, item core.CleanableItem)` (called 1-based, BEFORE each item).
- Consumes (from the `internal/fsx` task): `fsx.GetSize(path string) int64`.
- Produces (consumed by the Wails bridge task `app.go`):
  - `type Manager struct{ Root string }`
  - `func NewManager(home string) *Manager` — `Root = <home>/Library/Application Support/AppCleaner/Backups`
  - `type Info struct { Path string; Date time.Time; Size int64 }` (JSON tags `path`, `date`, `size`)
  - `type BackupOutcome struct { SessionDir string; BackedUp int; NotBackedUp []string }` (JSON tags `sessionDir`, `backedUp`, `notBackedUp`)
  - `type RestoreResult struct { Restored int; Failed int; Errors []string }` (JSON tags `restored`, `failed`, `errors`)
  - `func (m *Manager) BackupItems(ctx context.Context, home string, items []core.CleanableItem, progress core.ProgressFunc) BackupOutcome`
  - `func (m *Manager) List() []Info`
  - `func (m *Manager) Restore(sessionDir, home string) RestoreResult`
  - `func (m *Manager) CleanOld(retentionDays int) int`
  - `func (m *Manager) Delete(sessionDir string) error`
- Package-local (defined here, free to change later): `var now = time.Now` (determinism seam for session naming — tests override it), `sessionName()`, `under(path, root string) bool`, `resolveSession`, `restoreTarget(rel, home string) (string, error)`.

Behavioral notes the code below implements (spec §8 + porting-notes → utils → backup.ts):
- Session dir name = `time.Now().UTC().Format(time.RFC3339)` passed through `strings.NewReplacer(":", "-", ".", "-")` (filename-safe, e.g. `2026-07-07T18-45-45Z`).
- Backup = `os.Rename` (move) into `<session>/HOME/<path relative to home>` (literal `HOME` segment, CLI-compatible layout). Items not under `home`, and any rename failure (including `EXDEV` cross-volume), go to `NotBackedUp` — the caller permanently deletes those itself.
- Progress is invoked BEFORE each item, 1-based. On context cancellation the loop stops; unprocessed items are neither moved nor listed in `NotBackedUp` (so the caller does not permanently delete items that were never reached).
- `Restore` validates the session dir resolves strictly under `Root`; every restored file's relative path must start with `HOME/`, must not contain `..`, and its target must resolve under `home`; violations are per-file `Errors` + `Failed++`, the walk continues.
- `CleanOld(retentionDays)` removes session dirs whose mtime is older than N days (spec change #10: honors config `backupRetentionDays` instead of the CLI's hardcoded 7 — the caller passes the config value).
- `Delete` applies the same containment validation, then `os.RemoveAll`.

- [ ] **Step 1: Write the failing test**

Create `internal/backup/backup_test.go`:

```go
package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// withFixedNow pins the package's now() seam for deterministic session names.
func withFixedNow(t *testing.T, fixed time.Time) {
	t.Helper()
	orig := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = orig })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewManagerRoot(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	want := filepath.Join(home, "Library", "Application Support", "AppCleaner", "Backups")
	if m.Root != want {
		t.Fatalf("Root = %q, want %q", m.Root, want)
	}
}

func TestSessionNameIsFilenameSafe(t *testing.T) {
	withFixedNow(t, time.Date(2026, 7, 7, 18, 45, 45, 0, time.UTC))
	home := t.TempDir()
	m := NewManager(home)
	src := filepath.Join(home, "Library", "Caches", "a.txt")
	writeFile(t, src, "x")
	out := m.BackupItems(context.Background(), home,
		[]core.CleanableItem{{Path: src, Size: 1, Name: "a.txt"}}, nil)
	want := filepath.Join(m.Root, "2026-07-07T18-45-45Z")
	if out.SessionDir != want {
		t.Fatalf("SessionDir = %q, want %q (':' and '.' must become '-')", out.SessionDir, want)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	src := filepath.Join(home, "Library", "Caches", "com.example", "blob.bin")
	writeFile(t, src, "payload-123")
	item := core.CleanableItem{Path: src, Size: 11, Name: "blob.bin"}

	var progressCalls []int
	out := m.BackupItems(context.Background(), home, []core.CleanableItem{item},
		func(current, total int, it core.CleanableItem) {
			progressCalls = append(progressCalls, current)
		})

	if out.BackedUp != 1 || len(out.NotBackedUp) != 0 {
		t.Fatalf("outcome = %+v", out)
	}
	if len(progressCalls) != 1 || progressCalls[0] != 1 {
		t.Fatalf("progress calls = %v, want [1] (1-based, before each item)", progressCalls)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Fatal("source must be MOVED away by backup")
	}
	moved := filepath.Join(out.SessionDir, "HOME", "Library", "Caches", "com.example", "blob.bin")
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("backup layout wrong, want %s: %v", moved, err)
	}

	res := m.Restore(out.SessionDir, home)
	if res.Restored != 1 || res.Failed != 0 || len(res.Errors) != 0 {
		t.Fatalf("restore = %+v", res)
	}
	got, err := os.ReadFile(src)
	if err != nil || string(got) != "payload-123" {
		t.Fatalf("restored content = %q, err = %v", got, err)
	}
}

func TestNonHomeItemNotBackedUp(t *testing.T) {
	home := t.TempDir()
	other := t.TempDir()
	m := NewManager(home)
	outside := filepath.Join(other, "app.lproj")
	writeFile(t, outside, "keep")

	var progressCalls []int
	out := m.BackupItems(context.Background(), home,
		[]core.CleanableItem{{Path: outside, Name: "app.lproj"}},
		func(current, total int, it core.CleanableItem) {
			progressCalls = append(progressCalls, current)
		})

	if out.BackedUp != 0 || len(out.NotBackedUp) != 1 || out.NotBackedUp[0] != outside {
		t.Fatalf("outcome = %+v", out)
	}
	if len(progressCalls) != 1 {
		t.Fatalf("progress must fire even for skipped items, got %v", progressCalls)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("non-home item must be left in place for the caller to delete: %v", err)
	}
}

func TestRestoreRejectsSessionOutsideRoot(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	evil := filepath.Join(m.Root, "..", "evil")
	res := m.Restore(evil, home)
	if res.Restored != 0 || res.Failed != 1 || len(res.Errors) != 1 {
		t.Fatalf("restore = %+v", res)
	}
	if !strings.Contains(res.Errors[0], "Invalid backup directory") {
		t.Fatalf("error = %q", res.Errors[0])
	}
}

func TestRestoreRejectsNonHomeEntries(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	session := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	writeFile(t, filepath.Join(session, "etc", "passwd"), "evil")
	res := m.Restore(session, home)
	if res.Restored != 0 || res.Failed != 1 || len(res.Errors) != 1 {
		t.Fatalf("restore = %+v", res)
	}
	if !strings.Contains(res.Errors[0], "outside HOME structure") {
		t.Fatalf("error = %q", res.Errors[0])
	}
	if _, err := os.Stat(filepath.Join(home, "etc", "passwd")); !os.IsNotExist(err) {
		t.Fatal("outside-HOME entry must not be restored")
	}
}

func TestRestoreTargetRejectsTraversal(t *testing.T) {
	home := t.TempDir()
	// Defense-in-depth check on the unexported mapper: filepath.Rel can never
	// emit ".." for a walked entry, but the rule must still hold.
	if _, err := restoreTarget("HOME/../../etc/passwd", home); err == nil {
		t.Fatal("'..' in a session-relative path must be rejected")
	} else if !strings.Contains(err.Error(), "Suspicious path pattern detected") {
		t.Fatalf("error = %v", err)
	}
	if _, err := restoreTarget("NOTHOME/x", home); err == nil {
		t.Fatal("non-HOME prefix must be rejected")
	}
	got, err := restoreTarget("HOME/Library/Caches/x", home)
	if err != nil || got != filepath.Join(home, "Library", "Caches", "x") {
		t.Fatalf("target = %q, err = %v", got, err)
	}
}

func TestCleanOldRemovesOnlyOldSessions(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	oldDir := filepath.Join(m.Root, "2026-06-01T00-00-00Z")
	newDir := filepath.Join(m.Root, "2026-07-07T00-00-00Z")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	if removed := m.CleanOld(7); removed != 1 {
		t.Fatalf("CleanOld = %d, want 1", removed)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("old session must be removed")
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatal("recent session must be kept")
	}
}

func TestListNewestFirstWithRecursiveSize(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	older := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	newer := filepath.Join(m.Root, "2026-02-01T00-00-00Z")
	writeFile(t, filepath.Join(older, "HOME", "f.txt"), "12345") // 5 bytes
	if err := os.MkdirAll(newer, 0o755); err != nil {
		t.Fatal(err)
	}
	tOld := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(older, tOld, tOld); err != nil {
		t.Fatal(err)
	}

	infos := m.List()
	if len(infos) != 2 {
		t.Fatalf("len = %d, want 2", len(infos))
	}
	if infos[0].Path != newer || infos[1].Path != older {
		t.Fatalf("order = [%s, %s], want newest first", infos[0].Path, infos[1].Path)
	}
	if infos[1].Size != 5 {
		t.Fatalf("recursive size = %d, want 5", infos[1].Size)
	}
}

func TestDeleteValidatesContainment(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	if err := m.Delete("/tmp/not-a-backup-session"); err == nil {
		t.Fatal("session dir outside Root must be refused")
	}
	if err := m.Delete(m.Root); err == nil {
		t.Fatal("the backups root itself must be refused")
	}
	session := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("session not removed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backup/ -v`
Expected: FAIL — build errors (`undefined: now`, `undefined: NewManager`, `undefined: restoreTarget`); no implementation exists yet.

- [ ] **Step 3: Write minimal implementation**

Create `internal/backup/backup.go`:

```go
// Package backup implements the move-based undo mechanism: before a
// non-dry-run clean, items are os.Rename'd into a timestamped session
// directory and can later be restored, listed, expired, or deleted.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// now is the determinism seam for session naming; tests override it.
var now = time.Now

var sessionNameReplacer = strings.NewReplacer(":", "-", ".", "-")

// Manager owns the backups root:
// <home>/Library/Application Support/AppCleaner/Backups.
type Manager struct{ Root string }

func NewManager(home string) *Manager {
	return &Manager{Root: filepath.Join(home, "Library", "Application Support", "AppCleaner", "Backups")}
}

// Info describes one backup session directory.
type Info struct {
	Path string    `json:"path"`
	Date time.Time `json:"date"`
	Size int64     `json:"size"`
}

// BackupOutcome reports what BackupItems did. Paths in NotBackedUp were NOT
// moved (non-$HOME item, cross-volume EXDEV, or any rename error) — the
// caller permanently deletes those itself.
type BackupOutcome struct {
	SessionDir  string   `json:"sessionDir"`
	BackedUp    int      `json:"backedUp"`
	NotBackedUp []string `json:"notBackedUp"`
}

// RestoreResult reports a Restore run.
type RestoreResult struct {
	Restored int      `json:"restored"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors"`
}

func sessionName() string {
	return sessionNameReplacer.Replace(now().UTC().Format(time.RFC3339))
}

// under reports whether path is strictly inside root (both cleaned/absolute).
func under(path, root string) bool {
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// BackupItems moves each item into a new session directory laid out as
// <session>/HOME/<path relative to home>. Progress fires BEFORE each item,
// 1-based. On context cancellation the loop stops; remaining items are
// neither moved nor added to NotBackedUp.
func (m *Manager) BackupItems(ctx context.Context, home string, items []core.CleanableItem, progress core.ProgressFunc) BackupOutcome {
	out := BackupOutcome{}
	if len(items) == 0 {
		return out
	}
	home = filepath.Clean(home)
	sessionDir := filepath.Join(m.Root, sessionName())
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		for _, it := range items {
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
		}
		return out
	}
	out.SessionDir = sessionDir
	total := len(items)
	for i, it := range items {
		if progress != nil {
			progress(i+1, total, it)
		}
		if ctx.Err() != nil {
			break
		}
		p := filepath.Clean(it.Path)
		if !under(p, home) {
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
			continue
		}
		rel := strings.TrimPrefix(p, home+string(filepath.Separator))
		dest := filepath.Join(sessionDir, "HOME", rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
			continue
		}
		if err := os.Rename(p, dest); err != nil {
			// EXDEV (cross-volume) or any other failure: caller deletes permanently.
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
			continue
		}
		out.BackedUp++
	}
	return out
}

// List returns session directories under Root, newest first by mtime, each
// with its recursive size. Missing/unreadable Root yields nil.
func (m *Manager) List() []Info {
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return nil
	}
	var infos []Info
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(m.Root, e.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		infos = append(infos, Info{Path: p, Date: st.ModTime(), Size: fsx.GetSize(p)})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Date.After(infos[j].Date) })
	return infos
}

// resolveSession validates that sessionDir resolves strictly under Root.
func (m *Manager) resolveSession(sessionDir string) (string, error) {
	root, err := filepath.Abs(m.Root)
	if err != nil {
		return "", err
	}
	sd, err := filepath.Abs(sessionDir)
	if err != nil {
		return "", err
	}
	if !under(sd, root) {
		return "", errors.New("Invalid backup directory: must be within the App Cleaner backups folder")
	}
	return sd, nil
}

// restoreTarget maps a session-relative entry path to its restore
// destination, enforcing the HOME/ prefix, the no-".." rule, and home
// containment.
func restoreTarget(rel, home string) (string, error) {
	if !strings.HasPrefix(rel, "HOME"+string(filepath.Separator)) {
		return "", fmt.Errorf("Skipping file outside HOME structure: %s", rel)
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("Suspicious path pattern detected: %s", rel)
	}
	target := filepath.Join(home, rel[len("HOME/"):])
	abs, err := filepath.Abs(target)
	if err != nil || !under(abs, home) {
		return "", fmt.Errorf("Path traversal detected: %s resolves outside home directory", target)
	}
	return target, nil
}

// Restore moves every file in the session back under home. Directories are
// only traversed (empty dirs are not restored). Per-file failures are
// recorded and the walk continues.
func (m *Manager) Restore(sessionDir, home string) RestoreResult {
	res := RestoreResult{}
	sd, err := m.resolveSession(sessionDir)
	if err != nil {
		res.Failed = 1
		res.Errors = []string{err.Error()}
		return res
	}
	home = filepath.Clean(home)
	_ = filepath.WalkDir(sd, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to read %s: %v", path, werr))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(sd, path)
		if rerr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to resolve %s: %v", path, rerr))
			return nil
		}
		target, terr := restoreTarget(rel, home)
		if terr != nil {
			res.Failed++
			res.Errors = append(res.Errors, terr.Error())
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to restore %s: %v", d.Name(), err))
			return nil
		}
		if err := os.Rename(path, target); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to restore %s: %v", d.Name(), err))
			return nil
		}
		res.Restored++
		return nil
	})
	return res
}

// CleanOld removes session directories whose mtime is older than
// retentionDays days and returns how many were removed.
func (m *Manager) CleanOld(retentionDays int) int {
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return 0
	}
	cutoff := now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(m.Root, e.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.ModTime().Before(cutoff) {
			if err := os.RemoveAll(p); err == nil {
				removed++
			}
		}
	}
	return removed
}

// Delete removes one session directory after containment validation.
func (m *Manager) Delete(sessionDir string) error {
	sd, err := m.resolveSession(sessionDir)
	if err != nil {
		return err
	}
	return os.RemoveAll(sd)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/backup/ -v`
Expected: PASS — all 10 tests (`TestNewManagerRoot` … `TestDeleteValidatesContainment`), ending `ok  github.com/guhcostan/app-cleaner/internal/backup`.

- [ ] **Step 5: Commit**

```
git add internal/backup/backup.go internal/backup/backup_test.go
git commit -m "feat(backup): move-based undo sessions with restore, retention, and containment validation

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 16: App Uninstaller (`internal/uninstall`)

**Files:**
- Create: `internal/uninstall/related.go`, `internal/uninstall/uninstall.go`, `internal/uninstall/appicon.go`
- Modify: `go.mod`, `go.sum` (via `go get howett.net/plist@v1.0.1`)
- Test: `internal/uninstall/related_test.go`, `internal/uninstall/uninstall_test.go`, `internal/uninstall/appicon_test.go`

**Interfaces:**
- Consumes (from the `internal/core` task): `core.CleanableItem`.
- Consumes (from the `internal/fsx` task): `fsx.GetSize(path string) int64`, `fsx.IsProtectedPath(path string) bool`, `fsx.RemoveItems(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) fsx.RemoveOutcome`.
- Consumes (new module dependency): `howett.net/plist` v1.0.1 (XML **and** binary plist parsing — spec change #8).
- Produces (consumed by the Wails bridge task `app.go`):
  - `type RelatedPath struct { Path string; Size int64 }` (JSON tags `path`, `size`)
  - `type AppInfo struct { Name string; Path string; BundleID string; AppSize int64; RelatedPaths []RelatedPath; TotalSize int64; Running bool }` (JSON tags `name`, `path`, `bundleId`, `appSize`, `relatedPaths`, `totalSize`, `running`)
  - `type Summary struct { Uninstalled int; FreedSpace int64; Errors []string }` (JSON tags `uninstalled`, `freedSpace`, `errors`)
  - `func ListApps(ctx context.Context, appDirs []string, home string) []AppInfo` — sorted `TotalSize` desc
  - `func BundleID(appPath string) string`
  - `func FindRelatedPaths(appName, bundleID, home string) []RelatedPath`
  - `func IsAppRunning(appPath string) bool`
  - `func Uninstall(ctx context.Context, apps []AppInfo, dryRun bool, progress func(current, total int, appName string)) Summary`
  - `func AppIcon(ctx context.Context, r Runner, appPath, cacheDir string) string` — base64-encoded PNG of the app's icon, `""` on ANY failure; consumed by the bridge's lazy `GetAppIcon(path)`. `AppInfo` is UNCHANGED (no icon field — icons are fetched lazily via the bridge, never during `ListApps`).
- Package-local (defined here; a verifier should know about these):
  - `type Runner interface { Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) }` — same shape as `maintenance.Runner`, defined locally so `uninstall` never imports `maintenance` (no engine-package coupling/cycles).
  - `var runner Runner = execRunner{}` — seam for the `pgrep` call; tests swap it. `execRunner` is the real `exec.CommandContext` impl. This preserves the contract signature `IsAppRunning(appPath string) bool` (no injectable parameter) while keeping the binary behind an interface.
  - Helpers: `includable(path, home string) bool` (home containment + `!fsx.IsProtectedPath`; existence checked separately), `globToRegexp(pattern string) (*regexp.Regexp, error)`, `relatedTemplates` (the 11 templates), `isRunning(ctx, appPath)`, `removeOne(ctx, path, size)`.

Behavioral notes (spec §6 + porting-notes → commands → uninstall.ts):
- Discovery: for each existing `appDir`, readdir top-level entries ending `.app`; `os.Stat` must be a directory; `Name = strings.TrimSuffix(entry, ".app")`; `AppSize = fsx.GetSize(appPath)`; per-dir/per-entry errors are swallowed (skip and continue).
- `BundleID`: parse `Contents/Info.plist` with `howett.net/plist` (handles XML + binary); `CFBundleIdentifier` (trimmed) must match `^[a-zA-Z][a-zA-Z0-9.-]*$`, else fallback = `strings.ToLower(name)` with whitespace runs → `"."`.
- `FindRelatedPaths`: exactly 11 templates × 3 `{APP}` variations (verbatim / lowercased / whitespace-stripped); `{BID}` is always the resolved bundle id. Glob templates (`*` in the basename): readdir the parent dir and match entries with a case-insensitive anchored regexp built by escaping ALL regexp metachars then turning the escaped `\*` into `.*`. Every candidate must resolve under `home`, pass `!fsx.IsProtectedPath`, and exist (`os.Lstat`). Deduplicated **case-insensitively** (macOS default volumes are case-insensitive, so e.g. the verbatim and lowercased variations of an existing dir would otherwise both be listed and double-counted). Sizes computed here — AT SCAN TIME (spec §10.9) — via `fsx.GetSize`.
- Running check: `/usr/bin/pgrep -f <appPath>` through the injected `runner`; running ⇔ pgrep exits 0 AND prints a non-blank pid list.
- `Uninstall`: sequential; `progress(i+1, total, appName)` BEFORE each app; per app the bundle is removed via `fsx.RemoveItems` with a single-item slice (this is the fsx-style safeRemove: `ValidatePathSafety` + TOCTOU re-`Lstat` + symlink-aware remove). Bundle failure → error `"<name>: Failed to remove (security check failed or permission denied)"`, its related paths are SKIPPED, the loop continues. On success: `freed += AppSize` first, then each related path is removed and `freed += <pre-recorded size>` on success (spec change #9: sizes are never re-measured at delete time). Dry-run: every app counts as uninstalled and credits `TotalSize`, no disk IO.
- `AppIcon`: icon name = `CFBundleIconFile` from `Contents/Info.plist` (default `"AppIcon"` when the plist or key is missing; `".icns"` appended when the name has no extension); the source must exist at `Contents/Resources/<name>.icns`. Conversion = `r.Run(ctx, 10*time.Second, "/usr/bin/sips", "-s", "format", "png", icnsPath, "--out", pngPath)` with `pngPath = <cacheDir>/<sha1 hex of bundle path>.png`; when the cached PNG already exists the sips call is skipped entirely. Returns the PNG bytes base64-encoded; ANY failure (missing `.icns`, sips error, unreadable PNG, cacheDir not creatable) returns `""` — icons are cosmetic chrome fetched lazily by the bridge, so errors never propagate.

This task runs THREE test→impl cycles, then one commit.

- [ ] **Step 1 (cycle A): Add the plist dependency, then write the failing test for BundleID + FindRelatedPaths**

Run first: `go get howett.net/plist@v1.0.1`
Expected: `go.mod` gains `require howett.net/plist v1.0.1`.

Create `internal/uninstall/related_test.go`:

```go
package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func write(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func xmlInfoPlist(bundleID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
</dict>
</plist>
`, bundleID)
}

func makeApp(t *testing.T, dir, name string, infoPlist []byte) string {
	t.Helper()
	appPath := filepath.Join(dir, name+".app")
	mkdir(t, appPath, "Contents")
	if infoPlist != nil {
		write(t, filepath.Join(appPath, "Contents", "Info.plist"), string(infoPlist))
	}
	return appPath
}

func TestBundleIDFromXMLPlist(t *testing.T) {
	dir := t.TempDir()
	app := makeApp(t, dir, "XmlApp", []byte(xmlInfoPlist("com.example.xmlapp")))
	if got := BundleID(app); got != "com.example.xmlapp" {
		t.Fatalf("BundleID = %q, want com.example.xmlapp", got)
	}
}

func TestBundleIDFromBinaryPlist(t *testing.T) {
	dir := t.TempDir()
	data, err := plist.Marshal(map[string]string{"CFBundleIdentifier": "com.example.binapp"}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	app := makeApp(t, dir, "BinApp", data)
	if got := BundleID(app); got != "com.example.binapp" {
		t.Fatalf("BundleID = %q, want com.example.binapp", got)
	}
}

func TestBundleIDFallback(t *testing.T) {
	dir := t.TempDir()
	// Identifier failing ^[a-zA-Z][a-zA-Z0-9.-]*$ (leading digit + space) → fallback.
	app := makeApp(t, dir, "My Cool App", []byte(xmlInfoPlist("9bad id")))
	if got := BundleID(app); got != "my.cool.app" {
		t.Fatalf("BundleID = %q, want my.cool.app", got)
	}
	// Missing Info.plist → fallback too.
	app2 := makeApp(t, dir, "No Plist", nil)
	if got := BundleID(app2); got != "no.plist" {
		t.Fatalf("BundleID = %q, want no.plist", got)
	}
}

func TestFindRelatedPathsTemplatesVariationsAndGlob(t *testing.T) {
	home := t.TempDir()
	// verbatim {APP}
	appSupport := mkdir(t, home, "Library", "Application Support", "My App")
	write(t, filepath.Join(appSupport, "data.bin"), "1234") // 4 bytes → size assertion
	// whitespace-stripped {APP}
	mkdir(t, home, "Library", "Caches", "MyApp")
	// lowercased {APP}
	mkdir(t, home, "Library", "Logs", "my app")
	// {BID} template
	write(t, filepath.Join(home, "Library", "Preferences", "com.example.myapp.plist"), "x")
	// glob template Group Containers/*.{APP} (whitespace-stripped variation)
	mkdir(t, home, "Library", "Group Containers", "ABC123.MyApp")
	// noise that must NOT match
	mkdir(t, home, "Library", "Group Containers", "ABC123.OtherApp")

	got := FindRelatedPaths("My App", "com.example.myapp", home)

	// Compare case-insensitively: on macOS's case-insensitive default volumes
	// a differently-cased variation resolves to the same directory.
	byLower := map[string]RelatedPath{}
	for _, rp := range got {
		byLower[strings.ToLower(rp.Path)] = rp
	}
	want := []string{
		strings.ToLower(filepath.Join(home, "Library", "Application Support", "My App")),
		strings.ToLower(filepath.Join(home, "Library", "Caches", "MyApp")),
		strings.ToLower(filepath.Join(home, "Library", "Logs", "my app")),
		strings.ToLower(filepath.Join(home, "Library", "Preferences", "com.example.myapp.plist")),
		strings.ToLower(filepath.Join(home, "Library", "Group Containers", "ABC123.MyApp")),
	}
	for _, w := range want {
		if _, ok := byLower[w]; !ok {
			t.Errorf("missing related path %q (got %+v)", w, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %+v", len(got), len(want), got)
	}
	if rp := byLower[want[0]]; rp.Size != 4 {
		t.Fatalf("Application Support size = %d, want 4 (sized at scan time)", rp.Size)
	}
	noise := strings.ToLower(filepath.Join(home, "Library", "Group Containers", "ABC123.OtherApp"))
	if _, ok := byLower[noise]; ok {
		t.Fatal("glob must not match ABC123.OtherApp")
	}
}

func TestIncludableRejectsProtectedAndOutsideHome(t *testing.T) {
	home := t.TempDir()
	if !includable(filepath.Join(home, "Library", "Caches", "X"), home) {
		t.Fatal("in-home unprotected candidate must be includable")
	}
	// Real protected paths can't be created in a temp fixture, so the
	// predicate is exercised directly: /System/… is under home=/System but
	// fsx.IsProtectedPath must veto it.
	if includable("/System/Library/Caches/Foo", "/System") {
		t.Fatal("protected candidate must be rejected even when under home")
	}
	if includable(filepath.Join(home, "..", "escape"), home) {
		t.Fatal("candidate escaping home must be rejected")
	}
}

func TestGlobToRegexpEscapesMetachars(t *testing.T) {
	re, err := globToRegexp("*.My+App(1)")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("group.My+App(1)") {
		t.Fatal("literal metachars must match themselves")
	}
	if re.MatchString("group.MyXApp(1)") {
		t.Fatal("+ must not act as a regex quantifier")
	}
	if !re.MatchString("GROUP.my+app(1)") {
		t.Fatal("match must be case-insensitive")
	}
}
```

- [ ] **Step 2 (cycle A): Run test to verify it fails**

Run: `go test ./internal/uninstall/ -v`
Expected: FAIL — build errors (`undefined: BundleID`, `undefined: FindRelatedPaths`, `undefined: RelatedPath`, `undefined: includable`, `undefined: globToRegexp`).

- [ ] **Step 3 (cycle A): Write minimal implementation**

Create `internal/uninstall/related.go`:

```go
// Package uninstall implements app discovery, bundle-id resolution,
// related-path (leftover) search, the running-app check, and safe removal
// of app bundles plus their leftovers.
package uninstall

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"howett.net/plist"

	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// RelatedPath is a leftover file/dir associated with an app bundle.
// Size is recorded at scan time and reused for freed-space accounting.
type RelatedPath struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

var (
	bundleIDRe   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9.-]*$`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// BundleID parses <app>/Contents/Info.plist (XML or binary) and returns
// CFBundleIdentifier when it matches ^[a-zA-Z][a-zA-Z0-9.-]*$; otherwise it
// falls back to the lowercased app name with whitespace runs replaced by ".".
func BundleID(appPath string) string {
	name := strings.TrimSuffix(filepath.Base(appPath), ".app")
	fallback := whitespaceRe.ReplaceAllString(strings.ToLower(name), ".")
	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist"))
	if err != nil {
		return fallback
	}
	var info struct {
		CFBundleIdentifier string `plist:"CFBundleIdentifier"`
	}
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return fallback
	}
	id := strings.TrimSpace(info.CFBundleIdentifier)
	if bundleIDRe.MatchString(id) {
		return id
	}
	return fallback
}

// relatedTemplates are the CLI's 11 templates, relative to $HOME.
// {APP} expands with 3 variations; {BID} is always the resolved bundle id.
var relatedTemplates = []string{
	"Library/Application Support/{APP}",
	"Library/Preferences/{BID}.plist",
	"Library/Preferences/{APP}.plist",
	"Library/Caches/{APP}",
	"Library/Caches/{BID}",
	"Library/Logs/{APP}",
	"Library/Saved Application State/{BID}.savedState",
	"Library/WebKit/{APP}",
	"Library/HTTPStorages/{BID}",
	"Library/Containers/{BID}",
	"Library/Group Containers/*.{APP}",
}

// includable reports whether a candidate may be offered for deletion: it
// must resolve strictly under home and must not be a protected path.
// Existence is checked separately by the caller.
func includable(path, home string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	home = filepath.Clean(home)
	if !strings.HasPrefix(abs, home+string(filepath.Separator)) {
		return false
	}
	return !fsx.IsProtectedPath(abs)
}

// globToRegexp converts a template basename containing '*' into a
// case-insensitive anchored regexp: every regexp metachar is escaped, then
// the escaped \* becomes ".*" (exactly the CLI's matchPattern).
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	quoted := regexp.QuoteMeta(pattern)
	return regexp.Compile(`(?i)^` + strings.ReplaceAll(quoted, `\*`, `.*`) + `$`)
}

// FindRelatedPaths expands the 11 templates × 3 app-name variations
// (verbatim, lowercased, whitespace-stripped), resolves globs by readdir of
// the template's parent dir, and returns existing, home-contained,
// non-protected candidates. Candidates are deduplicated case-insensitively
// because macOS default volumes are case-insensitive (two casings of one
// path are the same file — listing both would double-count freed space).
// Sizes are computed here, at scan time, via fsx.GetSize.
func FindRelatedPaths(appName, bundleID, home string) []RelatedPath {
	home = filepath.Clean(home)
	variations := []string{
		appName,
		strings.ToLower(appName),
		whitespaceRe.ReplaceAllString(appName, ""),
	}
	seen := map[string]bool{}
	var out []RelatedPath
	add := func(candidate string) {
		abs, err := filepath.Abs(candidate)
		if err != nil || !includable(abs, home) {
			return
		}
		key := strings.ToLower(abs)
		if seen[key] {
			return
		}
		if _, err := os.Lstat(abs); err != nil {
			return
		}
		seen[key] = true
		out = append(out, RelatedPath{Path: abs, Size: fsx.GetSize(abs)})
	}
	for _, tpl := range relatedTemplates {
		for _, v := range variations {
			rel := strings.ReplaceAll(tpl, "{APP}", v)
			rel = strings.ReplaceAll(rel, "{BID}", bundleID)
			candidate := filepath.Join(home, rel)
			if !strings.Contains(candidate, "*") {
				add(candidate)
				continue
			}
			dir := filepath.Dir(candidate)
			re, err := globToRegexp(filepath.Base(candidate))
			if err != nil {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if re.MatchString(e.Name()) {
					add(filepath.Join(dir, e.Name()))
				}
			}
		}
	}
	return out
}
```

- [ ] **Step 4 (cycle A): Run test to verify it passes**

Run: `go test ./internal/uninstall/ -v`
Expected: PASS — `TestBundleIDFromXMLPlist`, `TestBundleIDFromBinaryPlist`, `TestBundleIDFallback`, `TestFindRelatedPathsTemplatesVariationsAndGlob`, `TestIncludableRejectsProtectedAndOutsideHome`, `TestGlobToRegexpEscapesMetachars`.

- [ ] **Step 5 (cycle B): Write the failing test for ListApps / IsAppRunning / Uninstall**

Create `internal/uninstall/uninstall_test.go` (same package — it reuses the `mkdir`/`write`/`makeApp`/`xmlInfoPlist` helpers from `related_test.go`):

```go
package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	out   string
	err   error
	calls [][]string
}

func (f *fakeRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{bin}, args...))
	return f.out, f.err
}

// swapRunner replaces the package-level pgrep seam for one test.
func swapRunner(t *testing.T, r Runner) {
	t.Helper()
	orig := runner
	runner = r
	t.Cleanup(func() { runner = orig })
}

func TestIsAppRunning(t *testing.T) {
	f := &fakeRunner{out: "123\n"}
	swapRunner(t, f)
	if !IsAppRunning("/Applications/Foo.app") {
		t.Fatal("want running=true when pgrep prints a pid")
	}
	want := "/usr/bin/pgrep -f /Applications/Foo.app"
	if len(f.calls) != 1 || strings.Join(f.calls[0], " ") != want {
		t.Fatalf("pgrep call = %v, want %q", f.calls, want)
	}

	swapRunner(t, &fakeRunner{err: errors.New("exit status 1")})
	if IsAppRunning("/Applications/Foo.app") {
		t.Fatal("want running=false when pgrep exits non-zero")
	}

	swapRunner(t, &fakeRunner{out: "  \n"})
	if IsAppRunning("/Applications/Foo.app") {
		t.Fatal("want running=false on blank pgrep output")
	}
}

func TestListAppsDiscoversAndSorts(t *testing.T) {
	swapRunner(t, &fakeRunner{err: errors.New("exit status 1")}) // nothing running
	appsDir := t.TempDir()
	home := t.TempDir()

	big := makeApp(t, appsDir, "Big", []byte(xmlInfoPlist("com.example.big")))
	write(t, filepath.Join(big, "Contents", "payload.bin"), strings.Repeat("x", 100))
	small := makeApp(t, appsDir, "Small", []byte(xmlInfoPlist("com.example.small")))
	write(t, filepath.Join(small, "Contents", "payload.bin"), "x")
	write(t, filepath.Join(appsDir, "NotABundle.app"), "plain file") // not a dir → skipped
	mkdir(t, appsDir, "NotAnApp")                                    // no .app suffix → skipped

	apps := ListApps(context.Background(), []string{appsDir, filepath.Join(appsDir, "missing")}, home)
	if len(apps) != 2 {
		t.Fatalf("len(apps) = %d, want 2: %+v", len(apps), apps)
	}
	if apps[0].Name != "Big" || apps[1].Name != "Small" {
		t.Fatalf("order = %s,%s want Big,Small (TotalSize desc)", apps[0].Name, apps[1].Name)
	}
	if apps[0].BundleID != "com.example.big" {
		t.Fatalf("BundleID = %q", apps[0].BundleID)
	}
	if apps[0].Running {
		t.Fatal("Running must be false when pgrep matches nothing")
	}
	if apps[0].AppSize <= apps[1].AppSize {
		t.Fatalf("AppSize ordering wrong: %d <= %d", apps[0].AppSize, apps[1].AppSize)
	}
	if apps[0].Path != big {
		t.Fatalf("Path = %q, want %q", apps[0].Path, big)
	}
}

func TestUninstallFreedSpaceUsesPreRecordedSizes(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "MyApp.app")
	write(t, filepath.Join(bundle, "Contents", "bin"), "1234567890") // 10 real bytes
	related := filepath.Join(base, "Library", "Caches", "MyApp")
	write(t, filepath.Join(related, "c.dat"), "abc") // 3 real bytes

	app := AppInfo{
		Name:         "MyApp",
		Path:         bundle,
		AppSize:      1000, // deliberately != real size: recorded at scan time
		RelatedPaths: []RelatedPath{{Path: related, Size: 5000}},
		TotalSize:    6000,
	}
	var prog []string
	s := Uninstall(context.Background(), []AppInfo{app}, false, func(cur, tot int, name string) {
		prog = append(prog, fmt.Sprintf("%d/%d %s", cur, tot, name))
	})
	if len(s.Errors) != 0 || s.Uninstalled != 1 {
		t.Fatalf("summary = %+v", s)
	}
	if s.FreedSpace != 6000 {
		t.Fatalf("FreedSpace = %d, want 6000 (pre-recorded sizes, never re-measured)", s.FreedSpace)
	}
	if _, err := os.Lstat(bundle); !os.IsNotExist(err) {
		t.Fatal("bundle not removed")
	}
	if _, err := os.Lstat(related); !os.IsNotExist(err) {
		t.Fatal("related path not removed")
	}
	if len(prog) != 1 || prog[0] != "1/1 MyApp" {
		t.Fatalf("progress = %v, want [1/1 MyApp]", prog)
	}
}

func TestUninstallBundleFailureSkipsRelated(t *testing.T) {
	base := t.TempDir()
	related := filepath.Join(base, "Library", "Caches", "GhostApp")
	write(t, filepath.Join(related, "c.dat"), "abc")
	app := AppInfo{
		Name:         "GhostApp",
		Path:         filepath.Join(base, "GhostApp.app"), // never created → ENOENT failure
		AppSize:      100,
		RelatedPaths: []RelatedPath{{Path: related, Size: 3}},
		TotalSize:    103,
	}
	s := Uninstall(context.Background(), []AppInfo{app}, false, nil)
	if s.Uninstalled != 0 || s.FreedSpace != 0 {
		t.Fatalf("summary = %+v", s)
	}
	wantErr := "GhostApp: Failed to remove (security check failed or permission denied)"
	if len(s.Errors) != 1 || s.Errors[0] != wantErr {
		t.Fatalf("Errors = %v, want [%q]", s.Errors, wantErr)
	}
	if _, err := os.Stat(related); err != nil {
		t.Fatal("related paths must be left untouched when the bundle removal fails")
	}
}

func TestUninstallDryRun(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "DryApp.app")
	write(t, filepath.Join(bundle, "Contents", "bin"), "x")
	app := AppInfo{Name: "DryApp", Path: bundle, AppSize: 10, TotalSize: 10}
	s := Uninstall(context.Background(), []AppInfo{app}, true, nil)
	if s.Uninstalled != 1 || s.FreedSpace != 10 || len(s.Errors) != 0 {
		t.Fatalf("summary = %+v", s)
	}
	if _, err := os.Stat(bundle); err != nil {
		t.Fatal("dry run must not touch disk")
	}
}
```

- [ ] **Step 6 (cycle B): Run test to verify it fails**

Run: `go test ./internal/uninstall/ -v`
Expected: FAIL — build errors (`undefined: Runner`, `undefined: runner`, `undefined: IsAppRunning`, `undefined: ListApps`, `undefined: AppInfo`, `undefined: Uninstall`).

- [ ] **Step 7 (cycle B): Write minimal implementation**

Create `internal/uninstall/uninstall.go`:

```go
package uninstall

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// Runner matches maintenance.Runner's shape but is defined locally so this
// package never imports maintenance (no engine-package coupling/cycles).
type Runner interface {
	Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error)
}

// runner is the seam for the pgrep call; tests replace it.
var runner Runner = execRunner{}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	return string(out), err
}

// AppInfo describes an installed app bundle and its leftovers.
type AppInfo struct {
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	BundleID     string        `json:"bundleId"`
	AppSize      int64         `json:"appSize"`
	RelatedPaths []RelatedPath `json:"relatedPaths"`
	TotalSize    int64         `json:"totalSize"`
	Running      bool          `json:"running"`
}

// Summary reports an Uninstall run.
type Summary struct {
	Uninstalled int      `json:"uninstalled"`
	FreedSpace  int64    `json:"freedSpace"`
	Errors      []string `json:"errors"`
}

// IsAppRunning reports whether any process matches the bundle path
// (pgrep -f <appPath>). pgrep exits non-zero when nothing matches.
func IsAppRunning(appPath string) bool {
	return isRunning(context.Background(), appPath)
}

func isRunning(ctx context.Context, appPath string) bool {
	out, err := runner.Run(ctx, 5*time.Second, "/usr/bin/pgrep", "-f", appPath)
	return err == nil && strings.TrimSpace(out) != ""
}

// ListApps enumerates top-level .app bundles in each existing appDir,
// resolves bundle ids, related paths, and sizes (all at scan time), and
// returns the apps sorted by TotalSize descending. Missing/unreadable dirs
// and non-directory .app entries are skipped silently, like the CLI.
func ListApps(ctx context.Context, appDirs []string, home string) []AppInfo {
	var apps []AppInfo
	for _, dir := range appDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			appPath := filepath.Join(dir, e.Name())
			st, err := os.Stat(appPath)
			if err != nil || !st.IsDir() {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".app")
			bid := BundleID(appPath)
			related := FindRelatedPaths(name, bid, home)
			appSize := fsx.GetSize(appPath)
			total := appSize
			for _, rp := range related {
				total += rp.Size
			}
			apps = append(apps, AppInfo{
				Name:         name,
				Path:         appPath,
				BundleID:     bid,
				AppSize:      appSize,
				RelatedPaths: related,
				TotalSize:    total,
				Running:      isRunning(ctx, appPath),
			})
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].TotalSize > apps[j].TotalSize })
	return apps
}

// removeOne deletes a single path through fsx.RemoveItems, which performs
// ValidatePathSafety plus the symlink-aware TOCTOU re-Lstat remove — the
// fsx-style safeRemove mandated by the spec.
func removeOne(ctx context.Context, path string, size int64) bool {
	item := core.CleanableItem{Path: path, Size: size, Name: filepath.Base(path)}
	out := fsx.RemoveItems(ctx, []core.CleanableItem{item}, false, nil)
	return len(out.Failures) == 0
}

// Uninstall removes each app bundle and then its related paths,
// sequentially. progress fires BEFORE each app, 1-based. Freed space uses
// the sizes recorded at scan time (never re-measured). A bundle failure
// records one error, SKIPS that app's related paths, and continues with the
// next app. Dry-run counts every app as uninstalled and credits TotalSize
// without touching disk.
func Uninstall(ctx context.Context, apps []AppInfo, dryRun bool, progress func(current, total int, appName string)) Summary {
	s := Summary{}
	total := len(apps)
	for i, app := range apps {
		if progress != nil {
			progress(i+1, total, app.Name)
		}
		if ctx.Err() != nil {
			break
		}
		if dryRun {
			s.Uninstalled++
			s.FreedSpace += app.TotalSize
			continue
		}
		if !removeOne(ctx, app.Path, app.AppSize) {
			s.Errors = append(s.Errors, app.Name+": Failed to remove (security check failed or permission denied)")
			continue
		}
		s.FreedSpace += app.AppSize
		for _, rp := range app.RelatedPaths {
			if removeOne(ctx, rp.Path, rp.Size) {
				s.FreedSpace += rp.Size
			}
		}
		s.Uninstalled++
	}
	return s
}
```

- [ ] **Step 8 (cycle B): Run test to verify it passes**

Run: `go test ./internal/uninstall/ -v`
Expected: PASS — all 11 tests across both files, ending `ok  github.com/guhcostan/app-cleaner/internal/uninstall`.

- [ ] **Step 9 (cycle C): Write the failing test for AppIcon**

Create `internal/uninstall/appicon_test.go` (same package — it reuses the `makeApp`/`write`/`xmlInfoPlist` helpers from `related_test.go`):

```go
package uninstall

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pngFixture stands in for sips output; AppIcon never inspects the bytes,
// it only round-trips them to base64.
var pngFixture = []byte("\x89PNG\r\n\x1a\nfake-png-bytes")

// sipsRunner fakes /usr/bin/sips: it writes pngFixture to the --out path
// and records every call.
type sipsRunner struct {
	t     *testing.T
	calls [][]string
}

func (f *sipsRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.t.Helper()
	f.calls = append(f.calls, append([]string{bin}, args...))
	if timeout != 10*time.Second {
		f.t.Fatalf("timeout = %v, want 10s", timeout)
	}
	outPath := ""
	for i, a := range args {
		if a == "--out" && i+1 < len(args) {
			outPath = args[i+1]
		}
	}
	if outPath == "" {
		f.t.Fatalf("no --out argument in %q", args)
	}
	if err := os.WriteFile(outPath, pngFixture, 0o644); err != nil {
		f.t.Fatal(err)
	}
	return "", nil
}

func xmlIconPlist(iconFile string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.iconapp</string>
	<key>CFBundleIconFile</key>
	<string>%s</string>
</dict>
</plist>
`, iconFile)
}

func TestAppIconConvertsCachesAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "icons") // does not exist yet → AppIcon must MkdirAll it
	// CFBundleIconFile has no extension → ".icns" must be appended.
	app := makeApp(t, dir, "IconApp", []byte(xmlIconPlist("MyIcon")))
	icns := filepath.Join(app, "Contents", "Resources", "MyIcon.icns")
	write(t, icns, "icns-bytes")

	f := &sipsRunner{t: t}
	got := AppIcon(context.Background(), f, app, cacheDir)
	if got == "" {
		t.Fatal("AppIcon = \"\", want base64 PNG for a bundle with a valid .icns")
	}
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil || string(decoded) != string(pngFixture) {
		t.Fatalf("base64 round-trip failed: err = %v, decoded = %q", err, decoded)
	}
	sum := sha1.Sum([]byte(app))
	wantPng := filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".png")
	want := strings.Join([]string{"/usr/bin/sips", "-s", "format", "png", icns, "--out", wantPng}, " ")
	if len(f.calls) != 1 || strings.Join(f.calls[0], " ") != want {
		t.Fatalf("sips calls = %q, want [%q]", f.calls, want)
	}

	// Second call: the cached PNG (sha1-of-bundle-path key) short-circuits sips.
	again := AppIcon(context.Background(), f, app, cacheDir)
	if again != got {
		t.Fatalf("cache hit returned different data: %q vs %q", again, got)
	}
	if len(f.calls) != 1 {
		t.Fatalf("sips calls across two AppIcon calls = %d, want exactly 1 (cache hit)", len(f.calls))
	}
}

func TestAppIconDefaultsToAppIconName(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()
	// No CFBundleIconFile key at all → default name "AppIcon" (+ ".icns").
	app := makeApp(t, dir, "PlainApp", []byte(xmlInfoPlist("com.example.plainapp")))
	write(t, filepath.Join(app, "Contents", "Resources", "AppIcon.icns"), "icns-bytes")

	f := &sipsRunner{t: t}
	if got := AppIcon(context.Background(), f, app, cacheDir); got == "" {
		t.Fatal("missing CFBundleIconFile must fall back to AppIcon.icns")
	}
	if len(f.calls) != 1 || f.calls[0][4] != filepath.Join(app, "Contents", "Resources", "AppIcon.icns") {
		t.Fatalf("sips calls = %q, want the AppIcon.icns source path", f.calls)
	}
}

func TestAppIconMissingIcnsReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()
	app := makeApp(t, dir, "NoIcon", []byte(xmlInfoPlist("com.example.noicon"))) // no Resources/*.icns

	f := &sipsRunner{t: t}
	if got := AppIcon(context.Background(), f, app, cacheDir); got != "" {
		t.Fatalf("AppIcon = %q, want \"\" when the .icns is missing", got)
	}
	if len(f.calls) != 0 {
		t.Fatalf("sips must not run when the .icns is missing: %q", f.calls)
	}
}
```

- [ ] **Step 10 (cycle C): Run test to verify it fails**

Run: `go test ./internal/uninstall/ -v`
Expected: FAIL — build error (`undefined: AppIcon`).

- [ ] **Step 11 (cycle C): Write minimal implementation**

Create `internal/uninstall/appicon.go`:

```go
package uninstall

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

// AppIcon returns the app's icon as a base64-encoded PNG, or "" on ANY
// failure — icons are cosmetic chrome, fetched lazily by the bridge's
// GetAppIcon, so errors never propagate. The icon name comes from
// CFBundleIconFile in Contents/Info.plist (default "AppIcon" when the
// plist or key is missing; ".icns" appended when the name has no
// extension). The .icns is converted to PNG with /usr/bin/sips into
// cacheDir, keyed by the sha1 of the bundle path; an existing cached PNG
// skips the conversion entirely.
func AppIcon(ctx context.Context, r Runner, appPath, cacheDir string) string {
	name := "AppIcon"
	if data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist")); err == nil {
		var info struct {
			CFBundleIconFile string `plist:"CFBundleIconFile"`
		}
		if _, err := plist.Unmarshal(data, &info); err == nil {
			if v := strings.TrimSpace(info.CFBundleIconFile); v != "" {
				name = v
			}
		}
	}
	if filepath.Ext(name) == "" {
		name += ".icns"
	}
	icnsPath := filepath.Join(appPath, "Contents", "Resources", name)
	if _, err := os.Stat(icnsPath); err != nil {
		return ""
	}
	sum := sha1.Sum([]byte(appPath))
	pngPath := filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".png")
	if _, err := os.Stat(pngPath); err != nil {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return ""
		}
		if _, err := r.Run(ctx, 10*time.Second, "/usr/bin/sips", "-s", "format", "png", icnsPath, "--out", pngPath); err != nil {
			return ""
		}
	}
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}
```

- [ ] **Step 12 (cycle C): Run test to verify it passes**

Run: `go test ./internal/uninstall/ -v`
Expected: PASS — all 14 tests across the three files, ending `ok  github.com/guhcostan/app-cleaner/internal/uninstall`.

- [ ] **Step 13: Commit**

```
git add go.mod go.sum internal/uninstall/related.go internal/uninstall/related_test.go internal/uninstall/uninstall.go internal/uninstall/uninstall_test.go internal/uninstall/appicon.go internal/uninstall/appicon_test.go
git commit -m "feat(uninstall): app discovery, plist bundle ids, related paths, lazy app icons, and safe uninstall

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 17: Maintenance Tasks (`internal/maintenance`)

**Files:**
- Create: `internal/maintenance/maintenance.go`, `internal/maintenance/exec.go`, `internal/maintenance/elevate.go`, `internal/maintenance/dns.go`, `internal/maintenance/purge.go`, `internal/maintenance/timemachine.go`
- Test: `internal/maintenance/maintenance_test.go`

**Interfaces:**
- Consumes: nothing from other engine packages (stdlib only).
- Produces (consumed by the Wails bridge task `app.go`):
  - `type Result struct { Success bool; Message string; Error string; RequiresAdmin bool }` (JSON tags `success`, `message`, `error,omitempty`, `requiresAdmin`)
  - `type Runner interface { Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) }` — same shape as `scanners.CmdRunner`, defined locally to avoid import coupling
  - `type Elevator interface { RunElevated(ctx context.Context, shellScript string) (string, error) }`
  - `type ExecRunner struct{}` — real `Runner` (exec.CommandContext, absolute paths, stderr becomes the error on non-zero exit)
  - `type OsaElevator struct{ Runner Runner }` — real `Elevator` via `osascript -e 'do shell script "<script>" with administrator privileges'`
  - `func FlushDNS(ctx context.Context, e Elevator) Result`
  - `func FreePurgeable(ctx context.Context, r Runner, e Elevator) Result`
  - `func ClearTMSnapshots(ctx context.Context, r Runner, e Elevator, progress func(done, total int, date, errMsg string)) Result`
- Package-local (a verifier should know about these): `userCanceled(err error) bool` (detects a dismissed macOS auth prompt — osascript reports `execution error: User canceled. (-128)`), `const elevatedTimeout = 120 * time.Second`, `var snapshotDateRe`.

The canonical contract now codifies the 120 s elevated budget explicitly: the CLI-derived 10 s/60 s timeouts apply to unprivileged `Runner` calls only, while ALL elevated osascript calls use `elevatedTimeout` (120 s), because the user types a password before the script starts.

Behavioral notes (spec §7 + porting-notes → maintenance):
- All binaries run via `exec.CommandContext` with absolute paths and arg slices — never a shell. The only "shell" text is the string handed to `do shell script`, built exclusively from fixed literals plus regex-validated snapshot dates.
- The `-e` argument embeds the script in an AppleScript string literal, so `OsaElevator` escapes `\` and `"` (the exec arg slice makes shell-level quoting moot; this escaping is what keeps the fixed scripts — which contain double quotes — intact). `elevatedTimeout` is 120 s because the user has to type a password before the script even starts (the CLI's 10 s/60 s timeouts applied to non-interactive `sudo -n` runs; they still apply to the unprivileged `Runner` calls below).
- `FlushDNS`: always elevated, both commands in ONE script (one password prompt): `/usr/bin/dscacheutil -flushcache && /usr/bin/killall -HUP mDNSResponder`. Success → `"DNS cache flushed successfully"`; failure → `Message "Failed to flush DNS cache"`, `Error` = err text, `RequiresAdmin` = user cancelled the prompt.
- `FreePurgeable`: try plain `/usr/sbin/purge` FIRST (60 s — spec change #4, reversed from the CLI's sudo-first); only a permission-style failure (stderr contains `Operation not permitted` or `Permission denied`) triggers the elevated fallback. Success either way → `"Purgeable space freed successfully"`; failure → `"Failed to free purgeable space"`.
- `ClearTMSnapshots`: list unprivileged via `/usr/bin/tmutil listlocalsnapshotdates` (30 s); keep ONLY lines matching `^\d{4}-\d{2}-\d{2}-\d{6}$` (injection guard). 0 dates → success `"No Time Machine local snapshots found"`. Otherwise ONE elevated invocation (one password prompt): `for d in <dates>; do /usr/bin/tmutil deletelocalsnapshots $d && echo "OK $d" || echo "ERR $d"; done`; parse the `OK <date>` / `ERR <date>` stdout lines and fire `progress(done, total, date, errMsg)` once per date (a date with no status line counts as failed). All failed → `Success=false`, `Error` = first per-date error; partial → `Success=true`, `"Deleted X/Y Time Machine snapshots (N error(s))"`; full → `"Deleted N Time Machine snapshot(s)"` with singular/plural on N.

This task runs TWO test→impl cycles, then one commit. `ExecRunner` itself is a thin real-binary wrapper: it is compile-verified here and exercised end-to-end in the `wails dev` integration smoke (unit tests use fakes only, per the testing rules).

- [ ] **Step 1 (cycle A): Write the failing test for the OsaElevator + shared types**

Create `internal/maintenance/maintenance_test.go`:

```go
package maintenance

import (
	"context"
	"testing"
	"time"
)

type runnerCall struct {
	bin     string
	args    []string
	timeout time.Duration
}

type fakeRunner struct {
	out   string
	err   error
	calls []runnerCall
}

func (f *fakeRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.calls = append(f.calls, runnerCall{bin: bin, args: args, timeout: timeout})
	return f.out, f.err
}

type fakeElevator struct {
	out     string
	err     error
	scripts []string
}

func (f *fakeElevator) RunElevated(ctx context.Context, script string) (string, error) {
	f.scripts = append(f.scripts, script)
	return f.out, f.err
}

func TestOsaElevatorBuildsEscapedOsascriptCall(t *testing.T) {
	r := &fakeRunner{out: "done"}
	e := OsaElevator{Runner: r}
	out, err := e.RunElevated(context.Background(), `echo "OK $d" || echo "ERR $d"`)
	if err != nil || out != "done" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(r.calls))
	}
	call := r.calls[0]
	if call.bin != "/usr/bin/osascript" {
		t.Fatalf("bin = %q", call.bin)
	}
	if call.timeout != elevatedTimeout {
		t.Fatalf("timeout = %v, want %v", call.timeout, elevatedTimeout)
	}
	wantStmt := `do shell script "echo \"OK $d\" || echo \"ERR $d\"" with administrator privileges`
	if len(call.args) != 2 || call.args[0] != "-e" || call.args[1] != wantStmt {
		t.Fatalf("args = %q, want [-e %q]", call.args, wantStmt)
	}
}

func TestUserCanceledDetection(t *testing.T) {
	if !userCanceled(errTest("execution error: User canceled. (-128)")) {
		t.Fatal("osascript cancel message must be detected")
	}
	if userCanceled(errTest("some other failure")) {
		t.Fatal("unrelated errors are not a cancel")
	}
	if userCanceled(nil) {
		t.Fatal("nil error is not a cancel")
	}
}

// errTest is a tiny helper so tests read cleanly.
type errTest string

func (e errTest) Error() string { return string(e) }
```

- [ ] **Step 2 (cycle A): Run test to verify it fails**

Run: `go test ./internal/maintenance/ -v`
Expected: FAIL — build errors (`undefined: OsaElevator`, `undefined: elevatedTimeout`, `undefined: userCanceled`).

- [ ] **Step 3 (cycle A): Write minimal implementation**

Create `internal/maintenance/maintenance.go`:

```go
// Package maintenance implements the three system maintenance tasks
// (DNS flush, purgeable space, Time Machine snapshots). External binaries
// run behind the Runner/Elevator interfaces so unit tests use fakes.
package maintenance

import (
	"context"
	"strings"
	"time"
)

// Result is the single shared result contract for all maintenance tasks.
type Result struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	Error         string `json:"error,omitempty"`
	RequiresAdmin bool   `json:"requiresAdmin"`
}

// Runner executes an external binary (absolute path, arg slice, no shell).
// Same shape as scanners.CmdRunner but defined locally so engine packages
// stay decoupled.
type Runner interface {
	Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error)
}

// Elevator runs a shell script with administrator privileges.
type Elevator interface {
	RunElevated(ctx context.Context, shellScript string) (string, error)
}

// userCanceled reports whether the user dismissed the macOS admin prompt
// (osascript reports: "execution error: User canceled. (-128)").
func userCanceled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "User canceled")
}
```

Create `internal/maintenance/exec.go`:

```go
package maintenance

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// ExecRunner is the production Runner: exec.CommandContext with a timeout,
// no shell. On non-zero exit the trimmed stderr becomes the error text
// (falling back to the exec error), mirroring the CLI's spawn wrapper.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), errors.New(msg)
	}
	return stdout.String(), nil
}
```

Create `internal/maintenance/elevate.go`:

```go
package maintenance

import (
	"context"
	"strings"
	"time"
)

// elevatedTimeout is generous because the user must type a password in the
// native macOS auth dialog before the shell script even starts.
const elevatedTimeout = 120 * time.Second

// OsaElevator runs a shell script with admin rights via
//
//	osascript -e 'do shell script "<script>" with administrator privileges'
//
// The script is embedded in an AppleScript string literal, so backslashes
// and double quotes are escaped. Callers only ever pass fixed literals plus
// regex-validated snapshot dates — never user-controlled text.
type OsaElevator struct{ Runner Runner }

func (e OsaElevator) RunElevated(ctx context.Context, shellScript string) (string, error) {
	escaped := strings.ReplaceAll(shellScript, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	stmt := `do shell script "` + escaped + `" with administrator privileges`
	return e.Runner.Run(ctx, elevatedTimeout, "/usr/bin/osascript", "-e", stmt)
}
```

- [ ] **Step 4 (cycle A): Run test to verify it passes**

Run: `go test ./internal/maintenance/ -v`
Expected: PASS — `TestOsaElevatorBuildsEscapedOsascriptCall`, `TestUserCanceledDetection`.

- [ ] **Step 5 (cycle B): Write the failing tests for the three tasks**

Append to `internal/maintenance/maintenance_test.go` (add `"errors"`, `"fmt"`, and `"strings"` to the existing import block):

```go
func TestFlushDNSSuccess(t *testing.T) {
	e := &fakeElevator{}
	res := FlushDNS(context.Background(), e)
	if !res.Success || res.Message != "DNS cache flushed successfully" || res.Error != "" {
		t.Fatalf("result = %+v", res)
	}
	wantScript := "/usr/bin/dscacheutil -flushcache && /usr/bin/killall -HUP mDNSResponder"
	if len(e.scripts) != 1 || e.scripts[0] != wantScript {
		t.Fatalf("scripts = %q, want [%q] (both commands, ONE prompt)", e.scripts, wantScript)
	}
}

func TestFlushDNSUserCanceled(t *testing.T) {
	e := &fakeElevator{err: errors.New("execution error: User canceled. (-128)")}
	res := FlushDNS(context.Background(), e)
	if res.Success || res.Message != "Failed to flush DNS cache" {
		t.Fatalf("result = %+v", res)
	}
	if !res.RequiresAdmin || !strings.Contains(res.Error, "User canceled") {
		t.Fatalf("cancel must set RequiresAdmin and keep the error: %+v", res)
	}
}

func TestFreePurgeablePlainFirst(t *testing.T) {
	r := &fakeRunner{}
	e := &fakeElevator{}
	res := FreePurgeable(context.Background(), r, e)
	if !res.Success || res.Message != "Purgeable space freed successfully" {
		t.Fatalf("result = %+v", res)
	}
	if len(e.scripts) != 0 {
		t.Fatal("must NOT elevate when the plain run succeeds")
	}
	if len(r.calls) != 1 || r.calls[0].bin != "/usr/sbin/purge" || len(r.calls[0].args) != 0 {
		t.Fatalf("plain call = %+v, want /usr/sbin/purge with no args", r.calls)
	}
	if r.calls[0].timeout != 60*time.Second {
		t.Fatalf("timeout = %v, want 60s", r.calls[0].timeout)
	}
}

func TestFreePurgeableElevatesOnPermissionFailure(t *testing.T) {
	r := &fakeRunner{err: errors.New("purge: Operation not permitted")}
	e := &fakeElevator{}
	res := FreePurgeable(context.Background(), r, e)
	if !res.Success || res.Message != "Purgeable space freed successfully" {
		t.Fatalf("result = %+v", res)
	}
	if len(e.scripts) != 1 || e.scripts[0] != "/usr/sbin/purge" {
		t.Fatalf("elevated scripts = %q, want [/usr/sbin/purge]", e.scripts)
	}
}

func TestFreePurgeableNonPermissionFailureDoesNotElevate(t *testing.T) {
	r := &fakeRunner{err: errors.New("some other failure")}
	e := &fakeElevator{}
	res := FreePurgeable(context.Background(), r, e)
	if res.Success || res.Message != "Failed to free purgeable space" || res.Error != "some other failure" {
		t.Fatalf("result = %+v", res)
	}
	if res.RequiresAdmin {
		t.Fatal("non-permission failure must not claim admin is required")
	}
	if len(e.scripts) != 0 {
		t.Fatal("must not elevate on a non-permission failure")
	}
}

func TestClearTMSnapshotsFiltersGarbageAndRunsOneElevatedCall(t *testing.T) {
	r := &fakeRunner{out: "Snapshot dates for all disks:\n2024-01-15-123456\nnot-a-date\n2024-01-16-654321\n\ncom.apple.TimeMachine.2024-01-17\n"}
	e := &fakeElevator{out: "OK 2024-01-15-123456\nOK 2024-01-16-654321\n"}
	var prog []string
	res := ClearTMSnapshots(context.Background(), r, e, func(done, total int, date, errMsg string) {
		prog = append(prog, fmt.Sprintf("%d/%d %s %q", done, total, date, errMsg))
	})
	if !res.Success || res.Message != "Deleted 2 Time Machine snapshots" {
		t.Fatalf("result = %+v", res)
	}
	if len(r.calls) != 1 || r.calls[0].bin != "/usr/bin/tmutil" || r.calls[0].args[0] != "listlocalsnapshotdates" {
		t.Fatalf("list call = %+v", r.calls)
	}
	if r.calls[0].timeout != 30*time.Second {
		t.Fatalf("list timeout = %v, want 30s", r.calls[0].timeout)
	}
	if len(e.scripts) != 1 {
		t.Fatalf("elevated invocations = %d, want exactly 1 (one password prompt)", len(e.scripts))
	}
	script := e.scripts[0]
	if !strings.HasPrefix(script, "for d in 2024-01-15-123456 2024-01-16-654321; do ") {
		t.Fatalf("script = %q", script)
	}
	if !strings.Contains(script, `/usr/bin/tmutil deletelocalsnapshots $d && echo "OK $d" || echo "ERR $d"; done`) {
		t.Fatalf("script = %q", script)
	}
	if strings.Contains(script, "not-a-date") || strings.Contains(script, "com.apple.TimeMachine") {
		t.Fatal("garbage lines must never reach the elevated script")
	}
	wantProg := []string{`1/2 2024-01-15-123456 ""`, `2/2 2024-01-16-654321 ""`}
	if len(prog) != 2 || prog[0] != wantProg[0] || prog[1] != wantProg[1] {
		t.Fatalf("progress = %v, want %v", prog, wantProg)
	}
}

func TestClearTMSnapshotsNoSnapshots(t *testing.T) {
	r := &fakeRunner{out: "Snapshot dates for all disks:\nnothing here\n"}
	e := &fakeElevator{}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if !res.Success || res.Message != "No Time Machine local snapshots found" {
		t.Fatalf("result = %+v", res)
	}
	if len(e.scripts) != 0 {
		t.Fatal("must not elevate when there is nothing to delete")
	}
}

func TestClearTMSnapshotsPartialSuccess(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n2024-01-16-654321\n"}
	e := &fakeElevator{out: "OK 2024-01-15-123456\nERR 2024-01-16-654321\n"}
	var errMsgs []string
	res := ClearTMSnapshots(context.Background(), r, e, func(done, total int, date, errMsg string) {
		errMsgs = append(errMsgs, errMsg)
	})
	if !res.Success || res.Message != "Deleted 1/2 Time Machine snapshots (1 error(s))" {
		t.Fatalf("result = %+v", res)
	}
	if len(errMsgs) != 2 || errMsgs[0] != "" || errMsgs[1] == "" {
		t.Fatalf("per-date errMsgs = %q", errMsgs)
	}
}

func TestClearTMSnapshotsTotalFailure(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n2024-01-16-654321\n"}
	e := &fakeElevator{out: "ERR 2024-01-15-123456\nERR 2024-01-16-654321\n"}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if res.Success || res.Message != "Failed to delete Time Machine snapshots" {
		t.Fatalf("result = %+v", res)
	}
	if res.Error != "2024-01-15-123456: failed to delete snapshot" {
		t.Fatalf("Error = %q, want the FIRST per-date error", res.Error)
	}
}

func TestClearTMSnapshotsSingularMessage(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n"}
	e := &fakeElevator{out: "OK 2024-01-15-123456\n"}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if !res.Success || res.Message != "Deleted 1 Time Machine snapshot" {
		t.Fatalf("result = %+v (singular, no trailing 's')", res)
	}
}

func TestClearTMSnapshotsListFailure(t *testing.T) {
	r := &fakeRunner{err: errors.New("boom")}
	e := &fakeElevator{}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if res.Success || res.Message != "Failed to list Time Machine snapshots" {
		t.Fatalf("result = %+v", res)
	}
	if res.Error != "tmutil not available or Time Machine is not configured on this Mac" {
		t.Fatalf("Error = %q", res.Error)
	}
}

func TestClearTMSnapshotsElevationCanceled(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n"}
	e := &fakeElevator{err: errors.New("execution error: User canceled. (-128)")}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if res.Success || res.Message != "Failed to delete Time Machine snapshots" || !res.RequiresAdmin {
		t.Fatalf("result = %+v", res)
	}
}
```

- [ ] **Step 6 (cycle B): Run test to verify it fails**

Run: `go test ./internal/maintenance/ -v`
Expected: FAIL — build errors (`undefined: FlushDNS`, `undefined: FreePurgeable`, `undefined: ClearTMSnapshots`).

- [ ] **Step 7 (cycle B): Write minimal implementation**

Create `internal/maintenance/dns.go`:

```go
package maintenance

import "context"

// FlushDNS flushes the macOS DNS cache. It always requires admin rights, so
// both commands run in ONE elevated shell script (one password prompt).
func FlushDNS(ctx context.Context, e Elevator) Result {
	_, err := e.RunElevated(ctx, "/usr/bin/dscacheutil -flushcache && /usr/bin/killall -HUP mDNSResponder")
	if err != nil {
		return Result{
			Success:       false,
			Message:       "Failed to flush DNS cache",
			Error:         err.Error(),
			RequiresAdmin: userCanceled(err),
		}
	}
	return Result{Success: true, Message: "DNS cache flushed successfully"}
}
```

Create `internal/maintenance/purge.go`:

```go
package maintenance

import (
	"context"
	"strings"
	"time"
)

// FreePurgeable invokes /usr/sbin/purge. It tries unprivileged FIRST (it
// usually works on modern macOS — spec change #4); only a permission-style
// failure triggers the elevated fallback.
func FreePurgeable(ctx context.Context, r Runner, e Elevator) Result {
	_, err := r.Run(ctx, 60*time.Second, "/usr/sbin/purge")
	if err == nil {
		return Result{Success: true, Message: "Purgeable space freed successfully"}
	}
	msg := err.Error()
	if !strings.Contains(msg, "Operation not permitted") && !strings.Contains(msg, "Permission denied") {
		return Result{Success: false, Message: "Failed to free purgeable space", Error: msg}
	}
	if _, eerr := e.RunElevated(ctx, "/usr/sbin/purge"); eerr != nil {
		return Result{
			Success:       false,
			Message:       "Failed to free purgeable space",
			Error:         eerr.Error(),
			RequiresAdmin: userCanceled(eerr),
		}
	}
	return Result{Success: true, Message: "Purgeable space freed successfully"}
}
```

Create `internal/maintenance/timemachine.go`:

```go
package maintenance

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// snapshotDateRe is a strict injection guard: only lines shaped like
// 2024-01-15-123456 are accepted as snapshot dates and interpolated into
// the elevated script.
var snapshotDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{6}$`)

// ClearTMSnapshots lists local Time Machine snapshots unprivileged, then
// deletes them all in ONE elevated invocation (one password prompt): the
// elevated shell script loops over the pre-validated dates and prints one
// "OK <date>" or "ERR <date>" line per date, parsed here for per-date
// progress and errors. A date with no status line counts as failed.
func ClearTMSnapshots(ctx context.Context, r Runner, e Elevator, progress func(done, total int, date, errMsg string)) Result {
	out, err := r.Run(ctx, 30*time.Second, "/usr/bin/tmutil", "listlocalsnapshotdates")
	if err != nil {
		return Result{
			Success: false,
			Message: "Failed to list Time Machine snapshots",
			Error:   "tmutil not available or Time Machine is not configured on this Mac",
		}
	}
	var dates []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if snapshotDateRe.MatchString(line) {
			dates = append(dates, line)
		}
	}
	if len(dates) == 0 {
		return Result{Success: true, Message: "No Time Machine local snapshots found"}
	}

	script := "for d in " + strings.Join(dates, " ") +
		`; do /usr/bin/tmutil deletelocalsnapshots $d && echo "OK $d" || echo "ERR $d"; done`
	stdout, err := e.RunElevated(ctx, script)
	if err != nil {
		return Result{
			Success:       false,
			Message:       "Failed to delete Time Machine snapshots",
			Error:         err.Error(),
			RequiresAdmin: userCanceled(err),
		}
	}

	status := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if date, ok := strings.CutPrefix(line, "OK "); ok && snapshotDateRe.MatchString(date) {
			status[date] = "OK"
			continue
		}
		if date, ok := strings.CutPrefix(line, "ERR "); ok && snapshotDateRe.MatchString(date) {
			status[date] = "ERR"
		}
	}

	total := len(dates)
	var errs []string
	for i, d := range dates {
		errMsg := ""
		if status[d] != "OK" { // "ERR" or no status line at all
			errMsg = d + ": failed to delete snapshot"
			errs = append(errs, errMsg)
		}
		if progress != nil {
			progress(i+1, total, d, errMsg)
		}
	}
	deleted := total - len(errs)
	switch {
	case deleted == 0:
		return Result{Success: false, Message: "Failed to delete Time Machine snapshots", Error: errs[0]}
	case len(errs) > 0:
		return Result{Success: true, Message: fmt.Sprintf("Deleted %d/%d Time Machine snapshots (%d error(s))", deleted, total, len(errs))}
	default:
		plural := "s"
		if deleted == 1 {
			plural = ""
		}
		return Result{Success: true, Message: fmt.Sprintf("Deleted %d Time Machine snapshot%s", deleted, plural)}
	}
}
```

- [ ] **Step 8 (cycle B): Run test to verify it passes**

Run: `go test ./internal/maintenance/ -v`
Expected: PASS — all 13 tests, ending `ok  github.com/guhcostan/app-cleaner/internal/maintenance`.

- [ ] **Step 9: Commit**

```
git add internal/maintenance/maintenance.go internal/maintenance/exec.go internal/maintenance/elevate.go internal/maintenance/dns.go internal/maintenance/purge.go internal/maintenance/timemachine.go internal/maintenance/maintenance_test.go
git commit -m "feat(maintenance): dns flush, purgeable space, and TM snapshot clearing with osascript elevation

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 18: Full Disk Access Probe (`internal/fda`)

**Files:**
- Create: `internal/fda/fda.go`
- Test: `internal/fda/fda_test.go`

**Interfaces:**
- Consumes: stdlib only.
- Produces (consumed by the Wails bridge task `app.go`: `CheckFDA() *bool` and `OpenFDASettings()`):
  - `func Check(home string) *bool` — tri-state: `true` = FDA granted, `false` = denied (EPERM/EACCES), `nil` = unknown (e.g. `~/Library/Safari` missing because Safari never ran)
  - `const SettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"`

Behavioral notes (spec §11 FDA probe + porting-notes → utils → fda.ts): probe `os.ReadDir(<home>/Library/Safari)` — a TCC-protected directory that macOS blocks with EPERM even for the owning user when the app lacks Full Disk Access. `errors.Is(err, syscall.EPERM)` / `errors.Is(err, syscall.EACCES)` unwraps the `*fs.PathError` down to the `syscall.Errno`.

- [ ] **Step 1: Write the failing test**

Create `internal/fda/fda_test.go`:

```go
package fda

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckReadableIsTrue(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Library", "Safari"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Check(home)
	if got == nil || !*got {
		t.Fatalf("Check = %v, want true", got)
	}
}

func TestCheckMissingDirIsUnknown(t *testing.T) {
	home := t.TempDir() // no Library/Safari → ENOENT → unknown
	if got := Check(home); got != nil {
		t.Fatalf("Check = %v, want nil (unknown)", *got)
	}
}

func TestCheckPermissionDeniedIsFalse(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: mode 000 does not block reads")
	}
	home := t.TempDir()
	safari := filepath.Join(home, "Library", "Safari")
	if err := os.MkdirAll(safari, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(safari, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(safari, 0o755) }) // let t.TempDir clean up
	got := Check(home)
	if got == nil || *got {
		t.Fatalf("Check = %v, want false (EACCES)", got)
	}
}

func TestSettingsURL(t *testing.T) {
	const want = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"
	if SettingsURL != want {
		t.Fatalf("SettingsURL = %q, want %q", SettingsURL, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/fda/ -v`
Expected: FAIL — build errors (`undefined: Check`, `undefined: SettingsURL`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/fda/fda.go`:

```go
// Package fda detects Full Disk Access by probing a TCC-protected
// directory. Note: in the GUI app, FDA must be granted to App Cleaner
// itself (not the user's terminal).
package fda

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// SettingsURL deep-links to System Settings → Privacy & Security → Full
// Disk Access.
const SettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

// Check probes <home>/Library/Safari (TCC-protected — macOS returns EPERM
// on reads without FDA, even for the owning user).
// Tri-state result: true = granted, false = denied (EPERM/EACCES),
// nil = unknown (any other error, e.g. ENOENT when Safari never ran).
func Check(home string) *bool {
	_, err := os.ReadDir(filepath.Join(home, "Library", "Safari"))
	if err == nil {
		return boolPtr(true)
	}
	// errors.Is unwraps the *fs.PathError to its syscall.Errno.
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return boolPtr(false)
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/fda/ -v`
Expected: PASS — all 4 tests (`TestCheckPermissionDeniedIsFalse` is SKIPped when running as root), ending `ok  github.com/guhcostan/app-cleaner/internal/fda`.

- [ ] **Step 5: Commit**

```
git add internal/fda/fda.go internal/fda/fda_test.go
git commit -m "feat(fda): tri-state Full Disk Access probe and settings deep link

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```
### Task 19: Directory Grouping (`internal/grouping`)

Ports `mac-cleaner-cli/src/utils/grouping.ts` + the `truncateDirectoryPath` half of `paths.ts` into one Go package. It converts a flat `[]core.CleanableItem` into the flat row list the frontend `ItemList` renders: items grouped by parent directory, directories ordered by their largest single file (hotspots first), files size-descending inside each group, 5 visible files per group by default with an "expand hint" row for the rest, and directory headers displayed as `~`-contracted, middle-elided paths capped at 50 chars.

**Files:**
- Create: `internal/grouping/grouping.go`
- Test: `internal/grouping/grouping_test.go`

**Interfaces:**
- Consumes: `core.CleanableItem` (fields `Path string`, `Size int64`, `Name string`) from `internal/core` (Task 2).
- Produces (used by Task 20 `App.GroupItems` and mirrored in `frontend/src/lib/types.ts`):
  - `type DisplayRow struct { Type string; DirectoryKey string; DisplayName string; Path string; Size int64; Name string; HiddenCount int; TotalFilesInDir int; Selectable bool }` — JSON tags exactly as in the contract (`type`, `directoryKey`, `displayName`, `path,omitempty`, `size,omitempty`, `name,omitempty`, `hiddenCount,omitempty`, `totalFilesInDir`, `selectable`).
  - `func GroupItems(items []core.CleanableItem, home string, expand map[string]int, defaultLimit int, absolutePaths bool) []DisplayRow`
  - `func TruncateDirectoryPath(p, home string, maxLen int) string`

Behavioral spec (normative, from `grouping.ts`/`paths.ts` via porting-notes):
1. Group items by `filepath.Dir(item.Path)`; groups keep first-appearance order as the tie-break baseline (JS `Map` insertion order + stable sort equivalent).
2. Sort files inside each group by `Size` descending (stable).
3. Sort groups by their largest single file's size descending (stable).
4. Per group emit: one `directory-header` row (`Selectable=false`, `DisplayName` = truncated dir path, `DirectoryKey` = absolute dir, `TotalFilesInDir` = group file count), then `visibleCount = min(expand[dir] (if present, else defaultLimit), len(files))` `file` rows (`Selectable=true`, `Name` and `DisplayName` = basename of `Path`, plus `Path`, `Size`, `DirectoryKey`, `TotalFilesInDir`), then — only if files remain hidden — one `expand-hint` row (`HiddenCount = len(files) − visibleCount`, `Selectable=false`).
5. `TruncateDirectoryPath`: contract `home` prefix to `~` (exact home → `~`; `home + "/"` prefix → `~/…`); if the display string length ≤ `maxLen` return it unchanged; otherwise split on `/` (dropping empty segments): ≤ 2 segments → hard-truncate to `display[:maxLen-3] + "..."`; else middle-elide to `<first>/.../<last>/<two>` where `<first>` is `~` if the path was contracted, else `/<segment0>`; if the elided form still exceeds `maxLen`, hard-truncate it the same way.
6. `absolutePaths=true` → the header `DisplayName` is the raw absolute dir path: no `~` contraction and no truncation.
7. `defaultLimit ≤ 0` falls back to 5. `expand` may be nil (Go nil-map reads are safe).

- [ ] **Step 1: Write the failing test**

Create `internal/grouping/grouping_test.go`:

```go
package grouping

import (
	"fmt"
	"strings"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

const home = "/Users/tester"

func item(path string, size int64) core.CleanableItem {
	return core.CleanableItem{
		Path: path,
		Size: size,
		Name: path[strings.LastIndex(path, "/")+1:],
	}
}

func rowTypes(rows []DisplayRow) string {
	types := make([]string, len(rows))
	for i, r := range rows {
		types[i] = r.Type
	}
	return strings.Join(types, ",")
}

func TestGroupItems_GroupsSortedByLargestSingleFile(t *testing.T) {
	items := []core.CleanableItem{
		item(home+"/Downloads/small.zip", 100),
		item(home+"/Documents/huge.pdf", 10000),
		item(home+"/Movies/medium.mp4", 5000),
		item(home+"/Downloads/big.zip", 900),
	}

	rows := GroupItems(items, home, nil, 5, false)

	wantTypes := "directory-header,file,directory-header,file,directory-header,file,file"
	if got := rowTypes(rows); got != wantTypes {
		t.Fatalf("row types = %s, want %s", got, wantTypes)
	}
	// Directory order is driven by the largest SINGLE file in each dir
	// (Documents 10000 > Movies 5000 > Downloads 900), not by dir totals.
	if rows[0].DirectoryKey != home+"/Documents" {
		t.Errorf("first group = %q, want ~/Documents", rows[0].DirectoryKey)
	}
	if rows[2].DirectoryKey != home+"/Movies" {
		t.Errorf("second group = %q, want ~/Movies", rows[2].DirectoryKey)
	}
	if rows[4].DirectoryKey != home+"/Downloads" {
		t.Errorf("third group = %q, want ~/Downloads", rows[4].DirectoryKey)
	}
	// Files inside a group are size-descending.
	if rows[5].Size != 900 || rows[6].Size != 100 {
		t.Errorf("Downloads files not size-desc: %d then %d", rows[5].Size, rows[6].Size)
	}
	// Header row shape.
	h := rows[0]
	if h.DisplayName != "~/Documents" || h.Selectable || h.TotalFilesInDir != 1 {
		t.Errorf("bad header row: %+v", h)
	}
	// File row shape.
	f := rows[1]
	if !f.Selectable || f.Name != "huge.pdf" || f.DisplayName != "huge.pdf" ||
		f.Path != home+"/Documents/huge.pdf" || f.Size != 10000 ||
		f.DirectoryKey != home+"/Documents" || f.TotalFilesInDir != 1 {
		t.Errorf("bad file row: %+v", f)
	}
}

func TestGroupItems_DefaultLimitAndExpandOverride(t *testing.T) {
	dir := home + "/Library/Caches/big-app"
	var items []core.CleanableItem
	for i := 0; i < 8; i++ {
		items = append(items, item(fmt.Sprintf("%s/f%d.dat", dir, i), int64(800-i*100)))
	}

	// Default limit 5 -> header + 5 files + expand hint.
	rows := GroupItems(items, home, nil, 5, false)
	if len(rows) != 7 {
		t.Fatalf("expected 7 rows, got %d (%s)", len(rows), rowTypes(rows))
	}
	if rows[1].Size != 800 || rows[5].Size != 400 {
		t.Errorf("visible files must be the 5 largest: got %d..%d", rows[1].Size, rows[5].Size)
	}
	hint := rows[6]
	if hint.Type != "expand-hint" || hint.HiddenCount != 3 || hint.TotalFilesInDir != 8 ||
		hint.Selectable || hint.DirectoryKey != dir {
		t.Errorf("bad expand-hint row: %+v", hint)
	}

	// Expand override showing everything -> no hint row.
	rows = GroupItems(items, home, map[string]int{dir: 8}, 5, false)
	if len(rows) != 9 {
		t.Fatalf("expanded: expected 9 rows, got %d (%s)", len(rows), rowTypes(rows))
	}
	for _, r := range rows {
		if r.Type == "expand-hint" {
			t.Fatal("fully expanded group must not emit an expand-hint row")
		}
	}

	// Partial expand override (6 of 8 visible).
	rows = GroupItems(items, home, map[string]int{dir: 6}, 5, false)
	if len(rows) != 8 {
		t.Fatalf("partial expand: expected 8 rows, got %d (%s)", len(rows), rowTypes(rows))
	}
	if rows[7].Type != "expand-hint" || rows[7].HiddenCount != 2 {
		t.Errorf("partial expand hint wrong: %+v", rows[7])
	}
}

func TestGroupItems_AbsolutePathsSkipContractionAndTruncation(t *testing.T) {
	longDir := home + "/very-long-folder-name-here/another-long-name/third-level/fourth-level"
	rows := GroupItems([]core.CleanableItem{item(longDir+"/file.zip", 10)}, home, nil, 5, true)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].DisplayName != longDir {
		t.Errorf("absolutePaths header = %q, want raw %q", rows[0].DisplayName, longDir)
	}
	if strings.Contains(rows[0].DisplayName, "~") {
		t.Error("absolutePaths must not contract home to ~")
	}
}

func TestGroupItems_Empty(t *testing.T) {
	if rows := GroupItems(nil, home, nil, 5, false); len(rows) != 0 {
		t.Fatalf("expected no rows for no items, got %d", len(rows))
	}
}

func TestTruncateDirectoryPath(t *testing.T) {
	name48 := strings.Repeat("d", 48) // "~/" + 48 chars == exactly 50
	cases := []struct {
		label string
		path  string
		want  string
	}{
		{"home itself contracts to tilde", home, "~"},
		{"short path under home stays intact", home + "/Downloads", "~/Downloads"},
		{"short path outside home stays intact", "/tmp/foo", "/tmp/foo"},
		{"exactly max length stays intact", home + "/" + name48, "~/" + name48},
		{
			"long home path middle-elided keeping last two segments",
			home + "/very-long-folder-name-here/another-long-name/third-level/fourth-level/fifth-level",
			"~/.../fourth-level/fifth-level",
		},
		{
			"long non-home path middle-elided keeping root segment",
			"/Volumes/ExternalDrive/some-deeply/nested/folder-tree/media/movies",
			"/Volumes/.../media/movies",
		},
		{
			"single overlong segment hard-truncated",
			"/" + strings.Repeat("a", 60),
			"/" + strings.Repeat("a", 46) + "...",
		},
		{
			"elided form still too long hard-truncated",
			home + "/x/y/" + strings.Repeat("b", 30) + "/" + strings.Repeat("c", 30),
			"~/.../" + strings.Repeat("b", 30) + "/" + strings.Repeat("c", 10) + "...",
		},
	}
	for _, tc := range cases {
		if got := TruncateDirectoryPath(tc.path, home, 50); got != tc.want {
			t.Errorf("%s: TruncateDirectoryPath(%q) = %q, want %q", tc.label, tc.path, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/grouping/`
Expected: FAIL — build errors `undefined: DisplayRow`, `undefined: GroupItems`, `undefined: TruncateDirectoryPath` (the package has only the test file so far).

- [ ] **Step 3: Write minimal implementation**

Create `internal/grouping/grouping.go`:

```go
// Package grouping turns a flat list of scanned items into the display-row
// structure the UI renders: items grouped by parent directory, directories
// with the largest single file first, with per-directory expand/collapse
// pagination. Mirrors the CLI contract in mac-cleaner-cli/src/utils/grouping.ts
// and truncateDirectoryPath in mac-cleaner-cli/src/utils/paths.ts.
package grouping

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// DisplayRow is one row of the grouped item list. The JSON shape is consumed
// verbatim by the frontend (lib/types.ts DisplayRow).
type DisplayRow struct {
	Type            string `json:"type"` // "directory-header" | "file" | "expand-hint"
	DirectoryKey    string `json:"directoryKey"`    // absolute dir path (grouping/expand key)
	DisplayName     string `json:"displayName"`     // header: truncated dir path; file: basename
	Path            string `json:"path,omitempty"`  // file rows only
	Size            int64  `json:"size,omitempty"`  // file rows only
	Name            string `json:"name,omitempty"`  // file rows only
	HiddenCount     int    `json:"hiddenCount,omitempty"` // expand-hint rows only
	TotalFilesInDir int    `json:"totalFilesInDir"`
	Selectable      bool   `json:"selectable"`
}

// GroupItems groups items by parent directory (filepath.Dir) and flattens the
// result into display rows:
//   - files inside a group are sorted by size descending (stable),
//   - groups are sorted by their largest single file descending (stable;
//     first-appearance order breaks ties, like the CLI's Map + stable sort),
//   - each group emits a directory-header row, then min(expand[dir] or
//     defaultLimit, len(files)) file rows, then an expand-hint row when files
//     remain hidden.
//
// home is used for "~" contraction in header display names; expand maps an
// absolute dir path to a per-directory visible-row override (nil is fine);
// defaultLimit <= 0 falls back to 5; absolutePaths=true renders raw absolute
// dir paths in headers (no contraction, no truncation).
func GroupItems(items []core.CleanableItem, home string, expand map[string]int, defaultLimit int, absolutePaths bool) []DisplayRow {
	if defaultLimit <= 0 {
		defaultLimit = 5
	}

	type dirGroup struct {
		dir     string
		files   []core.CleanableItem
		largest int64
	}

	// 1. Group by parent dir, preserving first-appearance order for stable ties.
	index := make(map[string]int)
	groups := []dirGroup{}
	for _, it := range items {
		dir := filepath.Dir(it.Path)
		i, ok := index[dir]
		if !ok {
			i = len(groups)
			index[dir] = i
			groups = append(groups, dirGroup{dir: dir})
		}
		groups[i].files = append(groups[i].files, it)
	}

	// 2. Files within each group: size descending.
	for i := range groups {
		files := groups[i].files
		sort.SliceStable(files, func(a, b int) bool { return files[a].Size > files[b].Size })
		groups[i].largest = files[0].Size // every group has >= 1 file by construction
	}

	// 3. Groups: largest single file descending.
	sort.SliceStable(groups, func(a, b int) bool { return groups[a].largest > groups[b].largest })

	// 4. Flatten into rows.
	rows := []DisplayRow{}
	for _, g := range groups {
		limit := defaultLimit
		if override, ok := expand[g.dir]; ok {
			limit = override
		}
		visible := limit
		if visible > len(g.files) {
			visible = len(g.files)
		}
		if visible < 0 {
			visible = 0
		}

		displayName := g.dir
		if !absolutePaths {
			displayName = TruncateDirectoryPath(g.dir, home, 50)
		}
		rows = append(rows, DisplayRow{
			Type:            "directory-header",
			DirectoryKey:    g.dir,
			DisplayName:     displayName,
			TotalFilesInDir: len(g.files),
			Selectable:      false,
		})
		for i := 0; i < visible; i++ {
			f := g.files[i]
			base := filepath.Base(f.Path)
			rows = append(rows, DisplayRow{
				Type:            "file",
				DirectoryKey:    g.dir,
				DisplayName:     base,
				Name:            base,
				Path:            f.Path,
				Size:            f.Size,
				TotalFilesInDir: len(g.files),
				Selectable:      true,
			})
		}
		if hidden := len(g.files) - visible; hidden > 0 {
			rows = append(rows, DisplayRow{
				Type:            "expand-hint",
				DirectoryKey:    g.dir,
				HiddenCount:     hidden,
				TotalFilesInDir: len(g.files),
				Selectable:      false,
			})
		}
	}
	return rows
}

// TruncateDirectoryPath renders a directory path for display: the home prefix
// contracts to "~", and paths longer than maxLen are middle-elided to
// "<first>/.../<last>/<two>" (keeping the last two segments), falling back to a
// hard "..."-suffixed cut when even the elided form (or a path with <= 2
// segments) exceeds maxLen. Ported from the CLI's truncateDirectoryPath.
func TruncateDirectoryPath(p, home string, maxLen int) string {
	display := p
	if home != "" {
		switch {
		case p == home:
			display = "~"
		case strings.HasPrefix(p, home+"/"):
			display = "~" + p[len(home):]
		}
	}
	if len(display) <= maxLen {
		return display
	}
	if maxLen < 4 {
		return display // degenerate maxLen: no room for "...", refuse to slice
	}

	parts := []string{}
	for _, part := range strings.Split(display, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) <= 2 {
		return display[:maxLen-3] + "..."
	}

	first := parts[0]
	if first != "~" {
		first = "/" + first
	}
	truncated := first + "/.../" + strings.Join(parts[len(parts)-2:], "/")
	if len(truncated) <= maxLen {
		return truncated
	}
	return truncated[:maxLen-3] + "..."
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/grouping/ -v`
Expected: PASS — all 6 test functions pass (`ok  github.com/guhcostan/app-cleaner/internal/grouping`).

- [ ] **Step 5: Commit**

```bash
git add internal/grouping/grouping.go internal/grouping/grouping_test.go
git commit -m "feat(grouping): add directory grouping and display-path truncation

Ports grouping.ts + truncateDirectoryPath from the CLI: groups items by
parent directory, orders groups by largest single file, paginates with
per-directory expand overrides, and middle-elides header paths to 50
chars with ~ contraction.

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

---

### Task 20: Wails Bridge (`app.go` + `main.go`)

Rewrites the scaffolded `app.go`/`main.go` (created by the project-scaffolding task from the Wails `react-ts` template) into the real bridge: every bound method from the contract, single-flight mutex guards, goroutines streaming the contract's events, panic recovery at the bridge boundary, and cancellation via stored `context.CancelFunc`s. Engine packages stay Wails-free — only these two files import the Wails runtime. Selection resolution and backup routing/splitting are extracted as pure package-local functions so they are unit-testable without a Wails runtime.

**Files:**
- Modify: `app.go` (full rewrite of the template file)
- Modify: `main.go` (full rewrite of the template file)
- Create: `app_test.go`
- Generated (by `wails build`, committed): `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/models.ts`, `frontend/wailsjs/runtime/*` — later frontend tasks import these bindings.

**Interfaces:**
- Consumes (all from the canonical contract / earlier tasks):
  - `core`: `Categories`, `CategoriesInOrder()`, `CategoryID`, `Category`, `CleanableItem`, `ScanResult`, `ScanSummary`, `CleanResult`, `CleanSummary`, `ProgressFunc`
  - `config`: `Config`, `Load(path string) Config`, `Save(c Config, path string) error`, `DefaultPath(home string) string`
  - `scanners`: `Options{Roots, Cfg, Runner}`, `DefaultRoots()`, `ExecRunner`, `Get(id core.CategoryID) (Scanner, bool)`, `RunScans(ctx, ids, opts, concurrency, onResult) core.ScanSummary`, `Scanner.Clean(ctx, items, dryRun, progress) core.CleanResult`
  - `backup`: `NewManager(home string) *Manager`, `Manager.BackupItems(ctx, home, items, progress) BackupOutcome`, `Manager.List() []Info`, `Manager.Restore(sessionDir, home string) RestoreResult`, `Manager.CleanOld(retentionDays int) int`, `Manager.Delete(sessionDir string) error`
  - `uninstall`: `ListApps(ctx, appDirs, home) []AppInfo`, `IsAppRunning(appPath string) bool`, `Uninstall(ctx, apps, dryRun, progress) Summary`, `AppIcon(ctx, r Runner, appPath, cacheDir string) string` (uninstall's package-local `Runner` has the same method set as `maintenance.Runner`, so the bridge's `execRunner` satisfies both)
  - `maintenance`: `Result`, `Runner`, `Elevator`, `FlushDNS(ctx, e) Result`, `FreePurgeable(ctx, r, e) Result`, `ClearTMSnapshots(ctx, r, e, progress) Result`
  - `fda`: `Check(home string) *bool`, `SettingsURL`
  - `grouping`: `GroupItems(...)`, `DisplayRow` (Task 19)
  - Wails: `wails.Run`, `options.App`, `runtime.EventsEmit`, `runtime.ClipboardSetText`
- Produces:
  - All bound methods exactly as in the contract's Wails-bridge section (`GetCategories`, `StartScan`, `CancelScan`, `GetScanResult`, `GroupItems`, `StartClean`, `CancelClean`, `ListApps`, `StartUninstall`, `IsAppRunning`, `GetAppIcon`, `CleanOldBackups`, `RunMaintenance`, `StartTMSnapshotsClear`, `CancelMaintenance`, `GetConfig`, `SaveConfig`, `ListBackups`, `RestoreBackup`, `DeleteBackup`, `CheckFDA`, `OpenFDASettings`, `RevealInFinder`, `CopyPath`) plus `type CleanOptions struct { DryRun bool; Backup bool }` (JSON tags `dryRun`/`backup`).
  - Events with exact names/payload keys: `scan:progress`, `scan:done`, `clean:progress`, `clean:done`, `backup:progress`, `uninstall:progress`, `uninstall:done`, `maintenance:progress`, `maintenance:done`.
  - Regenerated `frontend/wailsjs` bindings consumed by all later frontend tasks.
- Package-local (defined here, not in the contract — do not use outside `package main`):
  - `func resolveSelection(lastScan map[core.CategoryID]core.ScanResult, selection map[string][]string) map[core.CategoryID][]core.CleanableItem` — pure; unknown categories and empty path lists are skipped; selected paths are matched against the stored scan items (scan-result order preserved); "select all" is simply the caller passing every item path.
  - `func splitByBackup(resolved map[core.CategoryID][]core.CleanableItem, notBackedUp []string) (moved, remaining map[core.CategoryID][]core.CleanableItem)` — pure; partitions the resolved selection by whether `backup.BackupItems` moved the item (`moved`, already off disk, credited to the clean summary) or refused it (`remaining`, still on disk, handed to `Scanner.Clean`).
  - `var neverBackup = map[core.CategoryID]bool{"homebrew": true, "docker": true}` and `func splitNeverBackup(resolved map[core.CategoryID][]core.CleanableItem) (backupable, direct map[core.CategoryID][]core.CleanableItem)` — pure; spec §8: these categories are cleaned by their own tool (`brew cleanup` / `docker system prune`), so their items are routed straight to the clean path and never enter `backupMgr.BackupItems`.
  - `type execRunner struct{}` implementing `maintenance.Runner` — and, by identical method set, `uninstall.Runner` for `GetAppIcon` — (`exec.CommandContext` + per-call timeout, stderr folded into the error) and `type osaElevator struct{}` implementing `maintenance.Elevator` (`/usr/bin/osascript -e 'do shell script "…" with administrator privileges'` with `\` and `"` escaped).
  - Extra `App` fields beyond the assigned set: `uninstalling bool` (single-flight guard for `StartUninstall`; the contract exposes no `CancelUninstall`, so uninstall runs on the app context), `lastApps []uninstall.AppInfo` (cache of the last `ListApps` result that `StartUninstall` resolves names against), and `iconCacheDir string` (`filepath.Join(os.TempDir(), "appcleaner-icons")`, created with `MkdirAll` in `startup`, passed to `uninstall.AppIcon` by `GetAppIcon`).

Behavioral notes locked here:
- `StartScan` while scanning returns the error string `a scan is already running`; `StartClean` while cleaning returns `a clean is already running`; `StartUninstall`/`StartTMSnapshotsClear` guard likewise.
- `StartClean` with `opts.Backup && !opts.DryRun`: back up first (`backup:progress` streamed), collect `NotBackedUp`, then per category run `Scanner.Clean` on the remaining (not-backed-up) items only; moved items are credited into that category's `CleanResult` (`CleanedItems` +1 and `FreedSpace` +Size each) so the summary reflects everything removed from its original location. Dry-run never backs up (nothing may touch disk). Per spec §8, `homebrew` and `docker` items are cleaned via `brew cleanup`/`docker system prune` and are NEVER backed up: `splitNeverBackup` routes them straight to the clean path BEFORE the backup pass, so they never enter `backupMgr.BackupItems` (and never appear in `notBackedUp`); their scanner's `Clean` handles them (prune/cleanup).
- `clean:done` carries `notBackedUp: string[]` as a TOP-LEVEL sibling of `summary` (never nested inside it), exactly as the contract's event table specifies.
- Every goroutine has panic recovery that emits the corresponding `*:done` event with an `error` key so the UI never hangs; done events carry `cancelled: ctx.Err() != nil`.
- `SaveConfig` writes with `config.Save` then re-loads via `config.Load` so out-of-range fields snap back to defaults exactly as they would on next launch.
- `startup` also runs backup retention cleanup (`CleanOld(cfg.BackupRetentionDays)`) in a recovered goroutine, per spec §8 ("cleanup runs on app launch"); the same sweep is exposed on demand as the bound method `CleanOldBackups() int`. `startup` additionally creates the icon cache dir `filepath.Join(os.TempDir(), "appcleaner-icons")` (`MkdirAll`) that `GetAppIcon` hands to `uninstall.AppIcon`.

- [ ] **Step 1: Write the failing test**

Create `app_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Precondition: `main.go` embeds `frontend/dist`. If it is missing (fresh clone), build the frontend once first: `cd frontend && npm install && npm run build && cd ..`

Run: `go test .`
Expected: FAIL — build errors `undefined: resolveSelection`, `undefined: splitByBackup`, `undefined: splitNeverBackup`, and `(&App{}).RunMaintenance undefined` (the template `app.go` only has `Greet`).

- [ ] **Step 3: Write the implementation**

Replace the entire contents of `app.go` with:

```go
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/guhcostan/app-cleaner/internal/backup"
	"github.com/guhcostan/app-cleaner/internal/config"
	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fda"
	"github.com/guhcostan/app-cleaner/internal/grouping"
	"github.com/guhcostan/app-cleaner/internal/maintenance"
	"github.com/guhcostan/app-cleaner/internal/scanners"
	"github.com/guhcostan/app-cleaner/internal/uninstall"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound bridge between the React frontend and the engine
// packages under internal/. It is the ONLY place (with main.go) that touches
// the Wails runtime.
type App struct {
	ctx context.Context

	mu          sync.Mutex
	scanCancel  context.CancelFunc
	cleanCancel context.CancelFunc
	maintCancel context.CancelFunc
	scanning    bool
	cleaning    bool
	maintaining bool
	uninstalling bool

	lastScan     map[core.CategoryID]core.ScanResult
	lastApps     []uninstall.AppInfo
	cfg          config.Config
	backupMgr    *backup.Manager
	home         string
	iconCacheDir string
}

// CleanOptions is the options payload for StartClean.
type CleanOptions struct {
	DryRun bool `json:"dryRun"`
	Backup bool `json:"backup"`
}

func NewApp() *App {
	return &App{lastScan: make(map[core.CategoryID]core.ScanResult)}
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
	if len(rawIDs) == 0 {
		for _, cat := range core.CategoriesInOrder() {
			ids = append(ids, cat.ID)
		}
	} else {
		for _, raw := range rawIDs {
			id := core.CategoryID(raw)
			if _, ok := scanners.Get(id); ok { // unknown ids silently skipped
				ids = append(ids, id)
			}
		}
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

// splitByBackup partitions the resolved selection after a backup pass: items
// listed in notBackedUp were NOT moved into the backup session (non-$HOME or
// EXDEV) and must be permanently deleted by their scanner; everything else was
// moved off disk by backup.BackupItems and only needs summary credit.
func splitByBackup(resolved map[core.CategoryID][]core.CleanableItem, notBackedUp []string) (moved, remaining map[core.CategoryID][]core.CleanableItem) {
	skip := make(map[string]bool, len(notBackedUp))
	for _, p := range notBackedUp {
		skip[p] = true
	}
	moved = make(map[core.CategoryID][]core.CleanableItem)
	remaining = make(map[core.CategoryID][]core.CleanableItem)
	for id, items := range resolved {
		for _, it := range items {
			if skip[it.Path] {
				remaining[id] = append(remaining[id], it)
			} else {
				moved[id] = append(moved[id], it)
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
		movedByCat, remaining = splitByBackup(backupable, outcome.NotBackedUp)
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
		res := core.CleanResult{Category: cat}
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

// StartUninstall uninstalls the named apps (matched against the last ListApps
// result) in a goroutine, streaming uninstall:progress / uninstall:done.
func (a *App) StartUninstall(names []string, dryRun bool) error {
	a.mu.Lock()
	if a.uninstalling {
		a.mu.Unlock()
		return errors.New("an uninstall is already running")
	}
	byName := make(map[string]uninstall.AppInfo, len(a.lastApps))
	for _, info := range a.lastApps {
		byName[info.Name] = info
	}
	var apps []uninstall.AppInfo
	for _, n := range names {
		if info, ok := byName[n]; ok {
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
// in startup. execRunner satisfies uninstall.Runner (same method set as
// maintenance.Runner).
func (a *App) GetAppIcon(path string) string {
	return uninstall.AppIcon(a.ctx, execRunner{}, path, a.iconCacheDir)
}

// ---------------------------------------------------------------------------
// Maintenance
// ---------------------------------------------------------------------------

// execRunner is the real maintenance.Runner — and, sharing the same method
// set, uninstall.Runner (used by GetAppIcon): absolute binary path + arg
// slice, never a shell.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w (stderr: %s)", bin, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// osaElevator is the real maintenance.Elevator: native macOS admin prompt via
// osascript's `do shell script ... with administrator privileges`.
type osaElevator struct{}

func (osaElevator) RunElevated(ctx context.Context, shellScript string) (string, error) {
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(shellScript)
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e",
		`do shell script "`+esc+`" with administrator privileges`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("osascript: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// RunMaintenance runs the short synchronous tasks ("dns" | "purge"). Time
// Machine snapshot clearing is long-running and uses StartTMSnapshotsClear.
func (a *App) RunMaintenance(task string) maintenance.Result {
	switch task {
	case "dns":
		return maintenance.FlushDNS(a.ctx, osaElevator{})
	case "purge":
		return maintenance.FreePurgeable(a.ctx, execRunner{}, osaElevator{})
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

	result := maintenance.ClearTMSnapshots(ctx, execRunner{}, osaElevator{},
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

func (a *App) DeleteBackup(path string) error {
	return a.backupMgr.Delete(path)
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
```

Replace the entire contents of `main.go` with:

```go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "App Cleaner",
		Width:     1150,
		Height:    740,
		MinWidth:  940,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.guhcostan.appcleaner",
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "App Cleaner",
				Message: "Clean and maintain your Mac. 100% offline.",
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test . && go vet ./...`
Expected: PASS — `ok  github.com/guhcostan/app-cleaner` and no vet diagnostics.

- [ ] **Step 5: Verify the full app builds and bindings regenerate**

Run: `wails build`
Expected: build completes with `Built '<repo>/build/bin/App Cleaner.app' ...` and regenerates the JS bindings. Then verify the bindings cover the bridge surface:

```bash
grep -c "export function" frontend/wailsjs/go/main/App.d.ts
```
Expected: `24` (one export per bound method: GetCategories, StartScan, CancelScan, GetScanResult, GroupItems, StartClean, CancelClean, ListApps, StartUninstall, IsAppRunning, GetAppIcon, CleanOldBackups, RunMaintenance, StartTMSnapshotsClear, CancelMaintenance, GetConfig, SaveConfig, ListBackups, RestoreBackup, DeleteBackup, CheckFDA, OpenFDASettings, RevealInFinder, CopyPath).

```bash
grep -q "StartClean" frontend/wailsjs/go/main/App.d.ts && grep -q "DisplayRow" frontend/wailsjs/go/models.ts && echo bindings-ok
```
Expected: prints `bindings-ok`. Later frontend tasks import from `frontend/wailsjs/go/main/App` and `frontend/wailsjs/go/models`; the generated bindings are committed — the repo `.gitignore` does not ignore `frontend/wailsjs`.

- [ ] **Step 6: Commit**

```bash
git add app.go app_test.go main.go frontend/wailsjs
git commit -m "feat(app): implement Wails bridge with scan/clean/uninstall/maintenance bindings and events

Binds every contract method on main.App (including GetAppIcon and
CleanOldBackups), streams scan/clean/backup/uninstall/maintenance
events, guards single-flight operations with a mutex, recovers panics
into *:done events, routes homebrew/docker items past the backup pass
(spec §8), and wires startup config load, icon-cache setup, and backup
retention cleanup. Regenerates frontend/wailsjs bindings consumed by
the frontend tasks.

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```
### Task 21: Frontend Foundation (Tailwind v4, Vitest, typed lib)

Depends on: Task 20 (Wails bridge) — `frontend/wailsjs/` generated bindings must exist. The `frontend/` directory is the Wails `react-ts` template created during project scaffolding.

**Files:**
- Modify (overwrite): `frontend/package.json`
- Modify (overwrite): `frontend/vite.config.ts`
- Modify (overwrite): `frontend/tsconfig.json`
- Modify (overwrite): `frontend/src/style.css`
- Modify (overwrite): `frontend/src/main.tsx`
- Modify (overwrite): `frontend/src/App.tsx` (temporary shell placeholder; Task 22 rewrites it)
- Delete: `frontend/tsconfig.node.json`, `frontend/src/App.css`, `frontend/src/assets/` (template demo)
- Create: `frontend/src/test/setup.ts`
- Create: `frontend/src/lib/format.ts`
- Create: `frontend/src/lib/types.ts`
- Test: `frontend/src/lib/format.test.ts`

**Interfaces:**
- Consumes: `frontend/wailsjs/` generated bindings + runtime (Task 20 — stubbed as `window.runtime` / `window.go` by the test setup); `internal/core.FormatSize` contract (mirrored, not imported).
- Produces (later frontend tasks rely on these exact names):
  - `lib/format.ts`: `export function formatSize(bytes: number): string`
  - `lib/types.ts`: `Category`, `CleanableItem`, `ScanResult`, `ScanSummary`, `CleanResult`, `CleanSummary`, `DisplayRow`, `AppInfo`, `BackupInfo`, `RestoreResult`, `MaintenanceResult`, `Config`, `ExtraPaths`, `RelatedPath` + event payload types `ScanProgressEvent`, `ScanDoneEvent`, `CleanProgressEvent`, `CleanDoneEvent`, `UninstallProgressEvent`, `UninstallDoneEvent`, `BackupProgressEvent`, `MaintenanceProgressEvent`, `MaintenanceDoneEvent` (field names = the contract's JSON tags / event payload keys)
  - `src/test/setup.ts`: installs `window.runtime` + `window.go.main.App` stubs so any module importing `wailsjs/**` loads (and can be called as a resolved-null promise) under jsdom; registers testing-library `cleanup` after each test
  - npm scripts: `npm run test` (= `vitest run`), `npm run typecheck` (= `tsc --noEmit`)

- [ ] **Step 1: Verify the generated bindings exist**

Run from repo root:

```bash
ls frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/runtime/runtime.d.ts
```

Expected: both paths print (no error). If missing, run `wails generate module` from the repo root first (requires the Go module from Tasks 1–20 to compile).

- [ ] **Step 2: Replace package.json and install dependencies**

Overwrite `frontend/package.json` with exactly (npm will re-add the dependency blocks during install):

```json
{
  "name": "frontend",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit",
    "test": "vitest run"
  }
}
```

Then run (the template ships Vite 3 / TS 4, which are too old for `@tailwindcss/vite` — this upgrades them):

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install react@^18.2.0 react-dom@^18.2.0 zustand lucide-react
npm install -D typescript@^5.7.0 vite@^6.0.0 @vitejs/plugin-react@^4.3.0 @types/react@^18.3.0 @types/react-dom@^18.3.0 tailwindcss@^4.0.0 @tailwindcss/vite@^4.0.0 vitest@^3.0.0 jsdom @testing-library/react@^16.2.0 @testing-library/dom@^10.4.0 @testing-library/jest-dom@^6.6.0
```

Expected: both installs exit 0; `frontend/package.json` now contains `dependencies` (react, react-dom, zustand, lucide-react) and the dev dependencies above.

- [ ] **Step 3: Replace the tooling config and template demo files**

Overwrite `frontend/vite.config.ts`:

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
});
```

Overwrite `frontend/tsconfig.json` (drops the template's `tsconfig.node.json` project reference; includes `wailsjs` so binding imports typecheck):

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "resolveJsonModule": true,
    "forceConsistentCasingInFileNames": true,
    "useDefineForClassFields": true,
    "noEmit": true
  },
  "include": ["src", "wailsjs"]
}
```

Delete the template demo files:

```bash
rm -f frontend/tsconfig.node.json frontend/src/App.css
rm -rf frontend/src/assets
```

Overwrite `frontend/src/style.css` (Tailwind v4 entry + macOS base styles):

```css
@import "tailwindcss";

html,
body,
#root {
  height: 100%;
}

body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Helvetica Neue",
    Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  cursor: default;
  user-select: none;
  -webkit-user-select: none;
}

input,
textarea {
  user-select: text;
  -webkit-user-select: text;
}
```

Overwrite `frontend/src/main.tsx`:

```tsx
import React from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';
import App from './App';

const container = document.getElementById('root');
const root = createRoot(container!);
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

Overwrite `frontend/src/App.tsx` with a minimal placeholder (the template version imports the deleted `App.css`/logo; Task 22 builds the real shell):

```tsx
function App() {
  return (
    <div className="flex h-full items-center justify-center bg-neutral-50 text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400">
      App Cleaner
    </div>
  );
}

export default App;
```

Create `frontend/src/test/setup.ts` (Wails stubs — every store/component test relies on this):

```ts
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

afterEach(() => cleanup());

// Minimal stubs for the Wails runtime + Go bindings so any module importing
// files under frontend/wailsjs/ can be loaded (and called) under jsdom.
type Listener = (...data: unknown[]) => void;
const listeners = new Map<string, Listener[]>();

(window as unknown as Record<string, unknown>).runtime = {
  EventsOnMultiple(eventName: string, callback: Listener, _maxCallbacks: number) {
    const arr = listeners.get(eventName) ?? [];
    arr.push(callback);
    listeners.set(eventName, arr);
    return () => listeners.delete(eventName);
  },
  EventsOff(eventName: string) {
    listeners.delete(eventName);
  },
  EventsEmit(eventName: string, ...data: unknown[]) {
    for (const cb of listeners.get(eventName) ?? []) cb(...data);
  },
};

(window as unknown as Record<string, unknown>).go = {
  main: {
    // Every bound method resolves to null; tests drive the stores through the
    // exported event handlers instead of real backend calls.
    App: new Proxy({}, { get: () => () => Promise.resolve(null) }),
  },
};
```

- [ ] **Step 4: Write the failing test**

Create `frontend/src/lib/format.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { formatSize } from './format';

describe('formatSize', () => {
  it('mirrors core.FormatSize exactly (base-1024, 0 decimals for bytes, 1 above)', () => {
    expect(formatSize(0)).toBe('0 B');
    expect(formatSize(512)).toBe('512 B');
    expect(formatSize(1023)).toBe('1023 B');
    expect(formatSize(1024)).toBe('1.0 KB');
    expect(formatSize(1536)).toBe('1.5 KB');
    expect(formatSize(524288000)).toBe('500.0 MB');
    expect(formatSize(1073741824)).toBe('1.0 GB');
    expect(formatSize(1099511627776)).toBe('1.0 TB');
  });

  it('clamps garbage inputs to "0 B"', () => {
    expect(formatSize(-42)).toBe('0 B');
    expect(formatSize(Number.NaN)).toBe('0 B');
  });
});
```

- [ ] **Step 5: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/format.test.ts`
Expected: FAIL — `Failed to resolve import "./format"` (module does not exist yet).

- [ ] **Step 6: Write minimal implementation**

Create `frontend/src/lib/format.ts`:

```ts
const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

/**
 * Mirrors internal/core FormatSize: base-1024; "512 B" (0 decimals),
 * "1.5 KB" / "2.0 GB" (exactly 1 decimal above bytes). Index clamped to TB.
 */
export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  let i = Math.floor(Math.log(bytes) / Math.log(1024));
  if (i < 0) i = 0;
  if (i > UNITS.length - 1) i = UNITS.length - 1;
  const value = bytes / Math.pow(1024, i);
  return `${value.toFixed(i > 0 ? 1 : 0)} ${UNITS[i]}`;
}
```

Create `frontend/src/lib/types.ts` (TS mirrors of the contract's JSON tags — do not rename fields):

```ts
export type SafetyLevel = 'safe' | 'moderate' | 'risky';
export type CategoryGroup =
  | 'System Junk'
  | 'Development'
  | 'Storage'
  | 'Browsers'
  | 'Large Files';

export interface Category {
  id: string;
  name: string;
  group: CategoryGroup;
  description: string;
  safetyLevel: SafetyLevel;
  safetyNote?: string;
  supportsFileSelection?: boolean;
}

export interface CleanableItem {
  path: string;
  size: number;
  name: string;
  isDirectory: boolean;
  modifiedAt?: string; // RFC3339 from Go *time.Time
}

export interface ScanResult {
  category: Category;
  items: CleanableItem[];
  totalSize: number;
  error?: string;
}

export interface ScanSummary {
  results: ScanResult[];
  totalSize: number;
  totalItems: number;
}

export interface CleanResult {
  category: Category;
  cleanedItems: number;
  freedSpace: number;
  errors: string[];
}

export interface CleanSummary {
  results: CleanResult[];
  totalFreedSpace: number;
  totalCleanedItems: number;
  totalErrors: number;
}

export interface DisplayRow {
  type: 'directory-header' | 'file' | 'expand-hint';
  directoryKey: string;
  displayName: string;
  path?: string;
  size?: number;
  name?: string;
  hiddenCount?: number;
  totalFilesInDir: number;
  selectable: boolean;
}

export interface RelatedPath {
  path: string;
  size: number;
}

export interface AppInfo {
  name: string;
  path: string;
  bundleId: string;
  appSize: number;
  relatedPaths: RelatedPath[];
  totalSize: number;
  running: boolean;
}

export interface BackupInfo {
  path: string;
  date: string; // RFC3339 from Go time.Time
  size: number;
}

export interface RestoreResult {
  restored: number;
  failed: number;
  errors: string[];
}

export interface MaintenanceResult {
  success: boolean;
  message: string;
  error?: string;
  requiresAdmin: boolean;
}

export interface ExtraPaths {
  nodeModules: string[];
  projects: string[];
}

export interface Config {
  downloadsDaysOld: number;
  largeFilesMinSize: number;
  backupByDefault: boolean;
  backupRetentionDays: number;
  concurrency: number;
  showRisky: boolean;
  keepLanguages: string[];
  extraPaths: ExtraPaths;
}

// ---- Wails event payloads (exact keys from the contract's Events section) ----

export interface ScanProgressEvent {
  completed: number;
  total: number;
  categoryId: string;
  totalSize: number;
  itemCount: number;
  error?: string;
}

export interface ScanDoneEvent {
  summary: ScanSummary;
  cancelled?: boolean;
  error?: string;
}

export interface CleanProgressEvent {
  current: number;
  total: number;
  categoryId: string;
  itemName: string;
}

/** clean:done payload — summary = core.CleanSummary JSON; notBackedUp is a TOP-LEVEL sibling of summary. */
export interface CleanDoneEvent {
  summary: CleanSummary;
  notBackedUp: string[];
  cancelled?: boolean;
  error?: string;
}

export interface UninstallProgressEvent {
  current: number;
  total: number;
  appName: string;
}

export interface UninstallDoneEvent {
  uninstalled: number;
  freedSpace: number;
  errors: string[];
  cancelled?: boolean;
  error?: string;
}

export interface BackupProgressEvent {
  current: number;
  total: number;
  itemName: string;
}

export interface MaintenanceProgressEvent {
  done: number;
  total: number;
  date: string;
  error?: string;
}

export interface MaintenanceDoneEvent {
  result: MaintenanceResult;
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/lib/format.test.ts`
Expected: PASS — 1 file, 2 tests.

Then run the full gate:

```bash
cd frontend && npm run typecheck   # exit 0, no output
cd frontend && npm run test        # PASS (format.test.ts)
cd frontend && npm run build       # tsc + vite build -> frontend/dist/ created
```

- [ ] **Step 8: Commit**

```bash
git add -A frontend
git commit -m "feat(frontend): tailwind v4 + vitest foundation with typed lib

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 22: Zustand Stores + App Shell (Sidebar, FirstRun, FDA gate)

Depends on: Task 21.

**Files:**
- Create: `frontend/src/stores/scanStore.ts`
- Create: `frontend/src/stores/uiStore.ts`
- Create: `frontend/src/stores/cleanStore.ts`
- Create: `frontend/src/components/Sidebar.tsx`
- Create: `frontend/src/views/FirstRun.tsx`
- Modify (overwrite): `frontend/src/App.tsx`
- Test: `frontend/src/stores/scanStore.test.ts`
- Test: `frontend/src/stores/uiStore.test.ts`

**Interfaces:**
- Consumes: `../../wailsjs/go/main/App` bindings `GetCategories`, `StartScan`, `CancelScan`, `StartClean`, `CancelClean`, `GetConfig`, `SaveConfig`, `CheckFDA`, `OpenFDASettings`; `../../wailsjs/runtime/runtime` `EventsOn`; `lib/types.ts` (Task 21).
- Produces (exact contract shapes; † = package-local additions later tasks use — verifier note):
  - `stores/scanStore.ts`: `useScanStore` — `{status: 'idle'|'scanning'|'done', progress: {completed,total}, results: Record<string, ScanResult>, totalSize, selected: Record<string, Set<string>|'all'>, toggleCategory, setItemSelection, startScan, cancelScan, reset}` plus †`categories: Category[]`, †`itemCounts: Record<string, number>`, †`scanError?: string`, †`scanCancelled?: boolean` (both stored from the `scan:done` event), †`loadCategories()`. Exports †`type Selection = Set<string> | 'all'`, `handleScanProgress(ev)`, `handleScanDone(ev)` (test seams, also the real event handlers), †`isCategorySelected(sel)`, †`selectionTotals(results, selected): {items, size}`, †`selectedPaths(results, selected): Record<string, string[]>` (wire shape for `StartClean`).
  - `stores/uiStore.ts`: `useUiStore` — `{view, activeCategoryId?, fda: boolean|null, config?: Config, setView(view, activeCategoryId?), loadConfig, saveConfig, refreshFda}`; exports †`type View`.
  - `stores/cleanStore.ts`: `useCleanStore` — `{status: 'idle'|'confirming'|'cleaning'|'done', progress, summary?, notBackedUp: string[], openConfirm, startClean, cancelClean, reset}` plus †`error?`, †`cancelled`; exports †`interface CleanOptions {dryRun: boolean; backup: boolean}`, `handleCleanProgress(ev)`, `handleCleanDone(ev)`.
  - `components/Sidebar.tsx` (default export), `views/FirstRun.tsx` (default export).
  - `App.tsx` renders Sidebar + main pane with per-view placeholders that Tasks 23/24 (and the later Uninstaller/Maintenance/Backups/Settings tasks) swap for real views.

- [ ] **Step 1: Write the failing store tests**

Create `frontend/src/stores/scanStore.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest';
import {
  handleScanDone,
  handleScanProgress,
  isCategorySelected,
  selectedPaths,
  useScanStore,
} from './scanStore';
import type { Category, ScanResult, ScanSummary } from '../lib/types';

function cat(id: string, safetyLevel: Category['safetyLevel']): Category {
  return { id, name: id, group: 'Storage', description: '', safetyLevel };
}

function result(category: Category, sizes: Record<string, number>): ScanResult {
  const items = Object.entries(sizes).map(([path, size]) => ({
    path,
    size,
    name: path.split('/').pop() ?? path,
    isDirectory: false,
  }));
  return { category, items, totalSize: items.reduce((n, i) => n + i.size, 0) };
}

beforeEach(() => {
  useScanStore.getState().reset();
});

describe('handleScanProgress', () => {
  it('fills results and counts incrementally as scan:progress lands', () => {
    useScanStore.setState({ status: 'scanning', categories: [cat('trash', 'safe')] });
    handleScanProgress({ completed: 1, total: 16, categoryId: 'trash', totalSize: 2048, itemCount: 3 });
    const s = useScanStore.getState();
    expect(s.progress).toEqual({ completed: 1, total: 16 });
    expect(s.results['trash'].category.id).toBe('trash');
    expect(s.results['trash'].totalSize).toBe(2048);
    expect(s.itemCounts['trash']).toBe(3);
    expect(s.totalSize).toBe(2048);

    handleScanProgress({ completed: 2, total: 16, categoryId: 'system-cache', totalSize: 1000, itemCount: 1, error: 'boom' });
    const s2 = useScanStore.getState();
    expect(s2.progress.completed).toBe(2);
    expect(s2.results['system-cache'].error).toBe('boom');
    expect(s2.totalSize).toBe(3048);
  });
});

describe('handleScanDone auto-selection', () => {
  const summary: ScanSummary = {
    results: [
      result(cat('trash', 'safe'), { '/t/a': 100 }),
      result(cat('system-cache', 'moderate'), { '/c/b': 200 }),
      result(cat('ios-backups', 'risky'), { '/b/c': 300 }),
      result(cat('downloads', 'risky'), {}),
      result(cat('browser-cache', 'safe'), {}), // empty -> not auto-selected
    ],
    totalSize: 600,
    totalItems: 3,
  };

  it("selects 'all' for safe+moderate categories with items — risky never", () => {
    handleScanDone({ summary });
    const s = useScanStore.getState();
    expect(s.status).toBe('done');
    expect(s.selected['trash']).toBe('all');
    expect(s.selected['system-cache']).toBe('all');
    expect(s.selected['ios-backups']).toBeUndefined();
    expect(s.selected['downloads']).toBeUndefined();
    expect(s.selected['browser-cache']).toBeUndefined();
    expect(s.totalSize).toBe(600);
    expect(s.itemCounts['ios-backups']).toBe(1);
  });

  it('toggleCategory turns a risky category on as all, then off again', () => {
    handleScanDone({ summary });
    useScanStore.getState().toggleCategory('ios-backups');
    expect(useScanStore.getState().selected['ios-backups']).toBe('all');
    useScanStore.getState().toggleCategory('ios-backups');
    expect(useScanStore.getState().selected['ios-backups']).toBeUndefined();
  });

  it('setItemSelection stores Sets and drops empty ones', () => {
    handleScanDone({ summary });
    useScanStore.getState().setItemSelection('trash', new Set(['/t/a']));
    expect(useScanStore.getState().selected['trash']).toEqual(new Set(['/t/a']));
    useScanStore.getState().setItemSelection('trash', new Set());
    expect(useScanStore.getState().selected['trash']).toBeUndefined();
    expect(isCategorySelected(new Set())).toBe(false);
    expect(isCategorySelected('all')).toBe(true);
  });

  it('stores error and cancelled from scan:done, and clears them on a clean finish', () => {
    handleScanDone({ summary, cancelled: true, error: 'scan interrupted' });
    let s = useScanStore.getState();
    expect(s.status).toBe('done');
    expect(s.scanCancelled).toBe(true);
    expect(s.scanError).toBe('scan interrupted');

    handleScanDone({ summary });
    s = useScanStore.getState();
    expect(s.scanCancelled).toBe(false);
    expect(s.scanError).toBeUndefined();
  });
});

describe('selectedPaths', () => {
  it("maps 'all' and Set selections to the StartClean wire shape", () => {
    const r = {
      trash: result(cat('trash', 'safe'), { '/t/a': 1, '/t/b': 2 }),
      downloads: result(cat('downloads', 'risky'), { '/d/x': 3, '/d/y': 4 }),
    };
    const sel = { trash: 'all' as const, downloads: new Set(['/d/y']) };
    expect(selectedPaths(r, sel)).toEqual({ trash: ['/t/a', '/t/b'], downloads: ['/d/y'] });
  });
});
```

Create `frontend/src/stores/uiStore.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest';
import { useUiStore } from './uiStore';

beforeEach(() => {
  useUiStore.setState({ view: 'smart-scan', activeCategoryId: undefined, fda: null, config: undefined });
});

describe('uiStore view transitions', () => {
  it('starts on smart-scan with unknown FDA', () => {
    expect(useUiStore.getState().view).toBe('smart-scan');
    expect(useUiStore.getState().fda).toBeNull();
  });

  it('setView switches view and carries the active category id', () => {
    useUiStore.getState().setView('category', 'trash');
    expect(useUiStore.getState().view).toBe('category');
    expect(useUiStore.getState().activeCategoryId).toBe('trash');
  });

  it('setView without a category clears activeCategoryId', () => {
    useUiStore.getState().setView('category', 'trash');
    useUiStore.getState().setView('settings');
    expect(useUiStore.getState().view).toBe('settings');
    expect(useUiStore.getState().activeCategoryId).toBeUndefined();
  });

  it('can enter and leave first-run', () => {
    useUiStore.getState().setView('first-run');
    expect(useUiStore.getState().view).toBe('first-run');
    useUiStore.getState().setView('smart-scan');
    expect(useUiStore.getState().view).toBe('smart-scan');
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/stores`
Expected: FAIL — `Failed to resolve import "./scanStore"` and `"./uiStore"` (stores do not exist yet).

- [ ] **Step 3: Write the stores**

Create `frontend/src/stores/scanStore.ts`:

```ts
import { create } from 'zustand';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { CancelScan, GetCategories, StartScan } from '../../wailsjs/go/main/App';
import type { Category, ScanDoneEvent, ScanProgressEvent, ScanResult } from '../lib/types';

/** Per-category selection: 'all' = every item, Set = explicit item paths. */
export type Selection = Set<string> | 'all';

export interface ScanState {
  status: 'idle' | 'scanning' | 'done';
  progress: { completed: number; total: number };
  categories: Category[];
  results: Record<string, ScanResult>;
  itemCounts: Record<string, number>;
  totalSize: number;
  selected: Record<string, Selection>;
  scanError?: string;
  scanCancelled?: boolean;
  loadCategories: () => Promise<void>;
  toggleCategory: (id: string) => void;
  setItemSelection: (id: string, selection: Selection) => void;
  startScan: (ids?: string[]) => Promise<void>;
  cancelScan: () => void;
  reset: () => void;
}

const initialState = {
  status: 'idle' as const,
  progress: { completed: 0, total: 0 },
  categories: [] as Category[],
  results: {} as Record<string, ScanResult>,
  itemCounts: {} as Record<string, number>,
  totalSize: 0,
  selected: {} as Record<string, Selection>,
  scanError: undefined as string | undefined,
  scanCancelled: false as boolean | undefined,
};

/** True when the category has an active selection ('all' or a non-empty Set). */
export function isCategorySelected(sel: Selection | undefined): boolean {
  return sel === 'all' || (sel instanceof Set && sel.size > 0);
}

/** Item count + byte size of the current selection across all categories. */
export function selectionTotals(
  results: Record<string, ScanResult>,
  selected: Record<string, Selection>,
): { items: number; size: number } {
  let items = 0;
  let size = 0;
  for (const [id, sel] of Object.entries(selected)) {
    const result = results[id];
    if (!result || !isCategorySelected(sel)) continue;
    for (const item of result.items ?? []) {
      if (sel === 'all' || (sel instanceof Set && sel.has(item.path))) {
        items += 1;
        size += item.size;
      }
    }
  }
  return { items, size };
}

/** Wire shape for App.StartClean: categoryID -> selected item paths. */
export function selectedPaths(
  results: Record<string, ScanResult>,
  selected: Record<string, Selection>,
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const [id, sel] of Object.entries(selected)) {
    const result = results[id];
    if (!result || !isCategorySelected(sel)) continue;
    const paths = (result.items ?? [])
      .filter((it) => sel === 'all' || (sel instanceof Set && sel.has(it.path)))
      .map((it) => it.path);
    if (paths.length > 0) out[id] = paths;
  }
  return out;
}

export const useScanStore = create<ScanState>((set, get) => ({
  ...initialState,

  loadCategories: async () => {
    if (get().categories.length > 0) return;
    const cats = ((await GetCategories()) ?? []) as Category[];
    set({ categories: cats });
  },

  toggleCategory: (id) =>
    set((s) => {
      const next = { ...s.selected };
      if (isCategorySelected(next[id])) delete next[id];
      else next[id] = 'all';
      return { selected: next };
    }),

  setItemSelection: (id, selection) =>
    set((s) => {
      const next = { ...s.selected };
      if (selection !== 'all' && selection.size === 0) delete next[id];
      else next[id] = selection;
      return { selected: next };
    }),

  startScan: async (ids = []) => {
    await get().loadCategories();
    const total = ids.length > 0 ? ids.length : get().categories.length;
    set({
      status: 'scanning',
      progress: { completed: 0, total },
      results: {},
      itemCounts: {},
      selected: {},
      totalSize: 0,
      scanError: undefined,
      scanCancelled: false,
    });
    try {
      await StartScan(ids);
    } catch {
      set({ status: 'idle' }); // e.g. a scan is already running
    }
  },

  cancelScan: () => {
    void CancelScan();
  },

  reset: () => set({ ...initialState }),
}));

/** Exported for tests: applied on every `scan:progress` event. */
export function handleScanProgress(ev: ScanProgressEvent): void {
  useScanStore.setState((s) => {
    const category: Category =
      s.results[ev.categoryId]?.category ??
      s.categories.find((c) => c.id === ev.categoryId) ?? {
        id: ev.categoryId,
        name: ev.categoryId,
        group: 'System Junk',
        description: '',
        safetyLevel: 'moderate',
      };
    const results: Record<string, ScanResult> = {
      ...s.results,
      [ev.categoryId]: {
        category,
        items: s.results[ev.categoryId]?.items ?? [],
        totalSize: ev.totalSize,
        error: ev.error || undefined,
      },
    };
    return {
      progress: { completed: ev.completed, total: ev.total },
      results,
      itemCounts: { ...s.itemCounts, [ev.categoryId]: ev.itemCount },
      totalSize: Object.values(results).reduce((n, r) => n + r.totalSize, 0),
    };
  });
}

/** Exported for tests: applied on `scan:done`. Auto-selects safe+moderate ('all'); risky never. Also stores the event's error/cancelled flags. */
export function handleScanDone(ev: ScanDoneEvent): void {
  const results: Record<string, ScanResult> = {};
  const itemCounts: Record<string, number> = {};
  const selected: Record<string, Selection> = {};
  for (const r of ev.summary?.results ?? []) {
    const items = r.items ?? [];
    results[r.category.id] = { ...r, items };
    itemCounts[r.category.id] = items.length;
    if (r.category.safetyLevel !== 'risky' && items.length > 0 && !r.error) {
      selected[r.category.id] = 'all';
    }
  }
  useScanStore.setState({
    status: 'done',
    results,
    itemCounts,
    selected,
    totalSize: ev.summary?.totalSize ?? 0,
    scanError: ev.error || undefined,
    scanCancelled: ev.cancelled === true,
  });
}

EventsOn('scan:progress', handleScanProgress as (...data: any) => void);
EventsOn('scan:done', handleScanDone as (...data: any) => void);
```

Create `frontend/src/stores/uiStore.ts`:

```ts
import { create } from 'zustand';
import { CheckFDA, GetConfig, SaveConfig } from '../../wailsjs/go/main/App';
import type { Config } from '../lib/types';

export type View =
  | 'smart-scan'
  | 'category'
  | 'uninstaller'
  | 'maintenance'
  | 'backups'
  | 'settings'
  | 'first-run';

export interface UiState {
  view: View;
  activeCategoryId?: string;
  fda: boolean | null;
  config?: Config;
  setView: (view: View, activeCategoryId?: string) => void;
  loadConfig: () => Promise<void>;
  saveConfig: (c: Config) => Promise<void>;
  refreshFda: () => Promise<boolean | null>;
}

export const useUiStore = create<UiState>((set) => ({
  view: 'smart-scan',
  activeCategoryId: undefined,
  fda: null,
  config: undefined,

  setView: (view, activeCategoryId) => set({ view, activeCategoryId }),

  loadConfig: async () => {
    const config = (await GetConfig()) as Config | null;
    if (config) set({ config });
  },

  saveConfig: async (c) => {
    await SaveConfig(c);
    set({ config: c });
  },

  refreshFda: async () => {
    const fda = (await CheckFDA()) as boolean | null;
    set({ fda });
    return fda;
  },
}));
```

Create `frontend/src/stores/cleanStore.ts`:

```ts
import { create } from 'zustand';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { CancelClean, StartClean } from '../../wailsjs/go/main/App';
import { selectedPaths, useScanStore } from './scanStore';
import type { CleanDoneEvent, CleanProgressEvent, CleanSummary } from '../lib/types';

/** Mirrors main.CleanOptions JSON tags. */
export interface CleanOptions {
  dryRun: boolean;
  backup: boolean;
}

export interface CleanState {
  status: 'idle' | 'confirming' | 'cleaning' | 'done';
  progress: { current: number; total: number; categoryId: string; itemName: string };
  summary?: CleanSummary;
  notBackedUp: string[];
  error?: string;
  cancelled: boolean;
  openConfirm: () => void;
  startClean: (opts: CleanOptions) => Promise<void>;
  cancelClean: () => void;
  reset: () => void;
}

const initialState = {
  status: 'idle' as const,
  progress: { current: 0, total: 0, categoryId: '', itemName: '' },
  summary: undefined as CleanSummary | undefined,
  notBackedUp: [] as string[],
  error: undefined as string | undefined,
  cancelled: false,
};

export const useCleanStore = create<CleanState>((set) => ({
  ...initialState,

  openConfirm: () => set({ status: 'confirming' }),

  startClean: async (opts) => {
    const { results, selected } = useScanStore.getState();
    const selection = selectedPaths(results, selected);
    set({
      status: 'cleaning',
      progress: { current: 0, total: 0, categoryId: '', itemName: '' },
      summary: undefined,
      notBackedUp: [],
      error: undefined,
      cancelled: false,
    });
    try {
      await StartClean(selection, opts);
    } catch (e) {
      set({ status: 'done', error: String(e) });
    }
  },

  cancelClean: () => {
    void CancelClean();
  },

  reset: () => set({ ...initialState }),
}));

/** Exported for tests: applied on every `clean:progress` event. */
export function handleCleanProgress(ev: CleanProgressEvent): void {
  useCleanStore.setState({
    progress: {
      current: ev.current,
      total: ev.total,
      categoryId: ev.categoryId,
      itemName: ev.itemName,
    },
  });
}

/** Exported for tests: applied on `clean:done`. */
export function handleCleanDone(ev: CleanDoneEvent): void {
  useCleanStore.setState({
    status: 'done',
    summary: ev.summary,
    notBackedUp: ev.notBackedUp ?? [],
    cancelled: ev.cancelled === true,
    error: ev.error || undefined,
  });
}

EventsOn('clean:progress', handleCleanProgress as (...data: any) => void);
EventsOn('clean:done', handleCleanDone as (...data: any) => void);
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/stores`
Expected: PASS — 2 files, 10 tests.

- [ ] **Step 5: Build the app shell (Sidebar, FirstRun, App)**

Create `frontend/src/components/Sidebar.tsx`:

```tsx
import { Archive, Settings, ShieldAlert, Sparkles, Trash2, Wrench } from 'lucide-react';
import { useUiStore, type View } from '../stores/uiStore';

const NAV: Array<{ view: View; label: string; Icon: typeof Sparkles }> = [
  { view: 'smart-scan', label: 'Smart Scan', Icon: Sparkles },
  { view: 'uninstaller', label: 'Uninstaller', Icon: Trash2 },
  { view: 'maintenance', label: 'Maintenance', Icon: Wrench },
  { view: 'backups', label: 'Backups', Icon: Archive },
  { view: 'settings', label: 'Settings', Icon: Settings },
];

export default function Sidebar() {
  const view = useUiStore((s) => s.view);
  const fda = useUiStore((s) => s.fda);
  const setView = useUiStore((s) => s.setView);

  return (
    <aside className="flex w-52 shrink-0 flex-col border-r border-neutral-200 bg-neutral-50 pt-10 dark:border-neutral-800 dark:bg-neutral-950">
      <div className="px-4 pb-4 text-sm font-semibold text-neutral-500 dark:text-neutral-400">
        App Cleaner
      </div>
      <nav className="flex flex-col gap-1 px-2">
        {NAV.map(({ view: v, label, Icon }) => {
          const active = view === v || (v === 'smart-scan' && view === 'category');
          return (
            <button
              key={v}
              type="button"
              onClick={() => setView(v)}
              className={`flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
                active
                  ? 'bg-blue-600 text-white'
                  : 'text-neutral-700 hover:bg-neutral-200 dark:text-neutral-300 dark:hover:bg-neutral-800'
              }`}
            >
              <Icon size={16} />
              <span className="flex-1">{label}</span>
            </button>
          );
        })}
      </nav>
      {fda === false && (
        <button
          type="button"
          onClick={() => setView('first-run')}
          className="mx-2 mt-auto mb-3 flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs text-amber-600 hover:bg-neutral-200 dark:text-amber-400 dark:hover:bg-neutral-800"
        >
          <span className="h-2 w-2 shrink-0 rounded-full bg-amber-500" />
          <span className="flex-1">Limited disk access</span>
          <ShieldAlert size={14} />
        </button>
      )}
    </aside>
  );
}
```

Create `frontend/src/views/FirstRun.tsx`:

```tsx
import { useEffect } from 'react';
import { HardDrive, Mail, ShieldCheck, Trash2 } from 'lucide-react';
import { OpenFDASettings } from '../../wailsjs/go/main/App';
import { useUiStore } from '../stores/uiStore';

export default function FirstRun() {
  const fda = useUiStore((s) => s.fda);
  const setView = useUiStore((s) => s.setView);
  const refreshFda = useUiStore((s) => s.refreshFda);

  // Re-check whenever the window regains focus (user returning from System Settings).
  useEffect(() => {
    const onFocus = () => void refreshFda();
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, [refreshFda]);

  // Granted while on this screen -> continue automatically.
  useEffect(() => {
    if (fda === true) setView('smart-scan');
  }, [fda, setView]);

  return (
    <div className="flex h-full flex-col items-center justify-center gap-6 px-10 text-center">
      <ShieldCheck size={56} className="text-blue-600 dark:text-blue-400" />
      <h1 className="text-2xl font-bold text-neutral-900 dark:text-neutral-100">
        Grant Full Disk Access
      </h1>
      <p className="max-w-md text-sm text-neutral-600 dark:text-neutral-400">
        App Cleaner needs Full Disk Access to scan everything it can clean. Without it, some
        locations cannot be read:
      </p>
      <ul className="flex gap-6 text-sm text-neutral-700 dark:text-neutral-300">
        <li className="flex items-center gap-2">
          <Trash2 size={16} /> Trash
        </li>
        <li className="flex items-center gap-2">
          <HardDrive size={16} /> Safari cache
        </li>
        <li className="flex items-center gap-2">
          <Mail size={16} /> Mail attachments
        </li>
      </ul>
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => OpenFDASettings()}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          Open System Settings
        </button>
        <button
          type="button"
          onClick={() => void refreshFda()}
          className="rounded-md border border-neutral-300 px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
        >
          Re-check
        </button>
        <button
          type="button"
          onClick={() => setView('smart-scan')}
          className="px-2 py-2 text-sm text-neutral-500 hover:text-neutral-700 dark:text-neutral-400 dark:hover:text-neutral-200"
        >
          Continue without
        </button>
      </div>
      <p className="text-xs text-neutral-400 dark:text-neutral-500">
        System Settings → Privacy &amp; Security → Full Disk Access → enable App Cleaner, then
        return here.
      </p>
    </div>
  );
}
```

Overwrite `frontend/src/App.tsx`:

```tsx
import { useEffect } from 'react';
import { CheckFDA } from '../wailsjs/go/main/App';
import Sidebar from './components/Sidebar';
import FirstRun from './views/FirstRun';
import { useUiStore } from './stores/uiStore';

// Placeholder panes — swapped for real views by later tasks:
// SmartScan (Task 23), CategoryDetail (Task 24), Uninstaller/Maintenance/Backups/Settings (later tasks).
function Placeholder({ title }: { title: string }) {
  return (
    <div className="flex h-full items-center justify-center text-neutral-400 dark:text-neutral-500">
      {title}
    </div>
  );
}

function App() {
  const view = useUiStore((s) => s.view);

  useEffect(() => {
    void useUiStore.getState().loadConfig();
    // First-run gate (spec §12): shown unless FDA is confirmed granted —
    // false (denied) AND null (unknown) both land on the permission screen.
    // "Continue without" on that screen still lets the user proceed.
    void CheckFDA().then((v) => {
      const fda = v as boolean | null;
      useUiStore.setState({ fda });
      if (fda !== true) useUiStore.getState().setView('first-run');
    });
  }, []);

  if (view === 'first-run') {
    return (
      <div className="h-full bg-white dark:bg-neutral-900">
        <FirstRun />
      </div>
    );
  }

  return (
    <div className="flex h-full bg-white text-neutral-900 dark:bg-neutral-900 dark:text-neutral-100">
      <Sidebar />
      <main className="min-w-0 flex-1 overflow-y-auto">
        {view === 'smart-scan' && <Placeholder title="Smart Scan" />}
        {view === 'category' && <Placeholder title="Category" />}
        {view === 'uninstaller' && <Placeholder title="Uninstaller" />}
        {view === 'maintenance' && <Placeholder title="Maintenance" />}
        {view === 'backups' && <Placeholder title="Backups" />}
        {view === 'settings' && <Placeholder title="Settings" />}
      </main>
    </div>
  );
}

export default App;
```

- [ ] **Step 6: Verify the full gate**

```bash
cd frontend && npm run test        # PASS — format + scanStore + uiStore suites
cd frontend && npm run typecheck   # exit 0
cd frontend && npm run build       # vite build succeeds
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/stores frontend/src/components/Sidebar.tsx frontend/src/views/FirstRun.tsx frontend/src/App.tsx
git commit -m "feat(frontend): zustand stores and app shell with first-run FDA gate

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 23: SmartScan View (hero → scanning → grouped results)

Depends on: Tasks 21–22.

**Files:**
- Create: `frontend/src/components/SafetyBadge.tsx`
- Create: `frontend/src/components/SizeBar.tsx`
- Create: `frontend/src/components/CategoryCard.tsx`
- Create: `frontend/src/components/EmptyState.tsx`
- Create: `frontend/src/views/SmartScan.tsx`
- Modify: `frontend/src/App.tsx` (swap the Smart Scan placeholder)
- Test: `frontend/src/views/SmartScan.test.tsx`

**Interfaces:**
- Consumes: `useScanStore` (incl. †`scanError` / †`scanCancelled`), `isCategorySelected`, `selectionTotals` (Task 22); `useUiStore` (`setView('category', id)`, `config?.showRisky`); `useCleanStore.openConfirm` (Task 22); `formatSize` (Task 21).
- Produces (later tasks / verifier note — all package-local):
  - `components/SafetyBadge.tsx`: `default SafetyBadge({ level }: { level: SafetyLevel })` — safe=green, moderate=amber, risky=red (reused by Task 24 and the confirm modal task)
  - `components/SizeBar.tsx`: `default SizeBar({ size, maxSize }: { size: number; maxSize: number })`
  - `components/CategoryCard.tsx`: `default CategoryCard({ result, itemCount, selected, maxSize, onToggle, onOpen })`
  - `components/EmptyState.tsx`: `default EmptyState({ title, subtitle?, icon? })` (reused by Task 24 and later views)
  - `views/SmartScan.tsx`: `default SmartScan()` + named export `groupCategories(categories: Category[]): Array<{ group: string; categories: Category[] }>` (non-risky only, group order = first appearance); results state renders a dismissible warning banner from †`scanError` and a "Scan cancelled — partial results" note from †`scanCancelled`

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/views/SmartScan.test.tsx`:

```tsx
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import SmartScan, { groupCategories } from './SmartScan';
import { selectionTotals, useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';
import type { Category, Config, ScanResult } from '../lib/types';

function cat(
  id: string,
  name: string,
  safetyLevel: Category['safetyLevel'],
  group: Category['group'] = 'Storage',
): Category {
  return { id, name, group, description: '', safetyLevel };
}

function result(category: Category, sizes: Record<string, number>): ScanResult {
  const items = Object.entries(sizes).map(([path, size]) => ({
    path,
    size,
    name: path,
    isDirectory: false,
  }));
  return { category, items, totalSize: items.reduce((n, i) => n + i.size, 0) };
}

const CONFIG_SHOW_RISKY: Config = {
  downloadsDaysOld: 30,
  largeFilesMinSize: 524288000,
  backupByDefault: true,
  backupRetentionDays: 7,
  concurrency: 4,
  showRisky: true,
  keepLanguages: [],
  extraPaths: { nodeModules: [], projects: [] },
};

describe('selectionTotals mixed selection math', () => {
  it("counts 'all' categories fully and Set categories per path", () => {
    const trash = cat('trash', 'Trash', 'safe');
    const downloads = cat('downloads', 'Old Downloads', 'risky');
    const results = {
      trash: result(trash, { '/t/a': 100, '/t/b': 50 }),
      downloads: result(downloads, { '/d/x': 1000, '/d/y': 200 }),
    };
    const selected = { trash: 'all' as const, downloads: new Set(['/d/y']) };
    expect(selectionTotals(results, selected)).toEqual({ items: 3, size: 350 });
  });

  it('ignores empty Sets and categories without results', () => {
    const trash = cat('trash', 'Trash', 'safe');
    const results = { trash: result(trash, { '/t/a': 100 }) };
    expect(selectionTotals(results, { trash: new Set<string>(), ghost: 'all' })).toEqual({
      items: 0,
      size: 0,
    });
  });
});

describe('groupCategories', () => {
  it('buckets non-risky categories by group in first-appearance order', () => {
    const cats = [
      cat('system-cache', 'User Cache Files', 'moderate', 'System Junk'),
      cat('trash', 'Trash', 'safe', 'Storage'),
      cat('downloads', 'Old Downloads', 'risky', 'Storage'),
      cat('browser-cache', 'Browser Cache', 'safe', 'Browsers'),
      cat('launch-agents', 'Orphaned Launch Agents', 'moderate', 'System Junk'),
    ];
    const groups = groupCategories(cats);
    expect(groups.map((g) => g.group)).toEqual(['System Junk', 'Storage', 'Browsers']);
    expect(groups[0].categories.map((c) => c.id)).toEqual(['system-cache', 'launch-agents']);
    expect(groups[1].categories.map((c) => c.id)).toEqual(['trash']); // risky excluded
  });
});

describe('risky group in results state', () => {
  beforeEach(() => {
    useScanStore.getState().reset();
    useUiStore.setState({ view: 'smart-scan', activeCategoryId: undefined, config: undefined });
    const safe = cat('trash', 'Trash', 'safe');
    const risky = cat('ios-backups', 'iOS Backups', 'risky');
    useScanStore.setState({
      status: 'done',
      categories: [safe, risky],
      results: {
        trash: result(safe, { '/t/a': 10 }),
        'ios-backups': result(risky, { '/b/x': 99 }),
      },
      itemCounts: { trash: 1, 'ios-backups': 1 },
      totalSize: 109,
      selected: { trash: 'all' },
    });
  });

  it('is collapsed by default: risky card hidden until the chevron is clicked', () => {
    render(<SmartScan />);
    expect(screen.getByText('Trash')).toBeInTheDocument();
    expect(screen.queryByText('iOS Backups')).toBeNull();
    fireEvent.click(screen.getByText(/Risky \(1\)/));
    expect(screen.getByText('iOS Backups')).toBeInTheDocument();
  });

  it('respects config.showRisky for the initial expanded state', () => {
    useUiStore.setState({ config: CONFIG_SHOW_RISKY });
    render(<SmartScan />);
    expect(screen.getByText('iOS Backups')).toBeInTheDocument();
  });

  it('footer shows the selected totals and enables Clean', () => {
    render(<SmartScan />);
    expect(screen.getByText('1 items · 10 B selected')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Clean' })).toBeEnabled();
  });

  it('shows a dismissible scan error banner and a cancelled note', () => {
    useScanStore.setState({ scanError: 'system-cache: permission denied', scanCancelled: true });
    render(<SmartScan />);
    expect(screen.getByText('system-cache: permission denied')).toBeInTheDocument();
    expect(screen.getByText('Scan cancelled — partial results')).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Dismiss scan error'));
    expect(screen.queryByText('system-cache: permission denied')).toBeNull();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/views/SmartScan.test.tsx`
Expected: FAIL — `Failed to resolve import "./SmartScan"` (view does not exist yet).

- [ ] **Step 3: Write the components and view**

Create `frontend/src/components/SafetyBadge.tsx`:

```tsx
import type { SafetyLevel } from '../lib/types';

const STYLES: Record<SafetyLevel, string> = {
  safe: 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-400',
  moderate: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400',
  risky: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-400',
};

const LABELS: Record<SafetyLevel, string> = {
  safe: 'Safe',
  moderate: 'Moderate',
  risky: 'Risky',
};

export default function SafetyBadge({ level }: { level: SafetyLevel }) {
  return (
    <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${STYLES[level]}`}>
      {LABELS[level]}
    </span>
  );
}
```

Create `frontend/src/components/SizeBar.tsx`:

```tsx
interface Props {
  size: number;
  maxSize: number; // largest category size on screen; bar width is proportional
}

export default function SizeBar({ size, maxSize }: Props) {
  const pct = maxSize > 0 ? Math.max((size / maxSize) * 100, size > 0 ? 2 : 0) : 0;
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-neutral-200 dark:bg-neutral-800">
      <div
        className="h-full rounded-full bg-blue-500 dark:bg-blue-400"
        style={{ width: `${Math.min(pct, 100)}%` }}
      />
    </div>
  );
}
```

Create `frontend/src/components/EmptyState.tsx`:

```tsx
import type { LucideIcon } from 'lucide-react';
import { Sparkles } from 'lucide-react';

interface Props {
  title: string;
  subtitle?: string;
  icon?: LucideIcon;
}

export default function EmptyState({ title, subtitle, icon: Icon = Sparkles }: Props) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
      <Icon size={48} className="text-green-500" />
      <div className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">{title}</div>
      {subtitle && (
        <div className="text-sm text-neutral-500 dark:text-neutral-400">{subtitle}</div>
      )}
    </div>
  );
}
```

Create `frontend/src/components/CategoryCard.tsx`:

```tsx
import { ChevronRight } from 'lucide-react';
import SafetyBadge from './SafetyBadge';
import SizeBar from './SizeBar';
import { formatSize } from '../lib/format';
import type { ScanResult } from '../lib/types';

interface Props {
  result: ScanResult;
  itemCount: number;
  selected: boolean;
  maxSize: number;
  onToggle: () => void;
  onOpen: () => void; // click-through to the CategoryDetail view
}

export default function CategoryCard({
  result,
  itemCount,
  selected,
  maxSize,
  onToggle,
  onOpen,
}: Props) {
  const { category } = result;
  return (
    <div className="flex items-center gap-3 rounded-lg border border-neutral-200 bg-white px-3 py-2.5 hover:border-neutral-300 dark:border-neutral-800 dark:bg-neutral-900 dark:hover:border-neutral-700">
      <input
        type="checkbox"
        aria-label={`Select ${category.name}`}
        checked={selected}
        onChange={onToggle}
        className="h-4 w-4 accent-blue-600"
      />
      <button
        type="button"
        onClick={onOpen}
        className="flex min-w-0 flex-1 items-center gap-3 text-left"
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium text-neutral-900 dark:text-neutral-100">
              {category.name}
            </span>
            <SafetyBadge level={category.safetyLevel} />
          </div>
          <div className="mt-1 flex items-center gap-2">
            <div className="w-40 shrink-0">
              <SizeBar size={result.totalSize} maxSize={maxSize} />
            </div>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">
              {itemCount} items · {formatSize(result.totalSize)}
            </span>
          </div>
          {result.error && (
            <div className="mt-1 text-xs text-amber-600 dark:text-amber-400">{result.error}</div>
          )}
        </div>
        <ChevronRight size={16} className="shrink-0 text-neutral-400" />
      </button>
    </div>
  );
}
```

Create `frontend/src/views/SmartScan.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronRight, Loader2, Search, X } from 'lucide-react';
import CategoryCard from '../components/CategoryCard';
import EmptyState from '../components/EmptyState';
import { formatSize } from '../lib/format';
import type { Category } from '../lib/types';
import { useCleanStore } from '../stores/cleanStore';
import { isCategorySelected, selectionTotals, useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';

/** Non-risky categories bucketed by CategoryGroup; group order = first appearance in `categories` (spec §5 order via GetCategories). */
export function groupCategories(
  categories: Category[],
): Array<{ group: string; categories: Category[] }> {
  const out: Array<{ group: string; categories: Category[] }> = [];
  const index = new Map<string, number>();
  for (const c of categories) {
    if (c.safetyLevel === 'risky') continue;
    let i = index.get(c.group);
    if (i === undefined) {
      i = out.length;
      index.set(c.group, i);
      out.push({ group: c.group, categories: [] });
    }
    out[i].categories.push(c);
  }
  return out;
}

export default function SmartScan() {
  const status = useScanStore((s) => s.status);
  const progress = useScanStore((s) => s.progress);
  const categories = useScanStore((s) => s.categories);
  const results = useScanStore((s) => s.results);
  const itemCounts = useScanStore((s) => s.itemCounts);
  const totalSize = useScanStore((s) => s.totalSize);
  const selected = useScanStore((s) => s.selected);
  const scanError = useScanStore((s) => s.scanError);
  const scanCancelled = useScanStore((s) => s.scanCancelled);
  const config = useUiStore((s) => s.config);
  // null = untouched -> fall back to config.showRisky for the initial expanded state.
  const [riskyExpanded, setRiskyExpanded] = useState<boolean | null>(null);
  const riskyOpen = riskyExpanded ?? config?.showRisky ?? false;
  const [errorDismissed, setErrorDismissed] = useState(false);

  useEffect(() => {
    void useScanStore.getState().loadCategories();
  }, []);

  // A fresh scan gets a fresh (undismissed) banner.
  useEffect(() => {
    if (status === 'scanning') setErrorDismissed(false);
  }, [status]);

  // ---- Hero state ----
  if (status === 'idle') {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-6">
        <h1 className="text-3xl font-bold text-neutral-900 dark:text-neutral-100">App Cleaner</h1>
        <p className="text-sm text-neutral-500 dark:text-neutral-400">
          Find caches, logs, and leftover junk taking up space on your Mac.
        </p>
        <button
          type="button"
          onClick={() => void useScanStore.getState().startScan()}
          className="flex items-center gap-2 rounded-full bg-blue-600 px-8 py-4 text-lg font-semibold text-white shadow-lg hover:bg-blue-700"
        >
          <Search size={20} /> Smart Scan
        </button>
      </div>
    );
  }

  // ---- Scanning state ----
  if (status === 'scanning') {
    return (
      <div className="flex h-full flex-col px-8 py-8">
        <div className="mb-4 flex items-center gap-3">
          <Loader2 size={18} className="animate-spin text-blue-600 dark:text-blue-400" />
          <span className="text-sm font-medium">
            Scanning… {progress.completed}/{progress.total}
          </span>
          <button
            type="button"
            onClick={() => useScanStore.getState().cancelScan()}
            className="ml-auto rounded-md border border-neutral-300 px-3 py-1 text-sm text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
          >
            Cancel
          </button>
        </div>
        <div className="flex-1 space-y-1 overflow-y-auto">
          {categories.map((c) => {
            const r = results[c.id];
            return (
              <div key={c.id} className="flex items-center gap-2 rounded px-2 py-1.5 text-sm">
                <span className="w-56 truncate text-neutral-800 dark:text-neutral-200">
                  {c.name}
                </span>
                {r ? (
                  <span className="text-neutral-500 dark:text-neutral-400">
                    {itemCounts[c.id] ?? 0} items · {formatSize(r.totalSize)}
                  </span>
                ) : (
                  <span className="text-neutral-400 dark:text-neutral-600">pending…</span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    );
  }

  // ---- Results state (status === 'done') ----
  if (totalSize === 0) {
    return (
      <EmptyState title="Your Mac is already clean!" subtitle="Nothing to remove was found." />
    );
  }

  const hasContent = (c: Category) => {
    const r = results[c.id];
    return !!r && ((r.items?.length ?? 0) > 0 || !!r.error);
  };
  const maxSize = Math.max(0, ...Object.values(results).map((r) => r.totalSize));
  const groups = groupCategories(categories)
    .map((g) => ({ ...g, categories: g.categories.filter(hasContent) }))
    .filter((g) => g.categories.length > 0);
  const riskyCats = categories.filter((c) => c.safetyLevel === 'risky').filter(hasContent);
  const totals = selectionTotals(results, selected);

  const toggle = (id: string) => useScanStore.getState().toggleCategory(id);
  const openCategory = (id: string) => useUiStore.getState().setView('category', id);

  const renderCard = (c: Category) => (
    <CategoryCard
      key={c.id}
      result={results[c.id]}
      itemCount={itemCounts[c.id] ?? results[c.id].items?.length ?? 0}
      selected={isCategorySelected(selected[c.id])}
      maxSize={maxSize}
      onToggle={() => toggle(c.id)}
      onOpen={() => openCategory(c.id)}
    />
  );

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-y-auto px-6 py-4">
        {scanError && !errorDismissed && (
          <div className="mb-3 flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950 dark:text-amber-300">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            <span className="min-w-0 flex-1">{scanError}</span>
            <button
              type="button"
              aria-label="Dismiss scan error"
              onClick={() => setErrorDismissed(true)}
              className="shrink-0 rounded p-0.5 hover:bg-amber-100 dark:hover:bg-amber-900"
            >
              <X size={14} />
            </button>
          </div>
        )}
        {scanCancelled && (
          <div className="mb-3 text-xs text-neutral-500 dark:text-neutral-400">
            Scan cancelled — partial results
          </div>
        )}
        <div className="mb-4 text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          Found {formatSize(totalSize)} that can be cleaned
        </div>
        {groups.map((g) => (
          <section key={g.group} className="mb-5">
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
              {g.group}
            </h2>
            <div className="space-y-2">{g.categories.map(renderCard)}</div>
          </section>
        ))}
        {riskyCats.length > 0 && (
          <section className="mb-5">
            <button
              type="button"
              onClick={() => setRiskyExpanded(!riskyOpen)}
              className="mb-2 flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-red-600 dark:text-red-400"
            >
              {riskyOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              Risky ({riskyCats.length})
            </button>
            {riskyOpen && <div className="space-y-2">{riskyCats.map(renderCard)}</div>}
          </section>
        )}
      </div>
      <footer className="flex items-center gap-4 border-t border-neutral-200 px-6 py-3 dark:border-neutral-800">
        <span className="text-sm text-neutral-600 dark:text-neutral-400">
          {totals.items} items · {formatSize(totals.size)} selected
        </span>
        <button
          type="button"
          disabled={totals.items === 0}
          onClick={() => useCleanStore.getState().openConfirm()}
          className="ml-auto rounded-md bg-blue-600 px-6 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Clean
        </button>
      </footer>
    </div>
  );
}
```

Modify `frontend/src/App.tsx` — add the import (below the `FirstRun` import):

```tsx
import FirstRun from './views/FirstRun';
import SmartScan from './views/SmartScan';
```

and swap the placeholder line:

```tsx
        {view === 'smart-scan' && <SmartScan />}
```

(replacing `{view === 'smart-scan' && <Placeholder title="Smart Scan" />}`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/views/SmartScan.test.tsx`
Expected: PASS — 1 file, 7 tests.

Then the full gate:

```bash
cd frontend && npm run test        # PASS — all suites
cd frontend && npm run typecheck   # exit 0
cd frontend && npm run build       # vite build succeeds
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/SafetyBadge.tsx frontend/src/components/SizeBar.tsx frontend/src/components/CategoryCard.tsx frontend/src/components/EmptyState.tsx frontend/src/views/SmartScan.tsx frontend/src/views/SmartScan.test.tsx frontend/src/App.tsx
git commit -m "feat(frontend): smart scan view with grouped category cards

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 24: CategoryDetail View (grouped drill-down, flat lists, docker special-case)

Depends on: Tasks 21–23.

**Files:**
- Create: `frontend/src/lib/selection.ts`
- Create: `frontend/src/lib/paths.ts`
- Create: `frontend/src/components/ItemList.tsx`
- Create: `frontend/src/views/CategoryDetail.tsx`
- Modify: `frontend/src/lib/format.ts` (add `middleTruncate`)
- Modify: `app.go` (add the tiny `GetHome` binding) + regenerate `frontend/wailsjs`
- Modify: `frontend/src/App.tsx` (swap the Category placeholder)
- Test: `frontend/src/views/CategoryDetail.test.tsx`

**Interfaces:**
- Consumes: bindings `GroupItems(id: string, expand: Record<string, number>): Promise<DisplayRow[]>`, `RevealInFinder(path)`, `CopyPath(path)`, `GetHome(): Promise<string>` (added in this task); `useScanStore` (`results`, `selected`, `setItemSelection`), `isCategorySelected` (Task 22); `useUiStore` (`activeCategoryId`, `setView`); `SafetyBadge`, `EmptyState` (Task 23); `formatSize` (Task 21); `DisplayRow` type (Task 21, mirrors `grouping.DisplayRow`).
- Produces (package-local — verifier note):
  - `lib/selection.ts`: `DEFAULT_DIR_LIMIT = 5`, `EXPAND_INCREMENT = 10` (CLI file-picker parity: limit = (existing ?? 5) + 10), `bumpExpand(expand, directoryKey): Record<string, number>`, `togglePath(allPaths, sel, path): Selection`, `invertSelection(allPaths, sel): Selection`
  - `lib/format.ts`: `middleTruncate(s: string, maxLen: number): string`
  - `lib/paths.ts`: `contractHome(path: string, home: string): string` (pure display helper — item rows contract `$HOME` → `~` BEFORE `middleTruncate`) + `homeDir(): string` (module-level cache filled once from the `GetHome` binding at import time; `''` until resolved and under vitest, where paths render uncontracted)
  - `app.go`: †`GetHome() string` — tiny task-local bridge addition beyond the canonical bridge list (the contract permits documented local additions): returns `os.UserHomeDir()`, `""` on error
  - `components/ItemList.tsx`: `default ItemList({ items, selectable, isChecked, onToggle })` (flat rows; reused by later result/confirm views)
  - `views/CategoryDetail.tsx`: `default CategoryDetail()`

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/views/CategoryDetail.test.tsx`:

```tsx
import { beforeEach, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import CategoryDetail from './CategoryDetail';
import { bumpExpand, invertSelection, togglePath } from '../lib/selection';
import { contractHome } from '../lib/paths';
import { middleTruncate } from '../lib/format';
import { useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';
import type { Category } from '../lib/types';

describe('bumpExpand (expand-hint bump logic, CLI parity: (existing ?? 5) + 10)', () => {
  it('bumps an unexpanded directory from the default limit', () => {
    expect(bumpExpand({}, '/Users/x/Downloads')).toEqual({ '/Users/x/Downloads': 15 });
  });

  it('bumps an already-expanded directory further and leaves others alone', () => {
    const before = { '/a': 15, '/b': 25 };
    const after = bumpExpand(before, '/a');
    expect(after).toEqual({ '/a': 25, '/b': 25 });
    expect(before).toEqual({ '/a': 15, '/b': 25 }); // no mutation
  });
});

describe('selection math', () => {
  const all = ['/d/a', '/d/b', '/d/c'];

  it("invert of 'all' is nothing", () => {
    expect(invertSelection(all, 'all')).toEqual(new Set());
  });

  it('invert of nothing is everything', () => {
    expect(invertSelection(all, undefined)).toEqual(new Set(all));
    expect(invertSelection(all, new Set())).toEqual(new Set(all));
  });

  it('invert of a partial Set is its complement', () => {
    expect(invertSelection(all, new Set(['/d/b']))).toEqual(new Set(['/d/a', '/d/c']));
  });

  it("togglePath materializes 'all' into a Set minus the toggled path", () => {
    expect(togglePath(all, 'all', '/d/b')).toEqual(new Set(['/d/a', '/d/c']));
    expect(togglePath(all, new Set(['/d/a']), '/d/b')).toEqual(new Set(['/d/a', '/d/b']));
    expect(togglePath(all, new Set(['/d/a']), '/d/a')).toEqual(new Set());
  });
});

describe('middleTruncate', () => {
  it('elides the middle at maxLen', () => {
    expect(middleTruncate('short', 50)).toBe('short');
    const long = '/Users/someone/Library/Caches/com.example.app/Data/file.bin';
    const out = middleTruncate(long, 30);
    expect(out).toHaveLength(30);
    expect(out).toContain('...');
    expect(out.startsWith('/Users/someone')).toBe(true);
    expect(out.endsWith('file.bin')).toBe(true);
  });
});

describe('contractHome', () => {
  it('contracts home-prefixed paths and leaves everything else untouched', () => {
    expect(contractHome('/Users/x/Library/Caches/app', '/Users/x')).toBe('~/Library/Caches/app');
    expect(contractHome('/Users/x', '/Users/x')).toBe('~');
    expect(contractHome('/Users/xy/file', '/Users/x')).toBe('/Users/xy/file'); // prefix, not a path boundary
    expect(contractHome('/private/tmp/f', '/Users/x')).toBe('/private/tmp/f');
    expect(contractHome('docker:images', '/Users/x')).toBe('docker:images');
    expect(contractHome('/Users/x/file', '')).toBe('/Users/x/file'); // home unknown -> raw path
  });
});

describe('docker category rows are not selectable', () => {
  beforeEach(() => {
    useScanStore.getState().reset();
    const docker: Category = {
      id: 'docker',
      name: 'Docker',
      group: 'Development',
      description: '',
      safetyLevel: 'safe',
    };
    useScanStore.setState({
      status: 'done',
      results: {
        docker: {
          category: docker,
          items: [
            { path: 'docker:images', size: 1000, name: 'Docker images', isDirectory: false },
            { path: 'docker:build-cache', size: 500, name: 'Docker build cache', isDirectory: false },
          ],
          totalSize: 1500,
        },
      },
      itemCounts: { docker: 2 },
      totalSize: 1500,
    });
    useUiStore.setState({ view: 'category', activeCategoryId: 'docker' });
  });

  it('renders rows without checkboxes, hides select all/invert, shows the prune banner', () => {
    render(<CategoryDetail />);
    expect(
      screen.getByText('Docker space is cleaned all-at-once via docker system prune'),
    ).toBeInTheDocument();
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0);
    expect(screen.queryByText('Select all')).toBeNull();
    expect(screen.queryByText('Invert')).toBeNull();
    expect(screen.getByText('Docker images')).toBeInTheDocument();
    expect(screen.getByText('Docker build cache')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/views/CategoryDetail.test.tsx`
Expected: FAIL — `Failed to resolve import "./CategoryDetail"` / `"../lib/selection"` / `"../lib/paths"`, and `middleTruncate` is not exported from `../lib/format`.

- [ ] **Step 3: Write the implementation**

Add the tiny `GetHome` binding to `app.go` (task-local bridge addition — the frontend needs the real home dir to contract item paths to `~` for display, and deriving it from item paths is not reliable). Add `"os"` to app.go's imports if it is not already there:

```go
// GetHome returns the current user's home directory ("" if unresolvable).
// Frontend display helper: lets item rows contract $HOME to "~".
func (a *App) GetHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
```

Regenerate the bindings so the frontend can import it:

```bash
wails generate module
```

Expected: `frontend/wailsjs/go/main/App.d.ts` now declares `export function GetHome():Promise<string>;`.

Create `frontend/src/lib/paths.ts`:

```ts
import { GetHome } from '../../wailsjs/go/main/App';

let cachedHome = '';
// Resolved once at module load. Until the promise settles (and permanently
// under vitest, where the binding stub resolves null) paths render uncontracted.
void GetHome().then((h) => {
  cachedHome = h || '';
});

/** The user's home directory as reported by the backend ('' until resolved). */
export function homeDir(): string {
  return cachedHome;
}

/**
 * Contract a home-prefixed absolute path to "~/…" for display — applied BEFORE
 * middleTruncate so the elision budget is spent on the interesting tail.
 * Non-home paths (including docker: pseudo-paths) pass through untouched.
 */
export function contractHome(path: string, home: string): string {
  if (!home) return path;
  if (path === home) return '~';
  const prefix = home.endsWith('/') ? home : `${home}/`;
  return path.startsWith(prefix) ? `~/${path.slice(prefix.length)}` : path;
}
```

Append to `frontend/src/lib/format.ts` (below `formatSize`):

```ts
/** Middle-elide a string to exactly maxLen chars using '...' when it overflows. */
export function middleTruncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  const keep = maxLen - 3;
  const head = Math.ceil(keep / 2);
  const tail = Math.floor(keep / 2);
  return `${s.slice(0, head)}...${s.slice(s.length - tail)}`;
}
```

Create `frontend/src/lib/selection.ts`:

```ts
import type { Selection } from '../stores/scanStore';

/** CLI file-picker parity: 5 files visible per directory by default… */
export const DEFAULT_DIR_LIMIT = 5;
/** …and every expand bumps the per-directory limit by 10 ('m' key / expand-hint). */
export const EXPAND_INCREMENT = 10;

/** Returns a NEW expand map with limit[directoryKey] = (existing ?? 5) + 10. */
export function bumpExpand(
  expand: Record<string, number>,
  directoryKey: string,
): Record<string, number> {
  return {
    ...expand,
    [directoryKey]: (expand[directoryKey] ?? DEFAULT_DIR_LIMIT) + EXPAND_INCREMENT,
  };
}

/**
 * Toggle one path within a category selection.
 * `allPaths` = every selectable path in the category ('all' materializes into a Set).
 */
export function togglePath(
  allPaths: string[],
  sel: Selection | undefined,
  path: string,
): Selection {
  if (sel === 'all') {
    const next = new Set(allPaths);
    next.delete(path);
    return next;
  }
  const next = new Set(sel ?? []);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  return next;
}

/** Invert: every unselected selectable path becomes selected and vice versa. */
export function invertSelection(allPaths: string[], sel: Selection | undefined): Selection {
  if (sel === 'all') return new Set<string>();
  const current = sel ?? new Set<string>();
  return new Set(allPaths.filter((p) => !current.has(p)));
}
```

Create `frontend/src/components/ItemList.tsx`:

```tsx
import { Copy, Folder } from 'lucide-react';
import { CopyPath, RevealInFinder } from '../../wailsjs/go/main/App';
import { formatSize, middleTruncate } from '../lib/format';
import { contractHome, homeDir } from '../lib/paths';
import type { CleanableItem } from '../lib/types';

interface Props {
  items: CleanableItem[];
  selectable: boolean; // false for docker (rows informational only)
  isChecked: (path: string) => boolean;
  onToggle: (path: string) => void;
}

export default function ItemList({ items, selectable, isChecked, onToggle }: Props) {
  return (
    <ul className="divide-y divide-neutral-100 dark:divide-neutral-800">
      {items.map((item) => (
        <li key={item.path} className="group flex items-center gap-3 px-2 py-1.5 text-sm">
          {selectable && (
            <input
              type="checkbox"
              aria-label={`Select ${item.name}`}
              checked={isChecked(item.path)}
              onChange={() => onToggle(item.path)}
              className="h-4 w-4 accent-blue-600"
            />
          )}
          <span className="w-64 shrink-0 truncate font-medium text-neutral-900 dark:text-neutral-100">
            {item.name}
          </span>
          <span
            className="min-w-0 flex-1 truncate text-xs text-neutral-400 dark:text-neutral-500"
            title={item.path}
          >
            {middleTruncate(contractHome(item.path, homeDir()), 50)}
          </span>
          <span className="hidden shrink-0 items-center gap-1 group-hover:flex">
            <button
              type="button"
              title="Reveal in Finder"
              onClick={() => RevealInFinder(item.path)}
              className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
            >
              <Folder size={14} />
            </button>
            <button
              type="button"
              title="Copy path"
              onClick={() => CopyPath(item.path)}
              className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
            >
              <Copy size={14} />
            </button>
          </span>
          <span className="w-20 shrink-0 text-right text-neutral-500 dark:text-neutral-400">
            {formatSize(item.size)}
          </span>
        </li>
      ))}
    </ul>
  );
}
```

(Flat item rows contract `$HOME` → `~` via `contractHome` before `middleTruncate`; the full path stays in `title`. The grouped rows below and the docker special-case are unchanged — grouped directory headers already arrive home-contracted from the backend, and `docker:` pseudo-paths never match the home prefix.)

Create `frontend/src/views/CategoryDetail.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, ArrowLeft, Copy, Folder, Info } from 'lucide-react';
import { CopyPath, GroupItems, RevealInFinder } from '../../wailsjs/go/main/App';
import EmptyState from '../components/EmptyState';
import ItemList from '../components/ItemList';
import SafetyBadge from '../components/SafetyBadge';
import { formatSize, middleTruncate } from '../lib/format';
import { bumpExpand, invertSelection, togglePath } from '../lib/selection';
import type { DisplayRow } from '../lib/types';
import { useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';

export default function CategoryDetail() {
  const activeCategoryId = useUiStore((s) => s.activeCategoryId);
  const setView = useUiStore((s) => s.setView);
  const results = useScanStore((s) => s.results);
  const selected = useScanStore((s) => s.selected);
  const setItemSelection = useScanStore((s) => s.setItemSelection);

  const result = activeCategoryId ? results[activeCategoryId] : undefined;
  const grouped = result?.category.supportsFileSelection === true;
  const isDocker = result?.category.id === 'docker';

  // Per-directory expand limits, keyed by absolute dir path (grouping.DisplayRow.directoryKey).
  const [expand, setExpand] = useState<Record<string, number>>({});
  const [rows, setRows] = useState<DisplayRow[]>([]);

  useEffect(() => {
    if (!grouped || !activeCategoryId) return;
    let stale = false;
    void GroupItems(activeCategoryId, expand).then((r) => {
      if (!stale) setRows(((r ?? []) as DisplayRow[]));
    });
    return () => {
      stale = true;
    };
  }, [grouped, activeCategoryId, expand]);

  const onExpandHint = useCallback((directoryKey: string) => {
    setExpand((e) => bumpExpand(e, directoryKey));
  }, []);

  if (!result || !activeCategoryId) {
    return <EmptyState title="No category selected" subtitle="Run a scan and pick a category." />;
  }

  const items = result.items ?? [];
  const allPaths = items.map((i) => i.path);
  const sel = selected[activeCategoryId];
  const checked = (path: string) => sel === 'all' || (sel instanceof Set && sel.has(path));
  const toggleItem = (path: string) =>
    setItemSelection(activeCategoryId, togglePath(allPaths, sel, path));

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-neutral-200 px-6 py-4 dark:border-neutral-800">
        <div className="flex items-center gap-3">
          <button
            type="button"
            aria-label="Back"
            onClick={() => setView('smart-scan')}
            className="rounded p-1 text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800"
          >
            <ArrowLeft size={18} />
          </button>
          <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
            {result.category.name}
          </h1>
          <SafetyBadge level={result.category.safetyLevel} />
          <span className="ml-auto text-sm text-neutral-500 dark:text-neutral-400">
            {items.length} items · {formatSize(result.totalSize)}
          </span>
        </div>
        {result.category.safetyLevel !== 'safe' && result.category.safetyNote && (
          <div className="mt-3 flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950 dark:text-amber-300">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            {result.category.safetyNote}
          </div>
        )}
        {isDocker ? (
          <div className="mt-3 flex items-start gap-2 rounded-md bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:bg-blue-950 dark:text-blue-300">
            <Info size={14} className="mt-0.5 shrink-0" />
            Docker space is cleaned all-at-once via docker system prune
          </div>
        ) : (
          <div className="mt-3 flex items-center gap-2">
            <button
              type="button"
              onClick={() => setItemSelection(activeCategoryId, 'all')}
              className="rounded-md border border-neutral-300 px-3 py-1 text-xs text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
            >
              Select all
            </button>
            <button
              type="button"
              onClick={() => setItemSelection(activeCategoryId, invertSelection(allPaths, sel))}
              className="rounded-md border border-neutral-300 px-3 py-1 text-xs text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
            >
              Invert
            </button>
          </div>
        )}
      </header>

      {items.length === 0 ? (
        <EmptyState title="Nothing here" subtitle="This category has no cleanable items." />
      ) : grouped ? (
        <ul className="flex-1 overflow-y-auto px-6 py-2">
          {rows.map((row, i) => {
            if (row.type === 'directory-header') {
              return (
                <li
                  key={`h-${row.directoryKey}`}
                  className="mt-3 flex items-baseline gap-2 px-2 py-1 text-xs font-semibold text-neutral-500 dark:text-neutral-400"
                >
                  {row.displayName}
                  <span className="font-normal">({row.totalFilesInDir})</span>
                </li>
              );
            }
            if (row.type === 'expand-hint') {
              return (
                <li key={`e-${row.directoryKey}-${i}`}>
                  <button
                    type="button"
                    onClick={() => onExpandHint(row.directoryKey)}
                    className="px-2 py-1 text-xs text-blue-600 hover:underline dark:text-blue-400"
                  >
                    Show {row.hiddenCount} more
                  </button>
                </li>
              );
            }
            // type === 'file'
            const path = row.path ?? '';
            return (
              <li key={path} className="group flex items-center gap-3 px-2 py-1.5 text-sm">
                <input
                  type="checkbox"
                  aria-label={`Select ${row.name ?? row.displayName}`}
                  checked={checked(path)}
                  onChange={() => toggleItem(path)}
                  className="h-4 w-4 accent-blue-600"
                />
                <span className="w-64 shrink-0 truncate font-medium text-neutral-900 dark:text-neutral-100">
                  {row.displayName}
                </span>
                <span
                  className="min-w-0 flex-1 truncate text-xs text-neutral-400 dark:text-neutral-500"
                  title={path}
                >
                  {middleTruncate(path, 50)}
                </span>
                <span className="hidden shrink-0 items-center gap-1 group-hover:flex">
                  <button
                    type="button"
                    title="Reveal in Finder"
                    onClick={() => RevealInFinder(path)}
                    className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
                  >
                    <Folder size={14} />
                  </button>
                  <button
                    type="button"
                    title="Copy path"
                    onClick={() => CopyPath(path)}
                    className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
                  >
                    <Copy size={14} />
                  </button>
                </span>
                <span className="w-20 shrink-0 text-right text-neutral-500 dark:text-neutral-400">
                  {formatSize(row.size ?? 0)}
                </span>
              </li>
            );
          })}
        </ul>
      ) : (
        <div className="flex-1 overflow-y-auto px-6 py-2">
          <ItemList
            items={[...items].sort((a, b) => b.size - a.size)}
            selectable={!isDocker}
            isChecked={checked}
            onToggle={toggleItem}
          />
        </div>
      )}
    </div>
  );
}
```

Modify `frontend/src/App.tsx` — add the import (below the `SmartScan` import):

```tsx
import SmartScan from './views/SmartScan';
import CategoryDetail from './views/CategoryDetail';
```

and swap the placeholder line:

```tsx
        {view === 'category' && <CategoryDetail />}
```

(replacing `{view === 'category' && <Placeholder title="Category" />}`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/views/CategoryDetail.test.tsx`
Expected: PASS — 1 file, 9 tests.

Then the full gate:

```bash
go build ./...                     # app.go still compiles with GetHome
cd frontend && npm run test        # PASS — all suites (format, stores, SmartScan, CategoryDetail)
cd frontend && npm run typecheck   # exit 0
cd frontend && npm run build       # vite build succeeds
```

- [ ] **Step 5: Commit**

```bash
git add app.go frontend/wailsjs frontend/src/lib/selection.ts frontend/src/lib/paths.ts frontend/src/lib/format.ts frontend/src/components/ItemList.tsx frontend/src/views/CategoryDetail.tsx frontend/src/views/CategoryDetail.test.tsx frontend/src/App.tsx
git commit -m "feat(frontend): category detail view with grouped file selection

Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 25: Clean flow — ConfirmModal, ProgressOverlay, ResultsPanel, cleanStore

**Files:**
- Create: `frontend/src/stores/cleanStore.ts`
- Create: `frontend/src/components/ConfirmModal.tsx`
- Create: `frontend/src/components/ProgressOverlay.tsx`
- Create: `frontend/src/components/ResultsPanel.tsx`
- Create: `frontend/src/components/CleanFlow.tsx`
- Modify: `frontend/src/App.tsx` (mount `<CleanFlow />`)
- Test: `frontend/src/stores/cleanStore.test.ts`
- Test: `frontend/src/components/ConfirmModal.test.tsx`
- Test: `frontend/src/components/ResultsPanel.test.tsx`

Note: `frontend/wailsjs/go/main/App.(js|d.ts)` and `frontend/wailsjs/runtime/runtime.(js|d.ts)` were generated and committed by the Wails-bridge task. Tests never load the real modules — every test file replaces both with `vi.mock` factories.

**Interfaces:**
- Consumes (bindings, from the contract): `StartClean(selection: Record<string, string[]>, opts: {dryRun: boolean; backup: boolean}): Promise<void>`, `CancelClean(): Promise<void>`, `OpenFDASettings(): Promise<void>` from `../../wailsjs/go/main/App`; `EventsOn(name, cb)` / `EventsOff(name)` from `../../wailsjs/runtime/runtime`
- Consumes (earlier frontend tasks): `useScanStore` (`results: Record<string, ScanResult>`, `selected: Record<string, Set<string>|'all'>`, `reset()`), `useUiStore` (`config?: Config`, `setView(v)`, `loadConfig()`), `formatSize(bytes: number): string` from `lib/format`, TS types `Category`, `ScanResult`, `CleanResult`, `CleanSummary`, `Config` from `lib/types`
- Produces (contract): `useCleanStore` — `{status: 'idle'|'confirming'|'cleaning'|'done', progress, summary?, notBackedUp: string[], openConfirm, startClean, cancelClean, reset}`; exported event handlers `handleCleanProgress`, `handleCleanDone`, `handleBackupProgress` (subscribed on module load to `clean:progress`, `clean:done`, and `backup:progress {current, total, itemName}` so the overlay also moves during the backup phase)
- Produces (package-local, reused by Task 26): `ProgressOverlay` React component with props `{title: string; current: number; total: number; itemName: string; onCancel?: () => void}` (Cancel button hidden when `onCancel` is omitted)
- Produces (package-local, exported for tests): `computeSelectionStats(results, selected)`, `selectionAsPaths(results, selected)`, `defaultBackupEnabled(backupByDefault, categories)` in `ConfirmModal.tsx`; `needsFdaHint(summary)` in `ResultsPanel.tsx`; store field `lastDryRun: boolean` on `useCleanStore` (set by `startClean`, read by `ResultsPanel`/`CleanFlow`); store field `backingUp: boolean` on `useCleanStore` (set true by `handleBackupProgress`, cleared by `handleCleanProgress`; `CleanFlow` prefixes the overlay item name with `Backing up: ` while true)

- [ ] **Step 1: Write the failing store test**

Create `frontend/src/stores/cleanStore.test.ts`:

```ts
// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  StartClean: vi.fn().mockResolvedValue(undefined),
  CancelClean: vi.fn(),
}))

import { EventsOn } from '../../wailsjs/runtime/runtime'
import { StartClean, CancelClean } from '../../wailsjs/go/main/App'
import { useCleanStore, handleCleanProgress, handleCleanDone, handleBackupProgress } from './cleanStore'
import type { CleanSummary } from '../lib/types'

// The generated bindings are typed classes; erase types for mock assertions.
const StartCleanMock = StartClean as unknown as ReturnType<typeof vi.fn>
const CancelCleanMock = CancelClean as unknown as ReturnType<typeof vi.fn>
const EventsOnMock = EventsOn as unknown as ReturnType<typeof vi.fn>

beforeEach(() => {
  useCleanStore.getState().reset()
  StartCleanMock.mockClear()
  CancelCleanMock.mockClear()
})

describe('cleanStore', () => {
  it('registers the clean:progress, clean:done and backup:progress handlers on module load', () => {
    expect(EventsOnMock).toHaveBeenCalledWith('clean:progress', handleCleanProgress)
    expect(EventsOnMock).toHaveBeenCalledWith('clean:done', handleCleanDone)
    expect(EventsOnMock).toHaveBeenCalledWith('backup:progress', handleBackupProgress)
  })

  it('openConfirm moves to confirming', () => {
    useCleanStore.getState().openConfirm()
    expect(useCleanStore.getState().status).toBe('confirming')
  })

  it('handleCleanProgress moves to cleaning and records progress', () => {
    handleCleanProgress({ current: 3, total: 40, categoryId: 'trash', itemName: 'old.dmg' })
    const s = useCleanStore.getState()
    expect(s.status).toBe('cleaning')
    expect(s.progress).toEqual({ current: 3, total: 40, categoryId: 'trash', itemName: 'old.dmg' })
  })

  it('handleBackupProgress feeds the same progress state and flags the backup phase', () => {
    handleBackupProgress({ current: 2, total: 10, itemName: 'photo.jpg' })
    const s = useCleanStore.getState()
    expect(s.status).toBe('cleaning')
    expect(s.backingUp).toBe(true)
    expect(s.progress).toEqual({ current: 2, total: 10, categoryId: '', itemName: 'photo.jpg' })
    handleCleanProgress({ current: 1, total: 10, categoryId: 'trash', itemName: 'old.dmg' })
    expect(useCleanStore.getState().backingUp).toBe(false)
  })

  it('handleCleanDone stores summary, notBackedUp and finishes', () => {
    const summary: CleanSummary & { notBackedUp?: string[] } = {
      results: [],
      totalFreedSpace: 42,
      totalCleanedItems: 1,
      totalErrors: 0,
      notBackedUp: ['/Volumes/Ext/movie.mkv'],
    }
    handleCleanDone({ summary })
    const s = useCleanStore.getState()
    expect(s.status).toBe('done')
    expect(s.summary?.totalFreedSpace).toBe(42)
    expect(s.notBackedUp).toEqual(['/Volumes/Ext/movie.mkv'])
    expect(s.cancelled).toBe(false)
  })

  it('handleCleanDone records cancellation and errors', () => {
    handleCleanDone({ cancelled: true, error: 'boom' })
    const s = useCleanStore.getState()
    expect(s.status).toBe('done')
    expect(s.cancelled).toBe(true)
    expect(s.error).toBe('boom')
    expect(s.notBackedUp).toEqual([])
  })

  it('startClean calls the StartClean binding with paths + options and tracks dry-run', async () => {
    await useCleanStore
      .getState()
      .startClean({ trash: ['/Users/me/.Trash/a.zip'] }, { dryRun: true, backup: false })
    expect(StartCleanMock).toHaveBeenCalledWith(
      { trash: ['/Users/me/.Trash/a.zip'] },
      { dryRun: true, backup: false },
    )
    expect(useCleanStore.getState().status).toBe('cleaning')
    expect(useCleanStore.getState().lastDryRun).toBe(true)
  })

  it('cancelClean invokes the binding; reset returns to idle', () => {
    useCleanStore.getState().cancelClean()
    expect(CancelCleanMock).toHaveBeenCalledTimes(1)
    handleCleanDone({ cancelled: true })
    useCleanStore.getState().reset()
    const s = useCleanStore.getState()
    expect(s.status).toBe('idle')
    expect(s.summary).toBeUndefined()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/stores/cleanStore.test.ts`
Expected: FAIL — `Failed to resolve import "./cleanStore"` (module does not exist yet).

- [ ] **Step 3: Write the cleanStore implementation**

Create `frontend/src/stores/cleanStore.ts`:

```ts
import { create } from 'zustand'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { StartClean, CancelClean } from '../../wailsjs/go/main/App'
import type { CleanSummary } from '../lib/types'

export interface CleanProgress {
  current: number
  total: number
  categoryId: string
  itemName: string
}

export interface CleanDonePayload {
  summary?: CleanSummary & { notBackedUp?: string[] }
  notBackedUp?: string[] // tolerated at top level too
  cancelled?: boolean
  error?: string
}

export interface BackupProgress {
  current: number
  total: number
  itemName: string
}

const initialProgress: CleanProgress = { current: 0, total: 0, categoryId: '', itemName: '' }

interface CleanState {
  status: 'idle' | 'confirming' | 'cleaning' | 'done'
  progress: CleanProgress
  backingUp: boolean // true while backup:progress events drive the overlay
  summary?: CleanSummary
  notBackedUp: string[]
  cancelled: boolean
  error?: string
  lastDryRun: boolean
  openConfirm: () => void
  startClean: (
    selection: Record<string, string[]>,
    opts: { dryRun: boolean; backup: boolean },
  ) => Promise<void>
  cancelClean: () => void
  reset: () => void
}

export const useCleanStore = create<CleanState>()((set) => ({
  status: 'idle',
  progress: initialProgress,
  backingUp: false,
  summary: undefined,
  notBackedUp: [],
  cancelled: false,
  error: undefined,
  lastDryRun: false,
  openConfirm: () => set({ status: 'confirming' }),
  startClean: async (selection, opts) => {
    set({
      status: 'cleaning',
      progress: initialProgress,
      backingUp: false,
      summary: undefined,
      notBackedUp: [],
      cancelled: false,
      error: undefined,
      lastDryRun: opts.dryRun,
    })
    try {
      await StartClean(selection, opts)
    } catch (e) {
      // e.g. "a clean is already running" — surface it and finish the flow
      set({ status: 'done', error: String(e) })
    }
  },
  cancelClean: () => {
    void CancelClean()
  },
  reset: () =>
    set({
      status: 'idle',
      progress: initialProgress,
      backingUp: false,
      summary: undefined,
      notBackedUp: [],
      cancelled: false,
      error: undefined,
      lastDryRun: false,
    }),
}))

// Exported for tests; also registered as the live Wails event handlers below.
export function handleCleanProgress(data: CleanProgress): void {
  useCleanStore.setState({ status: 'cleaning', backingUp: false, progress: data })
}

// backup:progress drives the same overlay while items are moved into the backup
// session (before deletion); CleanFlow prefixes the item name with "Backing up: ".
export function handleBackupProgress(data: BackupProgress): void {
  useCleanStore.setState({
    status: 'cleaning',
    backingUp: true,
    progress: { current: data.current, total: data.total, categoryId: '', itemName: data.itemName },
  })
}

export function handleCleanDone(data: CleanDonePayload): void {
  useCleanStore.setState({
    status: 'done',
    summary: data.summary,
    notBackedUp: data.summary?.notBackedUp ?? data.notBackedUp ?? [],
    cancelled: Boolean(data.cancelled),
    error: data.error,
  })
}

EventsOn('clean:progress', handleCleanProgress)
EventsOn('backup:progress', handleBackupProgress)
EventsOn('clean:done', handleCleanDone)
```

- [ ] **Step 4: Run store test to verify it passes**

Run: `cd frontend && npx vitest run src/stores/cleanStore.test.ts`
Expected: PASS (8 tests).

- [ ] **Step 5: Write the failing ConfirmModal test**

Create `frontend/src/components/ConfirmModal.test.tsx`:

```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  GetCategories: vi.fn().mockResolvedValue([]),
  StartScan: vi.fn().mockResolvedValue(undefined),
  CancelScan: vi.fn(),
  GetScanResult: vi.fn(),
  GroupItems: vi.fn().mockResolvedValue([]),
  StartClean: vi.fn().mockResolvedValue(undefined),
  CancelClean: vi.fn(),
  GetConfig: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  CheckFDA: vi.fn().mockResolvedValue(null),
  OpenFDASettings: vi.fn(),
  RevealInFinder: vi.fn(),
  CopyPath: vi.fn(),
}))

import { StartClean } from '../../wailsjs/go/main/App'
import {
  ConfirmModal,
  computeSelectionStats,
  selectionAsPaths,
  defaultBackupEnabled,
} from './ConfirmModal'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { useCleanStore } from '../stores/cleanStore'
import type { Category, Config, ScanResult } from '../lib/types'

const StartCleanMock = StartClean as unknown as ReturnType<typeof vi.fn>

function cat(id: string, name: string, safety: Category['safetyLevel'], note = ''): Category {
  return {
    id,
    name,
    group: 'System Junk',
    description: '',
    safetyLevel: safety,
    ...(note ? { safetyNote: note } : {}),
  }
}

function result(c: Category, items: Array<{ path: string; size: number }>): ScanResult {
  return {
    category: c,
    items: items.map((it) => ({
      path: it.path,
      size: it.size,
      name: it.path.split('/').pop() ?? it.path,
      isDirectory: false,
    })),
    totalSize: items.reduce((a, b) => a + b.size, 0),
  }
}

const baseConfig: Config = {
  downloadsDaysOld: 30,
  largeFilesMinSize: 524288000,
  backupByDefault: true,
  backupRetentionDays: 7,
  concurrency: 4,
  showRisky: false,
  keepLanguages: [],
  extraPaths: { nodeModules: [], projects: [] },
}

const tempFiles = result(cat('temp-files', 'Temporary Files', 'safe'), [
  { path: '/tmp/a.log', size: 1024 },
  { path: '/tmp/b.log', size: 2048 },
])
const sysCache = result(cat('system-cache', 'User Cache Files', 'moderate'), [
  { path: '/Users/me/Library/Caches/x', size: 4096 },
])
const downloads = result(
  cat('downloads', 'Old Downloads', 'risky', 'May contain important files you forgot about'),
  [
    { path: '/Users/me/Downloads/old1.zip', size: 100 },
    { path: '/Users/me/Downloads/old2.zip', size: 200 },
  ],
)

beforeEach(() => {
  useCleanStore.getState().reset()
  useUiStore.setState({ config: baseConfig })
})

afterEach(() => {
  cleanup()
  StartCleanMock.mockClear()
})

describe('selection helpers', () => {
  it('computeSelectionStats counts items/sizes across \'all\' and Set selections', () => {
    const stats = computeSelectionStats(
      { 'temp-files': tempFiles, downloads },
      { 'temp-files': 'all', downloads: new Set(['/Users/me/Downloads/old2.zip']) },
    )
    expect(stats.itemCount).toBe(3)
    expect(stats.totalSize).toBe(1024 + 2048 + 200)
    expect(stats.categories.map((c) => c.id).sort()).toEqual(['downloads', 'temp-files'])
  })

  it('selectionAsPaths expands \'all\' and filters Sets, dropping empty categories', () => {
    expect(
      selectionAsPaths(
        { 'temp-files': tempFiles, downloads },
        { 'temp-files': 'all', downloads: new Set<string>() },
      ),
    ).toEqual({ 'temp-files': ['/tmp/a.log', '/tmp/b.log'] })
  })

  it('defaultBackupEnabled: off for safe-only, on for mixed, off when config disables it', () => {
    const safe = cat('temp-files', 'Temporary Files', 'safe')
    const moderate = cat('system-cache', 'User Cache Files', 'moderate')
    expect(defaultBackupEnabled(true, [safe])).toBe(false)
    expect(defaultBackupEnabled(true, [safe, moderate])).toBe(true)
    expect(defaultBackupEnabled(false, [safe, moderate])).toBe(false)
  })
})

describe('<ConfirmModal />', () => {
  it('backup toggle defaults OFF for a safe-only selection', () => {
    useScanStore.setState({ results: { 'temp-files': tempFiles }, selected: { 'temp-files': 'all' } })
    render(<ConfirmModal />)
    const toggle = screen.getByLabelText(/back up items before deleting/i) as HTMLInputElement
    expect(toggle.checked).toBe(false)
  })

  it('backup toggle defaults ON for a mixed selection and shows the risky safety note', () => {
    useScanStore.setState({
      results: { 'temp-files': tempFiles, downloads },
      selected: { 'temp-files': 'all', downloads: 'all' },
    })
    render(<ConfirmModal />)
    const toggle = screen.getByLabelText(/back up items before deleting/i) as HTMLInputElement
    expect(toggle.checked).toBe(true)
    expect(screen.getByText('May contain important files you forgot about')).toBeDefined()
  })

  it('shows item count + total size and starts the clean with the selection as paths', () => {
    useScanStore.setState({
      results: { 'system-cache': sysCache },
      selected: { 'system-cache': 'all' },
    })
    render(<ConfirmModal />)
    expect(screen.getByText(/1 item/)).toBeDefined()
    expect(screen.getByText(/4\.0 KB/)).toBeDefined()
    fireEvent.click(screen.getByLabelText(/dry run/i))
    fireEvent.click(screen.getByRole('button', { name: /^clean$/i }))
    expect(StartCleanMock).toHaveBeenCalledWith(
      { 'system-cache': ['/Users/me/Library/Caches/x'] },
      { dryRun: true, backup: true }, // moderate selection + backupByDefault:true
    )
  })

  it('Cancel resets the clean flow to idle', () => {
    useScanStore.setState({ results: { 'temp-files': tempFiles }, selected: { 'temp-files': 'all' } })
    useCleanStore.getState().openConfirm()
    render(<ConfirmModal />)
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(useCleanStore.getState().status).toBe('idle')
  })
})
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/ConfirmModal.test.tsx`
Expected: FAIL — `Failed to resolve import "./ConfirmModal"`.

- [ ] **Step 7: Write ConfirmModal**

Create `frontend/src/components/ConfirmModal.tsx`:

```tsx
import { useEffect, useMemo, useState } from 'react'
import { useScanStore } from '../stores/scanStore'
import { useCleanStore } from '../stores/cleanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import type { Category, ScanResult } from '../lib/types'

export interface SelectionStats {
  itemCount: number
  totalSize: number
  categories: Category[]
}

type Selected = Record<string, Set<string> | 'all'>

export function computeSelectionStats(
  results: Record<string, ScanResult>,
  selected: Selected,
): SelectionStats {
  let itemCount = 0
  let totalSize = 0
  const categories: Category[] = []
  for (const [id, sel] of Object.entries(selected)) {
    const r = results[id]
    if (!r || !r.items || r.items.length === 0) continue
    const items = sel === 'all' ? r.items : r.items.filter((it) => sel.has(it.path))
    if (items.length === 0) continue
    categories.push(r.category)
    itemCount += items.length
    for (const it of items) totalSize += it.size
  }
  return { itemCount, totalSize, categories }
}

export function selectionAsPaths(
  results: Record<string, ScanResult>,
  selected: Selected,
): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  for (const [id, sel] of Object.entries(selected)) {
    const r = results[id]
    if (!r || !r.items) continue
    const paths =
      sel === 'all'
        ? r.items.map((it) => it.path)
        : r.items.filter((it) => sel.has(it.path)).map((it) => it.path)
    if (paths.length > 0) out[id] = paths
  }
  return out
}

export function defaultBackupEnabled(backupByDefault: boolean, categories: Category[]): boolean {
  return (
    backupByDefault &&
    categories.some((c) => c.safetyLevel === 'moderate' || c.safetyLevel === 'risky')
  )
}

const safetyColors: Record<Category['safetyLevel'], string> = {
  safe: 'text-green-600 dark:text-green-400',
  moderate: 'text-amber-600 dark:text-amber-400',
  risky: 'text-red-600 dark:text-red-400',
}

export function ConfirmModal() {
  const results = useScanStore((s) => s.results)
  const selected = useScanStore((s) => s.selected)
  const config = useUiStore((s) => s.config)
  const startClean = useCleanStore((s) => s.startClean)

  useEffect(() => {
    if (!config) void useUiStore.getState().loadConfig()
  }, [config])

  const stats = useMemo(() => computeSelectionStats(results, selected), [results, selected])
  const [backup, setBackup] = useState(() =>
    defaultBackupEnabled(config?.backupByDefault ?? true, stats.categories),
  )
  const [dryRun, setDryRun] = useState(false)

  const onClean = () => {
    void startClean(selectionAsPaths(results, selected), { dryRun, backup })
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[480px] max-h-[80vh] overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <h2 className="text-lg font-semibold">Confirm clean</h2>
        <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-300">
          {stats.itemCount} item{stats.itemCount === 1 ? '' : 's'} —{' '}
          <span className="font-medium">{formatSize(stats.totalSize)}</span> will be freed
        </p>

        <ul className="mt-4 space-y-2">
          {stats.categories.map((c) => (
            <li key={c.id} className="text-sm">
              <span className="font-medium">{c.name}</span>{' '}
              <span className={safetyColors[c.safetyLevel]}>({c.safetyLevel})</span>
              {c.safetyLevel === 'risky' && c.safetyNote ? (
                <p className="mt-0.5 text-xs text-red-600 dark:text-red-400">{c.safetyNote}</p>
              ) : null}
            </li>
          ))}
        </ul>

        <div className="mt-5 space-y-2">
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={backup}
              onChange={(e) => setBackup(e.target.checked)}
            />
            Back up items before deleting (Undo)
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(e) => setDryRun(e.target.checked)}
            />
            Dry run (preview only, nothing is deleted)
          </label>
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            className="rounded-md px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
            onClick={() => useCleanStore.getState().reset()}
          >
            Cancel
          </button>
          <button
            className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-40"
            disabled={stats.itemCount === 0}
            onClick={onClean}
          >
            Clean
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/components/ConfirmModal.test.tsx`
Expected: PASS (7 tests).

- [ ] **Step 9: Write the failing ResultsPanel test**

Create `frontend/src/components/ResultsPanel.test.tsx`:

```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  GetCategories: vi.fn().mockResolvedValue([]),
  StartScan: vi.fn().mockResolvedValue(undefined),
  CancelScan: vi.fn(),
  GetScanResult: vi.fn(),
  GroupItems: vi.fn().mockResolvedValue([]),
  StartClean: vi.fn().mockResolvedValue(undefined),
  CancelClean: vi.fn(),
  GetConfig: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  CheckFDA: vi.fn().mockResolvedValue(null),
  OpenFDASettings: vi.fn(),
  RevealInFinder: vi.fn(),
  CopyPath: vi.fn(),
}))

import { OpenFDASettings } from '../../wailsjs/go/main/App'
import { ResultsPanel, needsFdaHint } from './ResultsPanel'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import type { Category, CleanSummary } from '../lib/types'

const OpenFDAMock = OpenFDASettings as unknown as ReturnType<typeof vi.fn>

function cat(id: string, name: string): Category {
  return { id, name, group: 'System Junk', description: '', safetyLevel: 'safe' }
}

const summaryWithEperm: CleanSummary = {
  results: [
    { category: cat('trash', 'Trash'), cleanedItems: 12, freedSpace: 1073741824, errors: [] },
    {
      category: cat('temp-files', 'Temporary Files'),
      cleanedItems: 5,
      freedSpace: 536870912,
      errors: ['Failed to remove 3 items (3 EPERM)'],
    },
  ],
  totalFreedSpace: 1610612736,
  totalCleanedItems: 17,
  totalErrors: 1,
}

beforeEach(() => {
  useCleanStore.getState().reset()
})

afterEach(() => {
  cleanup()
  OpenFDAMock.mockClear()
})

describe('needsFdaHint', () => {
  it('is true when any error string contains EPERM or EACCES, false otherwise', () => {
    expect(needsFdaHint(summaryWithEperm)).toBe(true)
    expect(
      needsFdaHint({
        results: [{ category: cat('trash', 'Trash'), cleanedItems: 1, freedSpace: 1, errors: ['Failed to remove 1 items (1 ENOENT)'] }],
        totalFreedSpace: 1,
        totalCleanedItems: 1,
        totalErrors: 1,
      }),
    ).toBe(false)
    expect(needsFdaHint(undefined)).toBe(false)
  })
})

describe('<ResultsPanel />', () => {
  it('renders the freed-space headline, per-category rows and verbatim errno breakdown', () => {
    useCleanStore.setState({
      status: 'done',
      summary: summaryWithEperm,
      notBackedUp: ['/Volumes/Ext/big.mkv'],
      cancelled: false,
    })
    render(<ResultsPanel />)
    expect(screen.getByText(/1\.5 GB freed/)).toBeDefined()
    expect(screen.getByText(/✓ Trash/)).toBeDefined()
    expect(screen.getByText(/✗ Temporary Files/)).toBeDefined()
    expect(screen.getByText('Failed to remove 3 items (3 EPERM)')).toBeDefined()
    expect(screen.queryByText('Some items are blocked for safety (system-protected paths)')).toBeNull()
    expect(screen.getByText(/1 item\(s\) not backed up \(outside home \/ other volume\)/)).toBeDefined()
  })

  it('shows the FDA call-to-action on EPERM errors and opens System Settings', () => {
    useCleanStore.setState({ status: 'done', summary: summaryWithEperm, notBackedUp: [] })
    render(<ResultsPanel />)
    const btn = screen.getByRole('button', { name: /grant full disk access/i })
    fireEvent.click(btn)
    expect(OpenFDAMock).toHaveBeenCalledTimes(1)
  })

  it('hides the FDA call-to-action when there are no permission errors', () => {
    useCleanStore.setState({
      status: 'done',
      summary: { results: [], totalFreedSpace: 0, totalCleanedItems: 0, totalErrors: 0 },
      notBackedUp: [],
    })
    render(<ResultsPanel />)
    expect(screen.queryByRole('button', { name: /grant full disk access/i })).toBeNull()
  })

  it('renders a muted safety note under errors that contain PROTECTED', () => {
    useCleanStore.setState({
      status: 'done',
      summary: {
        results: [
          {
            category: cat('system-cache', 'User Cache Files'),
            cleanedItems: 2,
            freedSpace: 2048,
            errors: ['Failed to remove 4 items (4 PROTECTED)'],
          },
        ],
        totalFreedSpace: 2048,
        totalCleanedItems: 2,
        totalErrors: 1,
      },
      notBackedUp: [],
    })
    render(<ResultsPanel />)
    expect(screen.getByText('Failed to remove 4 items (4 PROTECTED)')).toBeDefined()
    expect(
      screen.getByText('Some items are blocked for safety (system-protected paths)'),
    ).toBeDefined()
  })

  it('Done resets both stores and returns to the Smart Scan hero', () => {
    useCleanStore.setState({ status: 'done', summary: summaryWithEperm, notBackedUp: [] })
    useUiStore.setState({ view: 'category' })
    render(<ResultsPanel />)
    fireEvent.click(screen.getByRole('button', { name: /^done$/i }))
    expect(useCleanStore.getState().status).toBe('idle')
    expect(useScanStore.getState().status).toBe('idle')
    expect(useUiStore.getState().view).toBe('smart-scan')
  })
})
```

- [ ] **Step 10: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/ResultsPanel.test.tsx`
Expected: FAIL — `Failed to resolve import "./ResultsPanel"`.

- [ ] **Step 11: Write ResultsPanel, ProgressOverlay and CleanFlow**

Create `frontend/src/components/ResultsPanel.tsx`:

```tsx
import { OpenFDASettings } from '../../wailsjs/go/main/App'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import type { CleanSummary } from '../lib/types'

export function needsFdaHint(summary: CleanSummary | undefined): boolean {
  if (!summary) return false
  return summary.results.some((r) =>
    (r.errors ?? []).some((e) => e.includes('EPERM') || e.includes('EACCES')),
  )
}

export function ResultsPanel() {
  const summary = useCleanStore((s) => s.summary)
  const notBackedUp = useCleanStore((s) => s.notBackedUp)
  const cancelled = useCleanStore((s) => s.cancelled)
  const error = useCleanStore((s) => s.error)
  const lastDryRun = useCleanStore((s) => s.lastDryRun)

  const onDone = () => {
    useCleanStore.getState().reset()
    useScanStore.getState().reset()
    useUiStore.getState().setView('smart-scan')
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[520px] max-h-[85vh] overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <p className="text-3xl font-semibold text-green-600 dark:text-green-400">
          {formatSize(summary?.totalFreedSpace ?? 0)} {lastDryRun ? 'would be freed (dry run)' : 'freed'}
        </p>
        {cancelled ? (
          <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">
            Cancelled — partial results below.
          </p>
        ) : null}
        {error ? <p className="mt-1 text-sm text-red-600 dark:text-red-400">{error}</p> : null}

        <ul className="mt-4 space-y-2">
          {(summary?.results ?? []).map((r) => (
            <li key={r.category.id} className="text-sm">
              <span className={r.errors.length === 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                {r.errors.length === 0 ? `✓ ${r.category.name}` : `✗ ${r.category.name}`}
              </span>{' '}
              <span className="text-zinc-500 dark:text-zinc-400">
                {r.cleanedItems} item{r.cleanedItems === 1 ? '' : 's'} · {formatSize(r.freedSpace)}
              </span>
              {r.errors.map((e) => (
                <div key={e}>
                  <p className="mt-0.5 text-xs text-red-600 dark:text-red-400">{e}</p>
                  {e.includes('PROTECTED') ? (
                    <p className="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
                      Some items are blocked for safety (system-protected paths)
                    </p>
                  ) : null}
                </div>
              ))}
            </li>
          ))}
        </ul>

        {needsFdaHint(summary) ? (
          <div className="mt-4 rounded-md bg-amber-50 p-3 text-sm dark:bg-amber-950">
            <p className="text-amber-800 dark:text-amber-200">
              Some items could not be removed because App Cleaner lacks Full Disk Access.
            </p>
            <button
              className="mt-2 rounded-md bg-amber-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-amber-500"
              onClick={() => void OpenFDASettings()}
            >
              Grant Full Disk Access
            </button>
          </div>
        ) : null}

        {notBackedUp.length > 0 ? (
          <details className="mt-4 text-sm">
            <summary className="cursor-pointer text-zinc-600 dark:text-zinc-300">
              {notBackedUp.length} item(s) not backed up (outside home / other volume)
            </summary>
            <ul className="mt-2 space-y-1">
              {notBackedUp.map((p) => (
                <li key={p} className="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">
                  {p}
                </li>
              ))}
            </ul>
          </details>
        ) : null}

        <div className="mt-6 flex justify-end">
          <button
            className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
            onClick={onDone}
          >
            Done
          </button>
        </div>
      </div>
    </div>
  )
}
```

Create `frontend/src/components/ProgressOverlay.tsx` (prop-driven so Task 26 can reuse it):

```tsx
export interface ProgressOverlayProps {
  title: string
  current: number
  total: number
  itemName: string
  onCancel?: () => void
}

export function ProgressOverlay({ title, current, total, itemName, onCancel }: ProgressOverlayProps) {
  const pct = total > 0 ? Math.min(100, Math.round((current / total) * 100)) : 0
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[440px] rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <h2 className="text-lg font-semibold">{title}</h2>
        <p className="mt-1 truncate text-sm text-zinc-500 dark:text-zinc-400">{itemName || '…'}</p>
        <div className="mt-4 h-2 w-full overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-700">
          <div
            className="h-full rounded-full bg-indigo-600 transition-all"
            style={{ width: `${pct}%` }}
          />
        </div>
        <p className="mt-2 text-xs text-zinc-500 dark:text-zinc-400">
          {current} / {total}
        </p>
        {onCancel ? (
          <div className="mt-4 flex justify-end">
            <button
              className="rounded-md px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
              onClick={onCancel}
            >
              Cancel
            </button>
          </div>
        ) : null}
      </div>
    </div>
  )
}
```

Create `frontend/src/components/CleanFlow.tsx` (mounts the right clean-flow layer for the store status):

```tsx
import { useCleanStore } from '../stores/cleanStore'
import { ConfirmModal } from './ConfirmModal'
import { ProgressOverlay } from './ProgressOverlay'
import { ResultsPanel } from './ResultsPanel'

export function CleanFlow() {
  const status = useCleanStore((s) => s.status)
  const progress = useCleanStore((s) => s.progress)
  const lastDryRun = useCleanStore((s) => s.lastDryRun)
  const backingUp = useCleanStore((s) => s.backingUp)
  const cancelClean = useCleanStore((s) => s.cancelClean)

  if (status === 'confirming') return <ConfirmModal />
  if (status === 'cleaning')
    return (
      <ProgressOverlay
        title={lastDryRun ? 'Dry run…' : 'Cleaning…'}
        current={progress.current}
        total={progress.total}
        itemName={backingUp ? `Backing up: ${progress.itemName}` : progress.itemName}
        onCancel={cancelClean}
      />
    )
  if (status === 'done') return <ResultsPanel />
  return null
}
```

Modify `frontend/src/App.tsx` (created in the frontend-shell task): add

```tsx
import { CleanFlow } from './components/CleanFlow'
```

to the component imports, and render `<CleanFlow />` as the **last child of the outermost wrapper element** returned by `App()` (immediately before its closing tag), so the confirm/progress/results layers overlay every view:

```tsx
      {/* ...existing sidebar + view switch stay untouched... */}
      <CleanFlow />
    </div>
  )
```

- [ ] **Step 12: Run all Task 25 tests and the typecheck**

Run: `cd frontend && npx vitest run src/stores/cleanStore.test.ts src/components/ConfirmModal.test.tsx src/components/ResultsPanel.test.tsx && npm run typecheck`
Expected: PASS (3 files, 21 tests — 8 cleanStore + 7 ConfirmModal + 6 ResultsPanel) and `tsc --noEmit` exits 0 with no output.

- [ ] **Step 13: Commit**

```bash
git add frontend/src/stores/cleanStore.ts frontend/src/stores/cleanStore.test.ts \
  frontend/src/components/ConfirmModal.tsx frontend/src/components/ConfirmModal.test.tsx \
  frontend/src/components/ProgressOverlay.tsx frontend/src/components/ResultsPanel.tsx \
  frontend/src/components/ResultsPanel.test.tsx frontend/src/components/CleanFlow.tsx \
  frontend/src/App.tsx
git commit -m "feat(frontend): clean flow — confirm modal, progress overlay, results panel" \
  -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 26: Uninstaller view

**Files:**
- Create: `frontend/src/lib/uninstallMath.ts`
- Create: `frontend/src/components/UninstallConfirm.tsx`
- Create: `frontend/src/views/Uninstaller.tsx` (replaces the placeholder from the frontend-shell task if one exists — same path, keep both named and default exports so either import style in `App.tsx` keeps working)
- Modify: `frontend/src/App.tsx` (mount the real view for `view === 'uninstaller'` — Step 8)
- Test: `frontend/src/lib/uninstallMath.test.ts`
- Test: `frontend/src/views/Uninstaller.test.tsx`

**Interfaces:**
- Consumes (bindings): `ListApps(): Promise<AppInfo[]>`, `StartUninstall(names: string[], dryRun: boolean): Promise<void>`, `GetAppIcon(path: string): Promise<string>` (base64 PNG or `''` on failure; bound method per the contract) from `../../wailsjs/go/main/App`; `EventsOn`/`EventsOff` from `../../wailsjs/runtime/runtime`; events `uninstall:progress {current, total, appName}` and `uninstall:done {uninstalled, freedSpace, errors, cancelled?, error?}`
- Consumes (earlier tasks): `formatSize` from `lib/format`, `AppInfo` type from `lib/types` (`{name, path, bundleId, appSize, relatedPaths: {path, size}[], totalSize, running}`); `ProgressOverlay` from Task 25 (rendered **without** `onCancel` — the contract exposes no CancelUninstall binding, so the overlay shows no Cancel button here)
- Produces (package-local, exported for tests): `contractHome(p: string): string`, `selectionTotals(apps: AppInfo[], names: ReadonlySet<string>): {size: number; paths: number}`, `anySelectedRunning(apps: AppInfo[], names: ReadonlySet<string>): boolean` in `lib/uninstallMath.ts`; `UninstallConfirm` component with props `{apps: AppInfo[]; onCancel: () => void; onConfirm: (dryRun: boolean) => void}`

- [ ] **Step 1: Write the failing math-helper test**

Create `frontend/src/lib/uninstallMath.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { contractHome, selectionTotals, anySelectedRunning } from './uninstallMath'
import type { AppInfo } from './types'

const apps: AppInfo[] = [
  {
    name: 'OldApp',
    path: '/Applications/OldApp.app',
    bundleId: 'com.old.app',
    appSize: 100_000_000,
    relatedPaths: [
      { path: '/Users/me/Library/Caches/OldApp', size: 30_000_000 },
      { path: '/Users/me/Library/Preferences/com.old.app.plist', size: 20_000_000 },
    ],
    totalSize: 150_000_000,
    running: false,
  },
  {
    name: 'BusyApp',
    path: '/Applications/BusyApp.app',
    bundleId: 'com.busy.app',
    appSize: 200_000_000,
    relatedPaths: [],
    totalSize: 200_000_000,
    running: true,
  },
]

describe('selectionTotals', () => {
  it('total = Σ(bundle + related) per selected app; paths = Σ(1 + related count)', () => {
    const one = selectionTotals(apps, new Set(['OldApp']))
    expect(one.size).toBe(100_000_000 + 30_000_000 + 20_000_000) // bundle + related
    expect(one.size).toBe(apps[0].totalSize) // consistent with engine-computed totalSize
    expect(one.paths).toBe(3) // 1 bundle + 2 related

    const both = selectionTotals(apps, new Set(['OldApp', 'BusyApp']))
    expect(both.size).toBe(350_000_000)
    expect(both.paths).toBe(4)

    expect(selectionTotals(apps, new Set())).toEqual({ size: 0, paths: 0 })
  })
})

describe('anySelectedRunning', () => {
  it('is true only when a selected app is running', () => {
    expect(anySelectedRunning(apps, new Set(['OldApp']))).toBe(false)
    expect(anySelectedRunning(apps, new Set(['OldApp', 'BusyApp']))).toBe(true)
    expect(anySelectedRunning(apps, new Set())).toBe(false)
  })
})

describe('contractHome', () => {
  it('contracts /Users/<name> to ~ only as a leading segment', () => {
    expect(contractHome('/Users/me/Library/Caches/OldApp')).toBe('~/Library/Caches/OldApp')
    expect(contractHome('/Applications/OldApp.app')).toBe('/Applications/OldApp.app')
    expect(contractHome('/Volumes/Users/me/x')).toBe('/Volumes/Users/me/x')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/uninstallMath.test.ts`
Expected: FAIL — `Failed to resolve import "./uninstallMath"`.

- [ ] **Step 3: Write the math helpers**

Create `frontend/src/lib/uninstallMath.ts`:

```ts
import type { AppInfo } from './types'

// Display-only ~ contraction: the engine only ever returns $HOME-contained
// related paths, so a leading /Users/<name> segment is the home dir.
export function contractHome(p: string): string {
  return p.replace(/^\/Users\/[^/]+(?=\/|$)/, '~')
}

export function selectionTotals(
  apps: AppInfo[],
  names: ReadonlySet<string>,
): { size: number; paths: number } {
  let size = 0
  let paths = 0
  for (const app of apps) {
    if (!names.has(app.name)) continue
    size += app.appSize + app.relatedPaths.reduce((acc, r) => acc + r.size, 0)
    paths += 1 + app.relatedPaths.length
  }
  return { size, paths }
}

export function anySelectedRunning(apps: AppInfo[], names: ReadonlySet<string>): boolean {
  return apps.some((a) => names.has(a.name) && a.running)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/lib/uninstallMath.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Write the failing view test (running-app gating)**

Create `frontend/src/views/Uninstaller.test.tsx`:

```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  ListApps: vi.fn().mockResolvedValue([]),
  StartUninstall: vi.fn().mockResolvedValue(undefined),
  IsAppRunning: vi.fn().mockResolvedValue(false),
  GetAppIcon: vi.fn().mockResolvedValue(''),
}))

import { ListApps, StartUninstall, GetAppIcon } from '../../wailsjs/go/main/App'
import { Uninstaller } from './Uninstaller'
import type { AppInfo } from '../lib/types'

const ListAppsMock = ListApps as unknown as ReturnType<typeof vi.fn>
const StartUninstallMock = StartUninstall as unknown as ReturnType<typeof vi.fn>
const GetAppIconMock = GetAppIcon as unknown as ReturnType<typeof vi.fn>

const apps: AppInfo[] = [
  {
    name: 'OldApp',
    path: '/Applications/OldApp.app',
    bundleId: 'com.old.app',
    appSize: 100_000_000,
    relatedPaths: [{ path: '/Users/me/Library/Caches/OldApp', size: 50_000_000 }],
    totalSize: 150_000_000,
    running: false,
  },
  {
    name: 'BusyApp',
    path: '/Applications/BusyApp.app',
    bundleId: 'com.busy.app',
    appSize: 200_000_000,
    relatedPaths: [],
    totalSize: 200_000_000,
    running: true,
  },
]

beforeEach(() => {
  ListAppsMock.mockClear()
  ListAppsMock.mockResolvedValue(apps)
  StartUninstallMock.mockClear()
  GetAppIconMock.mockReset()
  GetAppIconMock.mockResolvedValue('')
})

afterEach(() => cleanup())

describe('<Uninstaller />', () => {
  it('lists apps with size, related chip and Running badge', async () => {
    render(<Uninstaller />)
    expect(await screen.findByText('OldApp')).toBeDefined()
    expect(screen.getByText('143.1 MB')).toBeDefined() // formatSize(150_000_000)
    expect(screen.getByText('+1 related')).toBeDefined()
    expect(screen.getByText('Running')).toBeDefined() // BusyApp badge
  })

  it('loads row icons lazily: PNG from GetAppIcon, letter-avatar fallback when empty', async () => {
    GetAppIconMock.mockResolvedValueOnce('iVBORfakePNG') // first row to mount = OldApp
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    const img = (await screen.findByAltText('OldApp icon')) as HTMLImageElement
    expect(img.src).toBe('data:image/png;base64,iVBORfakePNG')
    expect(GetAppIconMock).toHaveBeenCalledWith('/Applications/OldApp.app')
    expect(GetAppIconMock).toHaveBeenCalledWith('/Applications/BusyApp.app')
    expect(screen.getByText('B')).toBeDefined() // BusyApp returned '' → letter avatar
  })

  it('disables Uninstall with a "Quit the app first" tooltip while a selected app is running', async () => {
    render(<Uninstaller />)
    await screen.findByText('BusyApp')
    fireEvent.click(screen.getByLabelText('Select BusyApp'))
    const btn = screen.getByRole('button', { name: /uninstall 1 app/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
    expect(btn.title).toBe('Quit the app first')
  })

  it('enables Uninstall for a non-running selection and confirms into StartUninstall', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    fireEvent.click(screen.getByLabelText('Select OldApp'))
    const btn = screen.getByRole('button', { name: /uninstall 1 app/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(false)
    fireEvent.click(btn)
    // bespoke confirm modal: apps + related paths + total
    expect(await screen.findByText(/~\/Library\/Caches\/OldApp/)).toBeDefined()
    expect(screen.getByText(/143\.1 MB will be freed \(2 items\)/)).toBeDefined()
    fireEvent.click(screen.getByLabelText(/dry run/i))
    fireEvent.click(screen.getByRole('button', { name: /^uninstall$/i }))
    expect(StartUninstallMock).toHaveBeenCalledWith(['OldApp'], true)
  })

  it('Re-check calls ListApps again', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    expect(ListAppsMock).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: /re-check/i }))
    expect(ListAppsMock).toHaveBeenCalledTimes(2)
  })
})
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/views/Uninstaller.test.tsx`
Expected: FAIL — either `Failed to resolve import "./Uninstaller"` (fresh file) or, if a shell-task placeholder exists, failed assertions (`Unable to find an element with the text: OldApp`).

- [ ] **Step 7: Write UninstallConfirm and the Uninstaller view**

Create `frontend/src/components/UninstallConfirm.tsx`:

```tsx
import { useState } from 'react'
import { formatSize } from '../lib/format'
import { contractHome, selectionTotals } from '../lib/uninstallMath'
import type { AppInfo } from '../lib/types'

export interface UninstallConfirmProps {
  apps: AppInfo[] // the selected apps only
  onCancel: () => void
  onConfirm: (dryRun: boolean) => void
}

export function UninstallConfirm({ apps, onCancel, onConfirm }: UninstallConfirmProps) {
  const [dryRun, setDryRun] = useState(false)
  const totals = selectionTotals(apps, new Set(apps.map((a) => a.name)))

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[520px] max-h-[80vh] overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <h2 className="text-lg font-semibold">Uninstall applications</h2>
        <ul className="mt-4 space-y-3">
          {apps.map((app) => (
            <li key={app.name} className="text-sm">
              <span className="font-medium">✗ {app.name}</span>{' '}
              <span className="text-zinc-500 dark:text-zinc-400">({formatSize(app.totalSize)})</span>
              <ul className="mt-1 space-y-0.5">
                {app.relatedPaths.map((r) => (
                  <li key={r.path} className="truncate pl-4 font-mono text-xs text-zinc-500 dark:text-zinc-400">
                    └─ {contractHome(r.path)} ({formatSize(r.size)})
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
        <p className="mt-4 text-sm font-medium">
          Total: {formatSize(totals.size)} will be freed ({totals.paths} items)
        </p>
        <label className="mt-4 flex items-center gap-2 text-sm">
          <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />
          Dry run (preview only, nothing is deleted)
        </label>
        <div className="mt-6 flex justify-end gap-3">
          <button
            className="rounded-md px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500"
            onClick={() => onConfirm(dryRun)}
          >
            Uninstall
          </button>
        </div>
      </div>
    </div>
  )
}
```

Create (or replace the placeholder) `frontend/src/views/Uninstaller.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react'
import { ListApps, StartUninstall, GetAppIcon } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { formatSize } from '../lib/format'
import { anySelectedRunning, contractHome } from '../lib/uninstallMath'
import { ProgressOverlay } from '../components/ProgressOverlay'
import { UninstallConfirm } from '../components/UninstallConfirm'
import type { AppInfo } from '../lib/types'

interface UninstallDone {
  uninstalled: number
  freedSpace: number
  errors: string[]
  cancelled?: boolean
  error?: string
}

type Phase = 'list' | 'confirm' | 'running' | 'done'

interface RowIconProps {
  path: string
  name: string
  icon: string | undefined // undefined = not fetched yet; '' = fetched, none available
  onLoaded: (path: string, icon: string) => void
}

// Lazy per-row icon: GetAppIcon(path) is called once when the row first mounts;
// the result (base64 PNG or '') is cached in the parent's icons map so re-renders
// and re-mounts never re-fetch. Empty icon => letter avatar with the app initial.
function RowIcon({ path, name, icon, onLoaded }: RowIconProps) {
  useEffect(() => {
    if (icon === undefined) {
      GetAppIcon(path)
        .then((b64) => onLoaded(path, b64 ?? ''))
        .catch(() => onLoaded(path, ''))
    }
  }, [path, icon, onLoaded])
  if (icon) {
    return (
      <img
        src={`data:image/png;base64,${icon}`}
        alt={`${name} icon`}
        className="h-8 w-8 rounded-md"
      />
    )
  }
  return (
    <div className="flex h-8 w-8 items-center justify-center rounded-md bg-zinc-200 text-sm font-semibold text-zinc-600 dark:bg-zinc-700 dark:text-zinc-200">
      {name.charAt(0).toUpperCase()}
    </div>
  )
}

export function Uninstaller() {
  const [apps, setApps] = useState<AppInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [icons, setIcons] = useState<Record<string, string>>({})
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [expanded, setExpanded] = useState<string | null>(null)
  const [phase, setPhase] = useState<Phase>('list')
  const [progress, setProgress] = useState({ current: 0, total: 0, appName: '' })
  const [done, setDone] = useState<UninstallDone | null>(null)
  const [startError, setStartError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setApps((await ListApps()) ?? [])
    } finally {
      setLoading(false)
    }
  }, [])

  const cacheIcon = useCallback((path: string, icon: string) => {
    setIcons((prev) => (path in prev ? prev : { ...prev, [path]: icon }))
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    EventsOn('uninstall:progress', (d: { current: number; total: number; appName: string }) => {
      setPhase('running')
      setProgress(d)
    })
    EventsOn('uninstall:done', (d: UninstallDone) => {
      setDone(d)
      setPhase('done')
    })
    return () => {
      EventsOff('uninstall:progress')
      EventsOff('uninstall:done')
    }
  }, [])

  const toggle = (name: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })

  const blocked = anySelectedRunning(apps, selected)
  const selectedApps = apps.filter((a) => selected.has(a.name))

  const confirmUninstall = async (dryRun: boolean) => {
    setStartError(null)
    setPhase('running')
    setProgress({ current: 0, total: selected.size, appName: '' })
    try {
      await StartUninstall([...selected], dryRun)
    } catch (e) {
      setStartError(String(e))
      setPhase('list')
    }
  }

  const finish = () => {
    setPhase('list')
    setDone(null)
    setSelected(new Set())
    void refresh()
  }

  return (
    <div className="flex h-full flex-col p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Uninstaller</h1>
        <button
          className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800"
          onClick={() => void refresh()}
        >
          Re-check
        </button>
      </div>
      {startError ? (
        <p className="mt-2 text-sm text-red-600 dark:text-red-400">{startError}</p>
      ) : null}

      <div className="mt-4 flex-1 overflow-y-auto">
        {loading ? (
          <p className="text-sm text-zinc-500 dark:text-zinc-400">Scanning installed applications…</p>
        ) : (
          <ul className="divide-y divide-zinc-200 dark:divide-zinc-800">
            {apps.map((app) => (
              <li key={app.name} className="py-2">
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    aria-label={`Select ${app.name}`}
                    checked={selected.has(app.name)}
                    onChange={() => toggle(app.name)}
                  />
                  <RowIcon path={app.path} name={app.name} icon={icons[app.path]} onLoaded={cacheIcon} />
                  <button
                    className="flex-1 truncate text-left text-sm font-medium"
                    onClick={() => setExpanded(expanded === app.name ? null : app.name)}
                  >
                    {app.name}
                  </button>
                  {app.running ? (
                    <span className="rounded-full bg-red-100 px-2 py-0.5 text-xs text-red-700 dark:bg-red-950 dark:text-red-300">
                      Running
                    </span>
                  ) : null}
                  {app.relatedPaths.length > 0 ? (
                    <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                      +{app.relatedPaths.length} related
                    </span>
                  ) : null}
                  <span className="w-24 text-right text-sm text-zinc-500 dark:text-zinc-400">
                    {formatSize(app.totalSize)}
                  </span>
                </div>
                {expanded === app.name ? (
                  <ul className="mt-2 space-y-0.5 pl-8">
                    <li className="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">
                      {app.path} ({formatSize(app.appSize)})
                    </li>
                    {app.relatedPaths.map((r) => (
                      <li key={r.path} className="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">
                        {contractHome(r.path)} ({formatSize(r.size)})
                      </li>
                    ))}
                  </ul>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="mt-4 flex justify-end border-t border-zinc-200 pt-4 dark:border-zinc-800">
        <button
          className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500 disabled:opacity-40"
          disabled={selected.size === 0 || blocked}
          title={blocked ? 'Quit the app first' : undefined}
          onClick={() => setPhase('confirm')}
        >
          Uninstall {selected.size} app{selected.size === 1 ? '' : 's'}
        </button>
      </div>

      {phase === 'confirm' ? (
        <UninstallConfirm
          apps={selectedApps}
          onCancel={() => setPhase('list')}
          onConfirm={(dryRun) => void confirmUninstall(dryRun)}
        />
      ) : null}

      {phase === 'running' ? (
        <ProgressOverlay
          title="Uninstalling…"
          current={progress.current}
          total={progress.total}
          itemName={progress.appName}
        />
      ) : null}

      {phase === 'done' && done ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
          <div className="w-[480px] rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
            <p className="text-2xl font-semibold text-green-600 dark:text-green-400">
              {done.uninstalled} app{done.uninstalled === 1 ? '' : 's'} uninstalled ·{' '}
              {formatSize(done.freedSpace)} freed
            </p>
            {done.cancelled ? (
              <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">Cancelled — partial results.</p>
            ) : null}
            {done.error ? (
              <p className="mt-1 text-sm text-red-600 dark:text-red-400">{done.error}</p>
            ) : null}
            {done.errors?.length ? (
              <ul className="mt-3 space-y-1">
                {done.errors.map((e) => (
                  <li key={e} className="text-xs text-red-600 dark:text-red-400">
                    ✗ {e}
                  </li>
                ))}
              </ul>
            ) : null}
            <div className="mt-6 flex justify-end">
              <button
                className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
                onClick={finish}
              >
                Done
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}

export default Uninstaller
```

- [ ] **Step 8: Mount the view in App.tsx**

Modify `frontend/src/App.tsx` (created in the frontend-shell task): add

```tsx
import Uninstaller from './views/Uninstaller'
```

to the view imports, and in the `App()` view switch replace the placeholder JSX rendered for the uninstaller view with the real component, so the branch reads:

```tsx
      {view === 'uninstaller' && <Uninstaller />}
```

Everything else in `App.tsx` (sidebar, the other view branches, `<CleanFlow />`) stays untouched.

- [ ] **Step 9: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/uninstallMath.test.ts src/views/Uninstaller.test.tsx && npm run typecheck`
Expected: PASS (2 files, 8 tests — 3 uninstallMath + 5 Uninstaller); `tsc --noEmit` exits 0.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/lib/uninstallMath.ts frontend/src/lib/uninstallMath.test.ts \
  frontend/src/components/UninstallConfirm.tsx frontend/src/views/Uninstaller.tsx \
  frontend/src/views/Uninstaller.test.tsx frontend/src/App.tsx
git commit -m "feat(frontend): uninstaller view with running-app gating" \
  -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 27: Maintenance, Backups and Settings views

**Files:**
- Create: `frontend/src/lib/settings.ts`
- Create: `frontend/src/views/Maintenance.tsx` (replaces shell-task placeholder if present; keep named + default exports)
- Create: `frontend/src/views/Backups.tsx` (same)
- Create: `frontend/src/views/Settings.tsx` (same)
- Modify: `frontend/src/App.tsx` (mount the three real views — Step 6)
- Test: `frontend/src/lib/settings.test.ts`

**Interfaces:**
- Consumes (bindings): `RunMaintenance(task: string): Promise<MaintenanceResult>` (`'dns' | 'purge'`, synchronous promise), `StartTMSnapshotsClear(): Promise<void>`, `CancelMaintenance(): Promise<void>`, `CleanOldBackups(): Promise<number>` (retention sweep; resolves to the number of expired backups removed), `ListBackups(): Promise<BackupInfo[]>`, `RestoreBackup(path: string): Promise<RestoreResult>`, `DeleteBackup(path: string): Promise<void>` (rejects on error), `GetConfig()` / `SaveConfig(c)` (via `useUiStore.loadConfig` / `useUiStore.saveConfig(c: Config): Promise<void>`); events `maintenance:progress {done, total, date, error?}`, `maintenance:done {result}`
- Consumes (earlier tasks): `formatSize` from `lib/format`; types `MaintenanceResult {success, message, error?, requiresAdmin}`, `BackupInfo {path, date, size}` (`date` is the RFC3339 string a Go `time.Time` marshals to), `RestoreResult {restored, failed, errors}`, `Config` from `lib/types`; `useUiStore` (`config?`, `loadConfig`, `saveConfig`)
- Produces (package-local, exported for tests, in `lib/settings.ts`): `clampInt(n, min, max, fallback): number`, `mbToBytes(mb): number`, `bytesToMb(bytes): number`, `parseKeepLanguages(input): string[]`, `formatKeepLanguages(langs): string`, `parseLines(input): string[]`, `SettingsForm` interface, `toForm(c: Config): SettingsForm`, `fromForm(f: SettingsForm): Config`

- [ ] **Step 1: Write the failing settings-helpers test**

Create `frontend/src/lib/settings.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import {
  clampInt,
  mbToBytes,
  bytesToMb,
  parseKeepLanguages,
  formatKeepLanguages,
  parseLines,
  toForm,
  fromForm,
} from './settings'
import type { Config } from './types'

describe('MB <-> bytes conversion', () => {
  it('converts and clamps to the config bounds (1..102400 MB)', () => {
    expect(mbToBytes(500)).toBe(524288000)
    expect(bytesToMb(524288000)).toBe(500)
    expect(mbToBytes(0)).toBe(1048576) // clamped up to 1 MB
    expect(mbToBytes(999999)).toBe(102400 * 1048576) // clamped to 100 GiB
    expect(mbToBytes(Number.NaN)).toBe(500 * 1048576) // fallback = 500 MB default
  })

  it('round-trips', () => {
    expect(bytesToMb(mbToBytes(123))).toBe(123)
  })
})

describe('clampInt', () => {
  it('clamps, rounds and falls back on non-finite input', () => {
    expect(clampInt(0, 1, 365, 30)).toBe(1)
    expect(clampInt(400, 1, 365, 30)).toBe(365)
    expect(clampInt(4.6, 1, 16, 4)).toBe(5)
    expect(clampInt(Number.NaN, 1, 16, 4)).toBe(4)
  })
})

describe('keepLanguages parsing', () => {
  it('parses comma-separated tags, trimming, dropping empties and deduping', () => {
    expect(parseKeepLanguages('pt-BR, de , ,fr,de')).toEqual(['pt-BR', 'de', 'fr'])
    expect(parseKeepLanguages('')).toEqual([])
  })

  it('round-trips through format + parse', () => {
    const langs = ['pt-BR', 'de', 'fr']
    expect(parseKeepLanguages(formatKeepLanguages(langs))).toEqual(langs)
  })
})

describe('form mapping', () => {
  const cfg: Config = {
    downloadsDaysOld: 45,
    largeFilesMinSize: 524288000,
    backupByDefault: true,
    backupRetentionDays: 14,
    concurrency: 8,
    showRisky: true,
    keepLanguages: ['pt-BR'],
    extraPaths: { nodeModules: ['/Users/me/dev'], projects: ['/Users/me/work'] },
  }

  it('fromForm(toForm(cfg)) round-trips an in-bounds config', () => {
    expect(fromForm(toForm(cfg))).toEqual(cfg)
  })

  it('parseLines splits textarea input, dropping blanks and capping at 50', () => {
    expect(parseLines('/a\n\n  /b  \n')).toEqual(['/a', '/b'])
    expect(parseLines(Array.from({ length: 60 }, (_, i) => `/p${i}`).join('\n'))).toHaveLength(50)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/settings.test.ts`
Expected: FAIL — `Failed to resolve import "./settings"`.

- [ ] **Step 3: Write the settings helpers**

Create `frontend/src/lib/settings.ts`:

```ts
import type { Config } from './types'

export const MB = 1024 * 1024

// UI mirror of the internal/config bounds: invalid input falls back, out-of-range clamps.
export function clampInt(n: number, min: number, max: number, fallback: number): number {
  if (!Number.isFinite(n)) return fallback
  const i = Math.round(n)
  return Math.min(max, Math.max(min, i))
}

// largeFilesMinSize is edited in whole MB: 1 MB .. 102400 MB (= 100 GiB, the config ceiling).
export function mbToBytes(mb: number): number {
  return clampInt(mb, 1, 102400, 500) * MB
}

export function bytesToMb(bytes: number): number {
  return clampInt(bytes / MB, 1, 102400, 500)
}

export function parseKeepLanguages(input: string): string[] {
  const out: string[] = []
  for (const raw of input.split(',')) {
    const t = raw.trim()
    if (t && !out.includes(t)) out.push(t)
  }
  return out
}

export function formatKeepLanguages(langs: string[]): string {
  return langs.join(', ')
}

// One path per line; config caps extraPaths arrays at 50 entries each.
export function parseLines(input: string): string[] {
  return input
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l.length > 0)
    .slice(0, 50)
}

export interface SettingsForm {
  downloadsDaysOld: number
  largeFilesMB: number
  backupByDefault: boolean
  backupRetentionDays: number
  concurrency: number
  showRisky: boolean
  keepLanguages: string // comma-separated
  nodeModulesPaths: string // one path per line
  projectsPaths: string // one path per line
}

export function toForm(c: Config): SettingsForm {
  return {
    downloadsDaysOld: c.downloadsDaysOld,
    largeFilesMB: bytesToMb(c.largeFilesMinSize),
    backupByDefault: c.backupByDefault,
    backupRetentionDays: c.backupRetentionDays,
    concurrency: c.concurrency,
    showRisky: c.showRisky,
    keepLanguages: formatKeepLanguages(c.keepLanguages ?? []),
    nodeModulesPaths: (c.extraPaths?.nodeModules ?? []).join('\n'),
    projectsPaths: (c.extraPaths?.projects ?? []).join('\n'),
  }
}

export function fromForm(f: SettingsForm): Config {
  return {
    downloadsDaysOld: clampInt(f.downloadsDaysOld, 1, 365, 30),
    largeFilesMinSize: mbToBytes(f.largeFilesMB),
    backupByDefault: f.backupByDefault,
    backupRetentionDays: clampInt(f.backupRetentionDays, 1, 365, 7),
    concurrency: clampInt(f.concurrency, 1, 16, 4),
    showRisky: f.showRisky,
    keepLanguages: parseKeepLanguages(f.keepLanguages),
    extraPaths: {
      nodeModules: parseLines(f.nodeModulesPaths),
      projects: parseLines(f.projectsPaths),
    },
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/lib/settings.test.ts`
Expected: PASS (7 tests).

- [ ] **Step 5: Write the three views**

Create (or replace the placeholder) `frontend/src/views/Maintenance.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { RunMaintenance, StartTMSnapshotsClear, CancelMaintenance } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import type { MaintenanceResult } from '../lib/types'

interface TMDate {
  date: string
  error?: string
}

function AdminBadge() {
  return (
    <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-800 dark:bg-amber-900 dark:text-amber-200">
      Requires administrator
    </span>
  )
}

// Failed results that required admin get a "Retry with administrator" CTA so the
// user can re-trigger the task (and its osascript admin prompt) directly.
function ResultLine({ result, onRetry }: { result: MaintenanceResult; onRetry?: () => void }) {
  if (result.success) {
    return <p className="mt-2 text-sm text-green-600 dark:text-green-400">✓ {result.message}</p>
  }
  return (
    <div className="mt-2">
      <p className="text-sm text-red-600 dark:text-red-400">
        ✗ {result.message}
        {result.error ? ` — ${result.error}` : ''}
      </p>
      {result.requiresAdmin && onRetry ? (
        <button
          className="mt-1.5 rounded-md bg-amber-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-500"
          onClick={onRetry}
        >
          Retry with administrator
        </button>
      ) : null}
    </div>
  )
}

function Spinner() {
  return <p className="mt-2 animate-pulse text-sm text-zinc-500 dark:text-zinc-400">Running…</p>
}

export function Maintenance() {
  const [dns, setDns] = useState<{ running: boolean; result?: MaintenanceResult }>({ running: false })
  const [purge, setPurge] = useState<{ running: boolean; result?: MaintenanceResult }>({ running: false })
  const [tm, setTm] = useState<{ running: boolean; result?: MaintenanceResult; dates: TMDate[] }>({
    running: false,
    dates: [],
  })

  useEffect(() => {
    EventsOn('maintenance:progress', (d: { done: number; total: number; date: string; error?: string }) => {
      setTm((prev) => ({ ...prev, running: true, dates: [...prev.dates, { date: d.date, error: d.error }] }))
    })
    EventsOn('maintenance:done', (d: { result: MaintenanceResult }) => {
      setTm((prev) => ({ ...prev, running: false, result: d.result }))
    })
    return () => {
      EventsOff('maintenance:progress')
      EventsOff('maintenance:done')
    }
  }, [])

  const run = async (task: 'dns' | 'purge') => {
    const set = task === 'dns' ? setDns : setPurge
    set({ running: true })
    try {
      set({ running: false, result: await RunMaintenance(task) })
    } catch (e) {
      set({
        running: false,
        result: { success: false, message: 'Task failed', error: String(e), requiresAdmin: false },
      })
    }
  }

  const runTM = async () => {
    setTm({ running: true, dates: [] })
    try {
      await StartTMSnapshotsClear()
    } catch (e) {
      setTm({
        running: false,
        dates: [],
        result: { success: false, message: 'Could not start', error: String(e), requiresAdmin: true },
      })
    }
  }

  const card = 'rounded-xl border border-zinc-200 p-5 dark:border-zinc-800'
  const runBtn =
    'mt-3 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-40'

  return (
    <div className="space-y-4 p-6">
      <h1 className="text-xl font-semibold">Maintenance</h1>

      <div className={card}>
        <div className="flex items-center gap-2">
          <h2 className="font-medium">Flush DNS Cache</h2>
          <AdminBadge />
        </div>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Clears the macOS DNS resolver cache (dscacheutil + mDNSResponder). Fixes stale DNS lookups.
        </p>
        <button className={runBtn} disabled={dns.running} onClick={() => void run('dns')}>
          Run
        </button>
        {dns.running ? <Spinner /> : null}
        {dns.result ? <ResultLine result={dns.result} onRetry={() => void run('dns')} /> : null}
      </div>

      <div className={card}>
        <h2 className="font-medium">Free Purgeable Space</h2>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Asks macOS to release purgeable disk space (/usr/sbin/purge). Usually runs without
          privileges — may prompt for admin if the system refuses.
        </p>
        <button className={runBtn} disabled={purge.running} onClick={() => void run('purge')}>
          Run
        </button>
        {purge.running ? <Spinner /> : null}
        {purge.result ? <ResultLine result={purge.result} onRetry={() => void run('purge')} /> : null}
      </div>

      <div className={card}>
        <div className="flex items-center gap-2">
          <h2 className="font-medium">Clear Time Machine Snapshots</h2>
          <AdminBadge />
        </div>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Deletes local Time Machine snapshots (tmutil). One admin prompt deletes all snapshots.
        </p>
        <div className="flex items-center gap-3">
          <button className={runBtn} disabled={tm.running} onClick={() => void runTM()}>
            Run
          </button>
          {tm.running ? (
            <button
              className="mt-3 rounded-md px-3 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
              onClick={() => void CancelMaintenance()}
            >
              Cancel
            </button>
          ) : null}
        </div>
        {tm.running ? <Spinner /> : null}
        {tm.dates.length > 0 ? (
          <ul className="mt-2 space-y-0.5">
            {tm.dates.map((d) => (
              <li key={d.date} className="font-mono text-xs">
                {d.error ? (
                  <span className="text-red-600 dark:text-red-400">✗ {d.date} — {d.error}</span>
                ) : (
                  <span className="text-green-600 dark:text-green-400">✓ {d.date}</span>
                )}
              </li>
            ))}
          </ul>
        ) : null}
        {tm.result ? <ResultLine result={tm.result} onRetry={() => void runTM()} /> : null}
      </div>
    </div>
  )
}

export default Maintenance
```

Create (or replace the placeholder) `frontend/src/views/Backups.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react'
import { ListBackups, RestoreBackup, DeleteBackup, CleanOldBackups } from '../../wailsjs/go/main/App'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import type { BackupInfo } from '../lib/types'

type Pending = { action: 'restore' | 'delete'; path: string } | null

export function Backups() {
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [pending, setPending] = useState<Pending>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [removed, setRemoved] = useState(0)
  const config = useUiStore((s) => s.config)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setBackups((await ListBackups()) ?? [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void (async () => {
      try {
        setRemoved(await CleanOldBackups()) // retention sweep before listing
      } catch {
        setRemoved(0)
      }
      await refresh()
    })()
    if (!useUiStore.getState().config) void useUiStore.getState().loadConfig()
  }, [refresh])

  const execute = async () => {
    if (!pending) return
    const { action, path } = pending
    setPending(null)
    if (action === 'restore') {
      const r = await RestoreBackup(path)
      setMessage(
        r.failed > 0
          ? `Restored ${r.restored} item(s), ${r.failed} failed: ${r.errors[0] ?? ''}`
          : `Restored ${r.restored} item(s)`,
      )
    } else {
      try {
        await DeleteBackup(path)
        setMessage('Backup deleted')
      } catch (e) {
        setMessage(`Delete failed: ${String(e)}`)
      }
    }
    void refresh()
  }

  const smallBtn =
    'rounded-md border border-zinc-300 px-2.5 py-1 text-xs hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800'

  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold">Backups</h1>
      <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
        Backups older than {config?.backupRetentionDays ?? 7} days are removed automatically.
        {removed > 0 ? ` ${removed} expired backup${removed === 1 ? '' : 's'} just removed.` : ''}
      </p>
      {message ? <p className="mt-2 text-sm text-indigo-600 dark:text-indigo-400">{message}</p> : null}

      {loading ? (
        <p className="mt-4 text-sm text-zinc-500 dark:text-zinc-400">Loading…</p>
      ) : backups.length === 0 ? (
        <p className="mt-4 text-sm text-zinc-500 dark:text-zinc-400">
          No backups yet. Backups are created when you clean with “Back up items” enabled.
        </p>
      ) : (
        <ul className="mt-4 divide-y divide-zinc-200 dark:divide-zinc-800">
          {backups.map((b) => (
            <li key={b.path} className="flex items-center gap-4 py-3">
              <span className="flex-1 text-sm font-medium">{new Date(b.date).toLocaleString()}</span>
              <span className="text-sm text-zinc-500 dark:text-zinc-400">{formatSize(b.size)}</span>
              {pending?.path === b.path ? (
                <span className="flex items-center gap-2 text-sm">
                  {pending.action === 'restore' ? 'Restore this backup?' : 'Delete this backup permanently?'}
                  <button className={smallBtn} onClick={() => void execute()}>
                    Confirm
                  </button>
                  <button className={smallBtn} onClick={() => setPending(null)}>
                    Cancel
                  </button>
                </span>
              ) : (
                <span className="flex gap-2">
                  <button className={smallBtn} onClick={() => setPending({ action: 'restore', path: b.path })}>
                    Restore
                  </button>
                  <button
                    className={`${smallBtn} text-red-600 dark:text-red-400`}
                    onClick={() => setPending({ action: 'delete', path: b.path })}
                  >
                    Delete
                  </button>
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export default Backups
```

Create (or replace the placeholder) `frontend/src/views/Settings.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { useUiStore } from '../stores/uiStore'
import { toForm, fromForm } from '../lib/settings'
import type { SettingsForm } from '../lib/settings'

export function Settings() {
  const config = useUiStore((s) => s.config)
  const [form, setForm] = useState<SettingsForm | null>(null)
  const [toast, setToast] = useState<string | null>(null)

  useEffect(() => {
    if (!config) void useUiStore.getState().loadConfig()
    else if (form === null) setForm(toForm(config))
  }, [config, form])

  if (!form) {
    return <p className="p-6 text-sm text-zinc-500 dark:text-zinc-400">Loading settings…</p>
  }

  const patch = (p: Partial<SettingsForm>) => setForm({ ...form, ...p })
  const num = (v: number) => (Number.isFinite(v) ? v : '')

  const save = async () => {
    const cleaned = fromForm(form) // client-side clamping mirrors internal/config bounds
    try {
      await useUiStore.getState().saveConfig(cleaned)
      setForm(toForm(cleaned)) // show the clamped values back
      setToast('Settings saved')
    } catch (e) {
      setToast(`Save failed: ${String(e)}`)
    }
    window.setTimeout(() => setToast(null), 2500)
  }

  const row = 'flex items-center justify-between gap-4 py-3'
  const input =
    'w-28 rounded-md border border-zinc-300 bg-white px-2 py-1 text-sm dark:border-zinc-700 dark:bg-zinc-900'
  const area =
    'mt-1 w-full rounded-md border border-zinc-300 bg-white px-2 py-1 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-900'

  return (
    <div className="max-w-xl p-6">
      <h1 className="text-xl font-semibold">Settings</h1>

      <div className="mt-4 divide-y divide-zinc-200 dark:divide-zinc-800">
        <label className={row}>
          <span className="text-sm">Downloads considered old after (days, 1–365)</span>
          <input
            className={input}
            type="number"
            min={1}
            max={365}
            value={num(form.downloadsDaysOld)}
            onChange={(e) => patch({ downloadsDaysOld: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm">Large-file threshold (MB, 1–102400)</span>
          <input
            className={input}
            type="number"
            min={1}
            max={102400}
            value={num(form.largeFilesMB)}
            onChange={(e) => patch({ largeFilesMB: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm">Back up by default (moderate/risky categories)</span>
          <input
            type="checkbox"
            checked={form.backupByDefault}
            onChange={(e) => patch({ backupByDefault: e.target.checked })}
          />
        </label>

        <label className={row}>
          <span className="text-sm">Backup retention (days, 1–365)</span>
          <input
            className={input}
            type="number"
            min={1}
            max={365}
            value={num(form.backupRetentionDays)}
            onChange={(e) => patch({ backupRetentionDays: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm">Scan concurrency (1–16)</span>
          <input
            className={input}
            type="number"
            min={1}
            max={16}
            value={num(form.concurrency)}
            onChange={(e) => patch({ concurrency: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm">Show risky categories by default</span>
          <input
            type="checkbox"
            checked={form.showRisky}
            onChange={(e) => patch({ showRisky: e.target.checked })}
          />
        </label>

        <label className="block py-3">
          <span className="text-sm">Extra languages to keep (comma-separated, e.g. pt-BR, de)</span>
          <input
            className="mt-1 w-full rounded-md border border-zinc-300 bg-white px-2 py-1 text-sm dark:border-zinc-700 dark:bg-zinc-900"
            type="text"
            value={form.keepLanguages}
            onChange={(e) => patch({ keepLanguages: e.target.value })}
          />
        </label>

        <label className="block py-3">
          <span className="text-sm">Extra node_modules scan roots (one path per line, max 50)</span>
          <textarea
            className={area}
            rows={3}
            value={form.nodeModulesPaths}
            onChange={(e) => patch({ nodeModulesPaths: e.target.value })}
          />
        </label>

        <label className="block py-3">
          <span className="text-sm">Extra project scan roots (one path per line, max 50)</span>
          <textarea
            className={area}
            rows={3}
            value={form.projectsPaths}
            onChange={(e) => patch({ projectsPaths: e.target.value })}
          />
        </label>
      </div>

      <div className="mt-4 flex items-center gap-3">
        <button
          className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
          onClick={() => void save()}
        >
          Save
        </button>
        {toast ? <span className="text-sm text-green-600 dark:text-green-400">{toast}</span> : null}
      </div>
    </div>
  )
}

export default Settings
```

- [ ] **Step 6: Mount the three views in App.tsx**

Modify `frontend/src/App.tsx`: add

```tsx
import Maintenance from './views/Maintenance'
import Backups from './views/Backups'
import Settings from './views/Settings'
```

to the view imports, and in the `App()` view switch replace the placeholder JSX rendered for these three views with the real components, so the branches read:

```tsx
      {view === 'maintenance' && <Maintenance />}
      {view === 'backups' && <Backups />}
      {view === 'settings' && <Settings />}
```

Everything else in `App.tsx` (sidebar, smart-scan/category/uninstaller branches, `<CleanFlow />`) stays untouched.

- [ ] **Step 7: Run tests + typecheck to verify everything passes**

Run: `cd frontend && npx vitest run src/lib/settings.test.ts && npm run typecheck`
Expected: PASS (7 tests); `tsc --noEmit` exits 0.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/lib/settings.ts frontend/src/lib/settings.test.ts \
  frontend/src/views/Maintenance.tsx frontend/src/views/Backups.tsx frontend/src/views/Settings.tsx \
  frontend/src/App.tsx
git commit -m "feat(frontend): maintenance, backups, and settings views" \
  -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
```

### Task 28: Packaging — icon, bundle metadata, README, release verification

Packaging task: TDD steps are replaced by concrete build/verify steps.

**Files:**
- Create: `tools/genicon/main.go` (icon generator, stdlib only)
- Create: `build/appicon.png` (generated output — overwrites the Wails template placeholder)
- Modify: `wails.json` (full replacement below)
- Modify: `build/darwin/Info.plist` and `build/darwin/Info.dev.plist` (full replacements below)
- Modify: `main.go` (About panel in Mac options)
- Create: `README.md`
- Create: `LICENSE`

**Interfaces:**
- Consumes: the finished app from Tasks 1–27; `wails` CLI v2.12 on PATH; `mac.Options`/`mac.AboutInfo` from `github.com/wailsapp/wails/v2/pkg/options/mac`
- Produces: `build/bin/App Cleaner.app` (bundle id `com.guhcostan.appcleaner`, version 1.0.0), git tag `v1.0.0`. Nothing downstream consumes code from this task.

- [ ] **Step 1: Write the icon generator**

Create `tools/genicon/main.go` — renders a 1024×1024 PNG: rounded-square indigo→purple gradient with three white 4-point sparkles (astroid curves), anti-aliased, transparent outside the rounded square. Stdlib only; deterministic output.

```go
// Command genicon renders build/appicon.png (1024x1024) for the app bundle.
// Run from the repo root: go run ./tools/genicon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const dim = 1024

// smooth maps a signed distance v (positive = inside) to 0..1 coverage over an
// anti-aliasing band of width w.
func smooth(v, w float64) float64 {
	t := v/w + 0.5
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

// rectAlpha is the coverage of the rounded-square background:
// margin 64 px on every side, corner radius 180 px.
func rectAlpha(x, y float64) float64 {
	const m, r = 64.0, 180.0
	lo, hi := m+r, float64(dim)-m-r
	cx := math.Min(math.Max(x, lo), hi)
	cy := math.Min(math.Max(y, lo), hi)
	d := math.Hypot(x-cx, y-cy)
	return smooth(r-d, 2.0)
}

// starAlpha is the coverage of a 4-point sparkle (astroid |x|^(2/3)+|y|^(2/3)=r^(2/3)).
func starAlpha(x, y, cx, cy, r float64) float64 {
	dx, dy := math.Abs(x-cx), math.Abs(y-cy)
	e := math.Pow(r, 2.0/3.0)
	v := math.Pow(dx, 2.0/3.0) + math.Pow(dy, 2.0/3.0)
	return smooth(e-v, 0.08*e)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, dim, dim))
	top := [3]float64{0x4F, 0x46, 0xE5}    // indigo-600
	bottom := [3]float64{0x93, 0x33, 0xEA} // purple-600
	for py := 0; py < dim; py++ {
		for px := 0; px < dim; px++ {
			x, y := float64(px)+0.5, float64(py)+0.5
			a := rectAlpha(x, y)
			if a <= 0 {
				continue // fully transparent pixel
			}
			t := (x + y) / (2 * dim) // diagonal gradient
			cr := lerp(top[0], bottom[0], t)
			cg := lerp(top[1], bottom[1], t)
			cb := lerp(top[2], bottom[2], t)
			s := math.Max(starAlpha(x, y, 512, 540, 340),
				math.Max(starAlpha(x, y, 764, 268, 120), starAlpha(x, y, 292, 244, 70)))
			cr, cg, cb = lerp(cr, 255, s), lerp(cg, 255, s), lerp(cb, 255, s)
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(cr + 0.5),
				G: uint8(cg + 0.5),
				B: uint8(cb + 0.5),
				A: uint8(a*255 + 0.5),
			})
		}
	}
	f, err := os.Create("build/appicon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote build/appicon.png (1024x1024)")
}
```

- [ ] **Step 2: Generate and verify the icon**

Run (from the repo root):

```bash
go run ./tools/genicon
sips -g pixelWidth -g pixelHeight -g hasAlpha build/appicon.png
```

Expected: `wrote build/appicon.png (1024x1024)`, then sips prints `pixelWidth: 1024`, `pixelHeight: 1024`, `hasAlpha: yes`. Open it once (`open build/appicon.png`) — a purple rounded square with white sparkles. Wails packs `build/appicon.png` into the bundle automatically.

- [ ] **Step 3: Finalize wails.json**

Replace the entire contents of `wails.json` with:

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "App Cleaner",
  "outputfilename": "App Cleaner",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "guhcostan"
  },
  "info": {
    "companyName": "guhcostan",
    "productName": "App Cleaner",
    "productVersion": "1.0.0",
    "copyright": "© 2026 guhcostan — MIT",
    "comments": "Clean caches, junk, and leftover app files on macOS."
  }
}
```

- [ ] **Step 4: Finalize the Info.plists**

Replace the entire contents of `build/darwin/Info.plist` (the template Wails merges at build time; `{{…}}` placeholders are filled from wails.json) with:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>CFBundleName</key><string>{{.Info.ProductName}}</string>
    <key>CFBundleExecutable</key><string>{{.Name}}</string>
    <key>CFBundleIdentifier</key><string>com.guhcostan.appcleaner</string>
    <key>CFBundleVersion</key><string>{{.Info.ProductVersion}}</string>
    <key>CFBundleGetInfoString</key><string>{{.Info.Comments}}</string>
    <key>CFBundleShortVersionString</key><string>{{.Info.ProductVersion}}</string>
    <key>CFBundleIconFile</key><string>iconfile</string>
    <key>LSMinimumSystemVersion</key><string>11.0</string>
    <key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
    <key>NSHighResolutionCapable</key><string>true</string>
    <key>NSHumanReadableCopyright</key><string>© 2026 guhcostan — MIT</string>
</dict>
</plist>
```

Replace `build/darwin/Info.dev.plist` (used by `wails dev`) with the same dict plus the local-networking exception the dev server needs:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>CFBundleName</key><string>{{.Info.ProductName}}</string>
    <key>CFBundleExecutable</key><string>{{.Name}}</string>
    <key>CFBundleIdentifier</key><string>com.guhcostan.appcleaner</string>
    <key>CFBundleVersion</key><string>{{.Info.ProductVersion}}</string>
    <key>CFBundleGetInfoString</key><string>{{.Info.Comments}}</string>
    <key>CFBundleShortVersionString</key><string>{{.Info.ProductVersion}}</string>
    <key>CFBundleIconFile</key><string>iconfile</string>
    <key>LSMinimumSystemVersion</key><string>11.0</string>
    <key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
    <key>NSHighResolutionCapable</key><string>true</string>
    <key>NSHumanReadableCopyright</key><string>© 2026 guhcostan — MIT</string>
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsLocalNetworking</key>
        <true/>
    </dict>
</dict>
</plist>
```

(If the Wails template did not create `build/darwin/`, create the directory and both files exactly as above — Wails v2 uses them automatically when present.)

- [ ] **Step 5: Add the About panel to main.go**

In `main.go` (written by the bridge task):

1. Ensure the import block contains `"github.com/wailsapp/wails/v2/pkg/options/mac"` (the bridge task already imports it for the title-bar options; add it if missing).
2. Next to the existing `//go:embed frontend/dist` assets variable, add:

```go
//go:embed build/appicon.png
var iconBytes []byte
```

3. Inside the `&options.App{...}` literal passed to `wails.Run`, keep every existing `Mac` field (TitleBar, Appearance, …) unchanged and add/replace only the `About` field so the block reads:

```go
		Mac: &mac.Options{
			// ...existing TitleBar/Appearance settings from the bridge task stay as-is...
			About: &mac.AboutInfo{
				Title:   "App Cleaner",
				Message: "Version 1.0.0\n© 2026 guhcostan — MIT\n\nA macOS cleaning app — Go/Wails port of mac-cleaner-cli.",
				Icon:    iconBytes,
			},
		},
```

Verify it compiles: `go build ./...` — exits 0.

- [ ] **Step 6: Write README.md and LICENSE**

Create `README.md` at the repo root:

````markdown
# App Cleaner

A native macOS cleaning app — scan and remove caches, logs, old downloads, orphaned
`node_modules`, duplicate files and more; uninstall apps with their leftovers; run
system maintenance; undo cleans from move-based backups. 100% offline, no telemetry.

Go engine + [Wails v2](https://wails.io) shell + React/TypeScript UI. A desktop port of
[mac-cleaner-cli](https://github.com/guhcostan/mac-cleaner-cli) with full feature parity.

<!-- Screenshot: once available, add a capture of the Smart Scan view here, e.g. ![App Cleaner — Smart Scan](docs/screenshot.png) -->

## Features — 16 scan categories

| Category | Group | Safety | What it cleans |
|---|---|---|---|
| Temporary Files | System Junk | 🟢 safe | `/tmp` and `/var/folders` temp files |
| User Cache Files | System Junk | 🟡 moderate | Application caches in `~/Library/Caches` |
| System Log Files | System Junk | 🟡 moderate | System and application logs |
| Orphaned Launch Agents | System Junk | 🟡 moderate | Launch agents pointing to non-existent apps |
| Language Files | System Junk | 🔴 risky | Unused `.lproj` localizations in `/Applications` |
| Homebrew Cache | Development | 🟢 safe | Homebrew download cache and old versions |
| Docker | Development | 🟢 safe | Unused images, containers and build cache |
| Development Cache | Development | 🟡 moderate | npm, yarn, pnpm, pip, CocoaPods, Gradle, Cargo, Xcode DerivedData/Archives |
| Node Modules | Development | 🟡 moderate | `node_modules` in old (30d+) or orphaned projects |
| Trash | Storage | 🟢 safe | Files in the Trash bin |
| Old Downloads | Storage | 🔴 risky | Downloads older than N days (default 30) |
| iOS Backups | Storage | 🔴 risky | iPhone/iPad backups (MobileSync) |
| Mail Attachments | Storage | 🔴 risky | Mail.app downloaded attachments |
| Duplicate Files | Storage | 🔴 risky | Identical files (keeps the newest copy) |
| Browser Cache | Browsers | 🟢 safe | Chrome, Safari, Firefox and Arc caches |
| Large Files | Large Files | 🔴 risky | Files ≥ 500 MB in Downloads/Documents, for review |

Plus: **App Uninstaller** (bundle + 11 kinds of leftovers, running-app guard),
**Maintenance** (flush DNS, free purgeable space, clear Time Machine snapshots),
**Backups/Undo** (move-based, restorable, auto-pruned), **Settings**.

Safe+moderate categories are pre-selected after a scan; risky ones never are. Deletion
refuses system paths (`/System`, `/usr`, …) by design. Grant the app **Full Disk Access**
for Trash/Safari/Mail categories.

## Build

Requirements: macOS 11+, Go 1.26+, Node 18+, Wails CLI v2.12 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`).

```bash
wails doctor                             # verify toolchain
wails dev                                # live-reload development
wails build -clean                              # build/bin/App Cleaner.app (host arch, dev iterations)
wails build -clean -platform darwin/universal   # universal release build
```

## Testing

```bash
go test ./...                                  # engine + bridge
cd frontend && npm run typecheck && npm test   # tsc --noEmit + vitest
```

## Signing & notarization (follow-up — the app currently ships unsigned)

```bash
codesign --deep --force --options runtime \
  --sign "Developer ID Application: YOUR NAME (TEAMID)" "build/bin/App Cleaner.app"
ditto -c -k --keepParent "build/bin/App Cleaner.app" "App Cleaner.zip"
xcrun notarytool submit "App Cleaner.zip" --keychain-profile "notary" --wait
xcrun stapler staple "build/bin/App Cleaner.app"
```

## Credits

A GUI port of [mac-cleaner-cli](https://github.com/guhcostan/mac-cleaner-cli) by
[@guhcostan](https://github.com/guhcostan) — same categories, thresholds and safety
philosophy, rebuilt on a Go engine.

## License

[MIT](LICENSE)
````

Create `LICENSE` at the repo root:

```text
MIT License

Copyright (c) 2026 guhcostan

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 7: Full release verification**

Run each command from the repo root and confirm the expected result before moving on:

```bash
go test ./...
```
Expected: every package prints `ok` (or `[no test files]`); zero `FAIL` lines; exit 0.

```bash
cd frontend && npm run typecheck && npm test; cd ..
```
Expected: `tsc --noEmit` silent; vitest reports all test files passed, 0 failed.

```bash
wails build -clean -platform darwin/universal
```
Expected: exits 0; output ends with a “Built … build/bin/App Cleaner.app” line. The release
build is a universal (arm64 + x86_64) binary per spec §15; day-to-day dev iterations may use a
plain `wails build -clean` host-arch build for speed, but this final verification must use the
universal build.

```bash
lipo -archs "build/bin/App Cleaner.app/Contents/MacOS/App Cleaner"
```
Expected: `x86_64 arm64` (both architectures present).

```bash
/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "build/bin/App Cleaner.app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "build/bin/App Cleaner.app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :LSApplicationCategoryType' "build/bin/App Cleaner.app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "build/bin/App Cleaner.app/Contents/Info.plist"
```
Expected (in order): `com.guhcostan.appcleaner`, `11.0`, `public.app-category.utilities`, `1.0.0`.

```bash
open "build/bin/App Cleaner.app"
```
Then walk this manual smoke-test checklist in order — every item must pass:

1. **Launch**: the app opens a ≈ 1150×740 window; the Dock icon is the purple sparkle icon.
2. **FDA screen**: if Full Disk Access has not been granted, the First Run permissions screen appears — either grant FDA (button opens System Settings; relaunch if prompted) or skip/continue without it.
3. **Smart Scan**: run Smart Scan; per-category progress advances (completed/total counts up and category results stream in with sizes).
4. **Select**: after the scan, select a small safe category (e.g. Temporary Files); verify risky categories are not pre-selected.
5. **Detail view**: open the category's detail view; toggle a couple of items off and back on — the selection count/size updates.
6. **Dry-run clean**: start a Clean with **Dry-run ON**; the progress overlay advances during the run.
7. **Results**: the results panel reports item counts and “would be freed (dry run)” space; spot-check on disk that nothing was deleted (the listed files still exist).
8. **Settings round-trip**: open Settings, change a value (e.g. Downloads days 30 → 45), Save, quit and relaunch the app, and verify the changed value persisted.

Quit the app after checking.

- [ ] **Step 8: Commit and tag v1.0.0**

```bash
git add tools/genicon/main.go build/appicon.png wails.json \
  build/darwin/Info.plist build/darwin/Info.dev.plist main.go README.md LICENSE
git commit -m "chore: packaging — app icon, bundle metadata, README, license" \
  -m "Co-Authored-By: WOZCODE <contact@withwoz.com>"
git tag v1.0.0
git tag --list v1.0.0   # prints: v1.0.0
```
