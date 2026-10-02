import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

/** Glass control bar pinned under a scrolling list: selection summary left, action right. */
export function ActionBar({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('glass-2 mx-10 mb-5 flex items-center gap-4 rounded-[16px] px-5 py-3', className)}>
      {children}
    </div>
  )
}

export default ActionBar
