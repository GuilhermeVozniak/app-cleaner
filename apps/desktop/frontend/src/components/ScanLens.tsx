import { Search } from 'lucide-react'
import type { CSSProperties, ReactNode } from 'react'
import { formatSize } from '../lib/format'
import { cn } from '../lib/cn'

interface ScanLensProps {
  state: 'idle' | 'scanning'
  completed?: number
  total?: number
  totalSize?: number
  onScan: () => void
  /** Module hue for the glow ring; defaults to the accent blue. */
  hue?: string
  /** CTA label under the icon (idle state). */
  label?: string
  /** Replaces the search icon (idle state). */
  icon?: ReactNode
  size?: 'md' | 'lg'
}

/** The signature element: a circular layered-glass lens with a module-tinted
 * glow. Idle = the Scan button; scanning = the live meter. One bold moment —
 * everything around it stays quiet. */
export function ScanLens({
  state,
  completed = 0,
  total = 0,
  totalSize = 0,
  onScan,
  hue = 'var(--color-accent)',
  label = 'Smart Scan',
  icon,
  size = 'lg',
}: ScanLensProps) {
  const dim = size === 'lg' ? 'h-56 w-56' : 'h-40 w-40'
  const moduleVar = { '--module': hue } as CSSProperties
  const ring = (
    <>
      <div aria-hidden className="module-glow absolute inset-0 rounded-full glass-1" />
      <div aria-hidden className="absolute inset-3 rounded-full glass-2" />
      <div
        aria-hidden
        className="lens-sweep absolute inset-0 rounded-full"
        style={{
          background:
            'conic-gradient(from 0deg, transparent 0deg, rgb(255 255 255 / 0.35) 24deg, transparent 60deg)',
          maskImage: 'radial-gradient(closest-side, transparent 78%, black 80%)',
          WebkitMaskImage: 'radial-gradient(closest-side, transparent 78%, black 80%)',
        }}
      />
    </>
  )
  if (state === 'scanning') {
    return (
      <div
        className={cn('relative flex shrink-0 items-center justify-center', dim)}
        style={moduleVar}
        role="status"
      >
        {ring}
        <div className="relative z-10 flex flex-col items-center gap-1">
          <span className="nums text-2xl font-semibold text-ink">{completed}/{total}</span>
          <span className="nums text-sm text-ink-2">{formatSize(totalSize)}</span>
        </div>
      </div>
    )
  }
  return (
    <button
      type="button"
      onClick={onScan}
      style={moduleVar}
      className={cn(
        'group relative flex shrink-0 items-center justify-center rounded-full',
        dim,
        'transition-transform duration-150 hover:scale-[1.02] active:scale-[0.99]',
        'focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent',
      )}
    >
      {ring}
      <span className="relative z-10 flex flex-col items-center gap-2 text-ink">
        {icon ?? <Search size={28} style={{ color: 'var(--module)' }} />}
        <span className="text-lg font-semibold">{label}</span>
      </span>
    </button>
  )
}
