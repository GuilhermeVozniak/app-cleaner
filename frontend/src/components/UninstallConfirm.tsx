import { useState } from 'react'
import { formatSize } from '../lib/format'
import { contractHome, selectionTotals } from '../lib/uninstallMath'
import type { AppInfo } from '../lib/types'

export interface UninstallConfirmProps {
  apps: AppInfo[] // the selected apps only
  onCancel: () => void
  onConfirm: (dryRun: boolean) => void
}

export function UninstallConfirm({ apps, onCancel, onConfirm }: UninstallConfirmProps) {
  const [dryRun, setDryRun] = useState(false)
  const totals = selectionTotals(apps, new Set(apps.map((a) => a.name)))

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[520px] max-h-[80vh] overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <h2 className="text-lg font-semibold">Uninstall applications</h2>
        <ul className="mt-4 space-y-3">
          {apps.map((app) => (
            <li key={app.name} className="text-sm">
              <span className="font-medium">✗ {app.name}</span>{' '}
              <span className="text-zinc-500 dark:text-zinc-400">({formatSize(app.totalSize)})</span>
              <ul className="mt-1 space-y-0.5">
                {app.relatedPaths.map((r) => (
                  <li key={r.path} className="truncate pl-4 font-mono text-xs text-zinc-500 dark:text-zinc-400">
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
          <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />
          Dry run (preview only, nothing is deleted)
        </label>
        <div className="mt-6 flex justify-end gap-3">
          <button
            className="rounded-md px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500"
            onClick={() => onConfirm(dryRun)}
          >
            Uninstall
          </button>
        </div>
      </div>
    </div>
  )
}
