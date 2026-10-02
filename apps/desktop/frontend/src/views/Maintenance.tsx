import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Globe, HardDrive, History, ListChecks } from 'lucide-react'
import { RunMaintenance, StartTMSnapshotsClear, CancelMaintenance } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { ModuleIcon } from '../components/ModuleIcon'
import { ScanLens } from '../components/ScanLens'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { useUiStore } from '../stores/uiStore'
import type { MaintenanceResult } from '../lib/types'

interface TMDate {
  date: string
  error?: string
}

type TaskState = { running: boolean; result?: MaintenanceResult }

// Failed results that required admin get a "Retry with administrator" CTA so the
// user can re-trigger the task (and its osascript admin prompt) directly.
function ResultLine({ result, onRetry }: { result: MaintenanceResult; onRetry?: () => void }) {
  if (result.success) {
    return <p className="text-body text-safe">✓ {result.message}</p>
  }
  return (
    <div>
      <p className="text-body text-danger">
        ✗ {result.message}
        {result.error ? ` — ${result.error}` : ''}
      </p>
      {result.requiresAdmin && onRetry ? (
        <Button type="button" size="sm" variant="secondary" className="mt-2" onClick={onRetry}>
          Retry with administrator
        </Button>
      ) : null}
    </div>
  )
}

function Spinner() {
  return (
    <p className="flex items-center gap-2 text-body text-ink-2">
      <span
        aria-hidden
        className="inline-block h-3 w-3 animate-spin rounded-full border-[1.5px] border-[rgb(255_255_255/0.3)] border-t-ink"
      />
      Running…
    </p>
  )
}

/** One maintenance task: gem, title, what it does, Run, and the live result underneath. */
function TaskCard({
  Icon,
  title,
  description,
  admin,
  running,
  result,
  onRun,
  onRetry,
  extraActions,
  children,
}: {
  Icon: LucideIcon
  title: string
  description: string
  admin?: boolean
  running: boolean
  result?: MaintenanceResult
  onRun: () => void
  onRetry: () => void
  extraActions?: ReactNode
  children?: ReactNode
}) {
  return (
    <div className="glass-1 flex min-h-[190px] flex-col rounded-card p-5">
      <ModuleIcon Icon={Icon} size="md" />
      <div className="mt-4 flex items-center gap-2">
        <h2 className="text-card font-semibold text-ink">{title}</h2>
      </div>
      <p className="mt-1 text-body text-ink-2">{description}</p>
      {admin ? (
        <div className="mt-2">
          <Badge variant="moderate">Requires administrator</Badge>
        </div>
      ) : null}
      <div className="mt-3 space-y-2">
        {running ? <Spinner /> : null}
        {children}
        {result ? <ResultLine result={result} onRetry={onRetry} /> : null}
      </div>
      <div className="mt-auto flex items-center justify-end gap-2 pt-4">
        {extraActions}
        <Button type="button" variant="secondary" disabled={running} onClick={onRun}>
          Run
        </Button>
      </div>
    </div>
  )
}

