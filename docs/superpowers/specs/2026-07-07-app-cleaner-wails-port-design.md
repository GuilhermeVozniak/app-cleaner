# App Cleaner — Go + Wails Port of mac-cleaner-cli

**Date:** 2026-07-07
**Status:** Approved (design); pending implementation plan
**Source project:** https://github.com/guhcostan/mac-cleaner-cli (v1.3.5, TypeScript/Node CLI) — cloned read-only at `mac-cleaner-cli/` for reference
**Normative behavior reference:** `docs/reference/porting-notes.json` — exhaustive per-module extraction of the CLI's behavior (exact paths, thresholds, commands, algorithms). Where this spec summarizes, that file is authoritative for parity details.

## 1. Overview

Rebuild mac-cleaner-cli as **App Cleaner**, a distributable macOS desktop app: Go engine + Wails v2 shell + React/TypeScript frontend. Full feature parity with the CLI's capabilities (16 scan categories, app uninstaller, maintenance tasks, undo backups, config), re-imagined as a polished CleanMyMac-style GUI. 100% offline, no telemetry.

**Decisions locked with the user:**
- Distributable product (unsigned .app for now; signing/notarization documented as follow-up)
- Wails v2 (stable), React 18 + TypeScript frontend
- Architecture A: GUI-only app with the engine fully decoupled (a CLI can be added later on top of the same engine)
- Deletion: permanent delete (like the CLI) + the CLI's move-based backup mechanism enabled **by default for moderate/risky categories**, surfaced in the UI as "Undo" with retention
- Name: **App Cleaner**, bundle ID `com.guhcostan.appcleaner`

## 2. Non-goals (v1)

- No CLI binary, no `--json` automation surface (engine stays UI-agnostic so this can come later)
- No code signing / notarization / auto-update (documented, not implemented)
- No Windows/Linux support (macOS-only, like the original)
- No new cleaning categories beyond the CLI's 16
- No privileged helper daemon (SMAppService); elevation via one-shot `osascript` admin prompts only

## 3. Architecture

```
app-cleaner/
├── main.go                    # Wails bootstrap (window config, bindings, single instance)
├── app.go                     # Bound App struct: thin adapters, goroutine + event streaming, cancellation
├── wails.json / go.mod
├── build/                     # icons, Info.plist bits
├── internal/
│   ├── core/                  # Category registry, CleanableItem, ScanResult/CleanResult, safety levels
│   ├── fsx/                   # safe FS layer: listing, recursive sizing, filters, deletion engine, errno mapping
│   ├── scanners/              # one file per scanner (16) + Scanner interface + parallel runner
│   ├── uninstall/             # app discovery, bundle-ID parsing, related-path search, running-app check
│   ├── maintenance/           # dns.go, purge.go, timemachine.go, exec.go (spawn wrapper), elevate.go (osascript)
│   ├── backup/                # move-based backup sessions, restore, retention cleanup
│   ├── config/                # load/validate/save JSON config
│   └── fda/                   # Full Disk Access tri-state probe + System Settings deep link
├── frontend/                  # React 18 + TS + Vite + Tailwind CSS v4
│   └── src/
│       ├── App.tsx, main.tsx
│       ├── stores/            # zustand: scan store, clean store, ui store
│       ├── views/             # SmartScan, CategoryDetail, Uninstaller, Maintenance, Backups, Settings, FirstRun
│       ├── components/        # Sidebar, CategoryCard, SizeBar, SafetyBadge, ItemList, DirectoryGroup,
│       │                      #   ConfirmModal, ProgressOverlay, ResultsPanel, EmptyState
│       └── lib/               # generated Wails bindings wrappers, formatSize, event hooks
└── docs/
```

**Dependency rule:** `internal/*` packages never import Wails. `app.go` is the only file that touches the Wails runtime (events, dialogs, clipboard). Every engine entry point takes a `context.Context` (cancellation) and an optional progress callback `func(current, total int, item core.CleanableItem)` — invoked *before* processing each item, 1-based, mirroring the CLI contract.

