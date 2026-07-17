import { OpenFDASettings } from '../../wailsjs/go/main/App'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
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
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[520px] max-h-[85vh] overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <p className="text-3xl font-semibold text-green-600 dark:text-green-400">
          {formatSize(summary?.totalFreedSpace ?? 0)} {lastDryRun ? 'would be freed (dry run)' : 'freed'}
        </p>
        {cancelled ? (
          <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">
            Cancelled — partial results below.
          </p>
        ) : null}
        {error ? <p className="mt-1 text-sm text-red-600 dark:text-red-400">{error}</p> : null}

        <ul className="mt-4 space-y-2">
          {(summary?.results ?? []).map((r) => {
            // Defense in depth: a Go nil slice arrives as JSON null.
            const errors = r.errors ?? []
            return (
            <li key={r.category.id} className="text-sm">
              <span className={errors.length === 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                {errors.length === 0 ? `✓ ${r.category.name}` : `✗ ${r.category.name}`}
              </span>{' '}
              <span className="text-zinc-500 dark:text-zinc-400">
                {r.cleanedItems} item{r.cleanedItems === 1 ? '' : 's'} · {formatSize(r.freedSpace)}
              </span>
              {errors.map((e) => (
                <div key={e}>
                  <p className="mt-0.5 text-xs text-red-600 dark:text-red-400">{e}</p>
                  {e.includes('PROTECTED') ? (
                    <p className="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
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
          <div className="mt-4 rounded-md bg-amber-50 p-3 text-sm dark:bg-amber-950">
            <p className="text-amber-800 dark:text-amber-200">
              Some items could not be removed because App Cleaner lacks Full Disk Access.
            </p>
            <button
              className="mt-2 rounded-md bg-amber-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-amber-500"
              onClick={() => void OpenFDASettings()}
            >
              Grant Full Disk Access
            </button>
          </div>
        ) : null}

        {notBackedUp.length > 0 ? (
          <details className="mt-4 text-sm">
            <summary className="cursor-pointer text-zinc-600 dark:text-zinc-300">
              {notBackedUp.length} item(s) not backed up (outside home / other volume)
            </summary>
            <ul className="mt-2 space-y-1">
              {notBackedUp.map((p) => (
                <li key={p} className="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">
                  {p}
                </li>
              ))}
            </ul>
          </details>
        ) : null}

        <div className="mt-6 flex justify-end">
          <button
            className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
            onClick={onDone}
          >
            Done
          </button>
        </div>
      </div>
    </div>
  )
}
