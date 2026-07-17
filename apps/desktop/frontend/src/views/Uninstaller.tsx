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
  // Skipped while an uninstall is queued/running: a concurrent scan's
  // pre-deletion snapshot can resolve after uninstall:done and resurrect
  // just-deleted rows; finish()'s own reconciling refresh covers this once
  // the run completes.
  useEffect(() => {
    const phase = useUninstallerStore.getState().phase
    if (phase === 'waiting' || phase === 'running') return
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
