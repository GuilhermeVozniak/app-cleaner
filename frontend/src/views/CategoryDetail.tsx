import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, ArrowLeft, Copy, Folder, Info } from 'lucide-react';
import { CopyPath, GroupItems, RevealInFinder } from '../../wailsjs/go/main/App';
import EmptyState from '../components/EmptyState';
import ItemList from '../components/ItemList';
import SafetyBadge from '../components/SafetyBadge';
import { formatSize, middleTruncate } from '../lib/format';
import { bumpExpand, invertSelection, togglePath } from '../lib/selection';
import type { DisplayRow } from '../lib/types';
import { useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';

export default function CategoryDetail() {
  const activeCategoryId = useUiStore((s) => s.activeCategoryId);
  const setView = useUiStore((s) => s.setView);
  const results = useScanStore((s) => s.results);
  const selected = useScanStore((s) => s.selected);
  const setItemSelection = useScanStore((s) => s.setItemSelection);

  const result = activeCategoryId ? results[activeCategoryId] : undefined;
  const grouped = result?.category.supportsFileSelection === true;
  const isDocker = result?.category.id === 'docker';

  // Per-directory expand limits, keyed by absolute dir path (grouping.DisplayRow.directoryKey).
  const [expand, setExpand] = useState<Record<string, number>>({});
  const [rows, setRows] = useState<DisplayRow[]>([]);

  useEffect(() => {
    if (!grouped || !activeCategoryId) return;
    let stale = false;
    void GroupItems(activeCategoryId, expand).then((r) => {
      if (!stale) setRows(((r ?? []) as DisplayRow[]));
    });
    return () => {
      stale = true;
    };
  }, [grouped, activeCategoryId, expand]);

  const onExpandHint = useCallback((directoryKey: string) => {
    setExpand((e) => bumpExpand(e, directoryKey));
  }, []);

  if (!result || !activeCategoryId) {
    return <EmptyState title="No category selected" subtitle="Run a scan and pick a category." />;
  }

  const items = result.items ?? [];
  const allPaths = items.map((i) => i.path);
  const sel = selected[activeCategoryId];
  const checked = (path: string) => sel === 'all' || (sel instanceof Set && sel.has(path));
  const toggleItem = (path: string) =>
    setItemSelection(activeCategoryId, togglePath(allPaths, sel, path));

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-neutral-200 px-6 py-4 dark:border-neutral-800">
        <div className="flex items-center gap-3">
          <button
            type="button"
            aria-label="Back"
            onClick={() => setView('smart-scan')}
            className="rounded p-1 text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800"
          >
            <ArrowLeft size={18} />
          </button>
          <h1 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">
            {result.category.name}
          </h1>
          <SafetyBadge level={result.category.safetyLevel} />
          <span className="ml-auto text-sm text-neutral-500 dark:text-neutral-400">
            {items.length} items · {formatSize(result.totalSize)}
          </span>
        </div>
        {result.category.safetyLevel !== 'safe' && result.category.safetyNote && (
          <div className="mt-3 flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950 dark:text-amber-300">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            {result.category.safetyNote}
          </div>
        )}
        {isDocker ? (
          <div className="mt-3 flex items-start gap-2 rounded-md bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:bg-blue-950 dark:text-blue-300">
            <Info size={14} className="mt-0.5 shrink-0" />
            Docker space is cleaned all-at-once via docker system prune
          </div>
        ) : (
          <div className="mt-3 flex items-center gap-2">
            <button
              type="button"
              onClick={() => setItemSelection(activeCategoryId, 'all')}
              className="rounded-md border border-neutral-300 px-3 py-1 text-xs text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
            >
              Select all
            </button>
            <button
              type="button"
              onClick={() => setItemSelection(activeCategoryId, invertSelection(allPaths, sel))}
              className="rounded-md border border-neutral-300 px-3 py-1 text-xs text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
            >
              Invert
            </button>
          </div>
        )}
      </header>

      {items.length === 0 ? (
        <EmptyState title="Nothing here" subtitle="This category has no cleanable items." />
      ) : grouped ? (
        <ul className="flex-1 overflow-y-auto px-6 py-2">
          {rows.map((row, i) => {
            if (row.type === 'directory-header') {
              return (
                <li
                  key={`h-${row.directoryKey}`}
                  className="mt-3 flex items-baseline gap-2 px-2 py-1 text-xs font-semibold text-neutral-500 dark:text-neutral-400"
                >
                  {row.displayName}
                  <span className="font-normal">({row.totalFilesInDir})</span>
                </li>
              );
            }
            if (row.type === 'expand-hint') {
              return (
                <li key={`e-${row.directoryKey}-${i}`}>
                  <button
                    type="button"
                    onClick={() => onExpandHint(row.directoryKey)}
                    className="px-2 py-1 text-xs text-blue-600 hover:underline dark:text-blue-400"
                  >
                    Show {row.hiddenCount} more
                  </button>
                </li>
              );
            }
            // type === 'file'
            const path = row.path ?? '';
            return (
              <li key={path} className="group flex items-center gap-3 px-2 py-1.5 text-sm">
                <input
                  type="checkbox"
                  aria-label={`Select ${row.name ?? row.displayName}`}
                  checked={checked(path)}
                  onChange={() => toggleItem(path)}
                  className="h-4 w-4 accent-blue-600"
                />
                <span className="w-64 shrink-0 truncate font-medium text-neutral-900 dark:text-neutral-100">
                  {row.displayName}
                </span>
                <span
                  className="min-w-0 flex-1 truncate text-xs text-neutral-400 dark:text-neutral-500"
                  title={path}
                >
                  {middleTruncate(path, 50)}
                </span>
                <span className="hidden shrink-0 items-center gap-1 group-hover:flex">
                  <button
                    type="button"
                    title="Reveal in Finder"
                    onClick={() => RevealInFinder(path)}
                    className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
                  >
                    <Folder size={14} />
                  </button>
                  <button
                    type="button"
                    title="Copy path"
                    onClick={() => CopyPath(path)}
                    className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
                  >
                    <Copy size={14} />
                  </button>
                </span>
                <span className="w-20 shrink-0 text-right text-neutral-500 dark:text-neutral-400">
                  {formatSize(row.size ?? 0)}
                </span>
              </li>
            );
          })}
        </ul>
      ) : (
        <div className="flex-1 overflow-y-auto px-6 py-2">
          <ItemList
            items={[...items].sort((a, b) => b.size - a.size)}
            selectable={!isDocker}
            isChecked={checked}
            onToggle={toggleItem}
          />
        </div>
      )}
    </div>
  );
}