**Concurrency:** scanners run in parallel goroutines with a semaphore (default 4, config `concurrency` 1..16). Each scanner's failure is isolated: it reports `ScanResult.Error`, never aborts the batch ⚠ (deliberate change — the CLI silently dropped a crashed scanner from the summary; see §10). Deletion is sequential (progress-friendly, matches CLI).

## 4. Engine contracts (ported verbatim unless noted)

### 4.1 Types (`internal/core`)

```go
type CategoryID string        // 16 ids, same strings as CLI: "system-cache", ..., "launch-agents"
type SafetyLevel string       // "safe" | "moderate" | "risky"
type CategoryGroup string     // "System Junk" | "Development" | "Storage" | "Browsers" | "Large Files"

type Category struct {
    ID CategoryID; Name string; Group CategoryGroup; Description string
    SafetyLevel SafetyLevel; SafetyNote string; SupportsFileSelection bool
}
type CleanableItem struct {
    Path string; Size int64; Name string; IsDirectory bool; ModifiedAt *time.Time
}
type ScanResult struct { Category Category; Items []CleanableItem; TotalSize int64; Error string }
type ScanSummary struct { Results []ScanResult; TotalSize int64; TotalItems int }
type CleanResult struct { Category Category; CleanedItems int; FreedSpace int64; Errors []string }
```

The 16-entry `Categories` registry keeps the CLI's exact names, groups, descriptions, safety levels, and safety notes (see porting-notes.json → utils → types.ts).

### 4.2 Safe deletion layer (`internal/fsx`)

- **Allowed-path overrides (checked first, always deletable):** `/tmp`, `/private/tmp`, `/var/tmp`, `/private/var/tmp`, `/var/folders`, `/private/var/folders`.
- **Protected paths (refuse, exact or prefix+`/`):** `/System`, `/usr`, `/bin`, `/sbin`, `/etc`, `/var/log`, `/var/db`, `/var/root`, `/private/var/db`, `/private/var/root`, `/private/var/log`, `/Library/Apple`, `/Applications/Utilities`. Also refuse `/` and the exact home directory. Paths are absolutized/cleaned before checks.
- **TOCTOU guard:** re-`os.Lstat` immediately before each delete; if symlink → `os.Remove` the link only (never follow); else `os.RemoveAll`.
- **Errno string contract (drives UI breakdown):** `PROTECTED`, `EPERM`, `EACCES`, `ENOENT`, `UNKNOWN` — mapped from `syscall.Errno`. Per-category clean errors aggregate into one string: `Failed to remove N items (32 EPERM, 8 EACCES)` (codes sorted by count desc).
- **EPERM on SIP/TCC-owned temp files** (e.g. `/var/folders/.../T/com.apple.*`) is expected: counted, never treated as fatal, and the UI maps EPERM/EACCES-heavy results to a Full Disk Access hint.
- **Dry-run:** returns success for every item immediately (reports full item count + freed space without touching disk).
- **Sizing:** logical size via `Lstat` (symlinks = link size, never followed); directories = recursive sum. Sizes computed once at scan time and reused for freed-space accounting — never re-measured at delete time.
- **Walk philosophy:** filesystem walks never fail — every unreadable entry/root is skipped silently (size 0 / empty list).
- No shell is ever spawned for deletion; external binaries (brew/docker/tmutil/...) are exec'd with arg slices and absolute allowlisted paths only.

## 5. Scanner inventory (16)

Exact parity with the CLI except rows marked ⚠ (deliberate changes, §10). Full algorithmic detail per scanner: porting-notes.json → scanners.

