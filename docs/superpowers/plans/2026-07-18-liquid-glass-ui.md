# Liquid Glass UI Revamp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reskin the desktop frontend to Apple's Liquid Glass language on a shadcn-style component system, preserving every behavior, copy string, and store interaction.

**Architecture:** Foundation first (Tailwind v4 tokens + glass utilities + `cn()` + aurora Backdrop), then a `components/ui/` shadcn layer (CVA + Radix), then the shell, then view-by-view migration, then overlay/dialog migration, then a polish + gate pass. Every task lands with `npx vitest run` and `npx tsc --noEmit` green.

**Tech Stack:** React 18, Tailwind v4 (CSS-first `@theme`/`@utility`), class-variance-authority, clsx + tailwind-merge, Radix primitives (checkbox/dialog/progress/switch/tooltip/slot), lucide-react, zustand (untouched), vitest + Testing Library.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-18-liquid-glass-ui-design.md` — exact token values, glass recipe, radii (shell 20 / card 16 / control 10 / pill), motion (150/220ms, `cubic-bezier(0.32,0.72,0,1)`) live there and in Task 1's code; reuse, never re-invent.
- ZERO behavior change: all user-visible copy, aria-labels, roles, `title` attrs, and store wiring stay byte-identical. Existing tests may be edited ONLY where a query/assertion depended on incidental DOM structure (documented allowed edits: native-checkbox `.checked` → `aria-checked === 'true'` when Radix Checkbox replaces an `<input>`; container class/tag queries). Every such edit is listed in the task report.
- Allowed new deps (frontend package only): `class-variance-authority clsx tailwind-merge @radix-ui/react-checkbox @radix-ui/react-dialog @radix-ui/react-progress @radix-ui/react-switch @radix-ui/react-tooltip @radix-ui/react-slot`. Nothing else.
- No engine/bridge/Go changes; `wailsjs/` untouched.
- No raw color literals in components — token classes only (`bg-canvas`, `text-ink`, `text-ink-2`, `bg-accent`, `text-danger`, `bg-safe/15`…) plus the `glass-1`/`glass-2`/`nums` utilities.
- ≤ 2 stacked backdrop-filter regions; aurora canvas is static plain gradients.
- `prefers-reduced-transparency` → solid fills, no blur; `prefers-reduced-motion` → no non-essential animation.
- Coverage floor at the end: frontend lines ≥ 90% on `src/` (current 93.31%, allowed drop ≤ 3pts).
- Test commands: `cd apps/desktop/frontend && npx vitest run` and `npx tsc --noEmit`.
- Commit trailers on every commit:
  `Co-Authored-By: WOZCODE <contact@withwoz.com>`
  `Claude-Session: https://claude.ai/code/session_01MrKaJcom7uSdmXnRxiAjMd`

---

### Task 1: Foundation — deps, tokens, glass utilities, cn(), Backdrop

**Files:**
- Modify: `apps/desktop/frontend/package.json` (via bun add), `apps/desktop/frontend/src/style.css` (full rewrite below)
- Create: `apps/desktop/frontend/src/lib/cn.ts`, `apps/desktop/frontend/src/components/Backdrop.tsx`
- Test: `apps/desktop/frontend/src/lib/cn.test.ts`, `apps/desktop/frontend/src/components/Backdrop.test.tsx`

**Interfaces:**
- Produces: `cn(...inputs: ClassValue[]): string`; utility classes `glass-1`, `glass-2`, `nums`; token color classes `canvas`, `ink`, `ink-2`, `accent`, `danger`, `safe`, `moderate`, `risky`, `hairline`, `surface-solid`; radius tokens `rounded-shell|card|control`; `<Backdrop />` (fixed aurora layer). Every later task consumes these.

- [ ] **Step 1: Install deps**

```bash
cd apps/desktop/frontend && bun add class-variance-authority clsx tailwind-merge @radix-ui/react-checkbox @radix-ui/react-dialog @radix-ui/react-progress @radix-ui/react-switch @radix-ui/react-tooltip @radix-ui/react-slot
```

- [ ] **Step 2: Write failing tests**

`src/lib/cn.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { cn } from './cn'

describe('cn', () => {
  it('merges conditional classes and resolves tailwind conflicts', () => {
    expect(cn('p-2', false && 'hidden', 'p-4')).toBe('p-4')
    expect(cn('text-ink', undefined, 'font-mono')).toBe('text-ink font-mono')
  })
})
```