export function Maintenance() {
  const [dns, setDns] = useState<TaskState>({ running: false })
  const [purge, setPurge] = useState<TaskState>({ running: false })
  const [tm, setTm] = useState<TaskState & { dates: TMDate[] }>({ running: false, dates: [] })
  const [runningAll, setRunningAll] = useState(false)
  // Resolves the in-flight "Run All" step once maintenance:done arrives.
  const tmSettled = useRef<(() => void) | null>(null)

  useEffect(() => {
    EventsOn('maintenance:progress', (d: { done: number; total: number; date: string; error?: string }) => {
      setTm((prev) => ({ ...prev, running: true, dates: [...prev.dates, { date: d.date, error: d.error }] }))
    })
    EventsOn('maintenance:done', (d: { result: MaintenanceResult }) => {
      setTm((prev) => ({ ...prev, running: false, result: d.result }))
      tmSettled.current?.()
      tmSettled.current = null
    })
    return () => {
      EventsOff('maintenance:progress')
      EventsOff('maintenance:done')
    }
  }, [])

  const run = async (task: 'dns' | 'purge') => {
    const set = task === 'dns' ? setDns : setPurge
    set({ running: true })
    try {
      set({ running: false, result: await RunMaintenance(task) })
    } catch (e) {
      set({
        running: false,
        result: { success: false, message: 'Task failed', error: String(e), requiresAdmin: false },
      })
    }
  }

  /** Starts the Time Machine sweep; resolves true when it started. */
  const runTM = async (): Promise<boolean> => {
    setTm({ running: true, dates: [] })
    try {
      await StartTMSnapshotsClear()
      return true
    } catch (e) {
      setTm({
        running: false,
        dates: [],
        result: { success: false, message: 'Could not start', error: String(e), requiresAdmin: true },
      })
      return false
    }
  }

  const runAll = async () => {
    if (runningAll) return
    setRunningAll(true)
    try {
      await run('dns')
      await run('purge')
      await new Promise<void>((resolve) => {
        tmSettled.current = resolve
        void runTM().then((started) => {
          if (!started) {
            tmSettled.current = null
            resolve()
          }
        })
      })
    } finally {
      setRunningAll(false)
    }
  }

  const anyRunning = dns.running || purge.running || tm.running || runningAll

  return (
    <div className="relative flex h-full flex-col overflow-hidden">
      <div className="flex-1 overflow-y-auto px-10 pb-36 pt-8">
        <div className="materialize mx-auto max-w-5xl">
          <h1 className="text-center text-headline font-semibold text-ink">Keep your Mac in top shape</h1>
          <p className="mt-2 text-center text-card text-ink-2">
            Run the recommended maintenance tasks one at a time, or all at once.
          </p>

          <div className="mt-10 grid grid-cols-3 gap-4">
            <TaskCard
              Icon={Globe}
              title="Flush DNS Cache"
              description="Clears the macOS DNS resolver cache. Fixes stale lookups after network changes."
              admin
              running={dns.running}
              result={dns.result}
              onRun={() => void run('dns')}
              onRetry={() => void run('dns')}
            />
            <TaskCard
              Icon={HardDrive}
              title="Free Purgeable Space"
              description="Asks macOS to release disk space it is holding in reserve. Usually runs without a password."
              running={purge.running}
              result={purge.result}
              onRun={() => void run('purge')}
              onRetry={() => void run('purge')}
            />
            <TaskCard
              Icon={History}
              title="Clear Time Machine Snapshots"
              description="Deletes the local snapshots Time Machine keeps on this disk. Your backups stay untouched."
              admin
              running={tm.running}
              result={tm.result}
              onRun={() => void runTM()}
              onRetry={() => void runTM()}
              extraActions={
                tm.running ? (
                  <Button type="button" variant="ghost" onClick={() => void CancelMaintenance()}>
                    Cancel
                  </Button>
                ) : null
              }
            >
              {tm.dates.length > 0 ? (
                <ul className="space-y-0.5">
                  {tm.dates.map((d) => (
                    <li key={d.date} className="font-mono text-caption">
                      {d.error ? (
                        <span className="text-danger">✗ {d.date} — {d.error}</span>
                      ) : (
                        <span className="text-safe">✓ {d.date}</span>
                      )}
                    </li>
                  ))}
                </ul>
              ) : null}
            </TaskCard>

            <div className="glass-1 col-span-3 flex items-center gap-4 rounded-card px-5 py-4">
              <ModuleIcon Icon={ListChecks} size="md" />
              <div className="min-w-0 flex-1">
                <h2 className="text-card font-semibold text-ink">Login Items</h2>
                <p className="mt-0.5 text-body text-ink-2">See everything that starts automatically with your Mac.</p>
              </div>
              <Button type="button" variant="secondary" onClick={() => useUiStore.getState().setView('login-items')}>
                Open
              </Button>
            </div>
          </div>
        </div>
      </div>

      <div className="absolute inset-x-0 bottom-0 flex justify-center pb-2">
        {anyRunning ? (
          <ScanLens
            state="scanning"
            hue="var(--color-module-perf)"
            onScan={() => {}}
            caption={<span>{runningAll ? 'Running tasks…' : 'Working…'}</span>}
          />
        ) : (
          <ScanLens state="idle" hue="var(--color-module-perf)" label="Run All" onScan={() => void runAll()} />
        )}
      </div>
    </div>
  )
}

export default Maintenance
