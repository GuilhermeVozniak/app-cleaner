import type { LucideIcon } from 'lucide-react'
import { Sparkles } from 'lucide-react'
import { Stage } from './Stage'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import { Progress } from './ui/progress'
import { Button } from './ui/button'

export interface ProgressOverlayProps {
  title: string
  current: number
  total: number
  itemName: string
  onCancel?: () => void
  /** Gem icon for the stage; defaults to the Cleanup sparkle. */
  Icon?: LucideIcon
}

/** A running flow on its stage: title, current item, thin bar, fraction, optional Cancel. */
export function ProgressOverlay({ title, current, total, itemName, onCancel, Icon = Sparkles }: ProgressOverlayProps) {
  const pct = total > 0 ? Math.min(100, Math.round((current / total) * 100)) : 0
  return (
    <Dialog open>
      <DialogContent
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <Stage
          Icon={Icon}
          title={<DialogTitle>{title}</DialogTitle>}
          subtitle={<span className="block truncate">{itemName || '…'}</span>}
          footer={
            onCancel ? (
              <Button type="button" variant="secondary" onClick={onCancel}>
                Cancel
              </Button>
            ) : undefined
          }
        >
          <Progress value={pct} className="text-white" />
          <p className="nums mt-2 text-caption text-ink-2">
            {current} / {total}
          </p>
        </Stage>
      </DialogContent>
    </Dialog>
  )
}
