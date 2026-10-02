import { useState } from 'react'
import { Search } from 'lucide-react'
import { filterTools, TOOLS } from '../lib/modules'
import type { ToolDef } from '../lib/modules'
import { ModuleIcon } from '../components/ModuleIcon'
import { PageHeader } from '../components/PageHeader'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
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

/** My Tools: searchable grid of every tool. */
export default function MyTools() {
  const [query, setQuery] = useState('')
  const visible = filterTools(TOOLS, query)

  return (
    <div className="materialize mx-auto max-w-5xl px-10 pb-10 pt-6">
      <PageHeader
        title="My Tools"
        subtitle="Every tool in one place. Pick a task and go."
        aside={
          <div className="relative">
            <Search
              size={15}
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-3"
            />
            <Input
              type="search"
              role="searchbox"
              aria-label="Search tools"
              placeholder="Search…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="w-64 rounded-[12px] pl-9"
            />
          </div>
        }
      />

      {visible.length === 0 ? (
        <p className="mt-10 text-card text-ink-2">No tools match “{query}”.</p>
      ) : (
        <div className="mt-8 grid grid-cols-2 gap-4 lg:grid-cols-3">
          {visible.map((t) => (
            <Card key={t.id} className="flex min-h-[196px] flex-col p-5">
              <ModuleIcon Icon={t.Icon} hue={t.hue} size="md" />
              <div className="mt-4 text-card font-semibold text-ink">{t.name}</div>
              <div className="mt-1 text-body text-ink-2">{t.description}</div>
              <div className="mt-auto flex justify-end pt-4">
                <Button type="button" variant="primary" size="sm" onClick={() => runTool(t)}>
                  {t.action.kind === 'scan' ? 'Scan' : 'Open'}
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
