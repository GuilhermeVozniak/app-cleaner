import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

/** Floating glass control bar — the Tahoe "controls layer". Sticky inside a
 * scrolling content column; replaces flat bordered footers. */
export function ActionBar({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('glass-2 sticky bottom-4 z-10 mx-6 flex items-center gap-4 rounded-card px-5 py-3', className)}>
      {children}
    </div>
  )
}
