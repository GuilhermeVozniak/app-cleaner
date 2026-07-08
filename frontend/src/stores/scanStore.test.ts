import { beforeEach, describe, expect, it } from 'vitest';
import {
  handleScanDone,
  handleScanProgress,
  isCategorySelected,
  selectedPaths,
  useScanStore,
} from './scanStore';
import type { Category, ScanResult, ScanSummary } from '../lib/types';

function cat(id: string, safetyLevel: Category['safetyLevel']): Category {
  return { id, name: id, group: 'Storage', description: '', safetyLevel };
}

function result(category: Category, sizes: Record<string, number>): ScanResult {
  const items = Object.entries(sizes).map(([path, size]) => ({
    path,
    size,
    name: path.split('/').pop() ?? path,
    isDirectory: false,
  }));
  return { category, items, totalSize: items.reduce((n, i) => n + i.size, 0) };
}

beforeEach(() => {
  useScanStore.getState().reset();
});

describe('handleScanProgress', () => {
  it('fills results and counts incrementally as scan:progress lands', () => {
    useScanStore.setState({ status: 'scanning', categories: [cat('trash', 'safe')] });
    handleScanProgress({ completed: 1, total: 16, categoryId: 'trash', totalSize: 2048, itemCount: 3 });
    const s = useScanStore.getState();
    expect(s.progress).toEqual({ completed: 1, total: 16 });
    expect(s.results['trash'].category.id).toBe('trash');
    expect(s.results['trash'].totalSize).toBe(2048);
    expect(s.itemCounts['trash']).toBe(3);
    expect(s.totalSize).toBe(2048);

    handleScanProgress({ completed: 2, total: 16, categoryId: 'system-cache', totalSize: 1000, itemCount: 1, error: 'boom' });
    const s2 = useScanStore.getState();
    expect(s2.progress.completed).toBe(2);
    expect(s2.results['system-cache'].error).toBe('boom');
    expect(s2.totalSize).toBe(3048);
  });
});

describe('handleScanDone auto-selection', () => {
  const summary: ScanSummary = {
    results: [
      result(cat('trash', 'safe'), { '/t/a': 100 }),
      result(cat('system-cache', 'moderate'), { '/c/b': 200 }),
      result(cat('ios-backups', 'risky'), { '/b/c': 300 }),
      result(cat('downloads', 'risky'), {}),
      result(cat('browser-cache', 'safe'), {}), // empty -> not auto-selected
    ],
    totalSize: 600,
    totalItems: 3,
  };

  it("selects 'all' for safe+moderate categories with items — risky never", () => {
    handleScanDone({ summary });
    const s = useScanStore.getState();
    expect(s.status).toBe('done');
    expect(s.selected['trash']).toBe('all');
    expect(s.selected['system-cache']).toBe('all');
    expect(s.selected['ios-backups']).toBeUndefined();
    expect(s.selected['downloads']).toBeUndefined();
    expect(s.selected['browser-cache']).toBeUndefined();
    expect(s.totalSize).toBe(600);
    expect(s.itemCounts['ios-backups']).toBe(1);
  });

  it('toggleCategory turns a risky category on as all, then off again', () => {
    handleScanDone({ summary });
    useScanStore.getState().toggleCategory('ios-backups');
    expect(useScanStore.getState().selected['ios-backups']).toBe('all');
    useScanStore.getState().toggleCategory('ios-backups');
    expect(useScanStore.getState().selected['ios-backups']).toBeUndefined();
  });

  it('setItemSelection stores Sets and drops empty ones', () => {
    handleScanDone({ summary });
    useScanStore.getState().setItemSelection('trash', new Set(['/t/a']));
    expect(useScanStore.getState().selected['trash']).toEqual(new Set(['/t/a']));
    useScanStore.getState().setItemSelection('trash', new Set());
    expect(useScanStore.getState().selected['trash']).toBeUndefined();
    expect(isCategorySelected(new Set())).toBe(false);
    expect(isCategorySelected('all')).toBe(true);
  });

  it('stores error and cancelled from scan:done, and clears them on a clean finish', () => {
    handleScanDone({ summary, cancelled: true, error: 'scan interrupted' });
    let s = useScanStore.getState();
    expect(s.status).toBe('done');
    expect(s.scanCancelled).toBe(true);
    expect(s.scanError).toBe('scan interrupted');

    handleScanDone({ summary });
    s = useScanStore.getState();
    expect(s.scanCancelled).toBe(false);
    expect(s.scanError).toBeUndefined();
  });
});

describe('selectedPaths', () => {
  it("maps 'all' and Set selections to the StartClean wire shape", () => {
    const r = {
      trash: result(cat('trash', 'safe'), { '/t/a': 1, '/t/b': 2 }),
      downloads: result(cat('downloads', 'risky'), { '/d/x': 3, '/d/y': 4 }),
    };
    const sel = { trash: 'all' as const, downloads: new Set(['/d/y']) };
    expect(selectedPaths(r, sel)).toEqual({ trash: ['/t/a', '/t/b'], downloads: ['/d/y'] });
  });
});