`src/components/Backdrop.test.tsx`:
```tsx
// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import { Backdrop } from './Backdrop'

afterEach(() => cleanup())

describe('<Backdrop />', () => {
  it('renders a fixed, pointer-transparent, aria-hidden layer', () => {
    const { container } = render(<Backdrop />)
    const el = container.firstElementChild as HTMLElement
    expect(el.getAttribute('aria-hidden')).toBe('true')
    expect(el.className).toContain('pointer-events-none')
    expect(el.className).toContain('fixed')
  })
})
```

- [ ] **Step 3: Run to verify RED** — `npx vitest run src/lib/cn.test.ts src/components/Backdrop.test.tsx` → FAIL (modules missing).

- [ ] **Step 4: Implement**

`src/lib/cn.ts`:
```ts
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
```

`src/style.css` — REPLACE the whole file with:
```css
@import "tailwindcss";

@theme {
  --color-canvas: #f5f6f8;
  --color-surface-solid: #ffffff;
  --color-aurora-sky: #7cb8ff;
  --color-aurora-mint: #7fe0c3;
  --color-aurora-violet: #c6b3ff;
  --color-glass-1: rgb(255 255 255 / 0.55);
  --color-glass-2: rgb(255 255 255 / 0.68);
  --color-ink: #1d1d1f;
  --color-ink-2: #6e6e73;
  --color-accent: #0a84ff;
  --color-danger: #ff453a;
  --color-safe: #30d158;
  --color-moderate: #ff9f0a;
  --color-risky: #ff453a;
  --color-hairline: rgb(0 0 0 / 0.08);
  --radius-shell: 20px;
  --radius-card: 16px;
  --radius-control: 10px;
  --ease-glass: cubic-bezier(0.32, 0.72, 0, 1);
  --glass-shadow: 0 8px 24px rgb(0 0 0 / 0.10);
  --glass-shadow-2: 0 16px 40px rgb(0 0 0 / 0.14);
  --glass-ring: rgb(255 255 255 / 0.30);
  --aurora-alpha: 0.35;
}

@media (prefers-color-scheme: dark) {
  :root {
    --color-canvas: #0b0e14;
    --color-surface-solid: #171a22;
    --color-glass-1: rgb(23 26 34 / 0.55);
    --color-glass-2: rgb(28 32 42 / 0.68);
    --color-ink: #f5f5f7;
    --color-ink-2: #98989d;
    --color-hairline: rgb(255 255 255 / 0.10);
    --glass-shadow: 0 8px 24px rgb(0 0 0 / 0.35);
    --glass-shadow-2: 0 16px 40px rgb(0 0 0 / 0.45);
    --glass-ring: rgb(255 255 255 / 0.12);
    --aurora-alpha: 0.16;
  }
}

@utility glass-1 {
  background: var(--color-glass-1);
  backdrop-filter: blur(20px) saturate(1.4);
  -webkit-backdrop-filter: blur(20px) saturate(1.4);
  border: 1px solid var(--glass-ring);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.25), var(--glass-shadow);
}

@utility glass-2 {
  background: var(--color-glass-2);
  backdrop-filter: blur(28px) saturate(1.4);
  -webkit-backdrop-filter: blur(28px) saturate(1.4);
  border: 1px solid var(--glass-ring);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.25), var(--glass-shadow-2);
}

@utility nums {
  font-variant-numeric: tabular-nums;
}

@media (prefers-reduced-transparency: reduce) {
  .glass-1,
  .glass-2 {
    background: var(--color-surface-solid);
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}

html,
body,
#root {
  height: 100%;
}

body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Helvetica Neue",
    Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  cursor: default;
  user-select: none;
  -webkit-user-select: none;
  color: var(--color-ink);
  background: var(--color-canvas);
}

input,
textarea {
  user-select: text;
  -webkit-user-select: text;
}
```

