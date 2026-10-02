import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

// App.tsx eagerly imports every view (SmartScan, CategoryDetail, Uninstaller,
// Maintenance, Backups, Settings), which transitively load lib/paths.ts's own
// GetHome import from this module regardless of which view actually renders
// -- so this must be a partial mock (importOriginal), unlike single-view test
// files that only need to stub the handful of bindings they call directly.
vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../wailsjs/go/main/App')>()
  return { ...actual, CheckFDA: vi.fn() }
})

import { CheckFDA } from '../wailsjs/go/main/App'
import App from './App'
import { useUiStore } from './stores/uiStore'
import { useScanStore } from './stores/scanStore'

const CheckFDAMock = CheckFDA as unknown as ReturnType<typeof vi.fn>

beforeEach(() => {
  useUiStore.setState({ view: 'smart-scan', activeCategoryId: undefined, fda: null, config: undefined })
  useScanStore.getState().reset()
  CheckFDAMock.mockReset()
})

describe('<App />', () => {
  it('renders the main layout (sidebar + smart-scan view) on first paint, before FDA resolves', () => {
    CheckFDAMock.mockResolvedValue(true) // resolves after this synchronous assertion
    render(<App />)
    expect(screen.getByRole('complementary')).toBeInTheDocument() // Sidebar <aside>
  })

  it('routes to FirstRun once CheckFDA resolves false (denied)', async () => {
    CheckFDAMock.mockResolvedValue(false)
    render(<App />)
    expect(await screen.findByText('Grant Full Disk Access')).toBeInTheDocument()
    expect(screen.queryByRole('complementary')).toBeNull() // Sidebar not rendered on first-run
  })

  it('routes to FirstRun once CheckFDA resolves null (unknown)', async () => {
    CheckFDAMock.mockResolvedValue(null)
    render(<App />)
    expect(await screen.findByText('Grant Full Disk Access')).toBeInTheDocument()
  })

  it('stays off FirstRun once CheckFDA resolves true (granted)', async () => {
    CheckFDAMock.mockResolvedValue(true)
    render(<App />)
    await vi.waitFor(() => expect(screen.getByRole('complementary')).toBeInTheDocument())
    await vi.waitFor(() => expect(CheckFDAMock).toHaveBeenCalled())
    expect(useUiStore.getState().view).toBe('smart-scan')
  })

  it('switches the main pane to Maintenance for the maintenance view', () => {
    CheckFDAMock.mockResolvedValue(true)
    useUiStore.setState({ view: 'maintenance' })
    render(<App />)
    expect(screen.getByRole('complementary')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Keep your Mac in top shape' })).toBeInTheDocument()
  })

  it('switches the main pane to Settings for the settings view', () => {
    CheckFDAMock.mockResolvedValue(true)
    useUiStore.setState({ view: 'settings' })
    render(<App />)
    expect(screen.getByRole('complementary')).toBeInTheDocument()
    expect(screen.getByText('Loading settings…')).toBeInTheDocument()
  })

  it('switches the main pane to Backups for the backups view', () => {
    CheckFDAMock.mockResolvedValue(true)
    useUiStore.setState({ view: 'backups' })
    render(<App />)
    expect(screen.getByRole('complementary')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Backups' })).toBeInTheDocument()
  })
})
