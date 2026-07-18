# Backup session details + regression-test audit + coverage hardening — design

Date: 2026-07-18
Status: approved (design discussion in session)
Scope: engine `packages/engine/backup`, bridge `apps/desktop/app.go`, frontend `Backups.tsx`, then a test-hardening pass across the app.

## Part 1 — Backup session details (user decision: items via manifest)

### Problem
The Backups view lists sessions (date + size) but not what they contain, so
choosing what to restore relies on memory. Session dirs hold moved files under
`<session>/HOME/<path relative to home>`, but nothing records which cleanup
items went in.

### Engine (`packages/engine/backup`)
- `BackupItems` writes `items.json` at the session root AFTER the move loop:
  a JSON array of `{path, name, size}` for the MOVED items only (entries in
  `NotBackedUp` are excluded — they were not preserved). A manifest write
  failure is non-fatal: the backup already succeeded; details fall back to the
  walk. On context cancellation mid-loop the manifest still records exactly
  the moved-so-far items.
- `Restore` skips `items.json` at the session root — today its walk would
  reject any file outside `HOME/` and wrongly count it as a failed restore
  (this is a latent bug the manifest would have triggered; the skip gets a
  regression test). The manifest is NOT deleted on restore; it goes away with
  the session (Delete or retention sweep).
- New `Manager.Details(sessionDir, home string) (Details, error)` (`home` is
  needed to reconstruct original paths in the fallback walk):
  - Containment-validated via the existing `resolveSession` (same as
    Restore/Delete); invalid dirs return the same capitalized error.
  - `Details{Items []Item, FromManifest bool, Truncated int}` with
    `Item{Path, Name, Size}` — all slices non-nil (nil-slice→JSON-null is
    this codebase's recurring crash class).
  - Manifest present and parseable → its items verbatim, `FromManifest: true`.
  - Missing/corrupt manifest (legacy sessions) → walk `<session>/HOME/`,
    return contained FILES as items (original path reconstructed by mapping
    `HOME/<rel>` back under the caller's home, `Name` = base name, `Size`
    from disk), sorted by path, capped at 200 entries; `Truncated` = number
    of omitted files.

### Bridge (`apps/desktop/app.go`)
- One bound method `GetBackupDetails(path string) backup.Details` — thin
  passthrough using `a.home`, same pattern as `ListBackups`/`RestoreBackup`.
  Error from containment validation → empty Details (non-nil Items) — the
  view shows the empty state; invalid paths cannot originate from the UI.

### Frontend (`Backups.tsx`)
- Each session row gets a chevron toggle (one expanded at a time, like the
  Uninstaller). First expand calls `GetBackupDetails(path)` and caches the
  result per path for the view's lifetime; a deleted session drops from the
  list (cache entry harmless).
- Expanded: one line per item — `contractHome(path)` + `formatSize(size)` in
  the mono style of the Uninstaller's related-paths list. Tail line
  `…and N more files` when `truncated > 0`. `No details recorded for this
  backup` when items is empty. Normalize `items ?? []`.
- Restore/Delete flows untouched.

### Non-goals
- No category names in the manifest (items are self-descriptive; would
  change `BackupItems`' signature).
- No CLI detail view yet — `Manager.Details` is engine-level and reusable
  when wanted.

## Part 2 — Regression-test audit for fixed bugs (user requirement)

Verify a discriminating regression test exists for every bug fixed to date;
add any that are missing. Known fixes and their expected tests:
1. b3110e7 — nil `relatedPaths` → GUI crash: engine `related_test.go`
   non-nil test + Uninstaller null-tolerance test.
2. 48693cc — `clean:done` `errors:null` white screen: docker/homebrew/dry-run
   non-nil Errors tests + ResultsPanel null-errors/null-results tests.
3. 3687337 — running-app gate in `startValidated` (queued + direct) and
   mount-refresh race (view skips refresh in waiting/running).
4. This effort — `Restore` skipping `items.json` without counting it.
The audit is a checklist task: run each named test, confirm it fails when the
guard is reverted (spot-check at least one per fix by temporary revert or by
reading the test's discriminating assertion), record results.

## Part 3 — Coverage hardening (user requirement: "full test coverage")

Interpretation (agreed): every package's meaningful logic covered; pure
glue that cannot run headless is excluded WITH justification, not silently.
- Measure: `go test ./... -coverprofile` for packages/engine, apps/desktop,
  apps/cli; `vitest run --coverage` for the desktop frontend.
- Raise: add tests for uncovered branches in that order of value:
  error paths in engine packages (backup, fsx, scanners, uninstall,
  maintenance, grouping, config), desktop `app.go` pure helpers
  (resolveScanIDs/resolveSelection/splitters), CLI cmd/internal packages,
  frontend stores/views with untested branches (e.g. cleanStore/scanStore
  error paths, Settings/Maintenance/Backups views).
- Exclusions (documented in the coverage report file, not code): Wails
  runtime-bound goroutines (runScan/runClean/runTMClear event emitters —
  covered indirectly via extracted pure functions), `main.go` entrypoints,
  generated `wailsjs/`, `tools/genicon`.
- Deliverable: a short `docs/superpowers/coverage-2026-07-18.md` with
  before/after per-package numbers and the exclusion list; all suites green.
- Target: engine and CLI packages ≥ 80% line coverage (those already above
  stay above); `apps/desktop` is dominated by excluded Wails-bound goroutines,
  so its target is instead: every pure helper and headless-safe bound method
  covered, with the residual uncovered functions individually listed in the
  coverage report; frontend ≥ 80% lines on `src/` (excluding `wailsjs/`).

## Error handling
- `Details` on unreadable session/HOME dir → empty non-nil Items, no error
  (mirrors `List()` tolerance).
- Frontend fetch rejection → expanded area shows `Couldn't read backup
  details` and the row can be re-expanded to retry (no cache on failure).

## Testing (Part 1 specifics)
Engine: manifest written with moved-only items (incl. a NotBackedUp mix);
manifest survives cancellation with partial items; Restore skips manifest
(0 failed) and restores the rest; Details from manifest; Details fallback
walk sorted+capped with Truncated; containment rejection; non-nil Items
always. Frontend: expand renders contracted paths + sizes; truncation tail;
empty state; fetch-failure state + retry; null tolerance.
