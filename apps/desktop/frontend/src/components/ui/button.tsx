import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/cn'

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 rounded-control text-sm font-medium transition-[background,box-shadow,transform] duration-150 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:pointer-events-none disabled:opacity-40',
  {
    variants: {
      variant: {
        primary: 'bg-accent text-white shadow-sm hover:brightness-110 active:scale-[0.98]',
        destructive: 'bg-danger text-white shadow-sm hover:brightness-110 active:scale-[0.98]',
        ghost: 'text-ink hover:bg-hairline',
        glass: 'glass-1 text-ink hover:brightness-105 active:scale-[0.98]',
      },
      size: {
        sm: 'h-7 px-2.5 text-xs',
        md: 'h-9 px-4',
        lg: 'h-11 px-6 text-base font-semibold',
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
