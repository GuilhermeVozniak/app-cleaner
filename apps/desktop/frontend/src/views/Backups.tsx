import { useCallback, useEffect, useState } from 'react'
import { ListBackups, RestoreBackup, DeleteBackup, CleanOldBackups } from '../../wailsjs/go/main/App'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import type { BackupInfo } from '../lib/types'

type Pending = { action: 'restore' | 'delete'; path: string } | null

export function Backups() {
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [pending, setPending] = useState<Pending>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [removed, setRemoved] = useState(0)
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
            <li key={b.path} className="flex items-center gap-4 py-3">
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
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export default Backups
