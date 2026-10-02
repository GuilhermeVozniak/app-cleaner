import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Check, Loader2 } from 'lucide-react'
import { cn } from '../lib/cn'
import { ModuleIcon } from './ModuleIcon'

/**
 * Stage: the large glass panel a flow runs inside — "Removing leftovers…",
 * "Cleanup complete!". Big gem on the left, title and rows on the right,
 * actions bottom-right. Used inline (full-screen states) and inside dialogs.
 */
export function Stage({
  Icon,
  title,
  subtitle,
  children,
  footer,
  footerStart,
  className,
}: {
  Icon: LucideIcon
  title: ReactNode
  subtitle?: ReactNode
  children?: ReactNode
  /** Bottom-right actions. */
  footer?: ReactNode
  /** Bottom-left slot (e.g. View Log). */
  footerStart?: ReactNode
  className?: string
}) {
  return (
    <div className={cn('materialize flex min-h-[420px] flex-col px-10 pb-7 pt-12', className)}>
      <div className="flex flex-1 items-center gap-14">
        <div className="flex shrink-0 justify-center pl-4">
          <ModuleIcon Icon={Icon} size="hero" />
        </div>
        <div className="min-w-0 flex-1">
          {/* A div, so callers can pass their own heading (e.g. DialogTitle). */}
          <div className="text-headline font-semibold text-ink">{title}</div>
          {subtitle ? <p className="mt-1.5 text-card text-ink-2">{subtitle}</p> : null}
          {children ? <div className="mt-6">{children}</div> : null}
        </div>
      </div>
      {footer || footerStart ? (
        <div className="mt-8 flex items-center justify-between gap-4">
          <div>{footerStart}</div>
          <div className="flex items-center gap-3">{footer}</div>
        </div>
      ) : null}
    </div>
  )
}

export type StageRowState = 'pending' | 'running' | 'done' | 'failed'

/** One task line inside a Stage: icon, label, trailing value and a state mark. */
export function StageRow({
  Icon,
  label,
  value,
  state,
  hue,
  detail,
}: {
  Icon: LucideIcon
  label: ReactNode
  value?: ReactNode
  state: StageRowState
  hue?: string
  detail?: ReactNode
}) {
  return (
    <li className="flex items-center gap-4 py-2.5">
      <ModuleIcon Icon={Icon} size="sm" hue={hue} />
      <div className="min-w-0 flex-1">
        <div className="truncate text-card font-semibold text-ink">{label}</div>
        {detail ? <div className="truncate text-caption text-ink-2">{detail}</div> : null}
      </div>
      {value ? <span className="nums shrink-0 text-body text-ink-2">{value}</span> : null}
      <StateMark state={state} />
    </li>
  )
}

export function StateMark({ state }: { state: StageRowState }) {
  if (state === 'done')
    return <Check size={18} strokeWidth={2.5} className="shrink-0 text-ink" aria-label="Done" />
  if (state === 'failed')
    return (
      <span aria-label="Failed" className="shrink-0 text-card font-semibold text-danger">
        ✗
      </span>
    )
  if (state === 'running')
    return <Loader2 size={18} className="shrink-0 animate-spin text-ink" aria-label="Running" />
  return <span aria-hidden className="h-2.5 w-2.5 shrink-0 rounded-full bg-[rgb(255_255_255/0.28)]" />
}

export default Stage
