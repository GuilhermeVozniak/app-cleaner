import { useEffect, useState } from 'react'
import { RunMaintenance, StartTMSnapshotsClear, CancelMaintenance } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import type { MaintenanceResult } from '../lib/types'

interface TMDate {
  date: string
  error?: string
}

function AdminBadge() {
  return (
    <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-800 dark:bg-amber-900 dark:text-amber-200">
      Requires administrator
    </span>
  )
}

// Failed results that required admin get a "Retry with administrator" CTA so the
// user can re-trigger the task (and its osascript admin prompt) directly.
function ResultLine({ result, onRetry }: { result: MaintenanceResult; onRetry?: () => void }) {
  if (result.success) {
    return <p className="mt-2 text-sm text-green-600 dark:text-green-400">✓ {result.message}</p>
  }
  return (
    <div className="mt-2">
      <p className="text-sm text-red-600 dark:text-red-400">
        ✗ {result.message}
        {result.error ? ` — ${result.error}` : ''}
      </p>
      {result.requiresAdmin && onRetry ? (
        <button
          className="mt-1.5 rounded-md bg-amber-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-500"
          onClick={onRetry}
        >
          Retry with administrator
        </button>
      ) : null}
    </div>
  )
}

function Spinner() {
  return <p className="mt-2 animate-pulse text-sm text-zinc-500 dark:text-zinc-400">Running…</p>
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

  const card = 'rounded-xl border border-zinc-200 p-5 dark:border-zinc-800'
  const runBtn =
    'mt-3 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-40'

  return (
    <div className="space-y-4 p-6">
      <h1 className="text-xl font-semibold">Maintenance</h1>

      <div className={card}>
        <div className="flex items-center gap-2">
          <h2 className="font-medium">Flush DNS Cache</h2>
          <AdminBadge />
        </div>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Clears the macOS DNS resolver cache (dscacheutil + mDNSResponder). Fixes stale DNS lookups.
        </p>
        <button className={runBtn} disabled={dns.running} onClick={() => void run('dns')}>
          Run
        </button>
        {dns.running ? <Spinner /> : null}
        {dns.result ? <ResultLine result={dns.result} onRetry={() => void run('dns')} /> : null}
      </div>

      <div className={card}>
        <h2 className="font-medium">Free Purgeable Space</h2>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Asks macOS to release purgeable disk space (/usr/sbin/purge). Usually runs without
          privileges — may prompt for admin if the system refuses.
        </p>
        <button className={runBtn} disabled={purge.running} onClick={() => void run('purge')}>
          Run
        </button>
        {purge.running ? <Spinner /> : null}
        {purge.result ? <ResultLine result={purge.result} onRetry={() => void run('purge')} /> : null}
      </div>

      <div className={card}>
        <div className="flex items-center gap-2">
          <h2 className="font-medium">Clear Time Machine Snapshots</h2>
          <AdminBadge />
        </div>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Deletes local Time Machine snapshots (tmutil). One admin prompt deletes all snapshots.
        </p>
        <div className="flex items-center gap-3">
          <button className={runBtn} disabled={tm.running} onClick={() => void runTM()}>
            Run
          </button>
          {tm.running ? (
            <button
              className="mt-3 rounded-md px-3 py-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
              onClick={() => void CancelMaintenance()}
            >
              Cancel
            </button>
          ) : null}
        </div>
        {tm.running ? <Spinner /> : null}
        {tm.dates.length > 0 ? (
          <ul className="mt-2 space-y-0.5">
            {tm.dates.map((d) => (
              <li key={d.date} className="font-mono text-xs">
                {d.error ? (
                  <span className="text-red-600 dark:text-red-400">✗ {d.date} — {d.error}</span>
                ) : (
                  <span className="text-green-600 dark:text-green-400">✓ {d.date}</span>
                )}
              </li>
            ))}
          </ul>
        ) : null}
        {tm.result ? <ResultLine result={tm.result} onRetry={() => void runTM()} /> : null}
      </div>
    </div>
  )
}

export default Maintenance
