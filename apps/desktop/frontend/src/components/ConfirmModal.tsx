import { useEffect, useMemo, useState } from 'react'
import { useScanStore } from '../stores/scanStore'
import { useCleanStore } from '../stores/cleanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import type { Category, ScanResult } from '../lib/types'

export interface SelectionStats {
  itemCount: number
  totalSize: number
  categories: Category[]
}

type Selected = Record<string, Set<string> | 'all'>

export function computeSelectionStats(
  results: Record<string, ScanResult>,
  selected: Selected,
): SelectionStats {
  let itemCount = 0
  let totalSize = 0
  const categories: Category[] = []
  for (const [id, sel] of Object.entries(selected)) {
    const r = results[id]
    if (!r || !r.items || r.items.length === 0) continue
    const items = sel === 'all' ? r.items : r.items.filter((it) => sel.has(it.path))
    if (items.length === 0) continue
    categories.push(r.category)
    itemCount += items.length
    for (const it of items) totalSize += it.size
  }
  return { itemCount, totalSize, categories }
}

export function selectionAsPaths(
  results: Record<string, ScanResult>,
  selected: Selected,
): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  for (const [id, sel] of Object.entries(selected)) {
    const r = results[id]
    if (!r || !r.items) continue
    const paths =
      sel === 'all'
        ? r.items.map((it) => it.path)
        : r.items.filter((it) => sel.has(it.path)).map((it) => it.path)
    if (paths.length > 0) out[id] = paths
  }
  return out
}

export function defaultBackupEnabled(backupByDefault: boolean, categories: Category[]): boolean {
  return (
    backupByDefault &&
    categories.some((c) => c.safetyLevel === 'moderate' || c.safetyLevel === 'risky')
  )
}

const safetyColors: Record<Category['safetyLevel'], string> = {
  safe: 'text-safe',
  moderate: 'text-moderate',
  risky: 'text-risky',
}

export function ConfirmModal() {
  const results = useScanStore((s) => s.results)
  const selected = useScanStore((s) => s.selected)
  const config = useUiStore((s) => s.config)
  const startClean = useCleanStore((s) => s.startClean)

  useEffect(() => {
    if (!config) void useUiStore.getState().loadConfig()
  }, [config])

  const stats = useMemo(() => computeSelectionStats(results, selected), [results, selected])
  const [backup, setBackup] = useState(() =>
    defaultBackupEnabled(config?.backupByDefault ?? true, stats.categories),
  )
  const [dryRun, setDryRun] = useState(false)

  const onClean = () => {
    void startClean(selectionAsPaths(results, selected), { dryRun, backup })
  }

  return (
    <Dialog open>
      <DialogContent
        className="w-[480px] max-h-[80vh]"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogTitle className="text-lg font-semibold">Confirm clean</DialogTitle>
        <p className="mt-1 text-sm text-ink-2">
          {stats.itemCount} item{stats.itemCount === 1 ? '' : 's'} —{' '}
          <span className="font-medium">{formatSize(stats.totalSize)}</span> will be freed
        </p>

        <ul className="mt-4 space-y-2">
          {stats.categories.map((c) => (
            <li key={c.id} className="text-sm">
              <span className="font-medium">{c.name}</span>{' '}
              <span className={safetyColors[c.safetyLevel]}>({c.safetyLevel})</span>
              {c.safetyLevel === 'risky' && c.safetyNote ? (
                <p className="mt-0.5 text-xs text-risky">{c.safetyNote}</p>
              ) : null}
            </li>
          ))}
        </ul>

        <div className="mt-5 space-y-2">
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={backup}
              onChange={(e) => setBackup(e.target.checked)}
            />
            Back up items before deleting (Undo)
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(e) => setDryRun(e.target.checked)}
            />
            Dry run (preview only, nothing is deleted)
          </label>
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            className="rounded-control px-4 py-2 text-sm text-ink hover:bg-hairline"
            onClick={() => useCleanStore.getState().reset()}
          >
            Cancel
          </button>
          <button
            className="rounded-control bg-accent px-4 py-2 text-sm font-medium text-white hover:brightness-110 disabled:opacity-40"
            disabled={stats.itemCount === 0}
            onClick={onClean}
          >
            Clean
          </button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
