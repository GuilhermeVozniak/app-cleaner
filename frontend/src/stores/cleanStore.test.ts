// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  StartClean: vi.fn().mockResolvedValue(undefined),
  CancelClean: vi.fn(),
}))

import { EventsOn } from '../../wailsjs/runtime/runtime'
import { StartClean, CancelClean } from '../../wailsjs/go/main/App'
import { useCleanStore, handleCleanProgress, handleCleanDone, handleBackupProgress } from './cleanStore'
import type { CleanSummary } from '../lib/types'

// The generated bindings are typed classes; erase types for mock assertions.
const StartCleanMock = StartClean as unknown as ReturnType<typeof vi.fn>
const CancelCleanMock = CancelClean as unknown as ReturnType<typeof vi.fn>
const EventsOnMock = EventsOn as unknown as ReturnType<typeof vi.fn>

beforeEach(() => {
  useCleanStore.getState().reset()
  StartCleanMock.mockClear()
  CancelCleanMock.mockClear()
})

describe('cleanStore', () => {
  it('registers the clean:progress, clean:done and backup:progress handlers on module load', () => {
    expect(EventsOnMock).toHaveBeenCalledWith('clean:progress', handleCleanProgress)
    expect(EventsOnMock).toHaveBeenCalledWith('clean:done', handleCleanDone)
    expect(EventsOnMock).toHaveBeenCalledWith('backup:progress', handleBackupProgress)
  })

  it('openConfirm moves to confirming', () => {
    useCleanStore.getState().openConfirm()
    expect(useCleanStore.getState().status).toBe('confirming')
  })

  it('handleCleanProgress moves to cleaning and records progress', () => {
    handleCleanProgress({ current: 3, total: 40, categoryId: 'trash', itemName: 'old.dmg' })
    const s = useCleanStore.getState()
    expect(s.status).toBe('cleaning')
    expect(s.progress).toEqual({ current: 3, total: 40, categoryId: 'trash', itemName: 'old.dmg' })
  })

  it('handleBackupProgress feeds the same progress state and flags the backup phase', () => {
    handleBackupProgress({ current: 2, total: 10, itemName: 'photo.jpg' })
    const s = useCleanStore.getState()
    expect(s.status).toBe('cleaning')
    expect(s.backingUp).toBe(true)
    expect(s.progress).toEqual({ current: 2, total: 10, categoryId: '', itemName: 'photo.jpg' })
    handleCleanProgress({ current: 1, total: 10, categoryId: 'trash', itemName: 'old.dmg' })
    expect(useCleanStore.getState().backingUp).toBe(false)
  })

  it('handleCleanDone stores summary, notBackedUp and finishes', () => {
    const summary: CleanSummary & { notBackedUp?: string[] } = {
      results: [],
      totalFreedSpace: 42,
      totalCleanedItems: 1,
      totalErrors: 0,
      notBackedUp: ['/Volumes/Ext/movie.mkv'],
    }
    handleCleanDone({ summary })
    const s = useCleanStore.getState()
    expect(s.status).toBe('done')
    expect(s.summary?.totalFreedSpace).toBe(42)
    expect(s.notBackedUp).toEqual(['/Volumes/Ext/movie.mkv'])
    expect(s.cancelled).toBe(false)
  })

  it('handleCleanDone records cancellation and errors', () => {
    handleCleanDone({ cancelled: true, error: 'boom' })
    const s = useCleanStore.getState()
    expect(s.status).toBe('done')
    expect(s.cancelled).toBe(true)
    expect(s.error).toBe('boom')
    expect(s.notBackedUp).toEqual([])
  })

  it('startClean calls the StartClean binding with paths + options and tracks dry-run', async () => {
    await useCleanStore
      .getState()
      .startClean({ trash: ['/Users/me/.Trash/a.zip'] }, { dryRun: true, backup: false })
    expect(StartCleanMock).toHaveBeenCalledWith(
      { trash: ['/Users/me/.Trash/a.zip'] },
      { dryRun: true, backup: false },
    )
    expect(useCleanStore.getState().status).toBe('cleaning')
    expect(useCleanStore.getState().lastDryRun).toBe(true)
  })

  it('cancelClean invokes the binding; reset returns to idle', () => {
    useCleanStore.getState().cancelClean()
    expect(CancelCleanMock).toHaveBeenCalledTimes(1)
    handleCleanDone({ cancelled: true })
    useCleanStore.getState().reset()
    const s = useCleanStore.getState()
    expect(s.status).toBe('idle')
    expect(s.summary).toBeUndefined()
  })
})