`src/components/Backdrop.tsx`:
```tsx
/**
 * Static aurora canvas behind the shell: three large pre-blurred radial
 * gradients on the canvas color. Deliberately NO backdrop-filter and NO
 * animation — the glass layers above pay the blur cost, not the backdrop.
 */
export function Backdrop() {
  return (
    <div aria-hidden="true" className="pointer-events-none fixed inset-0 z-0">
      <div
        className="absolute inset-0"
        style={{
          background: [
            `radial-gradient(42% 38% at 18% 12%, color-mix(in srgb, var(--color-aurora-sky) calc(var(--aurora-alpha) * 100%), transparent), transparent 70%)`,
            `radial-gradient(46% 42% at 85% 20%, color-mix(in srgb, var(--color-aurora-violet) calc(var(--aurora-alpha) * 100%), transparent), transparent 70%)`,
            `radial-gradient(50% 46% at 50% 95%, color-mix(in srgb, var(--color-aurora-mint) calc(var(--aurora-alpha) * 100%), transparent), transparent 70%)`,
          ].join(', '),
        }}
      />
    </div>
  )
}

export default Backdrop
```

- [ ] **Step 5: GREEN + full suite** — `npx vitest run && npx tsc --noEmit` → all pass (existing 155 + 2 new; the style.css rewrite keeps all pre-existing base rules, so nothing regresses).

- [ ] **Step 6: Commit** — `feat(ui): liquid-glass foundation — tokens, glass utilities, cn(), aurora backdrop`

---

### Task 2: ui primitives (static) — button, card, badge, separator, input

**Files:**
- Create: `apps/desktop/frontend/src/components/ui/button.tsx`, `ui/card.tsx`, `ui/badge.tsx`, `ui/separator.tsx`, `ui/input.tsx`
- Test: `apps/desktop/frontend/src/components/ui/ui-static.test.tsx`

**Interfaces:**
- Consumes: `cn`, token/utility classes from Task 1.
- Produces (exact APIs later tasks import):
  - `Button` — `{ variant?: 'primary'|'destructive'|'ghost'|'glass'; size?: 'sm'|'md'|'lg'; asChild?: boolean } & ButtonHTMLAttributes`
  - `Card` — `{ elevation?: 1|2 } & HTMLAttributes<HTMLDivElement>`
  - `Badge` — `{ variant?: 'safe'|'moderate'|'risky'|'neutral' } & HTMLAttributes<HTMLSpanElement>`
  - `Separator` — `{ orientation?: 'horizontal'|'vertical' }`
  - `Input` — `InputHTMLAttributes<HTMLInputElement>` forwardRef

- [ ] **Step 1: Write failing smoke tests** — `ui-static.test.tsx`:
```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { Button } from './button'
import { Card } from './card'
import { Badge } from './badge'
import { Separator } from './separator'
import { Input } from './input'

afterEach(() => cleanup())

describe('ui static primitives', () => {
  it('Button renders every variant, fires onClick, and disables', () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Go</Button>)
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(onClick).toHaveBeenCalledTimes(1)
    for (const variant of ['primary', 'destructive', 'ghost', 'glass'] as const) {
      const { unmount } = render(<Button variant={variant}>{variant}</Button>)
      expect(screen.getByRole('button', { name: variant })).toBeDefined()
      unmount()
    }
    render(<Button disabled>Off</Button>)
    expect((screen.getByRole('button', { name: 'Off' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('Card applies glass elevation classes', () => {
    const { container } = render(<Card elevation={2}>x</Card>)
    expect((container.firstElementChild as HTMLElement).className).toContain('glass-2')
    const { container: c1 } = render(<Card>y</Card>)
    expect((c1.firstElementChild as HTMLElement).className).toContain('glass-1')
  })

  it('Badge variants render their label', () => {
    for (const variant of ['safe', 'moderate', 'risky', 'neutral'] as const) {
      const { unmount } = render(<Badge variant={variant}>{variant}</Badge>)
      expect(screen.getByText(variant)).toBeDefined()
      unmount()
    }
  })

  it('Separator is decorative; Input forwards value + onChange', () => {
    const { container } = render(<Separator />)
    expect((container.firstElementChild as HTMLElement).getAttribute('role')).toBe('none')
    const onChange = vi.fn()
    render(<Input aria-label="days" value="7" onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('days'), { target: { value: '9' } })
    expect(onChange).toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: RED** — `npx vitest run src/components/ui/ui-static.test.tsx` → FAIL (modules missing).

- [ ] **Step 3: Implement**

`ui/button.tsx`:
```tsx
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
```

`ui/card.tsx`:
```tsx
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
```

`ui/badge.tsx`:
```tsx
import type { HTMLAttributes } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/cn'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
  {
    variants: {
      variant: {
        safe: 'bg-safe/15 text-safe',
        moderate: 'bg-moderate/15 text-moderate',
        risky: 'bg-risky/15 text-risky',
        neutral: 'bg-hairline text-ink-2',
      },
    },
    defaultVariants: { variant: 'neutral' },
  },
)

