import { useState } from 'react'
import { AppWindow } from 'lucide-react'
import { formatSize } from '../lib/format'
import { contractHome, selectionTotals } from '../lib/uninstallMath'
import { Stage } from './Stage'
import { Button } from './ui/button'
import { Checkbox } from './ui/checkbox'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import type { AppInfo } from '../lib/types'

export interface UninstallConfirmProps {
  apps: AppInfo[] // the selected apps only
  onCancel: () => void
  onConfirm: (dryRun: boolean) => void
}

/** Review stage before an uninstall: each app with its leftovers, the total, a dry-run switch. */
export function UninstallConfirm({ apps, onCancel, onConfirm }: UninstallConfirmProps) {
  const [dryRun, setDryRun] = useState(false)
  const totals = selectionTotals(apps, new Set(apps.map((a) => a.path)))

  return (
    <Dialog open>
      <DialogContent
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <Stage
          Icon={AppWindow}
          title={
            <DialogTitle asChild>
              <span>Uninstall applications</span>
            </DialogTitle>
          }
          subtitle={`${formatSize(totals.size)} will be freed (${totals.paths} item${totals.paths === 1 ? '' : 's'})`}
          footerStart={
            <label className="flex items-center gap-2 text-body text-ink-2">
              <Checkbox checked={dryRun} onCheckedChange={(v) => setDryRun(v === true)} />
              Dry run (preview only, nothing is deleted)
            </label>
          }
          footer={
            <>
              <Button type="button" variant="secondary" onClick={onCancel}>
                Cancel
              </Button>
              <Button type="button" variant="destructive" onClick={() => onConfirm(dryRun)}>
                Uninstall
              </Button>
            </>
          }
        >
          <ul className="max-h-[38vh] divide-y divide-hairline overflow-y-auto rounded-[14px] bg-[rgb(255_255_255/0.06)] px-4">
            {apps.map((app) => (
              <li key={app.path} className="py-3">
                <div className="flex items-center justify-between gap-4">
                  <span className="truncate text-card font-semibold text-ink">{app.name}</span>
                  <span className="nums shrink-0 text-body text-ink-2">{formatSize(app.totalSize)}</span>
                </div>
                {app.relatedPaths.length > 0 ? (
                  <ul className="mt-1 space-y-0.5">
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
        </Stage>
      </DialogContent>
    </Dialog>
  )
}
