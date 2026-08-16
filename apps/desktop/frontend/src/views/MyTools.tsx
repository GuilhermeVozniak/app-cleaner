import { useState } from 'react'
import type { CSSProperties } from 'react'
import { Search } from 'lucide-react'
import { filterTools, TOOLS } from '../lib/modules'
import type { ToolDef } from '../lib/modules'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'

/** Runs a tool: single-category scan tools land on the Cleanup screen. */
export function runTool(tool: ToolDef): void {
  if (tool.action.kind === 'view') {
    useUiStore.getState().setView(tool.action.view)
    return
  }
  const { categoryId } = tool.action
  useUiStore.getState().setView('smart-scan')
  if (useScanStore.getState().status !== 'scanning') {
    void useScanStore.getState().startScan([categoryId])
  }
}

/** My Tools: searchable grid of every tool, CleanMyMac-style. */
export default function MyTools() {
  const [query, setQuery] = useState('')
  const visible = filterTools(TOOLS, query)

  return (
    <div className="flex h-full flex-col items-center overflow-y-auto px-8 pb-10 pt-12">
      <h1 className="text-3xl font-bold text-ink">My Tools</h1>
      <p className="mt-1 text-sm text-ink-2">All tools in one place. Pick the task — we handle the rest.</p>

      <div className="relative mt-6 w-full max-w-sm">
        <Search size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-2" />
        <Input
          type="search"
          role="searchbox"
          aria-label="Search tools"
          placeholder="Search tools…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full pl-9"
        />
      </div>

      {visible.length === 0 ? (
        <p className="mt-10 text-sm text-ink-2">No tools match “{query}”.</p>
      ) : (
        <div className="mt-8 grid w-full max-w-3xl gap-3 sm:grid-cols-2">
          {visible.map((t) => (
            <div
              key={t.id}
              style={{ '--module': t.hue } as CSSProperties}
              className="glass-1 flex flex-col rounded-card p-4"
            >
              <div className="flex items-center gap-3">
                <span
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[12px] bg-[color-mix(in_srgb,var(--module)_18%,transparent)]"
                  aria-hidden
                >
                  <t.Icon size={20} style={{ color: 'var(--module)' }} />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-ink">{t.name}</div>
                  <div className="truncate text-xs text-ink-2">{t.description}</div>
                </div>
              </div>
              <div className="mt-3 flex justify-end">
                <Button type="button" variant="glass" size="sm" onClick={() => runTool(t)}>
                  {t.action.kind === 'scan' ? 'Scan' : 'Open'}
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
