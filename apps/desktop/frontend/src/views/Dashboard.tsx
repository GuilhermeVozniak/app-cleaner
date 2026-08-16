import { useEffect, useState } from 'react'
import type { CSSProperties } from 'react'
import { ChevronRight, HardDrive, Search, Timer, Trash2 } from 'lucide-react'
import { GetActivityStats, GetDiskUsage } from '../../wailsjs/go/main/App'
import { MODULES } from '../lib/modules'
import { formatSize, timeAgo } from '../lib/format'
import { Card } from '../components/ui/card'
import { Progress } from '../components/ui/progress'
import { ScanLens } from '../components/ScanLens'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import type { ActivityStats, DiskUsage } from '../lib/types'

/** Health-card copy for the disk bar. Exported for tests. */
export function diskHeadline(u: DiskUsage | null): string {
  if (!u || u.total <= 0) return 'Disk usage unavailable'
  const pct = Math.round((u.used / u.total) * 100)
  if (pct >= 90) return 'Your disk is almost full'
  if (pct >= 70) return 'Your disk is filling up'
  return 'Your Mac is in great shape'
}

/**
 * Smart Care dashboard: health card + lifetime stats + module tiles, with
 * the Smart Scan lens as the centerpiece CTA.
 */
export default function Dashboard() {
  const setView = useUiStore((s) => s.setView)
  const scanStatus = useScanStore((s) => s.status)
  const [disk, setDisk] = useState<DiskUsage | null>(null)
  const [stats, setStats] = useState<ActivityStats | null>(null)

  useEffect(() => {
    GetDiskUsage()
      .then((d) => setDisk((d as DiskUsage | null) ?? null))
      .catch(() => setDisk(null))
    GetActivityStats()
      .then((s) => setStats((s as ActivityStats | null) ?? null))
      .catch(() => setStats(null))
  }, [])

  const startSmartScan = () => {
    setView('smart-scan')
    if (useScanStore.getState().status === 'idle') {
      void useScanStore.getState().startScan()
    }
  }

  const usedPct = disk && disk.total > 0 ? Math.min(100, Math.round((disk.used / disk.total) * 100)) : 0
  const tiles = MODULES.filter((m) => m.view !== 'dashboard')

  return (
    <div
      className="module-wash flex h-full flex-col items-center overflow-y-auto px-8 pb-10 pt-12"
      style={{ '--module': 'var(--color-module-care)' } as CSSProperties}
    >
      <h1 className="text-3xl font-bold text-ink">Smart Care</h1>
      <p className="mt-1 text-sm text-ink-2">Everything that keeps your Mac in shape, in one place.</p>

      <div className="mt-8 grid w-full max-w-3xl gap-3 sm:grid-cols-3">
        <Card className="p-4 sm:col-span-1">
          <div className="flex items-center gap-2 text-sm font-medium text-ink">
            <HardDrive size={15} className="text-[var(--color-module-care)]" />
            Mac Health
          </div>
          <p className="mt-2 text-xs text-ink-2">{diskHeadline(disk)}</p>
          {disk && disk.total > 0 ? (
            <>
              <Progress value={usedPct} className="mt-3 h-1.5" />
              <p className="nums mt-2 text-xs text-ink-2">
                {formatSize(disk.free)} free of {formatSize(disk.total)}
              </p>
            </>
          ) : null}
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 text-sm font-medium text-ink">
            <Trash2 size={15} className="text-[var(--color-module-cleanup)]" />
            Storage Cleaned
          </div>
          <p className="nums mt-2 text-2xl font-semibold text-ink">
            {formatSize(stats?.totalCleanedBytes ?? 0)}
          </p>
          <p className="nums mt-1 text-xs text-ink-2">
            {stats?.totalCleanedItems ?? 0} items · {stats?.cleanRuns ?? 0} clean
            {(stats?.cleanRuns ?? 0) === 1 ? '' : 's'}
          </p>
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 text-sm font-medium text-ink">
            <Timer size={15} className="text-[var(--color-module-perf)]" />
            Activity
          </div>
          <p className="nums mt-2 text-2xl font-semibold text-ink">{stats?.scanRuns ?? 0} scans</p>
          <p className="nums mt-1 text-xs text-ink-2">
            {stats?.appsUninstalled ?? 0} apps uninstalled
            {stats?.lastCleanAt ? ` · last clean ${timeAgo(stats.lastCleanAt)}` : ''}
          </p>
        </Card>
      </div>

      <div className="my-8">
        <ScanLens
          state="idle"
          hue="var(--color-module-care)"
          label={scanStatus === 'done' ? 'View Results' : 'Smart Scan'}
          icon={<Search size={28} style={{ color: 'var(--module)' }} />}
          onScan={startSmartScan}
        />
      </div>

      <div className="grid w-full max-w-3xl gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {tiles.map((m) => (
          <button
            key={m.view}
            type="button"
            onClick={() => setView(m.view)}
            style={{ '--module': m.hue } as CSSProperties}
            className="glass-1 group flex items-center gap-3 rounded-card p-4 text-left transition hover:brightness-105 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            <span
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[12px] bg-[color-mix(in_srgb,var(--module)_18%,transparent)]"
              aria-hidden
            >
              <m.Icon size={20} style={{ color: 'var(--module)' }} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-medium text-ink">{m.label}</span>
              <span className="block truncate text-xs text-ink-2">{m.description}</span>
            </span>
            <ChevronRight size={16} className="shrink-0 text-ink-2 transition group-hover:translate-x-0.5" />
          </button>
        ))}
      </div>
    </div>
  )
}
