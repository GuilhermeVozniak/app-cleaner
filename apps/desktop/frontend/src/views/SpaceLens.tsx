import { useState } from 'react'
import type { CSSProperties } from 'react'
import { ArrowLeft, FileText, Folder } from 'lucide-react'
import { BuildSpaceLens } from '../../wailsjs/go/main/App'
import { ModuleHero } from '../components/ModuleHero'
import { ModuleIcon } from '../components/ModuleIcon'
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
            <ScanLens
              state="scanning"
              hue={mod.hue}
              caption="Measuring your folders…"
              onScan={() => {}}
            />
          ) : (
            <ScanLens state="idle" hue={mod.hue} label="Scan" onScan={() => void build()} />
          )
        }
      >
        {status === 'error' ? <p className="mt-6 text-body text-danger">{error}</p> : null}
      </ModuleHero>
    )
  }

  // ---- done ----
  const current = root ? nodeAt(root, trail) : null
  const children = current?.children ?? []
  const maxSize = Math.max(1, ...children.map((c) => c.size))

  return (
    <div
      className="materialize mx-auto flex h-full max-w-5xl flex-col px-10 pb-10 pt-6"
      style={{ '--module': mod.hue } as CSSProperties}
    >
      <div className="flex items-center gap-3">
        {trail.length > 0 ? (
          <Button
            type="button"
            aria-label="Back"
            variant="ghost"
            size="sm"
            onClick={() => setTrail(trail.slice(0, -1))}
            className="h-9 w-9 px-0"
          >
            <ArrowLeft size={18} />
          </Button>
        ) : null}
        <h1 className="min-w-0 flex-1 truncate text-title font-semibold text-ink">
          {current ? contractHome(current.path, homeDir()) : ''}
        </h1>
        <span className="nums shrink-0 text-card text-ink-2">{formatSize(current?.size ?? 0)}</span>
        <Button type="button" variant="secondary" size="sm" onClick={() => void build()}>
          Rescan
        </Button>
      </div>

      <Card className="mt-5 flex-1 overflow-y-auto">
        {children.length === 0 ? (
          <p className="px-5 py-4 text-body text-ink-2">Empty folder. Nothing to show here.</p>
        ) : (
          <ul className="divide-y divide-hairline">
            {children.map((c) => {
              const row = (
                <>
                  {c.isDir ? (
                    <ModuleIcon Icon={Folder} size="xs" />
                  ) : (
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center text-ink-2">
                      <FileText size={16} />
                    </span>
                  )}
                  <span className="w-64 shrink-0 truncate text-left text-body font-semibold text-ink">
                    {c.name}
                  </span>
                  <div className="min-w-0 flex-1">
                    <Progress value={(c.size / maxSize) * 100} className="h-1.5 text-[var(--module)]" />
                  </div>
                  <span className="nums w-24 shrink-0 text-right text-body text-ink-2">
                    {formatSize(c.size)}
                  </span>
                </>
              )
              return (
                <li key={c.path}>
                  {c.isDir ? (
                    <button
                      type="button"
                      onClick={() => setTrail([...trail, c.path])}
                      className="focus-ring flex h-12 w-full items-center gap-4 px-4 text-left transition-colors hover:bg-glass-1"
                    >
                      {row}
                    </button>
                  ) : (
                    <div className="flex h-12 items-center gap-4 px-4">{row}</div>
                  )}
                </li>
              )
            })}
            {current?.truncated ? (
              <li className="px-4 py-2.5 text-caption text-ink-2">…and {current.truncated} smaller items</li>
            ) : null}
          </ul>
        )}
      </Card>
    </div>
  )
}
