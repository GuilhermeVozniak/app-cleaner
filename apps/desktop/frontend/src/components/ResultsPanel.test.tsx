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

import { OpenFDASettings } from '../../wailsjs/go/main/App'
import { ResultsPanel, needsFdaHint } from './ResultsPanel'
import { useCleanStore } from '../stores/cleanStore'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import type { Category, CleanSummary } from '../lib/types'

const OpenFDAMock = OpenFDASettings as unknown as ReturnType<typeof vi.fn>

function cat(id: string, name: string): Category {
  return { id, name, group: 'System Junk', description: '', safetyLevel: 'safe' }
}

const summaryWithEperm: CleanSummary = {
  results: [
    { category: cat('trash', 'Trash'), cleanedItems: 12, freedSpace: 1073741824, errors: [] },
    {
      category: cat('temp-files', 'Temporary Files'),
      cleanedItems: 5,
      freedSpace: 536870912,
      errors: ['Failed to remove 3 items (3 EPERM)'],
    },
  ],
  totalFreedSpace: 1610612736,
  totalCleanedItems: 17,
  totalErrors: 1,
}

beforeEach(() => {
  useCleanStore.getState().reset()
})

afterEach(() => {
  cleanup()
  OpenFDAMock.mockClear()
})

describe('needsFdaHint', () => {
  it('is true when any error string contains EPERM or EACCES, false otherwise', () => {
    expect(needsFdaHint(summaryWithEperm)).toBe(true)
    expect(
      needsFdaHint({
        results: [{ category: cat('trash', 'Trash'), cleanedItems: 1, freedSpace: 1, errors: ['Failed to remove 1 items (1 ENOENT)'] }],
        totalFreedSpace: 1,
        totalCleanedItems: 1,
        totalErrors: 1,
      }),
    ).toBe(false)
    expect(needsFdaHint(undefined)).toBe(false)
  })
})

describe('<ResultsPanel />', () => {
  it('renders the freed-space headline, per-category rows and verbatim errno breakdown', () => {
    useCleanStore.setState({
      status: 'done',
      summary: summaryWithEperm,
      notBackedUp: ['/Volumes/Ext/big.mkv'],
      cancelled: false,
    })
    render(<ResultsPanel />)
    expect(screen.getByText(/1\.5 GB freed/)).toBeDefined()
    expect(screen.getByText(/✓ Trash/)).toBeDefined()
    expect(screen.getByText(/✗ Temporary Files/)).toBeDefined()
    expect(screen.getByText('Failed to remove 3 items (3 EPERM)')).toBeDefined()
    expect(screen.queryByText('Some items are blocked for safety (system-protected paths)')).toBeNull()
    expect(screen.getByText(/1 item\(s\) not backed up \(outside home \/ other volume\)/)).toBeDefined()
  })

  it('shows the FDA call-to-action on EPERM errors and opens System Settings', () => {
    useCleanStore.setState({ status: 'done', summary: summaryWithEperm, notBackedUp: [] })
    render(<ResultsPanel />)
    const btn = screen.getByRole('button', { name: /grant full disk access/i })
    fireEvent.click(btn)
    expect(OpenFDAMock).toHaveBeenCalledTimes(1)
  })

  it('hides the FDA call-to-action when there are no permission errors', () => {
    useCleanStore.setState({
      status: 'done',
      summary: { results: [], totalFreedSpace: 0, totalCleanedItems: 0, totalErrors: 0 },
      notBackedUp: [],
    })
    render(<ResultsPanel />)
    expect(screen.queryByRole('button', { name: /grant full disk access/i })).toBeNull()
  })

  it('does not crash on null errors (nil Go slice from docker/homebrew/backed-up categories)', () => {
    // The Wails bridge serialises a nil []string as "errors":null; the panel
    // must normalise it instead of crashing on r.errors.length (white screen).
    const summary = {
      results: [
        { category: cat('homebrew', 'Homebrew'), cleanedItems: 3, freedSpace: 1024, errors: null },
      ],
      totalFreedSpace: 1024,
      totalCleanedItems: 3,
      totalErrors: 0,
    } as unknown as CleanSummary
    useCleanStore.setState({ status: 'done', summary, notBackedUp: [] })
    render(<ResultsPanel />)
    expect(screen.getByText(/✓ Homebrew/)).toBeDefined()
  })

  it('needsFdaHint tolerates null results (panic-path zero-value summary)', () => {
    expect(needsFdaHint({ results: null } as unknown as CleanSummary)).toBe(false)
  })

  it('renders a muted safety note under errors that contain PROTECTED', () => {
    useCleanStore.setState({
      status: 'done',
      summary: {
        results: [
          {
            category: cat('system-cache', 'User Cache Files'),
            cleanedItems: 2,
            freedSpace: 2048,
            errors: ['Failed to remove 4 items (4 PROTECTED)'],
          },
        ],
        totalFreedSpace: 2048,
        totalCleanedItems: 2,
        totalErrors: 1,
      },
      notBackedUp: [],
    })
    render(<ResultsPanel />)
    expect(screen.getByText('Failed to remove 4 items (4 PROTECTED)')).toBeDefined()
    expect(
      screen.getByText('Some items are blocked for safety (system-protected paths)'),
    ).toBeDefined()
  })

  it('Done resets both stores and returns to the Smart Scan hero', () => {
    useCleanStore.setState({ status: 'done', summary: summaryWithEperm, notBackedUp: [] })
    useUiStore.setState({ view: 'category' })
    render(<ResultsPanel />)
    fireEvent.click(screen.getByRole('button', { name: /^done$/i }))
    expect(useCleanStore.getState().status).toBe('idle')
    expect(useScanStore.getState().status).toBe('idle')
    expect(useUiStore.getState().view).toBe('smart-scan')
  })
})