export interface BadgeProps
  extends HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}
```

`ui/separator.tsx`:
```tsx
import { cn } from '../../lib/cn'

export function Separator({
  orientation = 'horizontal',
  className,
}: {
  orientation?: 'horizontal' | 'vertical'
  className?: string
}) {
  return (
    <div
      role="none"
      className={cn('bg-hairline', orientation === 'horizontal' ? 'h-px w-full' : 'w-px self-stretch', className)}
    />
  )
}
```

`ui/input.tsx`:
```tsx
import { forwardRef, type InputHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        'h-9 rounded-control border border-hairline bg-surface-solid/60 px-3 text-sm text-ink placeholder:text-ink-2 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent',
        className,
      )}
      {...props}
    />
  ),
)
Input.displayName = 'Input'
```

- [ ] **Step 4: GREEN + suite** — `npx vitest run && npx tsc --noEmit`.
- [ ] **Step 5: Commit** — `feat(ui): shadcn static primitives — button, card, badge, separator, input`

---

### Task 3: ui primitives (interactive) — checkbox, switch, progress, dialog, tooltip

**Files:**
- Create: `ui/checkbox.tsx`, `ui/switch.tsx`, `ui/progress.tsx`, `ui/dialog.tsx`, `ui/tooltip.tsx` (all under `apps/desktop/frontend/src/components/ui/`)
- Test: `apps/desktop/frontend/src/components/ui/ui-interactive.test.tsx`

**Interfaces:**
- Consumes: Task 1 tokens, `cn`.
- Produces:
  - `Checkbox` — Radix; props `{ checked, onCheckedChange, 'aria-label', disabled }`; renders `role="checkbox"` with `aria-checked`.
  - `Switch` — Radix; `{ checked, onCheckedChange, 'aria-label' }`.
  - `Progress` — Radix; `{ value: number }` 0-100; `role="progressbar"`.
  - `Dialog`, `DialogContent`, `DialogTitle` — Radix; `DialogContent` is a glass-2 panel in a Portal over a `bg-black/40 backdrop-blur-sm` overlay; accepts `className`; always renders `DialogTitle` (visually hidden allowed via className) for a11y.
  - `Tooltip` — Radix wrapper `({ content, children })` with its own Provider inside.

- [ ] **Step 1: Write failing smoke tests** — `ui-interactive.test.tsx`:
```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { Checkbox } from './checkbox'
import { Switch } from './switch'
import { Progress } from './progress'
import { Dialog, DialogContent, DialogTitle } from './dialog'

afterEach(() => cleanup())

describe('ui interactive primitives', () => {
  it('Checkbox toggles through aria-checked and fires onCheckedChange', () => {
    const onChange = vi.fn()
    render(<Checkbox aria-label="pick" checked={false} onCheckedChange={onChange} />)
    const box = screen.getByRole('checkbox', { name: 'pick' })
    expect(box.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(box)
    expect(onChange).toHaveBeenCalledWith(true)
  })

  it('Switch reflects checked state', () => {
    const onChange = vi.fn()
    render(<Switch aria-label="toggle" checked onCheckedChange={onChange} />)
    expect(screen.getByRole('switch', { name: 'toggle' }).getAttribute('aria-checked')).toBe('true')
  })

  it('Progress exposes its value', () => {
    render(<Progress value={40} />)
    expect(screen.getByRole('progressbar')).toBeDefined()
  })

  it('Dialog renders content in a portal with a title', () => {
    render(
      <Dialog open>
        <DialogContent>
          <DialogTitle>Confirm things</DialogTitle>
          body
        </DialogContent>
      </Dialog>,
    )
    expect(screen.getByRole('dialog')).toBeDefined()
    expect(screen.getByText('Confirm things')).toBeDefined()
  })
})
```

- [ ] **Step 2: RED**, then **Step 3: Implement**:

`ui/checkbox.tsx`:
```tsx
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
      'flex h-4.5 w-4.5 shrink-0 items-center justify-center rounded-[6px] border border-hairline bg-surface-solid/60 transition-colors duration-150 data-[state=checked]:border-accent data-[state=checked]:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:opacity-40',
      className,
    )}
    {...props}
  >
    <CheckboxPrimitive.Indicator>
      <Check size={12} strokeWidth={3} className="text-white" />
    </CheckboxPrimitive.Indicator>
  </CheckboxPrimitive.Root>
))
Checkbox.displayName = 'Checkbox'
```

`ui/switch.tsx`:
```tsx
import * as SwitchPrimitive from '@radix-ui/react-switch'
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import { cn } from '../../lib/cn'

