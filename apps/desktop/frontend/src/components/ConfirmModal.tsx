import { useEffect, useMemo, useState } from 'react'
import { Sparkles } from 'lucide-react'
import { useScanStore } from '../stores/scanStore'
import { useCleanStore } from '../stores/cleanStore'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import { Stage } from './Stage'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Checkbox } from './ui/checkbox'
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

const SAFETY_LABELS: Record<Category['safetyLevel'], string> = {
  safe: 'Safe',
  moderate: 'Moderate',
  risky: 'Risky',
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
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <Stage
          Icon={Sparkles}
          title={<DialogTitle>Confirm clean</DialogTitle>}
          subtitle={
            <>
              {stats.itemCount} item{stats.itemCount === 1 ? '' : 's'}, {formatSize(stats.totalSize)} will be freed
            </>
          }
          footer={
            <>
              <Button type="button" variant="secondary" onClick={() => useCleanStore.getState().reset()}>
                Cancel
              </Button>
              <Button type="button" variant="primary" disabled={stats.itemCount === 0} onClick={onClean}>
                Clean
              </Button>
            </>
          }
        >
          <ul className="max-h-[228px] divide-y divide-hairline overflow-y-auto pr-2">
            {stats.categories.map((c) => (
              <li key={c.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 py-2.5">
                <span className="text-card font-semibold text-ink">{c.name}</span>
                {c.safetyLevel !== 'safe' && (
                  <Badge variant={c.safetyLevel}>{SAFETY_LABELS[c.safetyLevel]}</Badge>
                )}
                {c.safetyLevel === 'risky' && c.safetyNote ? (
                  <span className="basis-full text-caption text-risky">{c.safetyNote}</span>
                ) : null}
              </li>
            ))}
          </ul>

          <div className="mt-6 space-y-3">
            <label className="flex items-center gap-3 text-body text-ink">
              <Checkbox checked={backup} onCheckedChange={(v) => setBackup(v === true)} />
              Back up items before deleting (Undo)
            </label>
            <label className="flex items-center gap-3 text-body text-ink">
              <Checkbox checked={dryRun} onCheckedChange={(v) => setDryRun(v === true)} />
              Dry run (preview only, nothing is deleted)
            </label>
          </div>
        </Stage>
      </DialogContent>
    </Dialog>
  )
}
