import { create } from 'zustand'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { StartClean, CancelClean } from '../../wailsjs/go/main/App'
import type { CleanSummary } from '../lib/types'

export interface CleanProgress {
  current: number
  total: number
  categoryId: string
  itemName: string
}

export interface CleanDonePayload {
  summary?: CleanSummary & { notBackedUp?: string[] }
  notBackedUp?: string[] // tolerated at top level too
  cancelled?: boolean
  error?: string
}

export interface BackupProgress {
  current: number
  total: number
  itemName: string
}

const initialProgress: CleanProgress = { current: 0, total: 0, categoryId: '', itemName: '' }

interface CleanState {
  status: 'idle' | 'confirming' | 'cleaning' | 'done'
  progress: CleanProgress
  backingUp: boolean // true while backup:progress events drive the overlay
  summary?: CleanSummary
  notBackedUp: string[]
  cancelled: boolean
  error?: string
  lastDryRun: boolean
  openConfirm: () => void
  startClean: (
    selection: Record<string, string[]>,
    opts: { dryRun: boolean; backup: boolean },
  ) => Promise<void>
  cancelClean: () => void
  reset: () => void
}

export const useCleanStore = create<CleanState>()((set) => ({
  status: 'idle',
  progress: initialProgress,
  backingUp: false,
  summary: undefined,
  notBackedUp: [],
  cancelled: false,
  error: undefined,
  lastDryRun: false,
  openConfirm: () => set({ status: 'confirming' }),
  startClean: async (selection, opts) => {
    set({
      status: 'cleaning',
      progress: initialProgress,
      backingUp: false,
      summary: undefined,
      notBackedUp: [],
      cancelled: false,
      error: undefined,
      lastDryRun: opts.dryRun,
    })
    try {
      await StartClean(selection, opts)
    } catch (e) {
      // e.g. "a clean is already running" — surface it and finish the flow
      set({ status: 'done', error: String(e) })
    }
  },
  cancelClean: () => {
    void CancelClean()
  },
  reset: () =>
    set({
      status: 'idle',
      progress: initialProgress,
      backingUp: false,
      summary: undefined,
      notBackedUp: [],
      cancelled: false,
      error: undefined,
      lastDryRun: false,
    }),
}))

// Exported for tests; also registered as the live Wails event handlers below.
export function handleCleanProgress(data: CleanProgress): void {
  useCleanStore.setState({ status: 'cleaning', backingUp: false, progress: data })
}

// backup:progress drives the same overlay while items are moved into the backup
// session (before deletion); CleanFlow prefixes the item name with "Backing up: ".
export function handleBackupProgress(data: BackupProgress): void {
  useCleanStore.setState({
    status: 'cleaning',
    backingUp: true,
    progress: { current: data.current, total: data.total, categoryId: '', itemName: data.itemName },
  })
}

export function handleCleanDone(data: CleanDonePayload): void {
  useCleanStore.setState({
    status: 'done',
    summary: data.summary,
    notBackedUp: data.summary?.notBackedUp ?? data.notBackedUp ?? [],
    cancelled: Boolean(data.cancelled),
    error: data.error,
  })
}

EventsOn('clean:progress', handleCleanProgress)
EventsOn('backup:progress', handleBackupProgress)
EventsOn('clean:done', handleCleanDone)
