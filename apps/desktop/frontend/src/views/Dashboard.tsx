import { useEffect, useState } from 'react'
import type { CSSProperties, ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Activity, Trash2 } from 'lucide-react'
import { GetActivityStats, GetDiskUsage } from '../../wailsjs/go/main/App'
import { MODULES } from '../lib/modules'
import { cn } from '../lib/cn'
import { formatSize, timeAgo } from '../lib/format'
import { ModuleIcon } from '../components/ModuleIcon'
import { Progress } from '../components/ui/progress'
import { ScanLens } from '../components/ScanLens'
import { useScanStore } from '../stores/scanStore'
import { useUninstallerStore } from '../stores/uninstallerStore'
import { useUiStore } from '../stores/uiStore'
import type { ActivityStats, DiskUsage } from '../lib/types'

/** Health headline for the disk state. Exported for tests. */
export function diskHeadline(u: DiskUsage | null): string {
  if (!u || u.total <= 0) return 'Disk usage unavailable'
  const pct = Math.round((u.used / u.total) * 100)
  if (pct >= 90) return 'Your disk is almost full'
  if (pct >= 70) return 'Your disk is filling up'
  return 'Your Mac is in great shape'
}

const plural = (n: number, one: string, many = `${one}s`) => (n === 1 ? one : many)

interface TileProps {
  label: string
  hue: string
  Icon: LucideIcon
  stat: ReactNode
  status: ReactNode
  /** Green status line for "all good" states. */
  good?: boolean
  onClick?: () => void
}

/** Smart Care tile: label top-left, the module's gem bleeding off the top-right corner, the stat and its status bottom-left. */
function Tile({ label, hue, Icon, stat, status, good, onClick }: TileProps) {
  const style = { '--module': hue } as CSSProperties
  const base = 'glass-1 relative flex h-[168px] flex-col overflow-hidden rounded-card p-5 text-left'
  const body = (
    <>
      <ModuleIcon Icon={Icon} size="xl" className="absolute -right-6 -top-7 rotate-6" />
      <span className="relative text-card text-ink-2">{label}</span>
      <span className="nums relative mt-auto text-stat font-semibold text-ink">{stat}</span>
      <span className={cn('relative mt-1 text-body', good ? 'text-safe' : 'text-ink-2')}>{status}</span>
    </>
  )
  if (!onClick) {
    return (
      <div style={style} className={base}>
        {body}
      </div>
    )
  }
  return (
    <button
      type="button"
      onClick={onClick}
      style={style}
      className={cn(base, 'focus-ring transition-colors duration-150 hover:bg-[rgb(255_255_255/0.12)]')}
    >
      {body}
    </button>
  )
}

/**
 * Smart Care: one health headline, a grid of module tiles that read out the
 * current state of each module, and the Smart Scan orb.
 */
export default function Dashboard() {
  const setView = useUiStore((s) => s.setView)
  const scanStatus = useScanStore((s) => s.status)
  const scanTotal = useScanStore((s) => s.totalSize)
  const scanProgress = useScanStore((s) => s.progress)
  const apps = useUninstallerStore((s) => s.apps)
  const appsUpdatedAt = useUninstallerStore((s) => s.lastScanAt)
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

  const mod = (view: string) => MODULES.find((m) => m.view === view)!
  const care = mod('dashboard')
  const cleanup = mod('smart-scan')
  const applications = mod('uninstaller')
  const performance = mod('maintenance')
  const lens = mod('space-lens')

  const hasDisk = !!disk && disk.total > 0
  const usedPct = hasDisk ? Math.min(100, Math.round((disk.used / disk.total) * 100)) : 0

  const cleanupTile =
    scanStatus === 'done'
      ? scanTotal > 0
        ? { stat: `${formatSize(scanTotal)} of junk`, status: 'Ready to clean', good: false }
        : { stat: 'No junk found', status: 'Cleaned', good: true }
      : scanStatus === 'scanning'
        ? { stat: 'Scanning…', status: `${scanProgress.completed} of ${scanProgress.total} categories`, good: false }
        : { stat: 'Find junk', status: 'Not scanned yet', good: false }

  const appsTile =
    apps.length > 0
      ? { stat: `${apps.length} ${plural(apps.length, 'app')}`, status: appsUpdatedAt ? `Updated ${timeAgo(appsUpdatedAt)}` : 'Installed on your Mac' }
      : { stat: 'Manage apps', status: 'Uninstall apps completely' }

  const cleanedItems = stats?.totalCleanedItems ?? 0
  const cleanRuns = stats?.cleanRuns ?? 0
  const scanRuns = stats?.scanRuns ?? 0
  const uninstalled = stats?.appsUninstalled ?? 0

  return (
    <div
      className="materialize flex h-full flex-col px-10 pb-2 pt-4"
      style={{ '--module': care.hue } as CSSProperties}
    >
      <div className="flex flex-col items-center text-center">
        <h1 className="text-headline font-semibold text-ink">{diskHeadline(disk)}</h1>
        {hasDisk ? (
          <div className="mt-3 flex w-72 flex-col items-center gap-2">
            <Progress value={usedPct} className="h-1 text-white" />
            <p className="nums text-caption text-ink-2">
              {formatSize(disk.free)} free of {formatSize(disk.total)}
            </p>
          </div>
        ) : (
          <p className="mt-2 text-caption text-ink-2">Run a scan to see what you can free up.</p>
        )}
      </div>

      <div className="mx-auto mt-6 grid w-full max-w-5xl grid-cols-3 gap-4">
        <Tile
          label={cleanup.label}
          hue={cleanup.hue}
          Icon={cleanup.Icon}
          stat={cleanupTile.stat}
          status={cleanupTile.status}
          good={cleanupTile.good}
          onClick={() => setView('smart-scan')}
        />
        <Tile
          label={applications.label}
          hue={applications.hue}
          Icon={applications.Icon}
          stat={appsTile.stat}
          status={appsTile.status}
          onClick={() => setView('uninstaller')}
        />
        <Tile
          label={performance.label}
          hue={performance.hue}
          Icon={performance.Icon}
          stat="3 tasks"
          status="Maintenance ready to run"
          onClick={() => setView('maintenance')}
        />
        <Tile
          label={lens.label}
          hue={lens.hue}
          Icon={lens.Icon}
          stat={hasDisk ? `${formatSize(disk.free)} free` : 'Space map'}
          status="See what takes up space"
          onClick={() => setView('space-lens')}
        />
        <Tile
          label="Storage cleaned"
          hue={cleanup.hue}
          Icon={Trash2}
          stat={formatSize(stats?.totalCleanedBytes ?? 0)}
          status={`${cleanedItems} ${plural(cleanedItems, 'item')} in ${cleanRuns} ${plural(cleanRuns, 'clean')}`}
          good={cleanedItems > 0}
        />
        <Tile
          label="Activity"
          hue={care.hue}
          Icon={Activity}
          stat={`${scanRuns} ${plural(scanRuns, 'scan')}`}
          status={
            `${uninstalled} ${plural(uninstalled, 'app')} uninstalled` +
            (stats?.lastCleanAt ? `, last clean ${timeAgo(stats.lastCleanAt)}` : '')
          }
        />
      </div>

      <div className="mt-auto flex justify-center pt-4">
        <ScanLens
          state="idle"
          hue={care.hue}
          label={scanStatus === 'done' ? 'View Results' : 'Smart Scan'}
          onScan={startSmartScan}
        />
      </div>
    </div>
  )
}
