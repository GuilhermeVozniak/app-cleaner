# Uninstaller Cached App List + Queued Uninstalls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Uninstaller keeps a cached, persisted app list visible while rescanning in the background, and uninstalls confirmed mid-scan are queued, validated against the fresh list, and skipped-with-a-note when apps are already gone.

**Architecture:** Frontend-only. A new zustand store (`uninstallerStore`) owns the app list (persisted to localStorage via `zustand/middleware` `persist`, `apps` + `lastScanAt` only), selection, icon cache, and the uninstall flow state machine (`list → confirm → waiting? → running → done`). Wails event handlers move from the component to module level (the `cleanStore` pattern). `Uninstaller.tsx` becomes a thin view over the store; only `expanded` stays local.

**Tech Stack:** React 18, zustand (+ built-in persist middleware, no new deps), vitest + @testing-library/react (jsdom), Wails v2 JS bindings.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-18-uninstaller-cached-list-design.md`.
- No Go/bridge changes; no new npm dependencies.
- Persist key `app-cleaner.uninstaller`, `version: 1`, partialize → `{ apps, lastScanAt }` ONLY (icons/selection/phase never persisted).
- Every `AppInfo[]` entering the store is normalized: `relatedPaths ?? []` (nil Go slice arrives as JSON null — same class as the b3110e7/48693cc crashes).
- Never call `StartUninstall` with data not validated against the freshest completed scan; on scan failure abort pending with the exact copy: `Couldn't verify installed apps — uninstall not started`.
- Frontend test command: `cd apps/desktop/frontend && npx vitest run <file>`; typecheck: `npx tsc --noEmit`.
- Commit trailers: `Co-Authored-By: WOZCODE <contact@withwoz.com>` and `Claude-Session: https://claude.ai/code/session_01MrKaJcom7uSdmXnRxiAjMd`.

---

### Task 1: `timeAgo` helper

**Files:**
- Modify: `apps/desktop/frontend/src/lib/format.ts`
- Test: `apps/desktop/frontend/src/lib/format.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `timeAgo(iso: string, now?: number): string` — `'just now'` under 60s, then `'N min ago'`, `'N h ago'`, `'N d ago'`; `''` for unparseable input. Task 3's view renders `Updated {timeAgo(lastScanAt)}`.

- [ ] **Step 1: Write the failing test** — append to `format.test.ts`:

```ts
import { formatSize, middleTruncate, timeAgo } from './format'

describe('timeAgo', () => {
  const now = Date.parse('2026-07-18T12:00:00Z')
  it('buckets seconds/minutes/hours/days', () => {
    expect(timeAgo('2026-07-18T11:59:30Z', now)).toBe('just now')
    expect(timeAgo('2026-07-18T11:55:00Z', now)).toBe('5 min ago')
    expect(timeAgo('2026-07-18T09:00:00Z', now)).toBe('3 h ago')
    expect(timeAgo('2026-07-15T12:00:00Z', now)).toBe('3 d ago')
  })
  it('returns empty string for unparseable input and clamps future times to just now', () => {
    expect(timeAgo('not-a-date', now)).toBe('')
    expect(timeAgo('2026-07-18T13:00:00Z', now)).toBe('just now')
  })
})
```

(Keep the existing import line's names — merge `timeAgo` into it.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/desktop/frontend && npx vitest run src/lib/format.test.ts`
Expected: FAIL — `timeAgo` is not exported.

- [ ] **Step 3: Write minimal implementation** — append to `format.ts`:

```ts
/** Coarse relative time for the "Updated …" hint. '' on unparseable input. */
export function timeAgo(iso: string, now: number = Date.now()): string {
  const t = Date.parse(iso)
  if (!Number.isFinite(t)) return ''
  const s = Math.max(0, Math.floor((now - t) / 1000))
  if (s < 60) return 'just now'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} min ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} h ago`
  return `${Math.floor(h / 24)} d ago`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/desktop/frontend && npx vitest run src/lib/format.test.ts`
Expected: PASS (all).

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/frontend/src/lib/format.ts apps/desktop/frontend/src/lib/format.test.ts
git commit -m "feat(uninstaller): timeAgo helper for the list-freshness hint"
```

---

### Task 2: `uninstallerStore`

**Files:**
- Create: `apps/desktop/frontend/src/stores/uninstallerStore.ts`
- Test: `apps/desktop/frontend/src/stores/uninstallerStore.test.ts`

