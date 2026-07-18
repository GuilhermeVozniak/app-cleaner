// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  ListApps: vi.fn().mockResolvedValue([]),
  StartUninstall: vi.fn().mockResolvedValue(undefined),
  IsAppRunning: vi.fn().mockResolvedValue(false),
  GetAppIcon: vi.fn().mockResolvedValue(''),
}))

import { ListApps, StartUninstall, GetAppIcon } from '../../wailsjs/go/main/App'
import { Uninstaller } from './Uninstaller'
import { useUninstallerStore } from '../stores/uninstallerStore'
import type { AppInfo } from '../lib/types'

const ListAppsMock = ListApps as unknown as ReturnType<typeof vi.fn>
const StartUninstallMock = StartUninstall as unknown as ReturnType<typeof vi.fn>
const GetAppIconMock = GetAppIcon as unknown as ReturnType<typeof vi.fn>

const apps: AppInfo[] = [
  {
    name: 'OldApp',
    path: '/Applications/OldApp.app',
    bundleId: 'com.old.app',
    appSize: 100_000_000,
    relatedPaths: [{ path: '/Users/me/Library/Caches/OldApp', size: 50_000_000 }],
    totalSize: 150_000_000,
    running: false,
  },
  {
    name: 'BusyApp',
    path: '/Applications/BusyApp.app',
    bundleId: 'com.busy.app',
    appSize: 200_000_000,
    relatedPaths: [],
    totalSize: 200_000_000,
    running: true,
  },
]

beforeEach(() => {
  useUninstallerStore.setState({
    apps: [], lastScanAt: null, scanning: false, selected: new Set<string>(),
    icons: {}, phase: 'list', pending: null, lastRequested: [], lastDryRun: false,
    skippedApps: [], progress: { current: 0, total: 0, appName: '' }, done: null, startError: null,
  })
  localStorage.clear()
  ListAppsMock.mockClear()
  ListAppsMock.mockResolvedValue(apps)
  StartUninstallMock.mockClear()
  StartUninstallMock.mockResolvedValue(undefined)
  GetAppIconMock.mockReset()
  GetAppIconMock.mockResolvedValue('')
})

afterEach(() => cleanup())

