import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/cn'

const buttonVariants = cva(
  'focus-ring inline-flex select-none items-center justify-center gap-2 whitespace-nowrap rounded-control text-body font-semibold transition-[background-color,box-shadow,transform,opacity,filter] duration-150 disabled:pointer-events-none disabled:opacity-40',
  {
    variants: {
      variant: {
        /** Solid white — the one action that moves the flow forward. */
        primary: 'bg-white text-ink-inverse shadow-[0_1px_2px_rgb(0_0_0/0.25)] hover:bg-[#ebe7ff] active:scale-[0.98]',
        /** Translucent white — review, cancel, secondary paths. */
        secondary: 'bg-fill text-ink hover:bg-fill-hover active:scale-[0.98]',
        glass: 'bg-fill text-ink hover:bg-fill-hover active:scale-[0.98]',
        ghost: 'text-ink-2 hover:bg-glass-1 hover:text-ink',
        destructive: 'bg-danger text-white hover:brightness-110 active:scale-[0.98]',
        /** Filled with the current module hue. */
        module:
          'bg-[var(--module)] text-white shadow-[0_6px_20px_color-mix(in_srgb,var(--module)_40%,transparent)] hover:brightness-110 active:scale-[0.98]',
      },
      size: {
        sm: 'h-7 px-3 text-caption',
        md: 'h-9 px-4',
        lg: 'h-11 px-6 text-card',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
)

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button'
    return <Comp ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
  },
)
Button.displayName = 'Button'
