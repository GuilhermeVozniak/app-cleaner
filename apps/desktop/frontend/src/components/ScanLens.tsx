import { Search } from 'lucide-react'
import { formatSize } from '../lib/format'
import { cn } from '../lib/cn'

interface ScanLensProps {
  state: 'idle' | 'scanning'
  completed?: number
  total?: number
  totalSize?: number
  onScan: () => void
}

/** The signature element: a circular layered-glass lens. Idle = the Smart
 * Scan button; scanning = the live meter. One bold moment — everything
 * around it stays quiet. */
export function ScanLens({ state, completed = 0, total = 0, totalSize = 0, onScan }: ScanLensProps) {
  const ring = (
    <>
      <div aria-hidden className="absolute inset-0 rounded-full glass-1" />
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
      <div className="relative flex h-56 w-56 items-center justify-center" role="status">
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
      className={cn(
        'group relative flex h-56 w-56 items-center justify-center rounded-full',
        'transition-transform duration-150 hover:scale-[1.02] active:scale-[0.99]',
        'focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent',
      )}
    >
      {ring}
      <span className="relative z-10 flex flex-col items-center gap-2 text-ink">
        <Search size={28} className="text-accent" />
        <span className="text-lg font-semibold">Smart Scan</span>
      </span>
    </button>
  )
}
