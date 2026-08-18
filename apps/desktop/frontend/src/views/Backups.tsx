import { useCallback, useEffect, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { ListBackups, RestoreBackup, DeleteBackup, CleanOldBackups, GetBackupDetails } from '../../wailsjs/go/main/App'
import { useUiStore } from '../stores/uiStore'
import { formatSize } from '../lib/format'
import { contractHome } from '../lib/uninstallMath'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import type { BackupDetails, BackupInfo } from '../lib/types'

type Pending = { action: 'restore' | 'delete'; path: string } | null
type Busy = { action: 'restore' | 'delete'; path: string } | null

export function Backups() {
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [pending, setPending] = useState<Pending>(null)
  const [busy, setBusy] = useState<Busy>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [removed, setRemoved] = useState(0)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [details, setDetails] = useState<Record<string, BackupDetails>>({})
  const [detailsErrors, setDetailsErrors] = useState<Record<string, string>>({})
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
    setBusy({ action, path })
    try {
      if (action === 'restore') {
        try {
          const r = await RestoreBackup(path)
          setMessage(
            r.failed > 0
              ? `Restored ${r.restored} item(s), ${r.failed} failed: ${r.errors[0] ?? ''}`
              : `Restored ${r.restored} item(s)`,
          )
        } catch (e) {
          setMessage(`Restore failed: ${String(e)}`)
        }
      } else {
        try {
          await DeleteBackup(path)
          setMessage('Backup deleted')
        } catch (e) {
          setMessage(`Delete failed: ${String(e)}`)
        }
      }
    } finally {
      setBusy(null)
    }
    void refresh()
  }

  const toggleExpand = async (path: string) => {
    if (expanded === path) {
      setExpanded(null)
      return
    }
    setExpanded(path)
    setDetailsErrors((prev) => {
      const { [path]: _omit, ...rest } = prev
      return rest
    })
    if (details[path]) return
    try {
      const d = await GetBackupDetails(path)
      setDetails((prev) => ({
        ...prev,
        // Defense in depth: a Go nil slice arrives as JSON null.
        [path]: { items: d?.items ?? [], fromManifest: Boolean(d?.fromManifest), truncated: d?.truncated ?? 0 },
      }))
    } catch {
      // no cache -> re-expand retries
      setDetailsErrors((prev) => ({ ...prev, [path]: "Couldn't read backup details" }))
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold text-ink">Backups</h1>
      <p className="mt-1 text-sm text-ink-2">
        Backups older than {config?.backupRetentionDays ?? 7} days are removed automatically.
        {removed > 0 ? ` ${removed} expired backup${removed === 1 ? '' : 's'} just removed.` : ''}
      </p>
      {message ? <p className="mt-2 text-sm text-accent">{message}</p> : null}

      {loading ? (
        <p className="mt-4 text-sm text-ink-2">Loading…</p>
      ) : backups.length === 0 ? (
        <p className="mt-4 text-sm text-ink-2">
          No backups yet. Backups are created when you clean with “Back up items” enabled.
        </p>
      ) : (
        <Card className="mt-4 p-3">
          <ul className="space-y-1">
            {backups.map((b) => (
              <li key={b.path} className="glass-1 rounded-control px-3 py-2 transition hover:brightness-105">
                <div className="flex items-center gap-4">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    aria-label={`Toggle details for backup ${new Date(b.date).toLocaleString()}`}
                    onClick={() => void toggleExpand(b.path)}
                    className="px-1.5 text-ink-2"
                  >
                    {expanded === b.path ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                  </Button>
                  <span className="flex-1 text-sm font-medium text-ink">{new Date(b.date).toLocaleString()}</span>
                  <span className="nums text-sm text-ink-2">{formatSize(b.size)}</span>
                  {busy?.path === b.path ? (
                    <span className="flex items-center gap-2 text-sm text-ink-2">
                      <span className="inline-block size-3 animate-spin rounded-full border-[1.5px] border-ink-2/40 border-t-ink" aria-hidden />
                      {busy.action === 'restore' ? 'Restoring…' : 'Deleting…'}
                    </span>
                  ) : pending?.path === b.path ? (
                    <span className="flex items-center gap-2 text-sm text-ink">
                      {pending.action === 'restore' ? 'Restore this backup?' : 'Delete this backup permanently?'}
                      <Button type="button" variant="glass" size="sm" onClick={() => void execute()}>
                        Confirm
                      </Button>
                      <Button type="button" variant="glass" size="sm" onClick={() => setPending(null)}>
                        Cancel
                      </Button>
                    </span>
                  ) : (
                    <span className="flex gap-2">
                      <Button
                        type="button"
                        variant="glass"
                        size="sm"
                        onClick={() => setPending({ action: 'restore', path: b.path })}
                      >
                        Restore
                      </Button>
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        onClick={() => setPending({ action: 'delete', path: b.path })}
                      >
                        Delete
                      </Button>
                    </span>
                  )}
                </div>
                {expanded === b.path ? (
                  <div className="mt-2 pl-8">
                    {!details[b.path] ? (
                      <p className="text-xs text-ink-2">{detailsErrors[b.path] ?? 'Loading…'}</p>
                    ) : details[b.path].items.length === 0 ? (
                      <p className="text-xs text-ink-2">No details recorded for this backup</p>
                    ) : (
                      <ul className="space-y-0.5">
                        {details[b.path].items.map((it) => (
                          <li key={it.path} className="nums truncate font-mono text-xs text-ink-2">
                            {contractHome(it.path)} ({formatSize(it.size)})
                          </li>
                        ))}
                        {details[b.path].truncated > 0 ? (
                          <li className="text-xs text-ink-2">…and {details[b.path].truncated} more files</li>
                        ) : null}
                      </ul>
                    )}
                  </div>
                ) : null}
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  )
}

export default Backups
