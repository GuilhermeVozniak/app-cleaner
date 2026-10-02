import { Sparkles } from 'lucide-react'
import { OpenFDASettings } from '../../wailsjs/go/main/App'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import { Stage } from './Stage'
import { Button } from './ui/button'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import type { CleanSummary } from '../lib/types'

export function needsFdaHint(summary: CleanSummary | undefined): boolean {
  if (!summary) return false
  return (summary.results ?? []).some((r) =>
    (r.errors ?? []).some((e) => e.includes('EPERM') || e.includes('EACCES')),
  )
}

/** "Cleanup complete" stage: freed space headline, per-category outcome, follow-ups. */
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

  const results = summary?.results ?? []
  const items = summary?.totalCleanedItems ?? 0
  const subtitle = cancelled
    ? 'Cancelled, partial results below.'
    : lastDryRun
      ? `${items} item${items === 1 ? '' : 's'} would be removed`
      : `${items} item${items === 1 ? '' : 's'} removed`

  return (
    <Dialog open>
      <DialogContent
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <Stage
          Icon={Sparkles}
          title={
            <DialogTitle>
              {formatSize(summary?.totalFreedSpace ?? 0)} {lastDryRun ? 'would be freed (dry run)' : 'freed'}
            </DialogTitle>
          }
          subtitle={subtitle}
          footer={
            <Button type="button" variant="primary" onClick={onDone}>
              Done
            </Button>
          }
        >
          {error ? <p className="mb-3 text-body text-danger">{error}</p> : null}

          <ul className="max-h-[300px] divide-y divide-hairline overflow-y-auto pr-2">
            {results.map((r) => {
              // Defense in depth: a Go nil slice arrives as JSON null.
              const errors = r.errors ?? []
              const ok = errors.length === 0
              return (
                <li key={r.category.id} className="py-2.5">
                  <div className="flex items-center gap-3">
                    <span className={ok ? 'text-card font-semibold text-safe' : 'text-card font-semibold text-danger'}>
                      {ok ? `✓ ${r.category.name}` : `✗ ${r.category.name}`}
                    </span>
                    <span className="nums ml-auto text-body text-ink-2">
                      {r.cleanedItems} item{r.cleanedItems === 1 ? '' : 's'}, {formatSize(r.freedSpace)}
                    </span>
                  </div>
                  {errors.map((e) => (
                    <div key={e} className="mt-1">
                      <p className="text-caption text-danger">{e}</p>
                      {e.includes('PROTECTED') ? (
                        <p className="mt-0.5 text-caption text-ink-2">
                          Some items are blocked for safety (system-protected paths)
                        </p>
                      ) : null}
                    </div>
                  ))}
                </li>
              )
            })}
          </ul>

          {needsFdaHint(summary) ? (
            <div className="mt-4 flex items-center gap-4 rounded-control bg-moderate/15 px-4 py-3 text-body">
              <p className="flex-1 text-moderate">
                Some items could not be removed because App Cleaner lacks Full Disk Access.
              </p>
              <Button type="button" variant="secondary" size="sm" onClick={() => void OpenFDASettings()}>
                Grant Full Disk Access
              </Button>
            </div>
          ) : null}

          {notBackedUp.length > 0 ? (
            <details className="mt-4 text-body">
              <summary className="cursor-pointer text-ink-2">
                {notBackedUp.length} item(s) not backed up (outside home / other volume)
              </summary>
              <ul className="mt-2 space-y-1">
                {notBackedUp.map((p) => (
                  <li key={p} className="truncate font-mono text-caption text-ink-2">
                    {p}
                  </li>
                ))}
              </ul>
            </details>
          ) : null}
        </Stage>
      </DialogContent>
    </Dialog>
  )
}
