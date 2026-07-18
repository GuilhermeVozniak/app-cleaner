import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  RunMaintenance: vi.fn(),
  StartTMSnapshotsClear: vi.fn(),
  CancelMaintenance: vi.fn(),
}))

import { EventsOn } from '../../wailsjs/runtime/runtime'
import { RunMaintenance, StartTMSnapshotsClear, CancelMaintenance } from '../../wailsjs/go/main/App'
import { Maintenance } from './Maintenance'

const RunMaintenanceMock = RunMaintenance as unknown as ReturnType<typeof vi.fn>
const StartTMSnapshotsClearMock = StartTMSnapshotsClear as unknown as ReturnType<typeof vi.fn>
const CancelMaintenanceMock = CancelMaintenance as unknown as ReturnType<typeof vi.fn>
const EventsOnMock = EventsOn as unknown as ReturnType<typeof vi.fn>

beforeEach(() => {
  RunMaintenanceMock.mockReset()
  StartTMSnapshotsClearMock.mockReset()
  CancelMaintenanceMock.mockClear()
  EventsOnMock.mockClear()
})

describe('<Maintenance />', () => {
  it('registers maintenance:progress and maintenance:done listeners on mount', () => {
    render(<Maintenance />)
    expect(EventsOnMock).toHaveBeenCalledWith('maintenance:progress', expect.any(Function))
    expect(EventsOnMock).toHaveBeenCalledWith('maintenance:done', expect.any(Function))
  })

  it('runs DNS flush and shows the success result', async () => {
    RunMaintenanceMock.mockResolvedValue({
      success: true,
      message: 'DNS cache flushed successfully',
      requiresAdmin: false,
    })
    render(<Maintenance />)
    const [dnsRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(dnsRun)
    expect(await screen.findByText('✓ DNS cache flushed successfully')).toBeInTheDocument()
    expect(RunMaintenanceMock).toHaveBeenCalledWith('dns')
  })

  it('shows a failed DNS result with a retry-with-admin CTA and retries', async () => {
    RunMaintenanceMock.mockResolvedValue({
      success: false,
      message: 'Flush failed',
      error: 'boom',
      requiresAdmin: true,
    })
    render(<Maintenance />)
    const [dnsRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(dnsRun)
    expect(await screen.findByText(/✗ Flush failed — boom/)).toBeInTheDocument()
    fireEvent.click(screen.getByText('Retry with administrator'))
    expect(RunMaintenanceMock).toHaveBeenCalledTimes(2)
  })

  it('runs purge and shows the success result', async () => {
    RunMaintenanceMock.mockResolvedValue({
      success: true,
      message: 'Purgeable space freed successfully',
      requiresAdmin: false,
    })
    render(<Maintenance />)
    const [, purgeRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(purgeRun)
    expect(await screen.findByText('✓ Purgeable space freed successfully')).toBeInTheDocument()
    expect(RunMaintenanceMock).toHaveBeenCalledWith('purge')
  })

  it('surfaces a thrown error from RunMaintenance as a failed result', async () => {
    RunMaintenanceMock.mockRejectedValue(new Error('network down'))
    render(<Maintenance />)
    const [dnsRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(dnsRun)
    expect(await screen.findByText(/✗ Task failed/)).toBeInTheDocument()
  })

  it('starts TM snapshot clearing, streams progress, and shows the final result', async () => {
    StartTMSnapshotsClearMock.mockResolvedValue(undefined)
    render(<Maintenance />)
    const [, , tmRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(tmRun)
    expect(StartTMSnapshotsClearMock).toHaveBeenCalledTimes(1)
    expect(await screen.findByText('Running…')).toBeInTheDocument()

    const progressHandler = EventsOnMock.mock.calls.find(([name]) => name === 'maintenance:progress')![1]
    progressHandler({ done: 1, total: 3, date: '2026-06-01' })
    expect(await screen.findByText('✓ 2026-06-01')).toBeInTheDocument()

    const doneHandler = EventsOnMock.mock.calls.find(([name]) => name === 'maintenance:done')![1]
    doneHandler({ result: { success: true, message: 'Snapshots cleared', requiresAdmin: true } })
    expect(await screen.findByText('✓ Snapshots cleared')).toBeInTheDocument()
  })

  it('shows a failed TM date entry', async () => {
    StartTMSnapshotsClearMock.mockResolvedValue(undefined)
    render(<Maintenance />)
    const [, , tmRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(tmRun)
    const progressHandler = EventsOnMock.mock.calls.find(([name]) => name === 'maintenance:progress')![1]
    progressHandler({ done: 1, total: 1, date: '2026-05-01', error: 'permission denied' })
    expect(await screen.findByText(/✗ 2026-05-01 — permission denied/)).toBeInTheDocument()
  })

  it('surfaces a thrown error from StartTMSnapshotsClear', async () => {
    StartTMSnapshotsClearMock.mockRejectedValue(new Error('boom'))
    render(<Maintenance />)
    const [, , tmRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(tmRun)
    expect(await screen.findByText(/✗ Could not start — Error: boom/)).toBeInTheDocument()
  })

  it('Cancel calls CancelMaintenance while TM clearing is running', async () => {
    StartTMSnapshotsClearMock.mockReturnValue(new Promise(() => {})) // never resolves -> stays "running"
    render(<Maintenance />)
    const [, , tmRun] = screen.getAllByRole('button', { name: 'Run' })
    fireEvent.click(tmRun)
    fireEvent.click(await screen.findByText('Cancel'))
    expect(CancelMaintenanceMock).toHaveBeenCalledTimes(1)
  })
})
