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
  const currentApps = useUninstallerStore.getState().apps
  const present = new Set(currentApps.map((a) => a.path))
  const runningPaths = new Set(currentApps.filter((a) => a.running).map((a) => a.path))
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
  // Safety gate against the CURRENT list — an app may have launched since the
  // (possibly stale) list the user confirmed against was rendered. Aborts the
  // whole run, dry runs included: uninstalling a running app is blocked
  // everywhere else in the product.
  const runningNow = live.filter((r) => runningPaths.has(r.path))
  if (runningNow.length > 0) {
    useUninstallerStore.setState({
      phase: 'list',
      startError: `Still running — quit first: ${runningNow.map((r) => r.name).join(', ')}. Uninstall not started.`,
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
