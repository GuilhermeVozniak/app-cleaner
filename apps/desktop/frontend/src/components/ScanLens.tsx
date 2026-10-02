import type { CSSProperties, ReactNode } from 'react'
import { formatSize } from '../lib/format'
import { cn } from '../lib/cn'

interface ScanLensProps {
  state: 'idle' | 'scanning'
  completed?: number
  total?: number
  totalSize?: number
  onScan: () => void
  /** When given, the busy orb becomes a Stop button. */
  onStop?: () => void
  /** Module hue for the orb; defaults to the accent violet. */
  hue?: string
  /** CTA label (idle state). */
  label?: string
  /** Optional glyph above the label (idle state). */
  icon?: ReactNode
  /** Caption under the busy orb; defaults to the progress fraction. */
  caption?: ReactNode
  /** Idle only: greys the orb out (e.g. nothing selected). */
  disabled?: boolean
  className?: string
}

/**
 * The orb: one glowing circular CTA per screen, bottom-centre. Idle it is the
 * Scan/Clean/Run button; busy it spins a ring and (optionally) turns into Stop,
 * with live progress underneath.
 */
export function ScanLens({
  state,
  completed = 0,
  total = 0,
  totalSize = 0,
  onScan,
  onStop,
  hue = 'var(--color-accent)',
  label = 'Smart Scan',
  icon,
  caption,
  disabled = false,
  className,
}: ScanLensProps) {
  const moduleVar = { '--module': hue } as CSSProperties

  if (state === 'scanning') {
    const face = (
      <span className="orb-busy relative flex h-[100px] w-[100px] items-center justify-center rounded-full text-ink">
        <span aria-hidden className="orb-ring absolute -inset-[7px] rounded-full" />
        {onStop ? <span className="text-card font-semibold">Stop</span> : null}
      </span>
    )
    return (
      <div
        className={cn('flex shrink-0 flex-col items-center gap-3', className)}
        style={moduleVar}
        role="status"
      >
        {onStop ? (
          <button
            type="button"
            onClick={onStop}
            aria-label="Stop"
            className="focus-ring rounded-full transition-transform duration-200 hover:scale-[1.03] active:scale-[0.98]"
          >
            {face}
          </button>
        ) : (
          face
        )}
        <span className="nums flex flex-col items-center text-caption text-ink-2">
          {caption ?? (
            <>
              <span className="text-ink">{completed}/{total}</span>
              <span>{formatSize(totalSize)}</span>
            </>
          )}
        </span>
      </div>
    )
  }

  return (
    <button
      type="button"
      onClick={onScan}
      disabled={disabled}
      style={moduleVar}
      className={cn(
        'orb focus-ring relative flex h-[100px] w-[100px] shrink-0 items-center justify-center rounded-full text-ink',
        'transition-[transform,opacity,filter] duration-200 ease-[var(--ease-glass)] hover:scale-[1.04] active:scale-[0.98]',
        'disabled:pointer-events-none disabled:opacity-45 disabled:saturate-50',
        className,
      )}
    >
      <span className="flex flex-col items-center gap-1">
        {icon}
        <span className="text-card font-semibold">{label}</span>
      </span>
    </button>
  )
}

export default ScanLens
