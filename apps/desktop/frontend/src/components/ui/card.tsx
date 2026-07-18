import { forwardRef, type HTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  elevation?: 1 | 2
}

export const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, elevation = 1, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(elevation === 2 ? 'glass-2' : 'glass-1', 'rounded-card', className)}
      {...props}
    />
  ),
)
Card.displayName = 'Card'