**Interfaces:**
- Consumes: `ListApps`, `StartUninstall` from `../../wailsjs/go/main/App`; `EventsOn` from `../../wailsjs/runtime/runtime`; `AppInfo` from `../lib/types`.
- Produces (Task 3 relies on these exact names):
  - `useUninstallerStore` with state `{ apps, lastScanAt, scanning, selected: Set<string>, icons, phase: 'list'|'confirm'|'waiting'|'running'|'done', pending, lastRequested, lastDryRun, skippedApps, progress, done, startError }`
  - actions `refresh()`, `toggle(path)`, `cacheIcon(path, icon)`, `openConfirm()`, `closeConfirm()`, `requestUninstall(dryRun)`, `finish()`
  - exported `UninstallDone` interface and test-exported `handleUninstallProgress`, `handleUninstallDone`.

- [ ] **Step 1: Write the failing test** — create `uninstallerStore.test.ts`:

```ts
// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), EventsOff: vi.fn() }))
vi.mock('../../wailsjs/go/main/App', () => ({
  ListApps: vi.fn().mockResolvedValue([]),
  StartUninstall: vi.fn().mockResolvedValue(undefined),
}))

import { ListApps, StartUninstall } from '../../wailsjs/go/main/App'
import {
  useUninstallerStore,
  handleUninstallDone,
  handleUninstallProgress,
} from './uninstallerStore'
import type { AppInfo } from '../lib/types'

const ListAppsMock = ListApps as unknown as ReturnType<typeof vi.fn>
const StartUninstallMock = StartUninstall as unknown as ReturnType<typeof vi.fn>

function app(name: string, path: string): AppInfo {
  return { name, path, bundleId: `com.x.${name}`, appSize: 10, relatedPaths: [], totalSize: 10, running: false }
}

function resetStore() {
  useUninstallerStore.setState({
    apps: [], lastScanAt: null, scanning: false, selected: new Set<string>(),
    icons: {}, phase: 'list', pending: null, lastRequested: [], lastDryRun: false,
    skippedApps: [], progress: { current: 0, total: 0, appName: '' }, done: null, startError: null,
  })
}

beforeEach(() => {
  resetStore()
  localStorage.clear()
  ListAppsMock.mockReset().mockResolvedValue([])
  StartUninstallMock.mockReset().mockResolvedValue(undefined)
})

describe('refresh', () => {
  it('keeps the cached list visible while scanning, then replaces it', async () => {
    useUninstallerStore.setState({ apps: [app('Old', '/Applications/Old.app')] })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    expect(useUninstallerStore.getState().scanning).toBe(true)
    expect(useUninstallerStore.getState().apps).toHaveLength(1) // cached list untouched
    resolve([app('New', '/Applications/New.app')])
    await p
    const s = useUninstallerStore.getState()
    expect(s.scanning).toBe(false)
    expect(s.apps.map((a) => a.name)).toEqual(['New'])
    expect(s.lastScanAt).toBeTruthy()
  })

  it('normalises null relatedPaths and prunes vanished selections', async () => {
    useUninstallerStore.setState({ selected: new Set(['/Applications/Gone.app', '/Applications/Kept.app']) })
    ListAppsMock.mockResolvedValueOnce([
      { ...app('Kept', '/Applications/Kept.app'), relatedPaths: null as unknown as AppInfo['relatedPaths'] },
    ])
    await useUninstallerStore.getState().refresh()
    const s = useUninstallerStore.getState()
    expect(s.apps[0].relatedPaths).toEqual([])
    expect([...s.selected]).toEqual(['/Applications/Kept.app'])
  })

  it('on failure keeps the cached list and does not clear lastScanAt', async () => {
    useUninstallerStore.setState({ apps: [app('Old', '/Applications/Old.app')], lastScanAt: '2026-07-18T00:00:00Z' })
    ListAppsMock.mockRejectedValueOnce(new Error('boom'))
    await useUninstallerStore.getState().refresh()
    const s = useUninstallerStore.getState()
    expect(s.scanning).toBe(false)
    expect(s.apps).toHaveLength(1)
    expect(s.lastScanAt).toBe('2026-07-18T00:00:00Z')
  })

  it('persists only apps and lastScanAt', async () => {
    ListAppsMock.mockResolvedValueOnce([app('A', '/Applications/A.app')])
    await useUninstallerStore.getState().refresh()
    useUninstallerStore.setState({ icons: { '/Applications/A.app': 'xxx' }, phase: 'confirm' })
    const raw = localStorage.getItem('app-cleaner.uninstaller')
    expect(raw).toBeTruthy()
    const persisted = JSON.parse(raw as string)
    expect(Object.keys(persisted.state).sort()).toEqual(['apps', 'lastScanAt'])
    expect(persisted.version).toBe(1)
  })
})

describe('requestUninstall', () => {
  it('starts immediately when idle, filtering nothing when all apps exist', () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')],
      selected: new Set(['/Applications/A.app']),
    })
    useUninstallerStore.getState().requestUninstall(false)
    expect(StartUninstallMock).toHaveBeenCalledWith(['/Applications/A.app'], false)
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('running')
    expect(s.skippedApps).toEqual([])
    expect(s.lastRequested).toEqual(['/Applications/A.app'])
  })

  it('queues while scanning and starts after the scan with only still-present apps, noting skipped names', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')],
      selected: new Set(['/Applications/A.app', '/Applications/B.app']),
    })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    expect(useUninstallerStore.getState().phase).toBe('waiting')
    expect(StartUninstallMock).not.toHaveBeenCalled()
    resolve([app('A', '/Applications/A.app')]) // B vanished in the meantime
    await p
    expect(StartUninstallMock).toHaveBeenCalledWith(['/Applications/A.app'], false)
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('running')
    expect(s.skippedApps).toEqual(['B'])
  })

  it('short-circuits to done (no StartUninstall) when every queued app is gone', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    resolve([])
    await p
    expect(StartUninstallMock).not.toHaveBeenCalled()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('done')
    expect(s.done).toEqual({ uninstalled: 0, freedSpace: 0, errors: [] })
    expect(s.skippedApps).toEqual(['A'])
  })

  it('aborts a pending uninstall with a visible error when the verifying scan fails', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    let reject!: (e: Error) => void
    ListAppsMock.mockReturnValueOnce(new Promise((_r, rj) => { reject = rj }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    reject(new Error('scan died'))
    await p
    expect(StartUninstallMock).not.toHaveBeenCalled()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('list')
    expect(s.pending).toBeNull()
    expect(s.startError).toBe("Couldn't verify installed apps — uninstall not started")
  })

  it('surfaces StartUninstall rejection and returns to the list', async () => {
    StartUninstallMock.mockRejectedValueOnce(new Error('an uninstall is already running'))
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    useUninstallerStore.getState().requestUninstall(false)
    await vi.waitFor(() => expect(useUninstallerStore.getState().phase).toBe('list'))
    expect(useUninstallerStore.getState().startError).toContain('already running')
  })
})

describe('uninstall events', () => {
  it('progress drives the running phase', () => {
    handleUninstallProgress({ current: 1, total: 2, appName: 'A' })
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('running')
    expect(s.progress).toEqual({ current: 1, total: 2, appName: 'A' })
  })

  it('done optimistically removes requested apps on a real run', () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')],
      lastRequested: ['/Applications/A.app'],
      lastDryRun: false,
    })
    handleUninstallDone({ uninstalled: 1, freedSpace: 10, errors: [] })
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('done')
    expect(s.apps.map((a) => a.name)).toEqual(['B'])
  })

  it('dry runs and cancelled runs never mutate the list', () => {
    const both = [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')]
    useUninstallerStore.setState({ apps: both, lastRequested: ['/Applications/A.app'], lastDryRun: true })
    handleUninstallDone({ uninstalled: 1, freedSpace: 10, errors: [] })
    expect(useUninstallerStore.getState().apps).toHaveLength(2)
    useUninstallerStore.setState({ apps: both, lastDryRun: false, phase: 'running' })
    handleUninstallDone({ uninstalled: 0, freedSpace: 0, errors: [], cancelled: true })
    expect(useUninstallerStore.getState().apps).toHaveLength(2)
  })
})

describe('finish', () => {
  it('clears flow state, keeps the list, and kicks a background refresh', async () => {
    useUninstallerStore.setState({
      apps: [app('B', '/Applications/B.app')],
      phase: 'done',
      done: { uninstalled: 1, freedSpace: 10, errors: [] },
      skippedApps: ['A'],
      selected: new Set(['/Applications/B.app']),
    })
    ListAppsMock.mockResolvedValueOnce([app('B', '/Applications/B.app')])
    useUninstallerStore.getState().finish()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('list')
    expect(s.done).toBeNull()
    expect(s.skippedApps).toEqual([])
    expect(s.selected.size).toBe(0)
    expect(s.apps).toHaveLength(1) // still visible
    await vi.waitFor(() => expect(useUninstallerStore.getState().scanning).toBe(false))
    expect(ListAppsMock).toHaveBeenCalledTimes(1)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/desktop/frontend && npx vitest run src/stores/uninstallerStore.test.ts`
