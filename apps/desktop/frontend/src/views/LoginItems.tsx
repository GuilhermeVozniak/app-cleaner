import { useEffect, useState } from 'react'
import { AlertTriangle, ArrowLeft, RefreshCw } from 'lucide-react'
import { ListLoginItems, RevealInFinder } from '../../wailsjs/go/main/App'
import EmptyState from '../components/EmptyState'
import { PageHeader } from '../components/PageHeader'
import { StartOver } from '../components/StartOver'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { useUiStore } from '../stores/uiStore'
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
    <div className="flex h-full flex-col">
      <StartOver label="Performance" icon={ArrowLeft} onClick={() => useUiStore.getState().setView('maintenance')} />
      <div className="flex-1 overflow-y-auto px-10 pb-10 pt-6">
        <div className="materialize mx-auto max-w-4xl">
          <PageHeader
            title="Login Items"
            subtitle="Everything registered to launch automatically. Items flagged broken point to a program that no longer exists; the Cleanup scan can remove orphaned agents safely."
            aside={
              <Button type="button" variant="secondary" disabled={loading} onClick={() => void refresh()}>
                <RefreshCw size={14} strokeWidth={2.25} />
                Refresh
              </Button>
            }
          />

          {loading && items === null ? (
            <p className="mt-8 text-body text-ink-2">Reading launch agents…</p>
          ) : (items ?? []).length === 0 ? (
            <div className="mt-16">
              <EmptyState title="No login items" subtitle="Nothing is set to launch automatically." />
            </div>
          ) : (
            <div className="mt-8 space-y-8">
              {groups.map((g) => (
                <section key={g.kind}>
                  <h2 className="mb-3 text-card font-semibold text-ink">{KIND_LABELS[g.kind]}</h2>
                  <ul className="glass-1 divide-y divide-hairline rounded-card px-4">
                    {g.items.map((it) => (
                      <li key={it.path} className="flex items-center gap-4 py-3">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="truncate text-body font-semibold text-ink">{it.label}</span>
                            {it.programMissing ? (
                              <Badge variant="risky">
                                <AlertTriangle size={11} /> broken
                              </Badge>
                            ) : null}
                            {it.runAtLoad ? <Badge variant="neutral">runs at load</Badge> : null}
                          </div>
                          {it.program ? (
                            <div className="mt-0.5 truncate font-mono text-caption text-ink-2">{it.program}</div>
                          ) : null}
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => RevealInFinder(it.path)}
                          className="shrink-0"
                        >
                          Reveal
                        </Button>
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
