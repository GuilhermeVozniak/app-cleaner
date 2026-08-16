import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetDiskUsage: vi.fn(),
  GetActivityStats: vi.fn(),
  GetCategories: vi.fn().mockResolvedValue([]),
  StartScan: vi.fn().mockResolvedValue(undefined),
  CancelScan: vi.fn(),
  GetConfig: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  CheckFDA: vi.fn().mockResolvedValue(null),
}))

import { GetDiskUsage, GetActivityStats, StartScan } from '../../wailsjs/go/main/App'
import Dashboard, { diskHeadline } from './Dashboard'
import { useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import type { ActivityStats, DiskUsage } from '../lib/types'

const GetDiskUsageMock = GetDiskUsage as unknown as ReturnType<typeof vi.fn>
const GetActivityStatsMock = GetActivityStats as unknown as ReturnType<typeof vi.fn>
const StartScanMock = StartScan as unknown as ReturnType<typeof vi.fn>

const GB = 1024 ** 3

function disk(total: number, used: number): DiskUsage {
  return { total, used, free: total - used }
}

const stats: ActivityStats = {
  totalCleanedBytes: 2 * GB,
  totalCleanedItems: 12,
  cleanRuns: 3,
  scanRuns: 7,
  appsUninstalled: 2,
  lastCleanAt: '',
}

beforeEach(() => {
  useScanStore.getState().reset()
  useUiStore.setState({ view: 'dashboard', fda: null, config: undefined })
  GetDiskUsageMock.mockReset().mockResolvedValue(null)
  GetActivityStatsMock.mockReset().mockResolvedValue(null)
  StartScanMock.mockClear()
})

describe('diskHeadline', () => {
  it('handles missing or empty disk info', () => {
    expect(diskHeadline(null)).toBe('Disk usage unavailable')
    expect(diskHeadline({ total: 0, used: 0, free: 0 })).toBe('Disk usage unavailable')
  })

  it('escalates copy with disk pressure', () => {
    expect(diskHeadline(disk(100, 30))).toBe('Your Mac is in great shape')
    expect(diskHeadline(disk(100, 75))).toBe('Your disk is filling up')
    expect(diskHeadline(disk(100, 95))).toBe('Your disk is almost full')
  })
})

describe('<Dashboard />', () => {
  it('renders health + activity cards from the backend stats', async () => {
    GetDiskUsageMock.mockResolvedValue(disk(500 * GB, 150 * GB))
    GetActivityStatsMock.mockResolvedValue(stats)
    render(<Dashboard />)
    await waitFor(() => expect(screen.getByText('Your Mac is in great shape')).toBeInTheDocument())
    expect(screen.getByText(/free of/)).toHaveTextContent('350.0 GB free of 500.0 GB')
    expect(screen.getByText('2.0 GB')).toBeInTheDocument()
    expect(screen.getByText(/12 items · 3 cleans/)).toBeInTheDocument()
    expect(screen.getByText('7 scans')).toBeInTheDocument()
    expect(screen.getByText(/2 apps uninstalled/)).toBeInTheDocument()
  })

  it('degrades gracefully when the backend returns nothing', async () => {
    render(<Dashboard />)
    await waitFor(() => expect(screen.getByText('Disk usage unavailable')).toBeInTheDocument())
    expect(screen.getByText('0 B')).toBeInTheDocument()
    expect(screen.getByText('0 scans')).toBeInTheDocument()
  })

  it('Smart Scan lens routes to Cleanup and starts a full scan when idle', async () => {
    render(<Dashboard />)
    fireEvent.click(screen.getByRole('button', { name: 'Smart Scan' }))
    expect(useUiStore.getState().view).toBe('smart-scan')
    await waitFor(() => expect(StartScanMock).toHaveBeenCalledTimes(1))
    expect(StartScanMock).toHaveBeenCalledWith([])
  })

  it('relabels the lens "View Results" after a scan and does not restart it', () => {
    useScanStore.setState({ status: 'done' })
    render(<Dashboard />)
    fireEvent.click(screen.getByRole('button', { name: 'View Results' }))
    expect(useUiStore.getState().view).toBe('smart-scan')
    expect(StartScanMock).not.toHaveBeenCalled()
  })

  it('module tiles navigate to their view', () => {
    render(<Dashboard />)
    fireEvent.click(screen.getByText('Space Lens'))
    expect(useUiStore.getState().view).toBe('space-lens')
  })
})
