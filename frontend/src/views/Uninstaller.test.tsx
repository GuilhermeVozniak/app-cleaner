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
  ListAppsMock.mockClear()
  ListAppsMock.mockResolvedValue(apps)
  StartUninstallMock.mockClear()
  GetAppIconMock.mockReset()
  GetAppIconMock.mockResolvedValue('')
})

afterEach(() => cleanup())

describe('<Uninstaller />', () => {
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
    fireEvent.click(screen.getByLabelText('Select BusyApp'))
    const btn = screen.getByRole('button', { name: /uninstall 1 app/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
    expect(btn.title).toBe('Quit the app first')
  })

  it('enables Uninstall for a non-running selection and confirms into StartUninstall', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    fireEvent.click(screen.getByLabelText('Select OldApp'))
    const btn = screen.getByRole('button', { name: /uninstall 1 app/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(false)
    fireEvent.click(btn)
    // bespoke confirm modal: apps + related paths + total
    expect(await screen.findByText(/~\/Library\/Caches\/OldApp/)).toBeDefined()
    expect(screen.getByText(/143\.1 MB will be freed \(2 items\)/)).toBeDefined()
    fireEvent.click(screen.getByLabelText(/dry run/i))
    fireEvent.click(screen.getByRole('button', { name: /^uninstall$/i }))
    expect(StartUninstallMock).toHaveBeenCalledWith(['OldApp'], true)
  })

  it('Re-check calls ListApps again', async () => {
    render(<Uninstaller />)
    await screen.findByText('OldApp')
    expect(ListAppsMock).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: /re-check/i }))
    expect(ListAppsMock).toHaveBeenCalledTimes(2)
  })
})