| ID | Safety | Scans | Notes |
|---|---|---|---|
| `system-cache` | moderate | `~/Library/Caches` immediate children | recursive sizing per child |
| `system-logs` | moderate | `~/Library/Logs` immediate children | ⚠ CLI also listed `/var/log`, which its own safety layer made undeletable (always `PROTECTED`); the port drops `/var/log` from scanning |
| `temp-files` | safe | `/tmp` children + `/private/var/folders/*/*/T` children | expect EPERM on system-owned entries; silent-skip |
| `trash` | safe | `~/.Trash` children | permanent delete; needs FDA to read |
| `downloads` | risky | `~/Downloads`, non-recursive, age (now − mtime) ≥ `downloadsDaysOld` days (default 30) | file-selection UI |
| `browser-cache` | safe | Chrome `~/Library/Caches/Google/Chrome`, Safari `~/Library/Caches/com.apple.Safari`, Firefox `~/Library/Caches/Firefox/Profiles`, Arc `~/Library/Caches/company.thebrowser.Browser` | one item per existing browser dir |
| `dev-cache` | moderate | npm `~/.npm/_cacache`, Yarn `~/Library/Caches/Yarn`, pnpm `~/Library/pnpm/store`, pip `~/.cache/pip`, CocoaPods `~/Library/Caches/CocoaPods`, Gradle `~/.gradle/caches`, Cargo `~/.cargo/registry` (each if size>0); Xcode DerivedData per-project children; Xcode Archives (one item, if size>0) | |
| `homebrew` | safe | `brew --cache` output, validated against allowlisted roots | brew binary only from `/opt/homebrew/bin`, `/usr/local/bin` (never $PATH); clean = `brew cleanup --prune=all` when cache root selected, else direct delete |
| `docker` | safe | `docker system df` reclaimable rows (images, containers, build cache) | docker binary allowlist; whole-category-only in the UI (rows informational, no per-row selection); clean = `docker system prune -af` (no `--volumes`, intentional); ⚠ parse sizes as SI units (kB/MB/GB = 1000-based) — the CLI wrongly used 1024; ⚠ `local volumes` row excluded — prune never frees it |
| `ios-backups` | risky | `~/Library/Application Support/MobileSync/Backup` children | name `iOS Backup: <UDID[:8]>...` |
| `mail-attachments` | risky | `~/Library/Containers/com.apple.mail/Data/Library/Mail Downloads` children | |
| `language-files` | risky | `/Applications/*.app/Contents/Resources/*.lproj` minus keep-list | ⚠ keep-list = for each system preferred language tag: the tag verbatim, its `-`→`_` variant, and its base language; union `en`, `Base`, and config `keepLanguages`; case-sensitive match (CLI hardcoded en/pt) |
| `large-files` | risky | `~/Downloads` + `~/Documents`, recursive depth ≤ 3, regular files ≥ `largeFilesMinSize` (default 500 MiB), dot-entries skipped | sorted size desc; file-selection UI |
| `node-modules` | moderate | roots `~/Projects ~/Developer ~/Code ~/dev ~/workspace ~/repos` + config `extraPaths`, depth ≤ 4: `node_modules` dirs where sibling `package.json` exists and project age (now − mtime) ≥ `downloadsDaysOld` days (default 30 — the CLI shares one age knob with downloads) (`(Nd old)`), or no `package.json` (`(orphaned)`, any age, size>0) | never recurses into node_modules |
| `duplicates` | risky | `~/Downloads ~/Documents ~/Desktop`, depth ≤ 5, files ≥ 1 MiB: group by size → MD5 → sets keep newest, list older copies as `name (dup of newest)` | ⚠ add the partial-hash (first 1 MiB) pre-filter the CLI defined but never wired up |
| `launch-agents` | moderate | `~/Library/LaunchAgents/*.plist` whose `Program`/`ProgramArguments[0]` points to a non-existent absolute path (system-binary prefixes exempt) | ⚠ parse plists properly with `howett.net/plist` (binary + XML) instead of the CLI's regex-on-text; clean still deletes the plist only |

## 6. Uninstaller (`internal/uninstall`)

Parity with the CLI plus two deliberate fixes:

