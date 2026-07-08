export interface ProgressOverlayProps {
  title: string
  current: number
  total: number
  itemName: string
  onCancel?: () => void
}

export function ProgressOverlay({ title, current, total, itemName, onCancel }: ProgressOverlayProps) {
  const pct = total > 0 ? Math.min(100, Math.round((current / total) * 100)) : 0
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40">
      <div className="w-[440px] rounded-xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
        <h2 className="text-lg font-semibold">{title}</h2>
        <p className="mt-1 truncate text-sm text-zinc-500 dark:text-zinc-400">{itemName || '…'}</p>
        <div className="mt-4 h-2 w-full overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-700">
          <div
            className="h-full rounded-full bg-indigo-600 transition-all"
            style={{ width: `${pct}%` }}
          />
        </div>
        <p className="mt-2 text-xs text-zinc-500 dark:text-zinc-400">
          {current} / {total}
        </p>
        {onCancel ? (
          <div className="mt-4 flex justify-end">
            <button
              className="rounded-md px-4 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
              onClick={onCancel}
            >
              Cancel
            </button>
          </div>
        ) : null}
      </div>
    </div>
  )
}