export const Switch = forwardRef<
  ElementRef<typeof SwitchPrimitive.Root>,
  ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitive.Root
    ref={ref}
    className={cn(
      'inline-flex h-6 w-10 items-center rounded-full border border-hairline bg-hairline transition-colors duration-150 data-[state=checked]:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
      className,
    )}
    {...props}
  >
    <SwitchPrimitive.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white shadow-sm transition-transform duration-150 data-[state=checked]:translate-x-[18px]" />
  </SwitchPrimitive.Root>
))
Switch.displayName = 'Switch'
```

`ui/progress.tsx`:
```tsx
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
```

`ui/dialog.tsx`:
```tsx
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import { cn } from '../../lib/cn'

export const Dialog = DialogPrimitive.Root
export const DialogTitle = DialogPrimitive.Title

export const DialogContent = forwardRef<
  ElementRef<typeof DialogPrimitive.Content>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Content>
>(({ className, children, ...props }, ref) => (
  <DialogPrimitive.Portal>
    <DialogPrimitive.Overlay className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm" />
    <DialogPrimitive.Content
      ref={ref}
      aria-describedby={undefined}
      className={cn(
        'glass-2 fixed left-1/2 top-1/2 z-50 max-h-[85vh] w-[520px] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-card p-6',
        className,
      )}
      {...props}
    >
      {children}
    </DialogPrimitive.Content>
  </DialogPrimitive.Portal>
))
DialogContent.displayName = 'DialogContent'
```

`ui/tooltip.tsx`:
```tsx
import * as TooltipPrimitive from '@radix-ui/react-tooltip'
import type { ReactNode } from 'react'

export function Tooltip({ content, children }: { content: ReactNode; children: ReactNode }) {
  return (
    <TooltipPrimitive.Provider delayDuration={300}>
      <TooltipPrimitive.Root>
        <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content className="glass-2 z-50 rounded-control px-2.5 py-1 text-xs text-ink" sideOffset={6}>
            {content}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  )
}
```

- [ ] **Step 4: GREEN + suite**, **Step 5: Commit** — `feat(ui): shadcn interactive primitives — checkbox, switch, progress, dialog, tooltip`

---

### Task 4: Shell — App, Sidebar, drag region, ActionBar

**Files:**
- Modify: `apps/desktop/frontend/src/App.tsx`, `src/components/Sidebar.tsx`
- Create: `src/components/ActionBar.tsx`
- Test: extend `src/App.test.tsx` and `src/components/` tests only per the allowed-edit rule; add `ActionBar` assertions to `ui-static.test.tsx` is NOT allowed (separate concern) — create `src/components/ActionBar.test.tsx`

**Interfaces:**
- Consumes: `Backdrop`, `Card`, `cn`, tokens.
- Produces: `ActionBar` — `{ children: ReactNode; className?: string }`, a floating glass-2 bar (`glass-2 rounded-card`) pinned inside the content column bottom (`sticky bottom-4`), used by SmartScan (Task 5) and Uninstaller (Task 6).

- [ ] **Step 1: Failing test** — `ActionBar.test.tsx`:
```tsx
// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { ActionBar } from './ActionBar'

afterEach(() => cleanup())

