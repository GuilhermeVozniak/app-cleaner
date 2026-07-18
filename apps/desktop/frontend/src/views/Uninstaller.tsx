import { useEffect, useState } from 'react'
import { GetAppIcon } from '../../wailsjs/go/main/App'
import { formatSize, timeAgo } from '../lib/format'
import { anySelectedRunning, contractHome } from '../lib/uninstallMath'
import { ProgressOverlay } from '../components/ProgressOverlay'
import { UninstallConfirm } from '../components/UninstallConfirm'
import { useUninstallerStore } from '../stores/uninstallerStore'
import { ActionBar } from '../components/ActionBar'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Checkbox } from '../components/ui/checkbox'
import { Dialog, DialogContent, DialogTitle } from '../components/ui/dialog'
import { Tooltip } from '../components/ui/tooltip'

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
    <div className="flex h-8 w-8 items-center justify-center rounded-md bg-hairline text-sm font-semibold text-ink-2">
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

  const uninstallButton = (
    <Button
      variant="destructive"
      disabled={selected.size === 0 || blocked}
      title={blocked ? 'Quit the app first' : undefined}
      onClick={() => store().openConfirm()}
      className="ml-auto"
    >
      Uninstall {selected.size} app{selected.size === 1 ? '' : 's'}
    </Button>
  )

  return (
    <div className="flex h-full flex-col p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Uninstaller</h1>
        <div className="flex items-center gap-3">
          {scanning && apps.length > 0 ? (
            <span className="text-xs text-ink-2">Refreshing…</span>
          ) : lastScanAt ? (
            <span className="text-xs text-ink-2">Updated {timeAgo(lastScanAt)}</span>
          ) : null}
          <Button variant="glass" size="sm" disabled={scanning} onClick={() => void store().refresh()}>
            Re-check
          </Button>
        </div>
      </div>
      {startError ? <p className="mt-2 text-sm text-danger">{startError}</p> : null}

      <div className="mt-4 flex-1 overflow-y-auto">
        {firstScan ? (
          <p className="text-sm text-ink-2">Scanning installed applications…</p>
        ) : (
          <ul className="space-y-1">
            {apps.map((app) => (
              <li
                key={app.path}
                className="glass-1 rounded-control px-3 py-2 transition hover:brightness-105"
              >
                <div className="flex items-center gap-3">
                  <Checkbox
                    aria-label={`Select ${app.name} (${app.path})`}
                    checked={selected.has(app.path)}
                    onCheckedChange={() => store().toggle(app.path)}
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
                  {app.running ? <Badge variant="risky">Running</Badge> : null}
                  {app.relatedPaths.length > 0 ? (
                    <Badge variant="neutral">+{app.relatedPaths.length} related</Badge>
                  ) : null}
                  <span className="nums w-24 text-right text-sm text-ink-2">
                    {formatSize(app.totalSize)}
                  </span>
                </div>
                {expanded === app.path ? (
                  <ul className="mt-2 space-y-0.5 pl-8">
                    <li className="truncate font-mono text-xs text-ink-2">
                      {app.path} ({formatSize(app.appSize)})
                    </li>
                    {app.relatedPaths.map((r) => (
                      <li key={r.path} className="truncate font-mono text-xs text-ink-2">
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

      <ActionBar>
        {blocked ? <Tooltip content="Quit the app first">{uninstallButton}</Tooltip> : uninstallButton}
      </ActionBar>

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
        <Dialog open>
          <DialogContent
            className="w-[480px]"
            onPointerDownOutside={(e) => e.preventDefault()}
            onEscapeKeyDown={(e) => e.preventDefault()}
          >
            <DialogTitle className="text-2xl font-semibold text-safe">
              {done.uninstalled} app{done.uninstalled === 1 ? '' : 's'} uninstalled ·{' '}
              {formatSize(done.freedSpace)} freed
            </DialogTitle>
            {skippedApps.length > 0 ? (
              <p className="mt-1 text-sm text-moderate">
                {skippedApps.length} app{skippedApps.length === 1 ? '' : 's'} already removed — skipped:{' '}
                {skippedApps.join(', ')}
              </p>
            ) : null}
            {done.cancelled ? (
              <p className="mt-1 text-sm text-moderate">Cancelled — partial results.</p>
            ) : null}
            {done.error ? (
              <p className="mt-1 text-sm text-danger">{done.error}</p>
            ) : null}
            {done.errors?.length ? (
              <ul className="mt-3 space-y-1">
                {done.errors.map((e) => (
                  <li key={e} className="text-xs text-danger">
                    ✗ {e}
                  </li>
                ))}
              </ul>
            ) : null}
            <div className="mt-6 flex justify-end">
              <button
                className="rounded-control bg-accent px-4 py-2 text-sm font-medium text-white hover:brightness-110"
                onClick={() => store().finish()}
              >
                Done
              </button>
            </div>
          </DialogContent>
        </Dialog>
      ) : null}
    </div>
  )
}

export default Uninstaller
