import { useEffect, useState } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { ListLoginItems, RevealInFinder } from '../../wailsjs/go/main/App'
import EmptyState from '../components/EmptyState'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import type { LoginItem } from '../lib/types'

const KIND_LABELS: Record<LoginItem['kind'], string> = {
  'user-agent': 'Your agents',
  'global-agent': 'All-user agents',
  daemon: 'System daemons',
}

/** Group login items by kind preserving list order. Exported for tests. */
export function groupLoginItems(items: LoginItem[]): Array<{ kind: LoginItem['kind']; items: LoginItem[] }> {
  const out: Array<{ kind: LoginItem['kind']; items: LoginItem[] }> = []
  const idx = new Map<string, number>()
  for (const it of items) {
    let i = idx.get(it.kind)
    if (i === undefined) {
      i = out.length
      idx.set(it.kind, i)
      out.push({ kind: it.kind, items: [] })
    }
    out[i].items.push(it)
  }
  return out
}

/**
 * Login Items: read-only listing of launchd agents/daemons. Removal of
 * orphaned agents stays in the Cleanup scan (launch-agents category) — this
 * screen is for visibility, with a Reveal shortcut.
 */
export default function LoginItems() {
  const [items, setItems] = useState<LoginItem[] | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    setLoading(true)
    try {
      setItems(((await ListLoginItems()) as LoginItem[] | null) ?? [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [])

  const groups = groupLoginItems(items ?? [])

  return (
    <div className="flex h-full flex-col p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-ink">Login Items</h1>
        <Button type="button" variant="glass" size="sm" disabled={loading} onClick={() => void refresh()}>
          <RefreshCw size={14} />
          Refresh
        </Button>
      </div>
      <p className="mt-1 text-sm text-ink-2">
        Everything registered to launch automatically. Items flagged “broken” point to a program that no
        longer exists — the Cleanup scan can remove orphaned agents safely.
      </p>

      {loading && items === null ? (
        <p className="mt-4 text-sm text-ink-2">Reading launch agents…</p>
      ) : (items ?? []).length === 0 ? (
        <EmptyState title="No login items" subtitle="Nothing is set to launch automatically." />
      ) : (
        <div className="mt-4 flex-1 space-y-5 overflow-y-auto">
          {groups.map((g) => (
            <section key={g.kind}>
              <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-ink-2">
                {KIND_LABELS[g.kind]}
              </h2>
              <Card className="space-y-1 p-3">
                {g.items.map((it) => (
                  <div key={it.path} className="glass-1 flex items-center gap-3 rounded-control px-3 py-2">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-sm font-medium text-ink">{it.label}</span>
                        {it.programMissing ? (
                          <Badge variant="risky" className="gap-1">
                            <AlertTriangle size={11} /> broken
                          </Badge>
                        ) : null}
                        {it.runAtLoad ? <Badge variant="neutral">runs at load</Badge> : null}
                      </div>
                      {it.program ? (
                        <div className="truncate font-mono text-xs text-ink-2">{it.program}</div>
                      ) : null}
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => RevealInFinder(it.path)}
                      className="shrink-0 text-xs text-ink-2"
                    >
                      Reveal
                    </Button>
                  </div>
                ))}
              </Card>
            </section>
          ))}
        </div>
      )}
    </div>
  )
}
