import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import SmartScan, { groupCategories } from './SmartScan';
import { selectionTotals, useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';
import type { Category, Config, ScanResult } from '../lib/types';

function cat(
  id: string,
  name: string,
  safetyLevel: Category['safetyLevel'],
  group: Category['group'] = 'Storage',
): Category {
  return { id, name, group, description: '', safetyLevel };
}

function result(category: Category, sizes: Record<string, number>): ScanResult {
  const items = Object.entries(sizes).map(([path, size]) => ({
    path,
    size,
    name: path,
    isDirectory: false,
  }));
  return { category, items, totalSize: items.reduce((n, i) => n + i.size, 0) };
}

const CONFIG_SHOW_RISKY: Config = {
  downloadsDaysOld: 30,
  largeFilesMinSize: 524288000,
  backupByDefault: true,
  backupRetentionDays: 7,
  concurrency: 4,
  showRisky: true,
  keepLanguages: [],
  extraPaths: { nodeModules: [], projects: [] },
};

describe('selectionTotals mixed selection math', () => {
  it("counts 'all' categories fully and Set categories per path", () => {
    const trash = cat('trash', 'Trash', 'safe');
    const downloads = cat('downloads', 'Old Downloads', 'risky');
    const results = {
      trash: result(trash, { '/t/a': 100, '/t/b': 50 }),
      downloads: result(downloads, { '/d/x': 1000, '/d/y': 200 }),
    };
    const selected = { trash: 'all' as const, downloads: new Set(['/d/y']) };
    expect(selectionTotals(results, selected)).toEqual({ items: 3, size: 350 });
  });

  it('ignores empty Sets and categories without results', () => {
    const trash = cat('trash', 'Trash', 'safe');
    const results = { trash: result(trash, { '/t/a': 100 }) };
    expect(selectionTotals(results, { trash: new Set<string>(), ghost: 'all' })).toEqual({
      items: 0,
      size: 0,
    });
  });
});

describe('groupCategories', () => {
  it('buckets non-risky categories by group in first-appearance order', () => {
    const cats = [
      cat('system-cache', 'User Cache Files', 'moderate', 'System Junk'),
      cat('trash', 'Trash', 'safe', 'Storage'),
      cat('downloads', 'Old Downloads', 'risky', 'Storage'),
      cat('browser-cache', 'Browser Cache', 'safe', 'Browsers'),
      cat('launch-agents', 'Orphaned Launch Agents', 'moderate', 'System Junk'),
    ];
    const groups = groupCategories(cats);
    expect(groups.map((g) => g.group)).toEqual(['System Junk', 'Storage', 'Browsers']);
    expect(groups[0].categories.map((c) => c.id)).toEqual(['system-cache', 'launch-agents']);
    expect(groups[1].categories.map((c) => c.id)).toEqual(['trash']); // risky excluded
  });
});

describe('risky group in results state', () => {
  beforeEach(() => {
    useScanStore.getState().reset();
    useUiStore.setState({ view: 'smart-scan', activeCategoryId: undefined, config: undefined });
    const safe = cat('trash', 'Trash', 'safe');
    const risky = cat('ios-backups', 'iOS Backups', 'risky');
    useScanStore.setState({
      status: 'done',
      categories: [safe, risky],
      results: {
        trash: result(safe, { '/t/a': 10 }),
        'ios-backups': result(risky, { '/b/x': 99 }),
      },
      itemCounts: { trash: 1, 'ios-backups': 1 },
      totalSize: 109,
      selected: { trash: 'all' },
    });
  });

  it('is collapsed by default: risky card hidden until the chevron is clicked', () => {
    render(<SmartScan />);
    expect(screen.getByText('Trash')).toBeInTheDocument();
    expect(screen.queryByText('iOS Backups')).toBeNull();
    fireEvent.click(screen.getByText(/Risky \(1\)/));
    expect(screen.getByText('iOS Backups')).toBeInTheDocument();
  });

  it('respects config.showRisky for the initial expanded state', () => {
    useUiStore.setState({ config: CONFIG_SHOW_RISKY });
    render(<SmartScan />);
    expect(screen.getByText('iOS Backups')).toBeInTheDocument();
  });

  it('footer shows the selected totals and enables Clean', () => {
    render(<SmartScan />);
    expect(screen.getByText('1 items · 10 B selected')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Clean' })).toBeEnabled();
  });

  it('shows a dismissible scan error banner and a cancelled note', () => {
    useScanStore.setState({ scanError: 'system-cache: permission denied', scanCancelled: true });
    render(<SmartScan />);
    expect(screen.getByText('system-cache: permission denied')).toBeInTheDocument();
    expect(screen.getByText('Scan cancelled — partial results')).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Dismiss scan error'));
    expect(screen.queryByText('system-cache: permission denied')).toBeNull();
  });
});
