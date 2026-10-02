import { useEffect, useState } from 'react'
import { AppWindow, Trash2 } from 'lucide-react'
import { GetAppIcon } from '../../wailsjs/go/main/App'
import { formatSize, timeAgo } from '../lib/format'
import { anySelectedRunning, contractHome, selectionTotals } from '../lib/uninstallMath'
import { ProgressOverlay } from '../components/ProgressOverlay'
import { UninstallConfirm } from '../components/UninstallConfirm'
import { ModuleIcon } from '../components/ModuleIcon'
import { ScanLens } from '../components/ScanLens'
import { Stage } from '../components/Stage'
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
        className="h-9 w-9 shrink-0 rounded-[9px] shadow-[0_2px_6px_rgb(0_0_0/0.3)]"
      />
    )
  }
  return (
    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-fill text-card font-semibold text-ink-2">
      {name.charAt(0).toUpperCase()}
    </div>
  )
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`

export function Uninstaller() {
  const apps = useUninstallerStore((s) => s.apps)
  const scanning = useUninstallerStore((s) => s.scanning)
  const lastScanAt = useUninstallerStore((s) => s.lastScanAt)
  const selected = useUninstallerStore((s) => s.selected)
  const icons = useUninstallerStore((s) => s.icons)
  const phase = useUninstallerStore((s) => s.phase)
  const progress = useUninstallerStore((s) => s.progress)
  const done = useUninstallerStore((s) => s.done)
  const lastDryRun = useUninstallerStore((s) => s.lastDryRun)
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
  const totals = selectionTotals(apps, selected)
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
      Uninstall {plural(selected.size, 'app')}
    </Button>
  )

  const doneNotes = done
    ? [
        skippedApps.length > 0 ? (
          <p key="skipped" className="text-body text-moderate">
            {plural(skippedApps.length, 'app')} already removed — skipped: {skippedApps.join(', ')}
          </p>
        ) : null,
        done.cancelled ? (
          <p key="cancelled" className="text-body text-moderate">
            Cancelled. Partial results above.
          </p>
        ) : null,
        done.error ? (
          <p key="error" className="text-body text-danger">
            {done.error}
          </p>
        ) : null,
        done.errors?.length ? (
          <ul key="errors" className="space-y-1">
            {done.errors.map((e) => (
              <li key={e} className="text-caption text-danger">
                ✗ {e}
              </li>
            ))}
          </ul>
        ) : null,
      ].filter(Boolean)
    : []

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-y-auto px-10 pb-6 pt-6">
        <div className="materialize mx-auto max-w-4xl">
          {firstScan ? (
            <div className="flex flex-col items-center pt-24">
              <ScanLens
                state="scanning"
                hue="var(--color-module-apps)"
                onScan={() => {}}
                caption={<span className="text-body text-ink-2">Scanning installed applications…</span>}
              />
            </div>
          ) : (
            <>
              <h1 className="text-center text-headline font-semibold text-ink">
                We've found {plural(apps.length, 'app')} on your Mac
              </h1>
              <div className="mt-3 flex items-center justify-center gap-3 text-body text-ink-2">
                {scanning && apps.length > 0 ? (
                  <span>Refreshing…</span>
                ) : lastScanAt ? (
                  <span>Updated {timeAgo(lastScanAt)}</span>
                ) : null}
                <Button variant="secondary" size="sm" disabled={scanning} onClick={() => void store().refresh()}>
                  Re-check
                </Button>
              </div>
              {startError ? <p className="mt-3 text-center text-body text-danger">{startError}</p> : null}

              {apps.length > 0 ? (
                <ul className="glass-1 mt-8 divide-y divide-hairline rounded-card px-2">
                  {apps.map((app) => (
                    <li key={app.path} className="px-2">
                      <div className="flex h-14 items-center gap-3">
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
                          type="button"
                          onClick={() => setExpanded(expanded === app.path ? null : app.path)}
                          className="focus-ring min-w-0 flex-1 truncate rounded-control px-1.5 py-1 text-left text-card font-semibold text-ink"
                        >
                          {app.name}
                        </button>
                        {app.running ? <Badge variant="risky">Running</Badge> : null}
                        {app.relatedPaths.length > 0 ? (
                          <Badge variant="neutral">+{app.relatedPaths.length} related</Badge>
                        ) : null}
                        <span className="nums w-24 shrink-0 text-right text-body text-ink-2">
                          {formatSize(app.totalSize)}
                        </span>
                      </div>
                      {expanded === app.path ? (
                        <ul className="mb-3 space-y-0.5 pl-[70px]">
                          <li className="truncate font-mono text-caption text-ink-2">
                            {app.path} ({formatSize(app.appSize)})
                          </li>
                          {app.relatedPaths.map((r) => (
                            <li key={r.path} className="truncate font-mono text-caption text-ink-2">
                              {contractHome(r.path)} ({formatSize(r.size)})
                            </li>
                          ))}
                        </ul>
                      ) : null}
                    </li>
                  ))}
                </ul>
              ) : null}
            </>
          )}
        </div>
      </div>

      <ActionBar>
        <span className="nums text-body text-ink-2">
          {plural(selected.size, 'app')} selected, {formatSize(totals.size)}
        </span>
        {blocked ? (
          // Disabled buttons swallow pointer events (Button base sets
          // disabled:pointer-events-none), so a focusable span carries the
          // tooltip trigger and the native title.
          <Tooltip content="Quit the app first">
            <span tabIndex={0} title="Quit the app first" className="ml-auto inline-block">
              {uninstallButton}
            </span>
          </Tooltip>
        ) : (
          uninstallButton
        )}
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
            onPointerDownOutside={(e) => e.preventDefault()}
            onEscapeKeyDown={(e) => e.preventDefault()}
          >
            <Stage
              Icon={AppWindow}
              title={
                <DialogTitle asChild>
                  <span>{lastDryRun ? 'Dry run complete' : 'Uninstall complete!'}</span>
                </DialogTitle>
              }
              footer={
                <Button type="button" variant="primary" onClick={() => store().finish()}>
                  Done
                </Button>
              }
            >
              <div className="flex items-center gap-4">
                <ModuleIcon Icon={Trash2} size="md" />
                <div>
                  <div className="text-title font-semibold text-ink">
                    {plural(done.uninstalled, 'app')} {lastDryRun ? 'would be uninstalled' : 'uninstalled'}
                  </div>
                  <div className="text-body text-ink-2">{formatSize(done.freedSpace)} {lastDryRun ? 'would be freed' : 'freed'}</div>
                </div>
              </div>
              {doneNotes.length > 0 ? <div className="mt-5 space-y-2">{doneNotes}</div> : null}
            </Stage>
          </DialogContent>
        </Dialog>
      ) : null}
    </div>
  )
}

export default Uninstaller