Expected: FAIL — cannot resolve `./uninstallerStore`.

- [ ] **Step 3: Write the implementation** — create `uninstallerStore.ts`:

```ts
import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { ListApps, StartUninstall } from '../../wailsjs/go/main/App'
import type { AppInfo } from '../lib/types'

export interface UninstallDone {
  uninstalled: number
  freedSpace: number
  errors: string[]
  cancelled?: boolean
  error?: string
}

export type UninstallerPhase = 'list' | 'confirm' | 'waiting' | 'running' | 'done'

interface RequestedApp {
  path: string
  name: string
}

interface UninstallerState {
  // Persisted (see partialize below):
  apps: AppInfo[]
  lastScanAt: string | null
  // In-memory:
  scanning: boolean
  selected: Set<string> // keyed by bundle path (names can collide, paths cannot)
  icons: Record<string, string> // lazy icon cache; survives view switches, never persisted
  phase: UninstallerPhase
  pending: { requested: RequestedApp[]; dryRun: boolean } | null
  lastRequested: string[] // paths of the in-flight run, for optimistic removal
  lastDryRun: boolean
  skippedApps: string[] // names dropped at validation (already removed)
  progress: { current: number; total: number; appName: string }
  done: UninstallDone | null
  startError: string | null
  refresh: () => Promise<void>
  toggle: (path: string) => void
  cacheIcon: (path: string, icon: string) => void
  openConfirm: () => void
  closeConfirm: () => void
  requestUninstall: (dryRun: boolean) => void
  finish: () => void
}

// A Go nil slice arrives as JSON null; normalise before anything touches
// .length/.map/.reduce (same defense as the ResultsPanel/relatedPaths fixes).
function normalizeApps(list: AppInfo[] | null | undefined): AppInfo[] {
  return (list ?? []).map((a) => ({ ...a, relatedPaths: a.relatedPaths ?? [] }))
}

// Validation gate: every uninstall funnels through here against the CURRENT
// list — immediately when idle, or after the in-flight scan when queued.
function startValidated(requested: RequestedApp[], dryRun: boolean): void {
  const present = new Set(useUninstallerStore.getState().apps.map((a) => a.path))
  const live = requested.filter((r) => present.has(r.path))
  const skipped = requested.filter((r) => !present.has(r.path)).map((r) => r.name)
  if (live.length === 0) {
    useUninstallerStore.setState({
      skippedApps: skipped,
      phase: 'done',
      done: { uninstalled: 0, freedSpace: 0, errors: [] },
    })
    return
  }
  useUninstallerStore.setState({
    skippedApps: skipped,
    phase: 'running',
    lastRequested: live.map((r) => r.path),
    lastDryRun: dryRun,
    progress: { current: 0, total: live.length, appName: '' },
  })
  StartUninstall(live.map((r) => r.path), dryRun).catch((e) => {
    useUninstallerStore.setState({ startError: String(e), phase: 'list' })
  })
}

export const useUninstallerStore = create<UninstallerState>()(
  persist(
    (set, get) => ({
      apps: [],
      lastScanAt: null,
      scanning: false,
      selected: new Set<string>(),
      icons: {},
      phase: 'list',
      pending: null,
      lastRequested: [],
      lastDryRun: false,
      skippedApps: [],
      progress: { current: 0, total: 0, appName: '' },
      done: null,
      startError: null,

      refresh: async () => {
        if (get().scanning) return
        set({ scanning: true })
        try {
          const apps = normalizeApps(await ListApps())
          const present = new Set(apps.map((a) => a.path))
          set((s) => ({
            apps,
            lastScanAt: new Date().toISOString(),
            scanning: false,
            selected: new Set([...s.selected].filter((p) => present.has(p))),
          }))
          const pending = get().pending
          if (pending) {
            set({ pending: null })
            startValidated(pending.requested, pending.dryRun)
          }
        } catch {
          // Keep the cached list. Never uninstall against unverified data:
          // a queued request is aborted, visibly.
          const hadPending = get().pending !== null
          set({
            scanning: false,
            pending: null,
            ...(hadPending
              ? {
                  phase: 'list' as const,
                  startError: "Couldn't verify installed apps — uninstall not started",
                }
              : {}),
          })
        }
      },

      toggle: (path) =>
        set((s) => {
          const next = new Set(s.selected)
          if (next.has(path)) next.delete(path)
          else next.add(path)
          return { selected: next }
        }),

      cacheIcon: (path, icon) =>
        set((s) => (path in s.icons ? s : { icons: { ...s.icons, [path]: icon } })),

      openConfirm: () => set({ phase: 'confirm' }),
      closeConfirm: () => set({ phase: 'list' }),

      requestUninstall: (dryRun) => {
        const s = get()
        const byPath = new Map(s.apps.map((a) => [a.path, a.name]))
        // Names captured NOW: a skipped app is absent from the fresh list, so
        // its display name only exists in the list the user confirmed against.
        const requested = [...s.selected].map((path) => ({
          path,
          name: byPath.get(path) ?? path,
        }))
        set({ startError: null, skippedApps: [], done: null })
        if (s.scanning) {
          set({ pending: { requested, dryRun }, phase: 'waiting' })
          return
        }
        startValidated(requested, dryRun)
      },

      finish: () => {
        set({
          phase: 'list',
          done: null,
          skippedApps: [],
          selected: new Set<string>(),
          lastRequested: [],
        })
        void get().refresh()
      },
    }),
    {
      name: 'app-cleaner.uninstaller',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ apps: s.apps, lastScanAt: s.lastScanAt }),
      merge: (persisted, current) => {
        const p = (persisted ?? {}) as Partial<Pick<UninstallerState, 'apps' | 'lastScanAt'>>
        return { ...current, apps: normalizeApps(p.apps), lastScanAt: p.lastScanAt ?? null }
      },
    },
  ),
)

// Exported for tests; registered module-level (not in the component) so a
// queued/running flow survives the Uninstaller view unmounting.
export function handleUninstallProgress(d: {
  current: number
  total: number
  appName: string
}): void {
  useUninstallerStore.setState({ phase: 'running', progress: d })
}

export function handleUninstallDone(d: UninstallDone): void {
  const s = useUninstallerStore.getState()
  // Optimistic removal on a real completed run; the finish() refresh
  // reconciles (a failed removal reappears). Dry/cancelled runs: untouched.
  const apps =
    !s.lastDryRun && !d.cancelled ? s.apps.filter((a) => !s.lastRequested.includes(a.path)) : s.apps
  useUninstallerStore.setState({ phase: 'done', done: d, apps })
}

EventsOn('uninstall:progress', handleUninstallProgress as (...data: any) => void)
EventsOn('uninstall:done', handleUninstallDone as (...data: any) => void)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/desktop/frontend && npx vitest run src/stores/uninstallerStore.test.ts`
Expected: PASS (13 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/frontend/src/stores/uninstallerStore.ts apps/desktop/frontend/src/stores/uninstallerStore.test.ts
git commit -m "feat(uninstaller): persisted app-list store with queued, validated uninstalls"
```

---

### Task 3: `Uninstaller.tsx` consumes the store

**Files:**
- Modify: `apps/desktop/frontend/src/views/Uninstaller.tsx` (full rewrite below)
- Test: `apps/desktop/frontend/src/views/Uninstaller.test.tsx`

**Interfaces:**
- Consumes: everything from Task 2's `useUninstallerStore`; `timeAgo` from Task 1; existing `RowIcon` pattern, `ProgressOverlay`, `UninstallConfirm`, `formatSize`, `contractHome`, `anySelectedRunning`.
- Produces: same default export `Uninstaller`; no interface changes for `App.tsx`.

- [ ] **Step 1: Update the test file** — in `Uninstaller.test.tsx`, add a store import + reset, and the new behavior tests. Add to the imports:

```ts
import { useUninstallerStore } from '../stores/uninstallerStore'
import type { AppInfo } from '../lib/types'
```

Replace the existing `beforeEach(...)` with:

```ts
beforeEach(() => {
  useUninstallerStore.setState({
    apps: [], lastScanAt: null, scanning: false, selected: new Set<string>(),
    icons: {}, phase: 'list', pending: null, lastRequested: [], lastDryRun: false,
    skippedApps: [], progress: { current: 0, total: 0, appName: '' }, done: null, startError: null,
  })
  localStorage.clear()
  ListAppsMock.mockClear()
  ListAppsMock.mockResolvedValue(apps)
  StartUninstallMock.mockClear()
  StartUninstallMock.mockResolvedValue(undefined)
  GetAppIconMock.mockReset()
  GetAppIconMock.mockResolvedValue('')
})
```

Append these tests inside the `describe('<Uninstaller />')` block:

```ts
it('keeps the cached list visible (no blocking state) while a background refresh runs', async () => {
  useUninstallerStore.setState({ apps, lastScanAt: '2026-07-18T00:00:00Z' })
  ListAppsMock.mockReturnValue(new Promise(() => {})) // scan never resolves
  render(<Uninstaller />)
  expect(screen.getByText('OldApp')).toBeDefined() // cached rows render immediately
  expect(screen.queryByText('Scanning installed applications…')).toBeNull()
  expect(await screen.findByText('Refreshing…')).toBeDefined()
})

it('shows the blocking scanning state only on a true first run (no cached apps)', async () => {
  ListAppsMock.mockReturnValue(new Promise(() => {}))
  render(<Uninstaller />)
  expect(await screen.findByText('Scanning installed applications…')).toBeDefined()
})

it('shows the waiting overlay when an uninstall is confirmed mid-scan', async () => {
  useUninstallerStore.setState({ apps, selected: new Set(['/Applications/OldApp.app']) })
  ListAppsMock.mockReturnValue(new Promise(() => {})) // keep the scan in flight
  render(<Uninstaller />)
  fireEvent.click(screen.getByRole('button', { name: /uninstall 1 app/i }))
  fireEvent.click(await screen.findByRole('button', { name: /^uninstall$/i }))
  expect(await screen.findByText('Waiting for app scan to finish…')).toBeDefined()
  expect(StartUninstallMock).not.toHaveBeenCalled()
})

it('lists skipped (already removed) apps in the done dialog', async () => {
  useUninstallerStore.setState({
    phase: 'done',
    done: { uninstalled: 1, freedSpace: 100, errors: [] },
    skippedApps: ['GhostApp'],
  })
  render(<Uninstaller />)
  expect(await screen.findByText(/1 app already removed — skipped: GhostApp/)).toBeDefined()
})
```

Note: the existing `'does not crash when the bridge returns null relatedPaths'`, `'lists apps…'`, `'Re-check…'`, dry-run/selection tests continue to pass unchanged — the store performs the same normalization and `ListApps`/`StartUninstall` calls.

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `cd apps/desktop/frontend && npx vitest run src/views/Uninstaller.test.tsx`
Expected: the 4 new tests FAIL (component still uses local state); old ones may fail too — fine.

- [ ] **Step 3: Rewrite the view** — replace `Uninstaller.tsx` with:

```tsx
import { useEffect, useState } from 'react'
import { GetAppIcon } from '../../wailsjs/go/main/App'
import { formatSize, timeAgo } from '../lib/format'
import { anySelectedRunning, contractHome } from '../lib/uninstallMath'
import { ProgressOverlay } from '../components/ProgressOverlay'
import { UninstallConfirm } from '../components/UninstallConfirm'
import { useUninstallerStore } from '../stores/uninstallerStore'

interface RowIconProps {
  path: string
  name: string
  icon: string | undefined // undefined = not fetched yet; '' = fetched, none available
  onLoaded: (path: string, icon: string) => void
}

// Lazy per-row icon; results cached in the store so view switches never refetch.
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
  const apps = useUninstallerStore((s) => s.apps)
  const scanning = useUninstallerStore((s) => s.scanning)
  const lastScanAt = useUninstallerStore((s) => s.lastScanAt)
  const selected = useUninstallerStore((s) => s.selected)
  const icons = useUninstallerStore((s) => s.icons)
  const phase = useUninstallerStore((s) => s.phase)
  const progress = useUninstallerStore((s) => s.progress)
  const done = useUninstallerStore((s) => s.done)
  const skippedApps = useUninstallerStore((s) => s.skippedApps)
  const startError = useUninstallerStore((s) => s.startError)
  const [expanded, setExpanded] = useState<string | null>(null)

  // Background refresh on every mount — the cached list stays visible.
  useEffect(() => {
    void useUninstallerStore.getState().refresh()
  }, [])

  const blocked = anySelectedRunning(apps, selected)
  const selectedApps = apps.filter((a) => selected.has(a.path))
  const firstScan = apps.length === 0 && scanning
  const store = useUninstallerStore.getState

  return (
    <div className="flex h-full flex-col p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Uninstaller</h1>
        <div className="flex items-center gap-3">
          {scanning && apps.length > 0 ? (
            <span className="text-xs text-zinc-500 dark:text-zinc-400">Refreshing…</span>
          ) : lastScanAt ? (
            <span className="text-xs text-zinc-500 dark:text-zinc-400">
              Updated {timeAgo(lastScanAt)}
            </span>
          ) : null}
          <button
            className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm hover:bg-zinc-100 disabled:opacity-40 dark:border-zinc-700 dark:hover:bg-zinc-800"
            disabled={scanning}
            onClick={() => void store().refresh()}
          >
            Re-check
          </button>
        </div>
      </div>
      {startError ? (
        <p className="mt-2 text-sm text-red-600 dark:text-red-400">{startError}</p>
      ) : null}

      <div className="mt-4 flex-1 overflow-y-auto">
        {firstScan ? (
          <p className="text-sm text-zinc-500 dark:text-zinc-400">Scanning installed applications…</p>
        ) : (
          <ul className="divide-y divide-zinc-200 dark:divide-zinc-800">
            {apps.map((app) => (
              <li key={app.path} className="py-2">
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    aria-label={`Select ${app.name} (${app.path})`}
                    checked={selected.has(app.path)}
                    onChange={() => store().toggle(app.path)}
                  />
                  <RowIcon
                    path={app.path}
                    name={app.name}
                    icon={icons[app.path]}
                    onLoaded={store().cacheIcon}
                  />
                  <button
                    className="flex-1 truncate text-left text-sm font-medium"
                    onClick={() => setExpanded(expanded === app.path ? null : app.path)}
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
                {expanded === app.path ? (
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
          onClick={() => store().openConfirm()}
        >
          Uninstall {selected.size} app{selected.size === 1 ? '' : 's'}
        </button>
      </div>

      {phase === 'confirm' ? (
        <UninstallConfirm
          apps={selectedApps}
          onCancel={() => store().closeConfirm()}
          onConfirm={(dryRun) => store().requestUninstall(dryRun)}
        />
      ) : null}

      {phase === 'waiting' ? (
        <ProgressOverlay
          title="Waiting for app scan to finish…"
          current={0}
          total={0}
          itemName="Verifying installed apps"
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
            {skippedApps.length > 0 ? (
              <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">
                {skippedApps.length} app{skippedApps.length === 1 ? '' : 's'} already removed — skipped:{' '}
                {skippedApps.join(', ')}
              </p>
            ) : null}
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
                onClick={() => store().finish()}
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

- [ ] **Step 4: Run the view tests until green**

Run: `cd apps/desktop/frontend && npx vitest run src/views/Uninstaller.test.tsx`
Expected: PASS (14 tests: 10 existing + 4 new). If an existing test fails, fix the TEST only if its setup assumed component-local state (e.g. it needs the store reset from Step 1); never weaken assertions about visible behavior.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/frontend/src/views/Uninstaller.tsx apps/desktop/frontend/src/views/Uninstaller.test.tsx
git commit -m "feat(uninstaller): cached list stays visible; queued uninstalls with skip notes"
```

---

### Task 4: Full verification

**Files:** none new.

**Interfaces:** n/a — regression gate.

- [ ] **Step 1: Full frontend suite + typecheck**

Run: `cd apps/desktop/frontend && npx vitest run && npx tsc --noEmit`
Expected: all files PASS (≥99 tests), no TS errors.

- [ ] **Step 2: Desktop Go tests still green (no Go changes expected — sanity)**

Run: `cd apps/desktop && go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 3: Commit any stragglers and report**

```bash
git status --short   # expect clean; commit leftovers with fix(uninstaller): … if any
```
