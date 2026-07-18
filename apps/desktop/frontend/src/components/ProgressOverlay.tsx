import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import { Progress } from './ui/progress'
import { Button } from './ui/button'

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
    <Dialog open>
      <DialogContent
        className="w-[440px]"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogTitle className="text-lg font-semibold">{title}</DialogTitle>
        <p className="mt-1 truncate text-sm text-ink-2">{itemName || '…'}</p>
        <Progress value={pct} className="mt-4" />
        <p className="nums mt-2 text-xs text-ink-2">
          {current} / {total}
        </p>
        {onCancel ? (
          <div className="mt-4 flex justify-end">
            <Button type="button" variant="ghost" onClick={onCancel}>
              Cancel
            </Button>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
