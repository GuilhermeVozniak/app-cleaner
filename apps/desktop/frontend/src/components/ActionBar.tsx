import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

/** Floating-styled glass control bar — the Tahoe "controls layer" look for
 * view footers (visual treatment only; call sites place it below the scroll
 * column). Replaces flat bordered footers. */
export function ActionBar({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('glass-2 mx-6 mb-4 flex items-center gap-4 rounded-card px-5 py-3', className)}>
      {children}
    </div>
  )
}