describe('<ActionBar />', () => {
  it('renders children inside a floating glass bar', () => {
    render(<ActionBar>controls</ActionBar>)
    const bar = screen.getByText('controls')
    expect(bar.closest('div')?.className).toContain('glass-2')
  })
})
```

- [ ] **Step 2: RED**, **Step 3: Implement**

`ActionBar.tsx`:
```tsx
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
```

Shell transformation contract (`App.tsx`, `Sidebar.tsx`) — exact requirements, structure preserved:
- `App.tsx`: render `<Backdrop />` as first child; root keeps `flex h-full` but drops `bg-white dark:bg-neutral-900` for `text-ink` (canvas comes from body); `<main>` unchanged semantically (`min-w-0 flex-1 overflow-y-auto relative z-10`); FirstRun branch unchanged. Add a 28px top drag strip: `<div className="fixed inset-x-0 top-0 h-7 z-20" style={{ '--wails-draggable': 'drag' } as CSSProperties} />` — Wails reads the `--wails-draggable: drag` style for the hidden-inset titlebar; the strip is `pointer-events-auto` and must NOT overlap interactive controls (sidebar/header content start below `pt-8`).
- `Sidebar.tsx`: becomes a floating glass rail — wrapper `m-2.5 mr-0 z-10` + `glass-2 rounded-shell` column, `pt-8` (below drag strip); each nav item becomes a pill button: base `rounded-control px-3 py-1.5 text-sm text-ink-2 hover:bg-hairline`, selected `bg-accent/15 text-accent font-medium`; keep every label, icon, click handler, and `aria-current`/selected semantics exactly as they are today.
- App/Sidebar tests: adapt ONLY structural queries (class-based) if any; behavior queries unchanged.

- [ ] **Step 4: GREEN + full suite + tsc**, **Step 5: Commit** — `feat(ui): glass shell — aurora, floating sidebar rail, drag strip, ActionBar`

---

### Task 5: SmartScan + the glass-lens hero

**Files:**
- Create: `src/components/ScanLens.tsx`, `src/components/ScanLens.test.tsx`
- Modify: `src/views/SmartScan.tsx`, `src/components/CategoryCard.tsx`, `src/components/SizeBar.tsx` (thin wrapper over `Progress`)
- Test: `src/views/SmartScan.test.tsx` (allowed-edit rule)

**Interfaces:**
- Consumes: Tasks 1-4 (`Card`, `Button`, `Badge`, `Progress`, `ActionBar`, tokens).
- Produces: `ScanLens` — `{ state: 'idle' | 'scanning'; completed?: number; total?: number; totalSize?: number; onScan: () => void }`. Idle: circular layered-glass disc (concentric rings via nested rounded-full glass divs, slow 6s specular sweep — a rotating conic-gradient highlight layer, `animation` disabled under reduced motion), acts as the scan button (`<button aria-label` KEPT IDENTICAL to today's "Smart Scan" button text semantics: the button's accessible name must remain exactly `Smart Scan`). Scanning: same disc, no button semantics, renders `completed/total` and `formatSize(totalSize)` in `nums` classes.

`ScanLens.tsx` (complete):
```tsx
import { Search } from 'lucide-react'
import { formatSize } from '../lib/format'
import { cn } from '../lib/cn'

interface ScanLensProps {
  state: 'idle' | 'scanning'
  completed?: number
  total?: number
  totalSize?: number
  onScan: () => void
}

/** The signature element: a circular layered-glass lens. Idle = the Smart
 * Scan button; scanning = the live meter. One bold moment — everything
 * around it stays quiet. */
export function ScanLens({ state, completed = 0, total = 0, totalSize = 0, onScan }: ScanLensProps) {
  const ring = (
    <>
      <div aria-hidden className="absolute inset-0 rounded-full glass-1" />
      <div aria-hidden className="absolute inset-3 rounded-full glass-2" />
      <div
        aria-hidden
        className="lens-sweep absolute inset-0 rounded-full"
        style={{
          background:
            'conic-gradient(from 0deg, transparent 0deg, rgb(255 255 255 / 0.35) 24deg, transparent 60deg)',
          maskImage: 'radial-gradient(closest-side, transparent 78%, black 80%)',
          WebkitMaskImage: 'radial-gradient(closest-side, transparent 78%, black 80%)',
        }}
      />
    </>
  )
  if (state === 'scanning') {
    return (
      <div className="relative flex h-56 w-56 items-center justify-center" role="status">
        {ring}
        <div className="relative z-10 flex flex-col items-center gap-1">
          <span className="nums text-2xl font-semibold text-ink">{completed}/{total}</span>
          <span className="nums text-sm text-ink-2">{formatSize(totalSize)}</span>
        </div>
      </div>
    )
  }
  return (
    <button
      type="button"
      onClick={onScan}
      className={cn(
        'group relative flex h-56 w-56 items-center justify-center rounded-full',
        'transition-transform duration-150 hover:scale-[1.02] active:scale-[0.99]',
        'focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent',
      )}
    >
      {ring}
      <span className="relative z-10 flex flex-col items-center gap-2 text-ink">
        <Search size={28} className="text-accent" />
        <span className="text-lg font-semibold">Smart Scan</span>
      </span>
    </button>
  )
}
```

Add to `style.css` (same task):
```css
@keyframes lens-sweep {
  to { transform: rotate(360deg); }
}
.lens-sweep {
  animation: lens-sweep 6s linear infinite;
}
```

`ScanLens.test.tsx`:
```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { ScanLens } from './ScanLens'