describe('<Uninstaller />', () => {
  it('does not crash when the bridge returns null relatedPaths (nil Go slice)', async () => {
    // A Go nil RelatedPaths slice serialises to JSON null; the raw Wails call
    // returns it unconverted. The view must normalise it to [] rather than
    // crash on app.relatedPaths.length/.map/.reduce.
    ListAppsMock.mockResolvedValue([
      {
        name: 'NakedApp',
        path: '/Applications/NakedApp.app',
        bundleId: 'com.naked.app',
        appSize: 1000,
        relatedPaths: null as unknown as AppInfo['relatedPaths'],
        totalSize: 1000,
        running: false,
      },
    ])
    render(<Uninstaller />)
    expect(await screen.findByText('NakedApp')).toBeDefined()
    expect(screen.queryByText(/related/)).toBeNull() // no "+N related" chip, no crash
  })

  it('lists apps with size, related chip and Running badge', async () => {
    render(<Uninstaller />)
    expect(await screen.findByText('OldApp')).toBeDefined()
    expect(screen.getByText('143.1 MB')).toBeDefined() // formatSize(150_000_000)
    expect(screen.getByText('+1 related')).toBeDefined()
    expect(screen.getByText('Running')).toBeDefined() // BusyApp badge
  })

  it('loads row icons lazily: PNG from GetAppIcon, letter-avatar fallback when empty', async () => {
    GetAppIconMock.mockResolvedValueOnce('iVBORfakePNG') // first row to mount = OldApp
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    const img = (await screen.findByAltText('OldApp icon')) as HTMLImageElement
    expect(img.src).toBe('data:image/png;base64,iVBORfakePNG')
    expect(GetAppIconMock).toHaveBeenCalledWith('/Applications/OldApp.app')
    expect(GetAppIconMock).toHaveBeenCalledWith('/Applications/BusyApp.app')
    expect(screen.getByText('B')).toBeDefined() // BusyApp returned '' → letter avatar
  })

  it('disables Uninstall with a "Quit the app first" tooltip while a selected app is running', async () => {
    render(<Uninstaller />)
    await screen.findByText('BusyApp')
    fireEvent.click(screen.getByLabelText('Select BusyApp (/Applications/BusyApp.app)'))
    const btn = screen.getByRole('button', { name: /uninstall 1 app/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
    expect(btn.title).toBe('Quit the app first')
  })

  it('enables Uninstall for a non-running selection and confirms into StartUninstall', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    fireEvent.click(screen.getByLabelText('Select OldApp (/Applications/OldApp.app)'))
    const btn = screen.getByRole('button', { name: /uninstall 1 app/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(false)
    fireEvent.click(btn)
    // bespoke confirm modal: apps + related paths + total
    expect(await screen.findByText(/~\/Library\/Caches\/OldApp/)).toBeDefined()
    expect(screen.getByText(/143\.1 MB will be freed \(2 items\)/)).toBeDefined()
    fireEvent.click(screen.getByLabelText(/dry run/i))
    fireEvent.click(screen.getByRole('button', { name: /^uninstall$/i }))
    expect(StartUninstallMock).toHaveBeenCalledWith(['/Applications/OldApp.app'], true)
  })

  it('Re-check calls ListApps again', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    expect(ListAppsMock).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: /re-check/i }))
    expect(ListAppsMock).toHaveBeenCalledTimes(2)
  })

  it('keys selection by bundle path: two same-named apps at different paths get independent checkboxes, correct totals, and both get uninstalled', async () => {
    const duplicateNameApps: AppInfo[] = [
      {
        name: 'OldApp',
        path: '/Applications/OldApp.app',
        bundleId: 'com.old.app',
        appSize: 100_000_000,
        relatedPaths: [],
        totalSize: 100_000_000,
        running: false,
      },
      {
        name: 'OldApp',
        path: '/Users/me/Applications/OldApp.app',
        bundleId: 'com.old.app',
        appSize: 50_000_000,
        relatedPaths: [],
        totalSize: 50_000_000,
        running: false,
      },
    ]
    ListAppsMock.mockResolvedValueOnce(duplicateNameApps)
    render(<Uninstaller />)
    expect(await screen.findAllByText('OldApp')).toHaveLength(2)

    const checkboxes = screen.getAllByLabelText(/^Select OldApp/)
    expect(checkboxes).toHaveLength(2)
    expect(checkboxes[0].getAttribute('aria-label')).toBe('Select OldApp (/Applications/OldApp.app)')
    expect(checkboxes[1].getAttribute('aria-label')).toBe('Select OldApp (/Users/me/Applications/OldApp.app)')

    // Selecting the first path only: checkbox state is independent per path.
    fireEvent.click(checkboxes[0])
    expect(checkboxes[0].getAttribute('aria-checked')).toBe('true')
    expect(checkboxes[1].getAttribute('aria-checked')).toBe('false')
    expect(screen.getByRole('button', { name: /uninstall 1 app/i })).toBeDefined()

    // Selecting the second path too: both are independently checked.
    fireEvent.click(checkboxes[1])
    expect(checkboxes[1].getAttribute('aria-checked')).toBe('true')
    const btn = screen.getByRole('button', { name: /uninstall 2 apps/i }) as HTMLButtonElement
    fireEvent.click(btn)

    // Correct combined totals (100MB + 50MB, no related paths => 2 items).
    expect(await screen.findByText(/143\.1 MB will be freed \(2 items\)/)).toBeDefined()
    fireEvent.click(screen.getByRole('button', { name: /^uninstall$/i }))
    expect(StartUninstallMock).toHaveBeenCalledWith(
      ['/Applications/OldApp.app', '/Users/me/Applications/OldApp.app'],
      false,
    )
  })

  it('renders an empty state (no rows) without crashing when ListApps resolves with no apps', async () => {
    ListAppsMock.mockResolvedValueOnce([])
    render(<Uninstaller />)
    const btn = (await screen.findByRole('button', {
      name: /uninstall 0 apps/i,
    })) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0)
    expect(screen.queryByText('Scanning installed applications…')).toBeNull()
  })

  it('disables the Uninstall button when zero apps are selected', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    const btn = screen.getByRole('button', { name: /uninstall 0 apps/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
    expect(btn.title).toBe('')
  })

  it('closing the confirm modal via Cancel returns to the list phase without calling StartUninstall', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    fireEvent.click(screen.getByLabelText('Select OldApp (/Applications/OldApp.app)'))
    const openBtn = screen.getByRole('button', { name: /uninstall 1 app/i })
    fireEvent.click(openBtn)
    expect(await screen.findByText('Uninstall applications')).toBeDefined()

    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))

    expect(screen.queryByText('Uninstall applications')).toBeNull()
    expect(StartUninstallMock).not.toHaveBeenCalled()
  })

  it('keeps the cached list visible (no blocking state) while a background refresh runs', async () => {
    useUninstallerStore.setState({ apps, lastScanAt: '2026-07-18T00:00:00Z' })
    ListAppsMock.mockReturnValue(new Promise(() => {})) // scan never resolves
    render(<Uninstaller />)
    expect(screen.getByText('OldApp')).toBeDefined() // cached rows render immediately
    expect(screen.queryByText('Scanning installed applications…')).toBeNull()
    expect(await screen.findByText('Refreshing…')).toBeDefined()
  })

  it('shows the blocking scanning state only on a true first run (no cached apps)', async () => {
    ListAppsMock.mockReturnValue(new Promise(() => {}))
    render(<Uninstaller />)
    expect(await screen.findByText('Scanning installed applications…')).toBeDefined()
  })

  it('shows the waiting overlay when an uninstall is confirmed mid-scan', async () => {
    useUninstallerStore.setState({ apps, selected: new Set(['/Applications/OldApp.app']) })
    ListAppsMock.mockReturnValue(new Promise(() => {})) // keep the scan in flight
    render(<Uninstaller />)
    fireEvent.click(screen.getByRole('button', { name: /uninstall 1 app/i }))
    fireEvent.click(await screen.findByRole('button', { name: /^uninstall$/i }))
    expect(await screen.findByText('Waiting for app scan to finish…')).toBeDefined()
    expect(StartUninstallMock).not.toHaveBeenCalled()
  })

  it('lists skipped (already removed) apps in the done dialog', async () => {
    useUninstallerStore.setState({
      phase: 'done',
      done: { uninstalled: 1, freedSpace: 100, errors: [] },
      skippedApps: ['GhostApp'],
    })
    render(<Uninstaller />)
    expect(await screen.findByText(/1 app already removed — skipped: GhostApp/)).toBeDefined()
  })

  it('skips the mount refresh when an uninstall is already waiting or running', () => {
    useUninstallerStore.setState({
      apps,
      phase: 'running',
      progress: { current: 1, total: 2, appName: 'OldApp' },
    })
    render(<Uninstaller />)
    expect(ListAppsMock).not.toHaveBeenCalled()
  })
})
