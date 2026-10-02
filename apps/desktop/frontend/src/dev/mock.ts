/**
 * Dev-only stand-in for the Wails bridge.
 *
 * Open the Vite dev server as `http://localhost:5173/?mock` and every Go
 * binding and runtime event is served from here with realistic data, so
 * the whole UI can be exercised (and screenshotted) in a plain browser.
 * Flags: `&fda=0` denies Full Disk Access, `&view=<view>` picks the first
 * screen, `&empty=1` makes scans find nothing, `&update=1` offers an update.
 *
 * Never imported by the production bundle (see main.tsx).
 */
import type {
  ActivityStats,
  AppInfo,
  BackupDetails,
  BackupInfo,
  Category,
  CleanableItem,
  Config,
  DiskUsage,
  DisplayRow,
  LoginItem,
  MaintenanceResult,
  ScanResult,
  SpaceLensNode,
  UpdateInfo,
} from '../lib/types'

type Listener = (...data: unknown[]) => void

const HOME = '/Users/guilherme'
const listeners = new Map<string, Listener[]>()

function emit(event: string, ...data: unknown[]) {
  for (const cb of listeners.get(event) ?? []) cb(...data)
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

// Deterministic pseudo-random so screenshots are stable between reloads.
let seed = 7
function rnd() {
  seed = (seed * 16807) % 2147483647
  return seed / 2147483647
}
const MB = 1024 * 1024
const size = (minMB: number, maxMB: number) => Math.round((minMB + rnd() * (maxMB - minMB)) * MB)

export const CATEGORIES: Category[] = [
  { id: 'system-cache', name: 'User Cache Files', group: 'System Junk', description: 'Caches apps can rebuild.', safetyLevel: 'moderate' },
  { id: 'system-logs', name: 'System Log Files', group: 'System Junk', description: 'Old diagnostic logs.', safetyLevel: 'moderate' },
  { id: 'temp-files', name: 'Temporary Files', group: 'System Junk', description: 'Leftover temp files.', safetyLevel: 'safe' },
  { id: 'language-files', name: 'Language Files', group: 'System Junk', description: 'Unused localisations.', safetyLevel: 'risky', safetyNote: 'Removes app translations you do not use.' },
  { id: 'launch-agents', name: 'Orphaned Launch Agents', group: 'System Junk', description: 'Agents whose program is gone.', safetyLevel: 'moderate' },
  { id: 'trash', name: 'Trash', group: 'Storage', description: 'Items in your Trash.', safetyLevel: 'safe' },
  { id: 'downloads', name: 'Old Downloads', group: 'Storage', description: 'Downloads you have not touched.', safetyLevel: 'risky', safetyNote: 'May contain important files you forgot about.', supportsFileSelection: true },
  { id: 'ios-backups', name: 'iOS Backups', group: 'Storage', description: 'Local device backups.', safetyLevel: 'risky', safetyNote: 'Device backups cannot be recreated.' },
  { id: 'mail-attachments', name: 'Mail Attachments', group: 'Storage', description: 'Attachments Mail downloaded.', safetyLevel: 'risky', safetyNote: 'Mail re-downloads them on demand.' },
  { id: 'duplicates', name: 'Duplicate Files', group: 'Storage', description: 'Identical files in several places.', safetyLevel: 'risky', safetyNote: 'Keeps one copy of each file.' },
  { id: 'browser-cache', name: 'Browser Cache', group: 'Browsers', description: 'Safari, Chrome and Firefox caches.', safetyLevel: 'safe' },
  { id: 'dev-cache', name: 'Development Cache', group: 'Development', description: 'Xcode, npm, pip, Go caches.', safetyLevel: 'moderate' },
  { id: 'homebrew', name: 'Homebrew Cache', group: 'Development', description: 'Downloaded bottles.', safetyLevel: 'safe' },
  { id: 'docker', name: 'Docker', group: 'Development', description: 'Unused images and build cache.', safetyLevel: 'safe' },
  { id: 'node-modules', name: 'Node Modules', group: 'Development', description: 'node_modules in old projects.', safetyLevel: 'moderate' },
  { id: 'large-files', name: 'Large Files', group: 'Large Files', description: 'Files over your threshold.', safetyLevel: 'risky', safetyNote: 'Review each file before removing.', supportsFileSelection: true },
]

const FILES: Record<string, Array<[string, number]>> = {
  'system-cache': [
    [`${HOME}/Library/Caches/com.apple.Safari`, size(120, 400)],
    [`${HOME}/Library/Caches/com.spotify.client`, size(300, 900)],
    [`${HOME}/Library/Caches/com.google.Chrome`, size(200, 600)],
    [`${HOME}/Library/Caches/Homebrew`, size(80, 200)],
    [`${HOME}/Library/Caches/com.figma.Desktop`, size(60, 300)],
  ],
  'system-logs': [
    [`${HOME}/Library/Logs/DiagnosticReports`, size(20, 90)],
    [`${HOME}/Library/Logs/Adobe`, size(5, 40)],
  ],
  'temp-files': [
    ['/private/tmp/com.apple.launchd.x1', size(1, 30)],
    [`${HOME}/Library/Application Support/CrashReporter/tmp`, size(2, 10)],
  ],
  'language-files': [['/Applications/Slack.app/Contents/Resources/de.lproj', size(10, 40)]],
  'launch-agents': [[`${HOME}/Library/LaunchAgents/com.adobe.ARMDCHelper.plist`, 0.004 * MB]],
  trash: [
    [`${HOME}/.Trash/Screenshot 2026-08-16 at 12.43.02.png`, size(1, 3)],
    [`${HOME}/.Trash/Xcode_15.4.xip`, size(1200, 3000)],
    [`${HOME}/.Trash/old-project`, size(100, 500)],
  ],
  downloads: [
    [`${HOME}/Downloads/Docker.dmg`, size(500, 700)],
    [`${HOME}/Downloads/Figma-124.3.5.zip`, size(100, 200)],
    [`${HOME}/Downloads/invoice-2025-11.pdf`, size(0.2, 1)],
    [`${HOME}/Downloads/node-v22.11.0.pkg`, size(70, 90)],
    [`${HOME}/Downloads/archive/photos-2024.zip`, size(900, 2000)],
    [`${HOME}/Downloads/archive/talk.key`, size(40, 90)],
    [`${HOME}/Downloads/archive/report-final-v2.pdf`, size(3, 9)],
    [`${HOME}/Downloads/archive/report-final-v3.pdf`, size(3, 9)],
    [`${HOME}/Downloads/archive/report-final-v4.pdf`, size(3, 9)],
    [`${HOME}/Downloads/archive/report-final-v5.pdf`, size(3, 9)],
    [`${HOME}/Downloads/archive/report-final-v6.pdf`, size(3, 9)],
  ],
  'ios-backups': [[`${HOME}/Library/Application Support/MobileSync/Backup/00008030-001A`, size(4000, 9000)]],
  'mail-attachments': [[`${HOME}/Library/Mail/V10/MailData/Attachments`, size(300, 1200)]],
  duplicates: [
    [`${HOME}/Documents/Scans/passport.jpg`, size(2, 4)],
    [`${HOME}/Desktop/passport.jpg`, size(2, 4)],
  ],
  'browser-cache': [
    [`${HOME}/Library/Caches/Google/Chrome/Default/Cache`, size(300, 800)],
    [`${HOME}/Library/Containers/com.apple.Safari/Data/Library/Caches`, size(100, 300)],
  ],
  'dev-cache': [
    [`${HOME}/Library/Developer/Xcode/DerivedData`, size(2000, 6000)],
    [`${HOME}/.npm/_cacache`, size(400, 900)],
    [`${HOME}/Library/Caches/go-build`, size(500, 1500)],
  ],
  homebrew: [[`${HOME}/Library/Caches/Homebrew/downloads`, size(200, 700)]],
  docker: [['Docker build cache and dangling images', size(1500, 4000)]],
  'node-modules': [
    [`${HOME}/Dev/old/landing-page/node_modules`, size(200, 500)],
    [`${HOME}/Dev/old/bot/node_modules`, size(150, 400)],
  ],
  'large-files': [
    [`${HOME}/Movies/Family trip.mov`, size(2000, 5000)],
    [`${HOME}/Downloads/Ubuntu-24.04.iso`, size(4000, 6000)],
    [`${HOME}/Documents/Design/brand-assets.sketch`, size(600, 900)],
  ],
}

function itemsFor(id: string): CleanableItem[] {
  return (FILES[id] ?? []).map(([path, bytes]) => ({
    path,
    size: Math.round(bytes),
    name: path.split('/').pop() ?? path,
    isDirectory: !/\.[a-z0-9]{2,4}$/i.test(path),
  }))
}

function resultFor(c: Category, empty: boolean): ScanResult {
  const items = empty ? [] : itemsFor(c.id)
  return { category: c, items, totalSize: items.reduce((n, i) => n + i.size, 0) }
}

const APPS: AppInfo[] = [
  ['Slack', 'com.tinyspeck.slackmacgap', 420, [['Library/Application Support/Slack', 310], ['Library/Caches/com.tinyspeck.slackmacgap', 180], ['Library/Preferences/com.tinyspeck.slackmacgap.plist', 0.02]], false],
  ['Figma', 'com.figma.Desktop', 380, [['Library/Application Support/Figma', 95], ['Library/Caches/com.figma.Desktop', 140]], true],
  ['Docker', 'com.docker.docker', 1900, [['Library/Containers/com.docker.docker', 9200], ['Library/Group Containers/group.com.docker', 20]], false],
  ['Zoom', 'us.zoom.xos', 160, [['Library/Application Support/zoom.us', 60], ['Library/Logs/zoom.us', 12]], false],
  ['Spotify', 'com.spotify.client', 290, [['Library/Application Support/Spotify', 1100], ['Library/Caches/com.spotify.client', 640]], false],
  ['Notion', 'notion.id', 310, [['Library/Application Support/Notion', 85]], false],
  ['Visual Studio Code', 'com.microsoft.VSCode', 540, [['Library/Application Support/Code', 720], ['.vscode', 1500]], false],
  ['Telegram', 'ru.keepcoder.Telegram', 210, [['Library/Group Containers/6N38VWS5BX.ru.keepcoder.Telegram', 2300]], false],
  ['Arc', 'company.thebrowser.Browser', 480, [['Library/Application Support/Arc', 410]], false],
  ['Raycast', 'com.raycast.macos', 180, [['Library/Application Support/com.raycast.macos', 40]], false],
  ['OBS', 'com.obsproject.obs-studio', 430, [], false],
].map(([name, bundleId, appMB, rel, running]) => {
  const relatedPaths = (rel as Array<[string, number]>).map(([p, mb]) => ({ path: `${HOME}/${p}`, size: Math.round(mb * MB) }))
  const appSize = Math.round((appMB as number) * MB)
  return {
    name: name as string,
    path: `/Applications/${name}.app`,
    bundleId: bundleId as string,
    appSize,
    relatedPaths,
    totalSize: appSize + relatedPaths.reduce((n, r) => n + r.size, 0),
    running: running as boolean,
  }
})

const BACKUPS: BackupInfo[] = [
  { path: `${HOME}/Library/Application Support/AppCleaner/backups/2026-10-01T18-02`, date: '2026-10-01T18:02:11Z', size: size(300, 900) },
  { path: `${HOME}/Library/Application Support/AppCleaner/backups/2026-09-28T09-41`, date: '2026-09-28T09:41:50Z', size: size(40, 120) },
  { path: `${HOME}/Library/Application Support/AppCleaner/backups/2026-09-20T22-15`, date: '2026-09-20T22:15:03Z', size: size(1500, 3000) },
]

const LOGIN_ITEMS: LoginItem[] = [
  { label: 'com.spotify.client.startuphelper', path: `${HOME}/Library/LaunchAgents/com.spotify.client.startuphelper.plist`, program: '/Applications/Spotify.app/Contents/MacOS/Spotify', kind: 'user-agent', runAtLoad: true, programMissing: false },
  { label: 'com.adobe.ARMDCHelper', path: `${HOME}/Library/LaunchAgents/com.adobe.ARMDCHelper.plist`, program: '/Library/Application Support/Adobe/ARMDC/Application/Acrobat Update Helper.app', kind: 'user-agent', runAtLoad: true, programMissing: true },
  { label: 'com.docker.vmnetd', path: '/Library/LaunchDaemons/com.docker.vmnetd.plist', program: '/Library/PrivilegedHelperTools/com.docker.vmnetd', kind: 'daemon', runAtLoad: true, programMissing: false },
  { label: 'com.google.keystone.agent', path: '/Library/LaunchAgents/com.google.keystone.agent.plist', program: '/Library/Google/GoogleSoftwareUpdate/GoogleSoftwareUpdate.bundle/Contents/MacOS/GoogleSoftwareUpdateAgent', kind: 'global-agent', runAtLoad: false, programMissing: false },
]

function lensTree(): SpaceLensNode {
  const dir = (name: string, path: string, children: SpaceLensNode[]): SpaceLensNode => ({
    name,
    path,
    isDir: true,
    size: children.reduce((n, c) => n + c.size, 0),
    children: [...children].sort((a, b) => b.size - a.size),
  })
  const file = (name: string, path: string, mb: number): SpaceLensNode => ({ name, path, isDir: false, size: Math.round(mb * MB) })
  return dir('guilherme', HOME, [
    dir('Library', `${HOME}/Library`, [
      dir('Developer', `${HOME}/Library/Developer`, [file('DerivedData', `${HOME}/Library/Developer/Xcode/DerivedData`, 4800)]),
      dir('Caches', `${HOME}/Library/Caches`, [file('com.spotify.client', `${HOME}/Library/Caches/com.spotify.client`, 640)]),
      dir('Application Support', `${HOME}/Library/Application Support`, [file('Spotify', `${HOME}/Library/Application Support/Spotify`, 1100)]),
    ]),
    dir('Movies', `${HOME}/Movies`, [file('Family trip.mov', `${HOME}/Movies/Family trip.mov`, 3900), file('Screen Recording.mov', `${HOME}/Movies/Screen Recording.mov`, 820)]),
    dir('Dev', `${HOME}/Dev`, [dir('pessoal', `${HOME}/Dev/pessoal`, [file('app-cleaner', `${HOME}/Dev/pessoal/app-cleaner`, 1300), file('tiles-spliter', `${HOME}/Dev/pessoal/tiles-spliter`, 400)])]),
    dir('Downloads', `${HOME}/Downloads`, [file('Ubuntu-24.04.iso', `${HOME}/Downloads/Ubuntu-24.04.iso`, 5100), file('Docker.dmg', `${HOME}/Downloads/Docker.dmg`, 610)]),
    dir('Pictures', `${HOME}/Pictures`, [file('Photos Library.photoslibrary', `${HOME}/Pictures/Photos Library.photoslibrary`, 24000)]),
    dir('Documents', `${HOME}/Documents`, [file('Design', `${HOME}/Documents/Design`, 900)]),
    file('.zsh_history', `${HOME}/.zsh_history`, 0.3),
  ])
}

export function installMock(params: URLSearchParams) {
  const empty = params.get('empty') === '1'
  const fda = params.get('fda') !== '0'
  const offerUpdate = params.get('update') === '1'
  let config: Config = {
    downloadsDaysOld: 30,
    largeFilesMinSize: 500 * MB,
    backupByDefault: true,
    backupRetentionDays: 7,
    concurrency: 4,
    showRisky: false,
    keepLanguages: ['pt-BR'],
    extraPaths: { nodeModules: [`${HOME}/Dev`], projects: [] },
  }
  let backups = [...BACKUPS]
  let apps = [...APPS]
  let scanCancelled = false
  let cleanCancelled = false

  const runtime = {
    EventsOnMultiple(eventName: string, callback: Listener) {
      const arr = listeners.get(eventName) ?? []
      arr.push(callback)
      listeners.set(eventName, arr)
      return () => listeners.set(eventName, (listeners.get(eventName) ?? []).filter((l) => l !== callback))
    },
    EventsOff(...names: string[]) {
      for (const n of names) listeners.delete(n)
    },
    EventsEmit: emit,
    LogDebug() {},
    LogInfo() {},
    LogWarning() {},
    LogError() {},
    BrowserOpenURL(url: string) {
      window.open(url, '_blank')
    },
  }

  const App = {
    CheckFDA: async () => fda,
    OpenFDASettings: async () => {},
    GetHome: async () => HOME,
    GetVersion: async () => '1.6.0',
    GetConfig: async () => config,
    SaveConfig: async (c: Config) => {
      config = c
    },
    CheckForUpdate: async (): Promise<UpdateInfo> => ({
      current: '1.6.0',
      latest: offerUpdate ? '1.7.0' : '1.6.0',
      available: offerUpdate,
      url: 'https://github.com/GuilhermeVozniak/app-cleaner/releases',
    }),
    DownloadUpdate: async () => {
      for (let i = 1; i <= 10; i++) {
        await sleep(180)
        emit('update:progress', { done: i * 4 * MB, total: 40 * MB })
      }
      return '/tmp/App Cleaner.zip'
    },
    InstallUpdate: async () => {
      await sleep(800)
    },
    OpenDownloadedUpdate: async () => {},
    OpenReleasePage: async () => {},
    GetDiskUsage: async (): Promise<DiskUsage> => {
      const total = 494 * 1024 * MB
      const used = 407 * 1024 * MB
      return { total, used, free: total - used }
    },
    GetActivityStats: async (): Promise<ActivityStats> => ({
      totalCleanedBytes: 7.37 * 1024 * MB,
      totalCleanedItems: 1203,
      cleanRuns: 4,
      scanRuns: 12,
      appsUninstalled: 3,
      lastCleanAt: new Date(Date.now() - 2 * 24 * 3600 * 1000).toISOString(),
    }),
    GetCategories: async () => CATEGORIES,
    StartScan: async (ids: string[]) => {
      scanCancelled = false
      const cats = ids.length ? CATEGORIES.filter((c) => ids.includes(c.id)) : CATEGORIES
      const results: ScanResult[] = []
      void (async () => {
        for (let i = 0; i < cats.length; i++) {
          await sleep(140 + rnd() * 160)
          if (scanCancelled) break
          const r = resultFor(cats[i], empty)
          results.push(r)
          emit('scan:progress', {
            completed: i + 1,
            total: cats.length,
            categoryId: cats[i].id,
            totalSize: r.totalSize,
            itemCount: r.items.length,
          })
        }
        emit('scan:done', {
          summary: {
            results,
            totalSize: results.reduce((n, r) => n + r.totalSize, 0),
            totalItems: results.reduce((n, r) => n + r.items.length, 0),
          },
          cancelled: scanCancelled,
        })
      })()
    },
    CancelScan: async () => {
      scanCancelled = true
    },
    GetScanResult: async (id: string) => resultFor(CATEGORIES.find((c) => c.id === id)!, empty),
    GroupItems: async (id: string, expand: Record<string, number>): Promise<DisplayRow[]> => {
      const items = itemsFor(id)
      const byDir = new Map<string, CleanableItem[]>()
      for (const it of items) {
        const dir = it.path.slice(0, it.path.lastIndexOf('/'))
        byDir.set(dir, [...(byDir.get(dir) ?? []), it])
      }
      const rows: DisplayRow[] = []
      for (const [dir, files] of byDir) {
        const limit = expand[dir] ?? 5
        rows.push({ type: 'directory-header', directoryKey: dir, displayName: dir.replace(HOME, '~'), totalFilesInDir: files.length, selectable: true })
        for (const f of files.slice(0, limit)) {
          rows.push({ type: 'file', directoryKey: dir, displayName: f.name, path: f.path, size: f.size, name: f.name, totalFilesInDir: files.length, selectable: true })
        }
        if (files.length > limit) {
          rows.push({ type: 'expand-hint', directoryKey: dir, displayName: '', hiddenCount: files.length - limit, totalFilesInDir: files.length, selectable: false })
        }
      }
      return rows
    },
    StartClean: async (selection: Record<string, string[]>, opts: { dryRun: boolean; backup: boolean }) => {
      cleanCancelled = false
      const entries = Object.entries(selection)
      const total = entries.reduce((n, [, p]) => n + p.length, 0)
      void (async () => {
        let i = 0
        if (opts.backup) {
          for (const [, paths] of entries) {
            for (const p of paths) {
              i++
              await sleep(90)
              emit('backup:progress', { current: i, total, itemName: p.split('/').pop() })
            }
          }
        }
        i = 0
        const results = []
        for (const [id, paths] of entries) {
          const cat = CATEGORIES.find((c) => c.id === id)!
          const all = itemsFor(id)
          let freed = 0
          for (const p of paths) {
            i++
            await sleep(110)
            if (cleanCancelled) break
            emit('clean:progress', { current: i, total, categoryId: id, itemName: p.split('/').pop() })
            freed += all.find((it) => it.path === p)?.size ?? 0
          }
          results.push({ category: cat, cleanedItems: paths.length, freedSpace: freed, errors: [] })
          if (cleanCancelled) break
        }
        emit('clean:done', {
          summary: {
            results,
            totalFreedSpace: results.reduce((n, r) => n + r.freedSpace, 0),
            totalCleanedItems: results.reduce((n, r) => n + r.cleanedItems, 0),
            totalErrors: 0,
          },
          notBackedUp: [],
          cancelled: cleanCancelled,
        })
      })()
    },
    CancelClean: async () => {
      cleanCancelled = true
    },
    ListApps: async () => {
      await sleep(600)
      return apps
    },
    GetAppIcon: async () => '',
    IsAppRunning: async (path: string) => apps.find((a) => a.path === path)?.running ?? false,
    StartUninstall: async (paths: string[], dryRun: boolean) => {
      void (async () => {
        let freed = 0
        for (let i = 0; i < paths.length; i++) {
          const app = apps.find((a) => a.path === paths[i])
          await sleep(700)
          emit('uninstall:progress', { current: i + 1, total: paths.length, appName: app?.name ?? paths[i] })
          freed += app?.totalSize ?? 0
        }
        if (!dryRun) apps = apps.filter((a) => !paths.includes(a.path))
        emit('uninstall:done', { uninstalled: paths.length, freedSpace: freed, errors: [] })
      })()
    },
    RunMaintenance: async (task: string): Promise<MaintenanceResult> => {
      await sleep(1200)
      const msg: Record<string, string> = {
        dns: 'DNS cache flushed',
        purge: 'Purgeable space released',
      }
      return { success: true, message: msg[task] ?? 'Done', requiresAdmin: task === 'dns' }
    },
    StartTMSnapshotsClear: async () => {
      void (async () => {
        const dates = ['2026-09-30-091500', '2026-09-30-101500', '2026-09-30-111500']
        for (let i = 0; i < dates.length; i++) {
          await sleep(500)
          emit('maintenance:progress', { done: i + 1, total: dates.length, date: dates[i] })
        }
        emit('maintenance:done', { result: { success: true, message: '3 snapshots removed', requiresAdmin: true } })
      })()
    },
    CancelMaintenance: async () => {},
    ListBackups: async () => backups,
    GetBackupDetails: async (path: string): Promise<BackupDetails> => ({
      items: itemsFor('system-cache').map((it) => ({ path: it.path, name: it.name, size: it.size })),
      fromManifest: true,
      truncated: path.endsWith('22-15') ? 120 : 0,
    }),
    RestoreBackup: async () => {
      await sleep(900)
      return { restored: 5, failed: 0, errors: [] }
    },
    DeleteBackup: async (path: string) => {
      await sleep(400)
      backups = backups.filter((b) => b.path !== path)
    },
    CleanOldBackups: async () => 0,
    ListLoginItems: async () => {
      await sleep(300)
      return LOGIN_ITEMS
    },
    BuildSpaceLens: async () => {
      await sleep(1400)
      return lensTree()
    },
    RevealInFinder: async (p: string) => console.info('[mock] reveal', p),
    CopyPath: async (p: string) => console.info('[mock] copy', p),
  }

  const w = window as unknown as Record<string, unknown>
  w.runtime = runtime
  w.go = { main: { App } }
  console.info('[mock] Wails bridge installed', { fda, empty, offerUpdate })
}
