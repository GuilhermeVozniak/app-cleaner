import { create } from 'zustand';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { CancelClean, StartClean } from '../../wailsjs/go/main/App';
import { selectedPaths, useScanStore } from './scanStore';
import type { CleanDoneEvent, CleanProgressEvent, CleanSummary } from '../lib/types';

/** Mirrors main.CleanOptions JSON tags. */
export interface CleanOptions {
  dryRun: boolean;
  backup: boolean;
}

export interface CleanState {
  status: 'idle' | 'confirming' | 'cleaning' | 'done';
  progress: { current: number; total: number; categoryId: string; itemName: string };
  summary?: CleanSummary;
  notBackedUp: string[];
  error?: string;
  cancelled: boolean;
  openConfirm: () => void;
  startClean: (opts: CleanOptions) => Promise<void>;
  cancelClean: () => void;
  reset: () => void;
}

const initialState = {
  status: 'idle' as const,
  progress: { current: 0, total: 0, categoryId: '', itemName: '' },
  summary: undefined as CleanSummary | undefined,
  notBackedUp: [] as string[],
  error: undefined as string | undefined,
  cancelled: false,
};

export const useCleanStore = create<CleanState>((set) => ({
  ...initialState,

  openConfirm: () => set({ status: 'confirming' }),

  startClean: async (opts) => {
    const { results, selected } = useScanStore.getState();
    const selection = selectedPaths(results, selected);
    set({
      status: 'cleaning',
      progress: { current: 0, total: 0, categoryId: '', itemName: '' },
      summary: undefined,
      notBackedUp: [],
      error: undefined,
      cancelled: false,
    });
    try {
      await StartClean(selection, opts);
    } catch (e) {
      set({ status: 'done', error: String(e) });
    }
  },

  cancelClean: () => {
    void CancelClean();
  },

  reset: () => set({ ...initialState }),
}));

/** Exported for tests: applied on every `clean:progress` event. */
export function handleCleanProgress(ev: CleanProgressEvent): void {
  useCleanStore.setState({
    progress: {
      current: ev.current,
      total: ev.total,
      categoryId: ev.categoryId,
      itemName: ev.itemName,
    },
  });
}

/** Exported for tests: applied on `clean:done`. */
export function handleCleanDone(ev: CleanDoneEvent): void {
  useCleanStore.setState({
    status: 'done',
    summary: ev.summary,
    notBackedUp: ev.notBackedUp ?? [],
    cancelled: ev.cancelled === true,
    error: ev.error || undefined,
  });
}

EventsOn('clean:progress', handleCleanProgress as (...data: any) => void);
EventsOn('clean:done', handleCleanDone as (...data: any) => void);
