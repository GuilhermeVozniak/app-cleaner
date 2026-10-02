import type { LucideIcon } from 'lucide-react'
import type { CSSProperties } from 'react'
import { cn } from '../lib/cn'

type Size = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'hero'

const SIZES: Record<Size, { box: string; glyph: number; stroke: number }> = {
  xs: { box: 'h-6 w-6 rounded-[7px]', glyph: 14, stroke: 2.25 },
  sm: { box: 'h-8 w-8 rounded-[9px]', glyph: 17, stroke: 2.1 },
  md: { box: 'h-10 w-10 rounded-[12px]', glyph: 22, stroke: 2 },
  lg: { box: 'h-14 w-14 rounded-[16px]', glyph: 30, stroke: 1.9 },
  xl: { box: 'h-[120px] w-[120px] rounded-[32px] gem-hero', glyph: 60, stroke: 1.5 },
  hero: { box: 'h-[176px] w-[176px] rounded-[46px] gem-hero', glyph: 88, stroke: 1.4 },
}

interface Props {
  Icon: LucideIcon
  /** Overrides the inherited --module hue. */
  hue?: string
  size?: Size
  className?: string
}

/** The module's glyph on a glossy rounded square of its own colour ("gem"). */
export function ModuleIcon({ Icon, hue, size = 'md', className }: Props) {
  const s = SIZES[size]
  return (
    <span
      aria-hidden
      style={hue ? ({ '--module': hue } as CSSProperties) : undefined}
      className={cn('gem relative inline-flex shrink-0 items-center justify-center text-white', s.box, className)}
    >
      <Icon
        size={s.glyph}
        strokeWidth={s.stroke}
        className={
          size === 'hero' || size === 'xl'
            ? 'drop-shadow-[0_10px_24px_rgb(0_0_0/0.35)]'
            : 'drop-shadow-[0_2px_6px_rgb(0_0_0/0.30)]'
        }
      />
    </span>
  )
}

export default ModuleIcon
