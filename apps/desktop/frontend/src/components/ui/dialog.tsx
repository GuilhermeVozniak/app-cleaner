import * as DialogPrimitive from '@radix-ui/react-dialog'
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import { cn } from '../../lib/cn'

export const Dialog = DialogPrimitive.Root
export const DialogTitle = DialogPrimitive.Title

/** Centred glass stage over a dimmed, blurred canvas. */
export const DialogContent = forwardRef<
  ElementRef<typeof DialogPrimitive.Content>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Content>
>(({ className, children, ...props }, ref) => (
  <DialogPrimitive.Portal>
    <DialogPrimitive.Overlay className="fixed inset-0 z-40 bg-[rgb(8_4_24/0.55)] backdrop-blur-md" />
    <DialogPrimitive.Content
      ref={ref}
      aria-describedby={undefined}
      className={cn(
        'glass-2 fixed left-1/2 top-1/2 z-50 max-h-[86vh] w-[min(880px,calc(100vw-96px))] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-[28px] text-ink',
        'animate-[stage-in_260ms_var(--ease-glass)_both]',
        className,
      )}
      {...props}
    >
      {children}
    </DialogPrimitive.Content>
  </DialogPrimitive.Portal>
))
DialogContent.displayName = 'DialogContent'
