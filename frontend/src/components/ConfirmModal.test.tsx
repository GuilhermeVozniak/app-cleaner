// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  GetCategories: vi.fn().mockResolvedValue([]),
  StartScan: vi.fn().mockResolvedValue(undefined),
  CancelScan: vi.fn(),
  GetScanResult: vi.fn(),
  GroupItems: vi.fn().mockResolvedValue([]),
  StartClean: vi.fn().mockResolvedValue(undefined),
  CancelClean: vi.fn(),
  GetConfig: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  CheckFDA: vi.fn().mockResolvedValue(null),
  OpenFDASettings: vi.fn(),
  RevealInFinder: vi.fn(),
  CopyPath: vi.fn(),
}))

import { StartClean } from '../../wailsjs/go/main/App'
import {
  ConfirmModal,
  computeSelectionStats,
  selectionAsPaths,
  defaultBackupEnabled,
} from './ConfirmModal'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { useCleanStore } from '../stores/cleanStore'
import type { Category, Config, ScanResult } from '../lib/types'

const StartCleanMock = StartClean as unknown as ReturnType<typeof vi.fn>

function cat(id: string, name: string, safety: Category['safetyLevel'], note = ''): Category {
  return {
    id,
    name,
    group: 'System Junk',
    description: '',
    safetyLevel: safety,
    ...(note ? { safetyNote: note } : {}),
  }
}

function result(c: Category, items: Array<{ path: string; size: number }>): ScanResult {
  return {
    category: c,
    items: items.map((it) => ({
      path: it.path,
      size: it.size,
      name: it.path.split('/').pop() ?? it.path,
      isDirectory: false,
    })),
    totalSize: items.reduce((a, b) => a + b.size, 0),
  }
}

const baseConfig: Config = {
  downloadsDaysOld: 30,
  largeFilesMinSize: 524288000,
  backupByDefault: true,
  backupRetentionDays: 7,
  concurrency: 4,
  showRisky: false,
  keepLanguages: [],
  extraPaths: { nodeModules: [], projects: [] },
}

const tempFiles = result(cat('temp-files', 'Temporary Files', 'safe'), [
  { path: '/tmp/a.log', size: 1024 },
  { path: '/tmp/b.log', size: 2048 },
])
const sysCache = result(cat('system-cache', 'User Cache Files', 'moderate'), [
  { path: '/Users/me/Library/Caches/x', size: 4096 },
])
const downloads = result(
  cat('downloads', 'Old Downloads', 'risky', 'May contain important files you forgot about'),
  [
    { path: '/Users/me/Downloads/old1.zip', size: 100 },
    { path: '/Users/me/Downloads/old2.zip', size: 200 },
  ],
)

beforeEach(() => {
  useCleanStore.getState().reset()
  useUiStore.setState({ config: baseConfig })
})

afterEach(() => {
  cleanup()
  StartCleanMock.mockClear()
})

describe('selection helpers', () => {
  it('computeSelectionStats counts items/sizes across \'all\' and Set selections', () => {
    const stats = computeSelectionStats(
      { 'temp-files': tempFiles, downloads },
      { 'temp-files': 'all', downloads: new Set(['/Users/me/Downloads/old2.zip']) },
    )
    expect(stats.itemCount).toBe(3)
    expect(stats.totalSize).toBe(1024 + 2048 + 200)
    expect(stats.categories.map((c) => c.id).sort()).toEqual(['downloads', 'temp-files'])
  })

  it('selectionAsPaths expands \'all\' and filters Sets, dropping empty categories', () => {
    expect(
      selectionAsPaths(
        { 'temp-files': tempFiles, downloads },
        { 'temp-files': 'all', downloads: new Set<string>() },
      ),
    ).toEqual({ 'temp-files': ['/tmp/a.log', '/tmp/b.log'] })
  })

  it('defaultBackupEnabled: off for safe-only, on for mixed, off when config disables it', () => {
    const safe = cat('temp-files', 'Temporary Files', 'safe')
    const moderate = cat('system-cache', 'User Cache Files', 'moderate')
    expect(defaultBackupEnabled(true, [safe])).toBe(false)
    expect(defaultBackupEnabled(true, [safe, moderate])).toBe(true)
    expect(defaultBackupEnabled(false, [safe, moderate])).toBe(false)
  })
})

describe('<ConfirmModal />', () => {
  it('backup toggle defaults OFF for a safe-only selection', () => {
    useScanStore.setState({ results: { 'temp-files': tempFiles }, selected: { 'temp-files': 'all' } })
    render(<ConfirmModal />)
    const toggle = screen.getByLabelText(/back up items before deleting/i) as HTMLInputElement
    expect(toggle.checked).toBe(false)
  })

  it('backup toggle defaults ON for a mixed selection and shows the risky safety note', () => {
    useScanStore.setState({
      results: { 'temp-files': tempFiles, downloads },
      selected: { 'temp-files': 'all', downloads: 'all' },
    })
    render(<ConfirmModal />)
    const toggle = screen.getByLabelText(/back up items before deleting/i) as HTMLInputElement
    expect(toggle.checked).toBe(true)
    expect(screen.getByText('May contain important files you forgot about')).toBeDefined()
  })

  it('shows item count + total size and starts the clean with the selection as paths', () => {
    useScanStore.setState({
      results: { 'system-cache': sysCache },
      selected: { 'system-cache': 'all' },
    })
    render(<ConfirmModal />)
    expect(screen.getByText(/1 item/)).toBeDefined()
    expect(screen.getByText(/4\.0 KB/)).toBeDefined()
    fireEvent.click(screen.getByLabelText(/dry run/i))
    fireEvent.click(screen.getByRole('button', { name: /^clean$/i }))
    expect(StartCleanMock).toHaveBeenCalledWith(
      { 'system-cache': ['/Users/me/Library/Caches/x'] },
      { dryRun: true, backup: true }, // moderate selection + backupByDefault:true
    )
  })

  it('Cancel resets the clean flow to idle', () => {
    useScanStore.setState({ results: { 'temp-files': tempFiles }, selected: { 'temp-files': 'all' } })
    useCleanStore.getState().openConfirm()
    render(<ConfirmModal />)
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(useCleanStore.getState().status).toBe('idle')
  })
})
