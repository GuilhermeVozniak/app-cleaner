import * as ProgressPrimitive from '@radix-ui/react-progress'
import { cn } from '../../lib/cn'

/** Thin bar. The fill takes the current text colour — set `text-safe` etc. on the root. */
export function Progress({ value, className }: { value: number; className?: string }) {
  return (
    <ProgressPrimitive.Root
      value={value}
      className={cn('h-1.5 w-full overflow-hidden rounded-full bg-[rgb(255_255_255/0.14)] text-white', className)}
    >
      <ProgressPrimitive.Indicator
        className="h-full rounded-full bg-current transition-[width] duration-200 ease-[var(--ease-glass)]"
        style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
      />
    </ProgressPrimitive.Root>
  )
}
