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
