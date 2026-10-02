import { RotateCcw, type LucideIcon } from 'lucide-react'
import type { CSSProperties } from 'react'
import { createPortal } from 'react-dom'

/**
 * The title-strip action at the window's top-left: "Start Over" for flows
 * that have a result on screen, or a Back link (pass `icon={ArrowLeft}`)
 * for nested screens. Portalled to <body> so a view's entrance transform
 * cannot capture its fixed positioning.
 */
export function StartOver({
  onClick,
  label = 'Start Over',
  icon: Icon = RotateCcw,
}: {
  onClick: () => void
  label?: string
  icon?: LucideIcon
}) {
  return createPortal(
    <button
      type="button"
      onClick={onClick}
      style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
      className="focus-ring fixed left-[84px] top-1.5 z-[35] flex h-8 items-center gap-1.5 rounded-control px-2 text-body font-medium text-ink-2 transition-colors hover:bg-glass-1 hover:text-ink"
    >
      <Icon size={14} strokeWidth={2.25} />
      {label}
    </button>,
    document.body,
  )
}

export default StartOver
