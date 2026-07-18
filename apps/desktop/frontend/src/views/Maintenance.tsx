import { useEffect, useState } from 'react'
import { RunMaintenance, StartTMSnapshotsClear, CancelMaintenance } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import type { MaintenanceResult } from '../lib/types'

interface TMDate {
  date: string
  error?: string
}

function AdminBadge() {
  return <Badge variant="moderate">Requires administrator</Badge>
}

// Failed results that required admin get a "Retry with administrator" CTA so the
// user can re-trigger the task (and its osascript admin prompt) directly.
function ResultLine({ result, onRetry }: { result: MaintenanceResult; onRetry?: () => void }) {
  if (result.success) {
    return <p className="mt-2 text-sm text-safe">✓ {result.message}</p>
  }
  return (
    <div className="mt-2">
      <p className="text-sm text-danger">
        ✗ {result.message}
        {result.error ? ` — ${result.error}` : ''}
      </p>
      {result.requiresAdmin && onRetry ? (
        <Button type="button" size="sm" className="mt-1.5 bg-moderate" onClick={onRetry}>
          Retry with administrator
        </Button>
      ) : null}
    </div>
  )
}

function Spinner() {
  return <p className="mt-2 animate-pulse text-sm text-ink-2">Running…</p>
}

export function Maintenance() {
  const [dns, setDns] = useState<{ running: boolean; result?: MaintenanceResult }>({ running: false })
  const [purge, setPurge] = useState<{ running: boolean; result?: MaintenanceResult }>({ running: false })
  const [tm, setTm] = useState<{ running: boolean; result?: MaintenanceResult; dates: TMDate[] }>({
    running: false,
    dates: [],
  })

  useEffect(() => {
    EventsOn('maintenance:progress', (d: { done: number; total: number; date: string; error?: string }) => {
      setTm((prev) => ({ ...prev, running: true, dates: [...prev.dates, { date: d.date, error: d.error }] }))
    })
    EventsOn('maintenance:done', (d: { result: MaintenanceResult }) => {
      setTm((prev) => ({ ...prev, running: false, result: d.result }))
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

  const runTM = async () => {
    setTm({ running: true, dates: [] })
    try {
      await StartTMSnapshotsClear()
    } catch (e) {
      setTm({
        running: false,
        dates: [],
        result: { success: false, message: 'Could not start', error: String(e), requiresAdmin: true },
      })
    }
  }

  return (
    <div className="space-y-4 p-6">
      <h1 className="text-xl font-semibold text-ink">Maintenance</h1>

      <Card className="p-5">
        <div className="flex items-center gap-2">
          <h2 className="font-medium text-ink">Flush DNS Cache</h2>
          <AdminBadge />
        </div>
        <p className="mt-1 text-sm text-ink-2">
          Clears the macOS DNS resolver cache (dscacheutil + mDNSResponder). Fixes stale DNS lookups.
        </p>
        <Button type="button" className="mt-3" disabled={dns.running} onClick={() => void run('dns')}>
          Run
        </Button>
        {dns.running ? <Spinner /> : null}
        {dns.result ? <ResultLine result={dns.result} onRetry={() => void run('dns')} /> : null}
      </Card>

      <Card className="p-5">
        <h2 className="font-medium text-ink">Free Purgeable Space</h2>
        <p className="mt-1 text-sm text-ink-2">
          Asks macOS to release purgeable disk space (/usr/sbin/purge). Usually runs without
          privileges — may prompt for admin if the system refuses.
        </p>
        <Button type="button" className="mt-3" disabled={purge.running} onClick={() => void run('purge')}>
          Run
        </Button>
        {purge.running ? <Spinner /> : null}
        {purge.result ? <ResultLine result={purge.result} onRetry={() => void run('purge')} /> : null}
      </Card>

      <Card className="p-5">
        <div className="flex items-center gap-2">
          <h2 className="font-medium text-ink">Clear Time Machine Snapshots</h2>
          <AdminBadge />
        </div>
        <p className="mt-1 text-sm text-ink-2">
          Deletes local Time Machine snapshots (tmutil). One admin prompt deletes all snapshots.
        </p>
        <div className="mt-3 flex items-center gap-3">
          <Button type="button" disabled={tm.running} onClick={() => void runTM()}>
            Run
          </Button>
          {tm.running ? (
            <Button type="button" variant="ghost" onClick={() => void CancelMaintenance()}>
              Cancel
            </Button>
          ) : null}
        </div>
        {tm.running ? <Spinner /> : null}
        {tm.dates.length > 0 ? (
          <ul className="mt-2 space-y-0.5">
            {tm.dates.map((d) => (
              <li key={d.date} className="font-mono text-xs">
                {d.error ? (
                  <span className="text-danger">✗ {d.date} — {d.error}</span>
                ) : (
                  <span className="text-safe">✓ {d.date}</span>
                )}
              </li>
            ))}
          </ul>
        ) : null}
        {tm.result ? <ResultLine result={tm.result} onRetry={() => void runTM()} /> : null}
      </Card>
    </div>
  )
}

export default Maintenance
