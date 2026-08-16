import { useState } from 'react'
import type { CSSProperties } from 'react'
import { ArrowLeft, FileText, Folder, Telescope } from 'lucide-react'
import { BuildSpaceLens } from '../../wailsjs/go/main/App'
import { ModuleHero } from '../components/ModuleHero'
import { ScanLens } from '../components/ScanLens'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Progress } from '../components/ui/progress'
import { MODULES } from '../lib/modules'
import { formatSize } from '../lib/format'
import { contractHome, homeDir } from '../lib/paths'
import type { SpaceLensNode } from '../lib/types'

/** Find the node at path[] inside root (drill-down navigation). Exported for tests. */
export function nodeAt(root: SpaceLensNode, trail: string[]): SpaceLensNode {
  let cur = root
  for (const p of trail) {
    const next = (cur.children ?? []).find((c) => c.path === p)
    if (!next) return cur
    cur = next
  }
  return cur
}

type Status = 'idle' | 'building' | 'done' | 'error'

/** Space Lens: build a size map of the home folder and drill into it. */
export default function SpaceLens() {
  const [status, setStatus] = useState<Status>('idle')
  const [root, setRoot] = useState<SpaceLensNode | null>(null)
  const [trail, setTrail] = useState<string[]>([])
  const [error, setError] = useState('')
  const mod = MODULES.find((m) => m.view === 'space-lens')!

  const build = async () => {
    setStatus('building')
    setError('')
    try {
      const n = (await BuildSpaceLens('')) as SpaceLensNode | null
      if (!n) throw new Error('empty result')
      setRoot(n)
      setTrail([])
      setStatus('done')
    } catch (e) {
      setError(String(e))
      setStatus('error')
    }
  }

  if (status === 'idle' || status === 'building' || status === 'error') {
    return (
      <ModuleHero
        module={mod}
        cta={
          status === 'building' ? (
            <div className="flex flex-col items-center gap-3" role="status">
              <ScanLens state="scanning" hue={mod.hue} completed={0} total={0} totalSize={0} onScan={() => {}} />
              <span className="text-sm text-ink-2">Measuring your folders…</span>
            </div>
          ) : (
            <ScanLens
              state="idle"
              hue={mod.hue}
              label="Scan"
              icon={<Telescope size={28} style={{ color: 'var(--module)' }} />}
              onScan={() => void build()}
            />
          )
        }
      >
        {status === 'error' ? <p className="mt-4 text-sm text-danger">{error}</p> : null}
      </ModuleHero>
    )
  }

  // ---- done ----
  const current = root ? nodeAt(root, trail) : null
  const children = current?.children ?? []
  const maxSize = Math.max(1, ...children.map((c) => c.size))

  return (
    <div className="flex h-full flex-col p-6" style={{ '--module': mod.hue } as CSSProperties}>
      <div className="flex items-center gap-3">
        {trail.length > 0 ? (
          <Button
            type="button"
            aria-label="Back"
            variant="ghost"
            size="sm"
            onClick={() => setTrail(trail.slice(0, -1))}
            className="px-1.5"
          >
            <ArrowLeft size={18} />
          </Button>
        ) : null}
        <h1 className="min-w-0 flex-1 truncate text-lg font-semibold text-ink">
          {current ? contractHome(current.path, homeDir()) : ''}
        </h1>
        <span className="nums shrink-0 text-sm text-ink-2">{formatSize(current?.size ?? 0)}</span>
        <Button type="button" variant="glass" size="sm" onClick={() => void build()}>
          Rescan
        </Button>
      </div>

      <Card className="mt-4 flex-1 overflow-y-auto p-3">
        {children.length === 0 ? (
          <p className="p-3 text-sm text-ink-2">Empty folder — nothing to show here.</p>
        ) : (
          <ul className="space-y-1">
            {children.map((c) => {
              const row = (
                <>
                  {c.isDir ? (
                    <Folder size={16} className="shrink-0" style={{ color: 'var(--module)' }} />
                  ) : (
                    <FileText size={16} className="shrink-0 text-ink-2" />
                  )}
                  <span className="w-56 shrink-0 truncate text-left text-sm font-medium text-ink">{c.name}</span>
                  <div className="min-w-0 flex-1">
                    <Progress value={(c.size / maxSize) * 100} className="h-1.5" />
                  </div>
                  <span className="nums w-20 shrink-0 text-right text-sm text-ink-2">{formatSize(c.size)}</span>
                </>
              )
              return (
                <li key={c.path}>
                  {c.isDir ? (
                    <button
                      type="button"
                      onClick={() => setTrail([...trail, c.path])}
                      className="glass-1 flex w-full items-center gap-3 rounded-control px-3 py-2 transition hover:brightness-105 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                    >
                      {row}
                    </button>
                  ) : (
                    <div className="glass-1 flex items-center gap-3 rounded-control px-3 py-2">{row}</div>
                  )}
                </li>
              )
            })}
            {current?.truncated ? (
              <li className="px-3 py-1 text-xs text-ink-2">…and {current.truncated} smaller items</li>
            ) : null}
          </ul>
        )}
      </Card>
    </div>
  )
}
