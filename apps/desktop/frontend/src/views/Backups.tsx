import { useCallback, useEffect, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { ListBackups, RestoreBackup, DeleteBackup, CleanOldBackups, GetBackupDetails } from '../../wailsjs/go/main/App'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import { contractHome } from '../lib/uninstallMath'
import type { BackupDetails, BackupInfo } from '../lib/types'

type Pending = { action: 'restore' | 'delete'; path: string } | null

export function Backups() {
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [pending, setPending] = useState<Pending>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [removed, setRemoved] = useState(0)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [details, setDetails] = useState<Record<string, BackupDetails>>({})
  const [detailsError, setDetailsError] = useState<string | null>(null)
  const config = useUiStore((s) => s.config)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setBackups((await ListBackups()) ?? [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void (async () => {
      try {
        setRemoved(await CleanOldBackups()) // retention sweep before listing
      } catch {
        setRemoved(0)
      }
      await refresh()
    })()
    if (!useUiStore.getState().config) void useUiStore.getState().loadConfig()
  }, [refresh])

  const execute = async () => {
    if (!pending) return
    const { action, path } = pending
    setPending(null)
    if (action === 'restore') {
      const r = await RestoreBackup(path)
      setMessage(
        r.failed > 0
          ? `Restored ${r.restored} item(s), ${r.failed} failed: ${r.errors[0] ?? ''}`
          : `Restored ${r.restored} item(s)`,
      )
    } else {
      try {
        await DeleteBackup(path)
        setMessage('Backup deleted')
      } catch (e) {
        setMessage(`Delete failed: ${String(e)}`)
      }
    }
    void refresh()
  }

  const toggleExpand = async (path: string) => {
    if (expanded === path) {
      setExpanded(null)
      return
    }
    setExpanded(path)
    setDetailsError(null)
    if (details[path]) return
    try {
      const d = await GetBackupDetails(path)
      setDetails((prev) => ({
        ...prev,
        // Defense in depth: a Go nil slice arrives as JSON null.
        [path]: { items: d?.items ?? [], fromManifest: Boolean(d?.fromManifest), truncated: d?.truncated ?? 0 },
      }))
    } catch {
      setDetailsError("Couldn't read backup details") // no cache -> re-expand retries
    }
  }

  const smallBtn =
    'rounded-md border border-zinc-300 px-2.5 py-1 text-xs hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800'

  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold">Backups</h1>
      <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
        Backups older than {config?.backupRetentionDays ?? 7} days are removed automatically.
        {removed > 0 ? ` ${removed} expired backup${removed === 1 ? '' : 's'} just removed.` : ''}
      </p>
      {message ? <p className="mt-2 text-sm text-indigo-600 dark:text-indigo-400">{message}</p> : null}

      {loading ? (
        <p className="mt-4 text-sm text-zinc-500 dark:text-zinc-400">Loading…</p>
      ) : backups.length === 0 ? (
        <p className="mt-4 text-sm text-zinc-500 dark:text-zinc-400">
          No backups yet. Backups are created when you clean with “Back up items” enabled.
        </p>
      ) : (
        <ul className="mt-4 divide-y divide-zinc-200 dark:divide-zinc-800">
          {backups.map((b) => (
            <li key={b.path} className="py-3">
              <div className="flex items-center gap-4">
                <button
                  aria-label={`Toggle details for backup ${new Date(b.date).toLocaleString()}`}
                  onClick={() => void toggleExpand(b.path)}
                  className="text-zinc-500 dark:text-zinc-400"
                >
                  {expanded === b.path ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </button>
                <span className="flex-1 text-sm font-medium">{new Date(b.date).toLocaleString()}</span>
                <span className="text-sm text-zinc-500 dark:text-zinc-400">{formatSize(b.size)}</span>
                {pending?.path === b.path ? (
                  <span className="flex items-center gap-2 text-sm">
                    {pending.action === 'restore' ? 'Restore this backup?' : 'Delete this backup permanently?'}
                    <button className={smallBtn} onClick={() => void execute()}>
                      Confirm
                    </button>
                    <button className={smallBtn} onClick={() => setPending(null)}>
                      Cancel
                    </button>
                  </span>
                ) : (
                  <span className="flex gap-2">
                    <button className={smallBtn} onClick={() => setPending({ action: 'restore', path: b.path })}>
                      Restore
                    </button>
                    <button
                      className={`${smallBtn} text-red-600 dark:text-red-400`}
                      onClick={() => setPending({ action: 'delete', path: b.path })}
                    >
                      Delete
                    </button>
                  </span>
                )}
              </div>
              {expanded === b.path ? (
                <div className="mt-2 pl-8">
                  {!details[b.path] ? (
                    <p className="text-xs text-zinc-500 dark:text-zinc-400">{detailsError ?? 'Loading…'}</p>
                  ) : details[b.path].items.length === 0 ? (
                    <p className="text-xs text-zinc-500 dark:text-zinc-400">No details recorded for this backup</p>
                  ) : (
                    <ul className="space-y-0.5">
                      {details[b.path].items.map((it) => (
                        <li key={it.path} className="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">
                          {contractHome(it.path)} ({formatSize(it.size)})
                        </li>
                      ))}
                      {details[b.path].truncated > 0 ? (
                        <li className="text-xs text-zinc-400 dark:text-zinc-500">…and {details[b.path].truncated} more files</li>
                      ) : null}
                    </ul>
                  )}
                </div>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export default Backups