afterEach(() => cleanup())

describe('<ScanLens />', () => {
  it('idle: is a button named Smart Scan that fires onScan', () => {
    const onScan = vi.fn()
    render(<ScanLens state="idle" onScan={onScan} />)
    fireEvent.click(screen.getByRole('button', { name: /smart scan/i }))
    expect(onScan).toHaveBeenCalledTimes(1)
  })
  it('scanning: shows live progress and size, no button', () => {
    render(<ScanLens state="scanning" completed={3} total={16} totalSize={1073741824} onScan={() => {}} />)
    expect(screen.getByText('3/16')).toBeDefined()
    expect(screen.getByText('1.0 GB')).toBeDefined()
    expect(screen.queryByRole('button')).toBeNull()
  })
})
```

SmartScan migration contract: hero state renders `<ScanLens state="idle" onScan={() => void useScanStore.getState().startScan()} />` beneath the existing h1/description (copy unchanged); scanning state keeps the per-category live list (rows become subtle `glass-1 rounded-control` rows) with ScanLens above it showing progress; results state: group sections → `Card` per group with the existing eyebrow headings, `CategoryCard` reskinned (glass row, `Badge` for safety, `Progress`-based `SizeBar`); footer → `ActionBar` (same children/copy/disabled logic); scan-complete materialize animation: wrap the results container in a div with class `animate-[materialize_220ms_var(--ease-glass)]` + add to style.css:
```css
@keyframes materialize {
  from { opacity: 0; transform: scale(0.98); filter: blur(4px); }
  to { opacity: 1; transform: scale(1); filter: blur(0); }
}
```
All copy (incl. "Found … that can be cleaned", cancel button, error banner + dismiss aria) byte-identical. Existing SmartScan tests: the hero button is still `role=button name=/smart scan/i` — no edits expected; permitted edits only for class-based queries.

- [ ] Steps: failing ScanLens tests → RED → implement → migrate SmartScan/CategoryCard/SizeBar → GREEN full suite + tsc → Commit `feat(ui): glass-lens scan hero + SmartScan on the glass system`

---

### Task 6: Uninstaller + CategoryDetail migration

**Files:** Modify `src/views/Uninstaller.tsx`, `src/views/CategoryDetail.tsx`, `src/components/ItemList.tsx`, their tests (allowed-edit rule only).

**Contract:**
- Replace every raw `<input type="checkbox">` with `Checkbox` keeping the exact same `aria-label` strings; permitted test edit: `(el as HTMLInputElement).checked` → `el.getAttribute('aria-checked') === 'true'` and `fireEvent.click` stays.
- Rows: `glass-1 rounded-control` hover states; related-paths sublist stays mono/`contractHome`; Running badge → `Badge variant="risky"` with SAME text `Running`; "+N related" chip → `Badge variant="neutral"` same text.
- Footer → `ActionBar` with the same `Uninstall N app(s)` button (now `Button variant="destructive"`), same disabled logic; keep the `title` attribute `Quit the app first` AND wrap in `Tooltip content="Quit the app first"` only when blocked.
- Header Re-check → `Button variant="glass" size="sm"`; "Refreshing…"/"Updated …" hints and all list/waiting/done-dialog copy unchanged (done dialog itself migrates in Task 8).
- CategoryDetail: header/back control reskinned with `Button variant="ghost"`, rows and directory grouping onto glass rows, selection checkboxes → `Checkbox` (same aria-labels), totals in `nums`.
- [ ] Steps: run view suites RED-free BEFORE (they pass), migrate, adapt only permitted queries, GREEN full suite + tsc, Commit `feat(ui): Uninstaller + CategoryDetail on the glass system`

---

### Task 7: Backups, Maintenance, Settings, FirstRun migration

**Files:** Modify `src/views/Backups.tsx`, `src/views/Maintenance.tsx`, `src/views/Settings.tsx`, `src/views/FirstRun.tsx`, tests per allowed-edit rule.

**Contract:**
- Backups: session rows → glass rows in a `Card`; chevron toggle keeps its exact aria-label; inline confirm strips stay INLINE (same copy/buttons — now `Button size="sm"` glass/destructive); details sublist unchanged semantically.
- Maintenance: task cards → `Card` grid; run buttons → `Button`; progress rendering onto `Progress`; all status copy identical.
- Settings: booleans (`backupByDefault`, `showRisky`) → `Switch` keeping label association (each Switch gets `aria-label` equal to the visible label text if the current markup relied on label proximity — permitted structural edit, called out); numeric/text fields → `Input` (same labels); Save button → `Button variant="primary"`; clamp/round-trip behavior untouched (`lib/settings.ts` not modified).
- FirstRun: glass Card layout, same copy and both buttons (`Button` primary/ghost).
- [ ] Steps: migrate → GREEN full suite + tsc → Commit `feat(ui): Backups, Maintenance, Settings, FirstRun on the glass system`

---

### Task 8: Overlays — dialogs and progress

**Files:** Modify `src/components/ConfirmModal.tsx`, `src/components/UninstallConfirm.tsx`, `src/components/ResultsPanel.tsx`, `src/components/ProgressOverlay.tsx`, `src/views/Uninstaller.tsx` (done dialog), tests per allowed-edit rule.

**Contract:**
- Each hand-rolled `fixed inset-0 … bg-black/40` overlay + panel becomes `Dialog open` + `DialogContent` (glass-2). `open` is driven by the SAME store/phase conditions used today; closing via Radix (Esc/overlay) must NOT introduce new behavior — pass `onPointerDownOutside={(e) => e.preventDefault()}` and `onEscapeKeyDown={(e) => e.preventDefault()}` wherever today's overlays cannot be dismissed that way (all of them — they close only via their buttons).
- `DialogTitle` wraps each panel's existing heading text (ResultsPanel: the freed-space headline; UninstallConfirm: `Uninstall applications`; ConfirmModal: its existing heading; done dialog: its headline). No copy changes.
- `ProgressOverlay` keeps its exact props and copy, renders inside `DialogContent` with `Progress value={pct}` replacing the hand-rolled bar (`{current} / {total}` line stays, `nums`).
- Permitted test edits: none expected — Radix portals render into document.body and Testing Library `screen` queries already search there; if a container-scoped query breaks, adapting it to `screen` is the allowed structural edit.
- [ ] Steps: migrate one component at a time running its test file each time → GREEN full suite + tsc → Commit `feat(ui): overlays on Dialog/Progress — confirm, results, uninstall, progress`

---

### Task 9: Polish + gate

**Files:** `src/style.css` (final tidy), any file with leftover `zinc-`/`neutral-` classes; `docs/superpowers/coverage-2026-07-18.md` NOT touched (that report is historical).

- [ ] **Step 1:** Sweep for leftovers: `grep -rn 'zinc-\|neutral-\|#[0-9a-fA-F]\{3,6\}' apps/desktop/frontend/src --include='*.tsx' | grep -v test` must return ONLY hits inside `ui/` token definitions (expected: none) — fix any stragglers to tokens.
- [ ] **Step 2:** Focus audit: every interactive element shows the accent `focus-visible` ring (primitives provide it; verify view-level custom buttons were migrated to `Button`).
- [ ] **Step 3:** Reduced-motion/transparency verification: assert the two media blocks exist in built CSS (`npx vite build` then grep `dist/assets/*.css` for `prefers-reduced-transparency` and `prefers-reduced-motion`).
- [ ] **Step 4:** Coverage gate: `npx vitest run --coverage` → lines ≥ 90% on src/. If below, add render-branch tests to the thinnest new files (ui primitives variants, ScanLens states, ActionBar).
- [ ] **Step 5:** Full gate: `npx vitest run && npx tsc --noEmit` and `cd apps/desktop && go build ./...` (embedding still compiles) all green.
- [ ] **Step 6:** Manual visual smoke (human or `task dev:desktop` screenshot): hero lens idle/scanning, results glass, each view light+dark, one dialog, reduced-transparency via System Settings if available. Record observations in the task report.
- [ ] **Step 7:** Commit — `feat(ui): liquid-glass polish pass — token sweep, a11y, motion fallbacks, coverage gate`
