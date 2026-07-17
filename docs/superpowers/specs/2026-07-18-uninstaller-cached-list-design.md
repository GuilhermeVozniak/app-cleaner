# Uninstaller cached app list + queued uninstalls — design

Date: 2026-07-18
Status: approved (design discussion in session)
Scope: `apps/desktop/frontend` only — no Go/bridge changes.

## Problem

The Uninstaller view holds its app list in component state. Every view switch,
app restart, and post-uninstall `refresh()` throws the list away and puts the
user back into the blocking "Scanning installed applications…" state, even
though the previous list is almost always still accurate. There is also no way
to act while a scan is in flight.

## Goals

1. The app list is cached — in memory across view switches and persisted across
   app restarts — and stays visible while a fresh scan runs in the background.
2. The blocking loading state appears only when there is no cached list at all
   (true first run).
3. An uninstall confirmed while a scan is in flight is acknowledged, queued,
   and executed only after the scan finishes, against the fresh list.
4. Queued apps that no longer exist are skipped and reported ("already
   removed"), never silently ignored (user decision: skip + note).

## Non-goals

- No Go-side caching or new bridge methods/events.
- No change to how `ListApps` scans or how `StartUninstall` removes apps.
- Icons are not persisted (large base64; Go already disk-caches the PNGs).

## Design

### New `stores/uninstallerStore.ts` (zustand)

Persisted via `zustand/middleware` `persist` to localStorage, key
`app-cleaner.uninstaller`, `version: 1`, `partialize` → `{ apps, lastScanAt }`:

- `apps: AppInfo[]` — always normalized (`relatedPaths ?? []`) before storage.
- `lastScanAt: string | null` — ISO timestamp of the last completed scan.

In-memory only:

- `scanning: boolean` — a `ListApps` call is in flight.
- `selected: Set<string>` — selection by bundle path (moves out of the
  component: refresh() prunes it and finish() clears it).
- `icons: Record<string, string>` — lazy icon cache (moves out of the
  component so it survives view switches).
- `phase: 'list' | 'confirm' | 'waiting' | 'running' | 'done'` — the flow
  state that today lives in the component. `waiting` is new: confirmed but
  queued behind a scan.
- `pending: { paths: string[]; dryRun: boolean } | null` — the queued request.
- `skippedApps: string[]` — names dropped at validation time (already gone).
- `progress`, `done`, `startError` — as today, relocated from the component.

Actions:

- `refresh()` — no-op if `scanning`. Sets `scanning: true`, keeps `apps`
  rendered, calls `ListApps`. On resolve: normalize, set `apps` +
  `lastScanAt`, prune `selected` paths that vanished, `scanning: false`, then
  if `pending` exists run the validation step below. On reject:
  `scanning: false`, cached list untouched; if `pending` exists, abort it with
  a visible error ("Couldn't verify installed apps — uninstall not started") —
  never uninstall against unverified data.
- `requestUninstall(paths, dryRun)` — if `scanning`: store as `pending`, set
  `phase: 'waiting'` (overlay: "Waiting for app scan to finish…"). Otherwise
  validate immediately.
- Validation (shared by both entry points): split requested paths into
  still-present vs gone by bundle path against the current `apps`. Gone →
  `skippedApps` (names). If nothing survives: `phase: 'done'` with a
  zero-count summary + skipped note, `StartUninstall` is NOT called. Otherwise
  `StartUninstall(present, dryRun)` and `phase: 'running'`.
- `finish()` — close the done dialog, clear selection/done/skipped, kick a
  background `refresh()`.

Event wiring: `uninstall:progress` / `uninstall:done` handlers move to
module-level `EventsOn` in the store (exact `cleanStore` pattern, exported for
tests) so a queued/running flow survives the view unmounting.

On `uninstall:done` for a real run (not dry-run, not cancelled): optimistically
remove the requested paths from `apps`, then background-`refresh()` to
reconcile (a failed removal reappears seconds later). Dry runs never mutate
the list.

### `views/Uninstaller.tsx`

- Reads everything from the store; local state keeps only `expanded`.
- Blocking "Scanning installed applications…" only when
  `apps.length === 0 && scanning`.
- While `scanning` with a cached list: small spinner on the Re-check button and
  an "Updated … ago" hint from `lastScanAt`; the list stays interactive.
- `waiting` phase renders the ProgressOverlay with title "Waiting for app scan
  to finish…" (indeterminate, 0/0), no Cancel; it hands off to the normal
  "Uninstalling…" overlay when the queued run starts.
- Done dialog: when `skippedApps` is non-empty, an amber line —
  "N app(s) already removed — skipped: <names>".

## Error handling

- `ListApps` rejection: keep cached list; abort any `pending` with the visible
  error above; Re-check remains available.
- `StartUninstall` rejection: unchanged behavior (error banner, back to list).
- Corrupt/old persisted state: `persist` `version` mismatch discards it →
  first-run loading state. `apps` re-normalized on rehydrate.

## Testing

Store tests (mock bridge + runtime, style of `scanStore.test.ts`):
list retained during refresh; queue while scanning → validated start after
scan; skip+note; all-skipped short-circuit (no `StartUninstall` call);
scan-failure aborts pending; optimistic removal on done; dry-run leaves list;
partialize excludes icons/phase.

Component tests: cached list + inline indicator instead of blocking state;
blocking state on true first run; waiting overlay after mid-scan confirm;
skipped names in done dialog.