- **Discovery:** top-level `.app` bundles in `/Applications` and `~/Applications`; per-app total = bundle size + related paths sizes; sorted by total desc. App icons extracted lazily for the UI (bridge `GetAppIcon`: `CFBundleIconFile` → `.icns` → PNG via `/usr/bin/sips`, returned base64, cached per bundle; letter-avatar fallback in the UI).
- **Bundle ID:** parse `Contents/Info.plist` (proper plist parser — handles binary plists the CLI's regex missed); must match `^[a-zA-Z][a-zA-Z0-9.-]*$`; fallback `appname` lowercased, spaces→`.`.
- **Related paths:** the CLI's 11 `$HOME`-scoped templates (Application Support, Preferences ×2, Caches ×2, Logs, Saved Application State, WebKit, HTTPStorages, Containers, Group Containers glob) with the 3 name variations (verbatim / lowercase / whitespace-stripped); glob matching with all regex metachars escaped; every candidate must resolve under `$HOME` and pass `isProtectedPath`.
- **⚠ Running-app check (new):** before uninstalling, check if the app is running (`pgrep -f` on bundle path / NSRunningApplication via `osascript`); if running, the UI requires quitting first.
- **⚠ Freed-space accounting fix:** the CLI measured related-path sizes *after* deleting them (always 0); the port sums sizes before deletion.
- Deletion: `safeRemove` per path (same safety layer); main bundle failure skips its related paths and continues with the next app.

## 7. Maintenance (`internal/maintenance`)

Shared result: `MaintenanceResult { Success bool; Message string; Error string; RequiresAdmin bool }` (single definition; CLI duplicated it). All commands run via a spawn wrapper: absolute binary paths, arg slices, `exec.CommandContext` with timeout, stderr captured.

| Task | Commands | Timeout | Elevation |
|---|---|---|---|
| Flush DNS | `/usr/bin/dscacheutil -flushcache` then `/usr/bin/killall -HUP mDNSResponder` | 10 s | always admin |
| Free purgeable | `/usr/sbin/purge` | 60 s | ⚠ try unprivileged **first** (usually works), elevate only on permission failure — CLI tried sudo first |
| Time Machine snapshots | `/usr/bin/tmutil listlocalsnapshotdates` → per-date `deletelocalsnapshots <d>`; dates strictly `^\d{4}-\d{2}-\d{2}-\d{6}$` | 30 s list / 60 s per delete | list unprivileged; delete admin. Sequential; per-date errors collected; partial success = success with `X/Y (N error(s))` |

**⚠ Elevation strategy:** `sudo -n` is useless from a .app. Admin tasks run through `osascript -e 'do shell script "..." with administrator privileges'` (native macOS password prompt), commands built only from the fixed strings + regex-validated dates above. `RequiresAdmin` in the result drives the UI ("Requires administrator" state before the prompt). Time Machine deletion runs as **one** elevated invocation: a single `do shell script` loops over the pre-validated dates and prints one status line per date to stdout (parsed for per-date progress/errors) — exactly one password prompt. Total failure (0 deleted) → `Success=false` with the first error; partial success keeps the CLI's `Deleted X/Y … (N error(s))` message shape. Unprivileged Runner calls keep the CLI timeouts in the table above; elevated osascript calls use a 120 s budget, since the user must type a password before the script starts.

## 8. Backup / Undo (`internal/backup`)

CLI mechanism ported, relocated for a GUI app:

- Session dir: `~/Library/Application Support/AppCleaner/Backups/<ISO-timestamp with : . → ->/`
- Backup = `os.Rename` (move) of each item into the session dir under `HOME/<path relative to home>` (literal `HOME` segment, first-occurrence replacement — CLI-compatible layout). ⚠ On `EXDEV` (cross-volume, e.g. items on another APFS volume) fall back to permanent delete for that item and record it as *not backed up* — the CLI just failed silently. ⚠ Items whose path is **not under `$HOME`** (e.g. language-files under `/Applications`) are likewise never backed up — same fallback: permanent delete, recorded as not backed up.
- Restore: validate the session dir resolves under the backups root; each restored file's target must land under `$HOME` (paths with `..` or non-`HOME/` prefixes are rejected per file).
- Retention: ⚠ honor `backupRetentionDays` from config (default 7) — the CLI hardcoded 7 and ignored its own config field. Cleanup runs on app launch and from the Backups view.
- Defaults: the confirm modal's backup toggle is **global for the batch**, defaulting to on when config `backupByDefault` is set and any selected category is moderate or risky. Items cleaned via `brew cleanup` / `docker prune` (virtual items) and non-`$HOME` items are never backed up.

## 9. Config (`internal/config`)

`~/Library/Application Support/AppCleaner/config.json`, shallow-merged over defaults, invalid fields dropped with a warning (never errors):

```json
{
  "downloadsDaysOld": 30,          // int 1..365
  "largeFilesMinSize": 524288000,  // int 1KB..100GB
  "backupByDefault": true,          // for moderate/risky categories
  "backupRetentionDays": 7,        // int 1..365
  "concurrency": 4,                // int 1..16
  "showRisky": false,              // GUI equivalent of --risky
  "keepLanguages": [],             // extra .lproj names to keep (merged with system languages + en + Base)
  "extraPaths": { "nodeModules": [], "projects": [] }  // ≤50 each; must resolve under $HOME, /Users, or /Volumes
}
```

Both `extraPaths` arrays extend the node-modules scanner's search roots (`projects` = additional project directories to walk); nonexistent paths are skipped silently. No CLI-era `~/.maccleanerrc` migration — this is a new app at a new, GUI-conventional path.

## 10. Deliberate changes from the CLI (consolidated)

1. `/var/log` no longer scanned (undeletable-by-design in the CLI).
2. Backup ON by default for moderate/risky categories; surfaced as Undo.
3. Elevation via native admin prompt (osascript), not `sudo -n`.
4. Purgeable: unprivileged attempt first, then elevate.
5. Docker sizes parsed as SI (decimal) units, matching what docker prints.
6. Language keep-list = system preferred languages + `en` + `Base`, configurable (CLI hardcoded pt/en).
7. Duplicates: partial-hash pre-filter before full MD5.
8. Plists (launch agents, bundle IDs) parsed with a real plist library (binary-safe), not regex.
9. Uninstaller: running-app check added; related-path freed-space measured before deletion (CLI bug).
10. Backup honors `backupRetentionDays`; `EXDEV` and non-`$HOME` no-backup fallbacks made explicit.
11. Time Machine snapshots task exposed in the UI (implemented but undocumented in the CLI).
12. FDA hint becomes a first-run permission screen + persistent sidebar indicator.
13. Failed scanners surface as `ScanResult.Error` on their category card (the CLI silently dropped a crashed scanner from the summary).
14. Docker `local volumes` excluded from results (the CLI listed it as reclaimable although `prune` without `--volumes` never frees it).
15. Config + backups relocated to `~/Library/Application Support/AppCleaner/` (was `~/.maccleanerrc` / `~/.config/mac-cleaner-cli/` and `~/.mac-cleaner-cli/backup`).

Everything else — paths, thresholds, filters, sorting, naming formats, error philosophy — is ported as-is.

## 11. Wails bridge (`app.go`)

Bound methods (all JSON-serializable structs; long operations spawn a goroutine, return immediately, and stream events):

```
GetCategories() []core.Category
StartScan(ids []string) error            // full scan when ids empty
CancelScan()
GetScanResult(id string) core.ScanResult // items for drill-down
GroupItems(id string, expand map[string]int) []DisplayRow  // §12 directory grouping
StartClean(sel map[string][]string, opts CleanOptions) error  // opts: DryRun, Backup
CancelClean()
ListApps() []AppInfo                      // uninstaller
StartUninstall(names []string, dryRun bool) error  // named like StartScan/StartClean
GetAppIcon(path string) string                     // lazy base64 PNG via sips; "" on failure
CleanOldBackups() int                              // retention sweep (also runs at launch)
IsAppRunning(path string) bool
RunMaintenance(task string) MaintenanceResult   // "dns" | "purge" — synchronous (JS promise); explicit exception to the long-op rule
StartTMSnapshotsClear() error / CancelMaintenance()  // streams maintenance:progress / maintenance:done
GetConfig() / SaveConfig(c Config)
ListBackups() []BackupInfo / RestoreBackup(path string) RestoreResult / DeleteBackup(path string)
CheckFDA() *bool                          // tri-state: true/false/unknown
OpenFDASettings()                         // x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles
RevealInFinder(path string) / CopyPath(path string)
```

**Events (Go → JS):** `scan:progress {completed,total,categoryId,totalSize,itemCount,error}` (per scanner finish), `scan:done {summary, cancelled?, error?}`, `clean:progress {current,total,categoryId,itemName}`, `clean:done {summary, notBackedUp, cancelled?, error?}`, `uninstall:progress {current,total,appName}`, `uninstall:done {uninstalled, freedSpace, errors, cancelled?, error?}`, `backup:progress {current,total,itemName}`, `maintenance:progress {done,total,date,error?}`, `maintenance:done {result}`. One scan and one clean may run at a time (guarded by mutex; second call returns an error).

**Bridge types:** `DisplayRow` mirrors the CLI grouping contract (porting-notes → grouping.ts / checkbox.ts): `{type: "directory-header"|"file"|"expand-hint", directoryKey /*absolute dir path*/, displayName, path?, size?, name?, hiddenCount?, totalFilesInDir, selectable}`. `AppInfo {name, path, bundleId, appSize, relatedPaths: []{path,size}, totalSize, running}`. `BackupInfo {path, date, size}`. `RestoreResult {restored, failed, errors}`.

**FDA probe:** attempt to read `~/Library/Safari`: readable → true; `EPERM`/`EACCES` → false; other (e.g. ENOENT) → unknown. Note the GUI app needs FDA granted to **App Cleaner itself** (not the terminal).

## 12. Frontend

**Stack:** React 18 + TypeScript + Vite (Wails template), Tailwind CSS v4, zustand for state, lucide-react icons. Dark/light follows system (`prefers-color-scheme`). Window ~1150×740, hidden title bar with inset traffic lights for a native feel.

**Views:**
- **First Run / Permissions** — shown when FDA ≠ true: what FDA unlocks (Trash, Safari cache, Mail), button → `OpenFDASettings()`, re-check on window focus, "Continue without" allowed (sidebar keeps an amber indicator).
- **Smart Scan (home)** — hero scan button → live progress (per-category rows completing with sizes) → results: category cards grouped by CategoryGroup, each with size bar (proportional to largest), safety badge (🟢 safe / 🟡 moderate / 🔴 risky), checkbox, item count. Safe+moderate pre-selected; risky categories collapsed under a "Risky" section, hidden unless `showRisky` or expanded manually, never pre-selected. Footer: total selected size + **Clean** button.
- **Category detail** (drill-down, replaces the CLI file-picker) — every filesystem-backed category supports per-item selection here (GUI equivalent of the CLI's `-f` always on); Docker is whole-category-only (rows informational). Items sorted by size desc; categories flagged `SupportsFileSelection` additionally get directory grouping per the CLI contract: groups ordered by largest-single-file desc, 5 visible files per group + "show N more" expander keyed by absolute dir path; select all / invert; per-row: name, middle-truncated path (~50 chars, `~` contraction), size, Reveal in Finder, Copy path.
- **Clean confirm modal** — items count, space to free, backup toggle (default per §8), dry-run toggle; risky items called out with their safety notes. Then progress (per-item name streaming) → results panel: freed space, per-category ✓/✗ rows, errno breakdown; EPERM/EACCES-heavy → inline FDA call-to-action.
- **Uninstaller** — app list (icon, name, bundle + related sizes, "+N related" chip) sorted by total desc; selecting shows related paths tree; running apps flagged and blocked until quit; confirm modal → progress → results.
- **Maintenance** — three cards (Flush DNS, Free Purgeable Space, Clear Time Machine Snapshots) with description, admin badge where relevant, run button with inline result/error.
- **Backups** — sessions listed newest-first (date, size), Restore / Delete per session, retention note.
- **Settings** — thresholds (downloads age, large-file min size), backup defaults + retention, concurrency, show-risky default, language keep-list, extra scan roots for node-modules/projects.

**State flow:** views call bound methods via generated bindings; zustand stores subscribe to Wails events and hold scan/clean/uninstall state; components render from stores only. All sizes formatted base-1024 — 0 decimals for bytes, one decimal above (`formatSize`, CLI-compatible).

## 13. Error handling

- Scan: per-scanner `Error` string rendered as a warning row on its category card; batch never aborts; a scanner returning 0 items with no error simply doesn't render.
- Clean: aggregate errno breakdown per category (§4.2); `PROTECTED` rendered distinctly ("blocked for safety"); `ENOENT` stays a failure code in the breakdown like the CLI (item vanished between scan and clean — no freed-space credit).
- Maintenance: `Success=false` + `RequiresAdmin` → UI offers the admin prompt path; other errors shown verbatim.
- Engine panics: recovered at the app.go boundary, surfaced as a toast + `scan:done`/`clean:done` with error so the UI never hangs.
- Cancellation: context-based; partial results are reported (`cancelled: true` flag in done events).

## 14. Testing

- **Go unit tests** (primary): `fsx` (path safety table tests, TOCTOU/symlink behavior, errno mapping, recursive sizing, dry-run), each scanner against `t.TempDir()` fixtures (age/size filters, naming formats, keep-lists, orphan detection, duplicate sets keep-newest), `backup` (move layout, HOME mapping, restore validation, retention, EXDEV fallback path), `config` (bounds, invalid-field dropping), `uninstall` (bundle-id parsing incl. binary plist, template variations, glob escaping), `maintenance` (date-regex validation, result shaping — command execution mocked via an injected runner).
- **Frontend:** `tsc --noEmit` + Vite production build as local release gates (GitHub Actions CI is a post-v1 follow-up); component smoke tests (vitest + testing-library) for the scan store event handling and the confirm modal math.
- **Integration smoke:** `wails build` must produce `App Cleaner.app`; `wails dev` manual pass over scan→select→dry-run→results.
- Rule: tests never touch real user data — all FS tests run under temp dirs; external binaries behind interfaces with fakes.

## 15. Packaging & distribution

- `wails build -platform darwin/universal` → `build/bin/App Cleaner.app`; bundle ID `com.guhcostan.appcleaner`; app icon generated during implementation (`build/appicon.png`, produced as part of the build setup).
- Info.plist: `LSMinimumSystemVersion` 11.0, `NSHumanReadableCopyright`, category `public.app-category.utilities`.
- README with build instructions (`wails doctor`, `wails dev`, `wails build`) and a "Signing & notarization" follow-up section (`codesign` + `notarytool` outline).
- Git: this repo is the product repo; `mac-cleaner-cli/` reference clone stays untracked (.gitignore).

## 16. Suggested implementation milestones

1. Engine foundations: `core`, `fsx`, `config` (+ tests)
2. The 16 scanners + parallel runner (+ tests)
3. `backup`, `uninstall`, `maintenance`, `fda` (+ tests)
4. Wails bridge: `app.go`, events, generated bindings
5. Frontend views + styling + packaging (`wails build`)

## 17. References

- `docs/reference/porting-notes.json` — normative CLI behavior extraction (scanners / commands / utils / maintenance, with porting notes per module)
- `mac-cleaner-cli/` — original source (untracked reference clone)
- Wails v2 docs: https://wails.io/docs/introduction
