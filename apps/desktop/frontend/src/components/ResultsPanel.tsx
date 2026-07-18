import { OpenFDASettings } from '../../wailsjs/go/main/App'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import { Button } from './ui/button'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import type { CleanSummary } from '../lib/types'

export function needsFdaHint(summary: CleanSummary | undefined): boolean {
  if (!summary) return false
  return (summary.results ?? []).some((r) =>
    (r.errors ?? []).some((e) => e.includes('EPERM') || e.includes('EACCES')),
  )
}

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

  return (
    <Dialog open>
      <DialogContent
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogTitle className="text-3xl font-semibold text-safe">
          {formatSize(summary?.totalFreedSpace ?? 0)} {lastDryRun ? 'would be freed (dry run)' : 'freed'}
        </DialogTitle>
        {cancelled ? (
          <p className="mt-1 text-sm text-moderate">
            Cancelled — partial results below.
          </p>
        ) : null}
        {error ? <p className="mt-1 text-sm text-danger">{error}</p> : null}

        <ul className="mt-4 space-y-2">
          {(summary?.results ?? []).map((r) => {
            // Defense in depth: a Go nil slice arrives as JSON null.
            const errors = r.errors ?? []
            return (
            <li key={r.category.id} className="text-sm">
              <span className={errors.length === 0 ? 'text-safe' : 'text-danger'}>
                {errors.length === 0 ? `✓ ${r.category.name}` : `✗ ${r.category.name}`}
              </span>{' '}
              <span className="text-ink-2">
                {r.cleanedItems} item{r.cleanedItems === 1 ? '' : 's'} · {formatSize(r.freedSpace)}
              </span>
              {errors.map((e) => (
                <div key={e}>
                  <p className="mt-0.5 text-xs text-danger">{e}</p>
                  {e.includes('PROTECTED') ? (
                    <p className="mt-0.5 text-xs text-ink-2">
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
          <div className="mt-4 rounded-control bg-moderate/15 p-3 text-sm">
            <p className="text-moderate">
              Some items could not be removed because App Cleaner lacks Full Disk Access.
            </p>
            <Button
              type="button"
              variant="primary"
              onClick={() => void OpenFDASettings()}
              className="mt-2 h-auto bg-moderate px-3 py-1.5 hover:brightness-110"
            >
              Grant Full Disk Access
            </Button>
          </div>
        ) : null}

        {notBackedUp.length > 0 ? (
          <details className="mt-4 text-sm">
            <summary className="cursor-pointer text-ink-2">
              {notBackedUp.length} item(s) not backed up (outside home / other volume)
            </summary>
            <ul className="mt-2 space-y-1">
              {notBackedUp.map((p) => (
                <li key={p} className="truncate font-mono text-xs text-ink-2">
                  {p}
                </li>
              ))}
            </ul>
          </details>
        ) : null}

        <div className="mt-6 flex justify-end">
          <Button type="button" variant="primary" onClick={onDone}>
            Done
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
