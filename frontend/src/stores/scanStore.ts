import { create } from 'zustand';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { CancelScan, GetCategories, StartScan } from '../../wailsjs/go/main/App';
import type { Category, ScanDoneEvent, ScanProgressEvent, ScanResult } from '../lib/types';

/** Per-category selection: 'all' = every item, Set = explicit item paths. */
export type Selection = Set<string> | 'all';

export interface ScanState {
  status: 'idle' | 'scanning' | 'done';
  progress: { completed: number; total: number };
  categories: Category[];
  results: Record<string, ScanResult>;
  itemCounts: Record<string, number>;
  totalSize: number;
  selected: Record<string, Selection>;
  scanError?: string;
  scanCancelled?: boolean;
  loadCategories: () => Promise<void>;
  toggleCategory: (id: string) => void;
  setItemSelection: (id: string, selection: Selection) => void;
  startScan: (ids?: string[]) => Promise<void>;
  cancelScan: () => void;
  reset: () => void;
}

const initialState = {
  status: 'idle' as const,
  progress: { completed: 0, total: 0 },
  categories: [] as Category[],
  results: {} as Record<string, ScanResult>,
  itemCounts: {} as Record<string, number>,
  totalSize: 0,
  selected: {} as Record<string, Selection>,
  scanError: undefined as string | undefined,
  scanCancelled: false as boolean | undefined,
};

/** True when the category has an active selection ('all' or a non-empty Set). */
export function isCategorySelected(sel: Selection | undefined): boolean {
  return sel === 'all' || (sel instanceof Set && sel.size > 0);
}

/** Item count + byte size of the current selection across all categories. */
export function selectionTotals(
  results: Record<string, ScanResult>,
  selected: Record<string, Selection>,
): { items: number; size: number } {
  let items = 0;
  let size = 0;
  for (const [id, sel] of Object.entries(selected)) {
    const result = results[id];
    if (!result || !isCategorySelected(sel)) continue;
    for (const item of result.items ?? []) {
      if (sel === 'all' || (sel instanceof Set && sel.has(item.path))) {
        items += 1;
        size += item.size;
      }
    }
  }
  return { items, size };
}

/** Wire shape for App.StartClean: categoryID -> selected item paths. */
export function selectedPaths(
  results: Record<string, ScanResult>,
  selected: Record<string, Selection>,
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const [id, sel] of Object.entries(selected)) {
    const result = results[id];
    if (!result || !isCategorySelected(sel)) continue;
    const paths = (result.items ?? [])
      .filter((it) => sel === 'all' || (sel instanceof Set && sel.has(it.path)))
      .map((it) => it.path);
    if (paths.length > 0) out[id] = paths;
  }
  return out;
}

export const useScanStore = create<ScanState>((set, get) => ({
  ...initialState,

  loadCategories: async () => {
    if (get().categories.length > 0) return;
    const cats = ((await GetCategories()) ?? []) as Category[];
    set({ categories: cats });
  },

  toggleCategory: (id) =>
    set((s) => {
      const next = { ...s.selected };
      if (isCategorySelected(next[id])) delete next[id];
      else next[id] = 'all';
      return { selected: next };
    }),

  setItemSelection: (id, selection) =>
    set((s) => {
      const next = { ...s.selected };
      if (selection !== 'all' && selection.size === 0) delete next[id];
      else next[id] = selection;
      return { selected: next };
    }),

  startScan: async (ids = []) => {
    await get().loadCategories();
    const total = ids.length > 0 ? ids.length : get().categories.length;
    set({
      status: 'scanning',
      progress: { completed: 0, total },
      results: {},
      itemCounts: {},
      selected: {},
      totalSize: 0,
      scanError: undefined,
      scanCancelled: false,
    });
    try {
      await StartScan(ids);
    } catch {
      set({ status: 'idle' }); // e.g. a scan is already running
    }
  },

  cancelScan: () => {
    void CancelScan();
  },

  reset: () => set({ ...initialState }),
}));

/** Exported for tests: applied on every `scan:progress` event. */
export function handleScanProgress(ev: ScanProgressEvent): void {
  useScanStore.setState((s) => {
    const category: Category =
      s.results[ev.categoryId]?.category ??
      s.categories.find((c) => c.id === ev.categoryId) ?? {
        id: ev.categoryId,
        name: ev.categoryId,
        group: 'System Junk',
        description: '',
        safetyLevel: 'moderate',
      };
    const results: Record<string, ScanResult> = {
      ...s.results,
      [ev.categoryId]: {
        category,
        items: s.results[ev.categoryId]?.items ?? [],
        totalSize: ev.totalSize,
        error: ev.error || undefined,
      },
    };
    return {
      progress: { completed: ev.completed, total: ev.total },
      results,
      itemCounts: { ...s.itemCounts, [ev.categoryId]: ev.itemCount },
      totalSize: Object.values(results).reduce((n, r) => n + r.totalSize, 0),
    };
  });
}

/** Exported for tests: applied on `scan:done`. Auto-selects safe+moderate ('all'); risky never. Also stores the event's error/cancelled flags. */
export function handleScanDone(ev: ScanDoneEvent): void {
  const results: Record<string, ScanResult> = {};
  const itemCounts: Record<string, number> = {};
  const selected: Record<string, Selection> = {};
  for (const r of ev.summary?.results ?? []) {
    const items = r.items ?? [];
    results[r.category.id] = { ...r, items };
    itemCounts[r.category.id] = items.length;
    if (r.category.safetyLevel !== 'risky' && items.length > 0 && !r.error) {
      selected[r.category.id] = 'all';
    }
  }
  useScanStore.setState({
    status: 'done',
    results,
    itemCounts,
    selected,
    totalSize: ev.summary?.totalSize ?? 0,
    scanError: ev.error || undefined,
    scanCancelled: ev.cancelled === true,
  });
}

EventsOn('scan:progress', handleScanProgress as (...data: any) => void);
EventsOn('scan:done', handleScanDone as (...data: any) => void);
