import * as CheckboxPrimitive from '@radix-ui/react-checkbox'
import { Check } from 'lucide-react'
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import { cn } from '../../lib/cn'

export const Checkbox = forwardRef<
  ElementRef<typeof CheckboxPrimitive.Root>,
  ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root>
>(({ className, ...props }, ref) => (
  <CheckboxPrimitive.Root
    ref={ref}
    className={cn(
      'focus-ring flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-[6px] border border-[rgb(255_255_255/0.35)] bg-[rgb(255_255_255/0.08)] transition-colors duration-150',
      'data-[state=checked]:border-transparent data-[state=checked]:bg-[var(--module,var(--color-accent))] data-[state=indeterminate]:border-transparent data-[state=indeterminate]:bg-[var(--module,var(--color-accent))]',
      'disabled:opacity-40',
      className,
    )}
    {...props}
  >
    <CheckboxPrimitive.Indicator>
      <Check size={12} strokeWidth={3.25} className="text-white drop-shadow-[0_1px_1px_rgb(0_0_0/0.3)]" />
    </CheckboxPrimitive.Indicator>
  </CheckboxPrimitive.Root>
))
Checkbox.displayName = 'Checkbox'
