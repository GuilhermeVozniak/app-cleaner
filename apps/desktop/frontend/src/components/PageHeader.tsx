import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props {
  title: string
  subtitle?: ReactNode
  /** Right-aligned slot: search field, refresh button, etc. */
  aside?: ReactNode
  className?: string
}

/** Left-aligned page title for list and grid screens (My Tools, Backups, Settings). */
export function PageHeader({ title, subtitle, aside, className }: Props) {
  return (
    <header className={cn('flex items-start justify-between gap-8', className)}>
      <div className="min-w-0">
        <h1 className="text-display font-normal text-ink">{title}</h1>
        {subtitle ? <p className="mt-2 max-w-xl text-card text-ink-2">{subtitle}</p> : null}
      </div>
      {aside ? <div className="shrink-0 pt-3">{aside}</div> : null}
    </header>
  )
}

export default PageHeader
