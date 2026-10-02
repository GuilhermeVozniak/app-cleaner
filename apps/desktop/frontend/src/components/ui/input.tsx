import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        'focus-ring h-9 rounded-control border border-hairline bg-[rgb(255_255_255/0.08)] px-3 text-body text-ink [color-scheme:dark] placeholder:text-ink-3 focus-visible:bg-[rgb(255_255_255/0.12)]',
        className,
      )}
      {...props}
    />
  ),
)
Input.displayName = 'Input'
