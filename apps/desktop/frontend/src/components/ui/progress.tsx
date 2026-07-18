import * as ProgressPrimitive from '@radix-ui/react-progress'
import { cn } from '../../lib/cn'

export function Progress({ value, className }: { value: number; className?: string }) {
  return (
    <ProgressPrimitive.Root
      value={value}
      className={cn('h-2 w-full overflow-hidden rounded-full bg-hairline', className)}
    >
      <ProgressPrimitive.Indicator
        className="h-full rounded-full bg-accent transition-[width] duration-150"
        style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
      />
    </ProgressPrimitive.Root>
  )
}
