import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import CategoryDetail from './CategoryDetail';
import { bumpExpand, invertSelection, togglePath } from '../lib/selection';
import { contractHome } from '../lib/paths';
import { middleTruncate } from '../lib/format';
import { useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';
import type { Category, DisplayRow } from '../lib/types';

// GetHome is read exactly once, at module-load time of ../lib/paths (see that
// file's comment): reconfiguring the mock later can't change an already-
// resolved cachedHome. So the resolved value must be correct from the start —
// hence a fixed mockResolvedValue at vi.hoisted time rather than in beforeEach.
// GroupItems, by contrast, is invoked fresh on every render via useEffect, so
// its return value can be set per-test in beforeEach.
const { mockGetHome, mockGroupItems } = vi.hoisted(() => ({
  mockGetHome: vi.fn().mockResolvedValue('/Users/tester'),
  mockGroupItems: vi.fn().mockResolvedValue([]),
}));

vi.mock('../../wailsjs/go/main/App', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../wailsjs/go/main/App')>();
  return { ...actual, GetHome: mockGetHome, GroupItems: mockGroupItems };
});

describe('bumpExpand (expand-hint bump logic, CLI parity: (existing ?? 5) + 10)', () => {
  it('bumps an unexpanded directory from the default limit', () => {
    expect(bumpExpand({}, '/Users/x/Downloads')).toEqual({ '/Users/x/Downloads': 15 });
  });

  it('bumps an already-expanded directory further and leaves others alone', () => {
    const before = { '/a': 15, '/b': 25 };
    const after = bumpExpand(before, '/a');
    expect(after).toEqual({ '/a': 25, '/b': 25 });
    expect(before).toEqual({ '/a': 15, '/b': 25 }); // no mutation
  });
});

describe('selection math', () => {
  const all = ['/d/a', '/d/b', '/d/c'];

  it("invert of 'all' is nothing", () => {
    expect(invertSelection(all, 'all')).toEqual(new Set());
  });

  it('invert of nothing is everything', () => {
    expect(invertSelection(all, undefined)).toEqual(new Set(all));
    expect(invertSelection(all, new Set())).toEqual(new Set(all));
  });

  it('invert of a partial Set is its complement', () => {
    expect(invertSelection(all, new Set(['/d/b']))).toEqual(new Set(['/d/a', '/d/c']));
  });

  it("togglePath materializes 'all' into a Set minus the toggled path", () => {
    expect(togglePath(all, 'all', '/d/b')).toEqual(new Set(['/d/a', '/d/c']));
    expect(togglePath(all, new Set(['/d/a']), '/d/b')).toEqual(new Set(['/d/a', '/d/b']));
    expect(togglePath(all, new Set(['/d/a']), '/d/a')).toEqual(new Set());
  });
});

describe('middleTruncate', () => {
  it('elides the middle at maxLen', () => {
    expect(middleTruncate('short', 50)).toBe('short');
    const long = '/Users/someone/Library/Caches/com.example.app/Data/file.bin';
    const out = middleTruncate(long, 30);
    expect(out).toHaveLength(30);
    expect(out).toContain('...');
    expect(out.startsWith('/Users/someone')).toBe(true);
    expect(out.endsWith('file.bin')).toBe(true);
  });
});

describe('contractHome', () => {
  it('contracts home-prefixed paths and leaves everything else untouched', () => {
    expect(contractHome('/Users/x/Library/Caches/app', '/Users/x')).toBe('~/Library/Caches/app');
    expect(contractHome('/Users/x', '/Users/x')).toBe('~');
    expect(contractHome('/Users/xy/file', '/Users/x')).toBe('/Users/xy/file'); // prefix, not a path boundary
    expect(contractHome('/private/tmp/f', '/Users/x')).toBe('/private/tmp/f');
    expect(contractHome('docker:images', '/Users/x')).toBe('docker:images');
    expect(contractHome('/Users/x/file', '')).toBe('/Users/x/file'); // home unknown -> raw path
  });
});

describe('docker category rows are not selectable', () => {
  beforeEach(() => {
    useScanStore.getState().reset();
    const docker: Category = {
      id: 'docker',
      name: 'Docker',
      group: 'Development',
      description: '',
      safetyLevel: 'safe',
    };
    useScanStore.setState({
      status: 'done',
      results: {
        docker: {
          category: docker,
          items: [
            { path: 'docker:images', size: 1000, name: 'Docker images', isDirectory: false },
            { path: 'docker:build-cache', size: 500, name: 'Docker build cache', isDirectory: false },
          ],
          totalSize: 1500,
        },
      },
      itemCounts: { docker: 2 },
      totalSize: 1500,
    });
    useUiStore.setState({ view: 'category', activeCategoryId: 'docker' });
  });

  it('renders rows without checkboxes, hides select all/invert, shows the prune banner', () => {
    render(<CategoryDetail />);
    expect(
      screen.getByText('Docker space is cleaned all-at-once via docker system prune'),
    ).toBeInTheDocument();
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0);
    expect(screen.queryByText('Select all')).toBeNull();
    expect(screen.queryByText('Invert')).toBeNull();
    expect(screen.getByText('Docker images')).toBeInTheDocument();
    expect(screen.getByText('Docker build cache')).toBeInTheDocument();
  });
});

describe('grouped (drill-down) file rows contract the home directory before truncation', () => {
  const home = '/Users/tester';
  const filePath = `${home}/Downloads/big-file.zip`;

  beforeEach(() => {
    useScanStore.getState().reset();
    const downloads: Category = {
      id: 'downloads',
      name: 'Old Downloads',
      group: 'Storage',
      description: '',
      safetyLevel: 'risky',
      supportsFileSelection: true,
    };
    useScanStore.setState({
      status: 'done',
      results: {
        downloads: {
          category: downloads,
          items: [{ path: filePath, size: 12345, name: 'big-file.zip', isDirectory: false }],
          totalSize: 12345,
        },
      },
      itemCounts: { downloads: 1 },
      totalSize: 12345,
    });
    useUiStore.setState({ view: 'category', activeCategoryId: 'downloads' });

    const rows: DisplayRow[] = [
      {
        type: 'directory-header',
        directoryKey: `${home}/Downloads`,
        displayName: 'Downloads',
        totalFilesInDir: 1,
        selectable: false,
      },
      {
        type: 'file',
        directoryKey: `${home}/Downloads`,
        displayName: 'big-file.zip',
        path: filePath,
        name: 'big-file.zip',
        size: 12345,
        totalFilesInDir: 1,
        selectable: true,
      },
    ];
    mockGroupItems.mockResolvedValue(rows);
  });

  it("shows the grouped file row's path home-contracted (~/…), not the raw absolute path", async () => {
    render(<CategoryDetail />);
    const expected = middleTruncate(contractHome(filePath, home), 50);
    expect(expected.startsWith('~/')).toBe(true);

    expect(await screen.findByText(expected)).toBeInTheDocument();
    expect(screen.queryByText(filePath)).toBeNull();
  });
});
