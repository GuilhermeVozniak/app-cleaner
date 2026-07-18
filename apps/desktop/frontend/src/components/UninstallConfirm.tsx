import { useState } from 'react'
import { formatSize } from '../lib/format'
import { contractHome, selectionTotals } from '../lib/uninstallMath'
import { Button } from './ui/button'
import { Checkbox } from './ui/checkbox'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import type { AppInfo } from '../lib/types'

export interface UninstallConfirmProps {
  apps: AppInfo[] // the selected apps only
  onCancel: () => void
  onConfirm: (dryRun: boolean) => void
}

export function UninstallConfirm({ apps, onCancel, onConfirm }: UninstallConfirmProps) {
  const [dryRun, setDryRun] = useState(false)
  const totals = selectionTotals(apps, new Set(apps.map((a) => a.path)))

  return (
    <Dialog open>
      <DialogContent
        className="max-h-[80vh]"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogTitle className="text-lg font-semibold">Uninstall applications</DialogTitle>
        <ul className="mt-4 space-y-3">
          {apps.map((app) => (
            <li key={app.path} className="text-sm">
              <span className="font-medium">✗ {app.name}</span>{' '}
              <span className="text-ink-2">({formatSize(app.totalSize)})</span>
              <ul className="mt-1 space-y-0.5">
                {app.relatedPaths.map((r) => (
                  <li key={r.path} className="truncate pl-4 font-mono text-xs text-ink-2">
                    └─ {contractHome(r.path)} ({formatSize(r.size)})
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
        <p className="mt-4 text-sm font-medium">
          Total: {formatSize(totals.size)} will be freed ({totals.paths} items)
        </p>
        <label className="mt-4 flex items-center gap-2 text-sm">
          <Checkbox checked={dryRun} onCheckedChange={(v) => setDryRun(v === true)} />
          Dry run (preview only, nothing is deleted)
        </label>
        <div className="mt-6 flex justify-end gap-3">
          <Button type="button" variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={() => onConfirm(dryRun)}>
            Uninstall
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
