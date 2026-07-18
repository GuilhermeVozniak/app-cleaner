import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, ArrowLeft, Copy, Folder, Info } from 'lucide-react';
import { CopyPath, GroupItems, RevealInFinder } from '../../wailsjs/go/main/App';
import EmptyState from '../components/EmptyState';
import ItemList from '../components/ItemList';
import SafetyBadge from '../components/SafetyBadge';
import { Button } from '../components/ui/button';
import { Checkbox } from '../components/ui/checkbox';
import { formatSize, middleTruncate } from '../lib/format';
import { contractHome, homeDir } from '../lib/paths';
import { bumpExpand, invertSelection, toggleDirectory, togglePath } from '../lib/selection';
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
      <header className="border-b border-hairline px-6 py-4">
        <div className="flex items-center gap-3">
          <Button
            type="button"
            aria-label="Back"
            variant="ghost"
            size="sm"
            onClick={() => setView('smart-scan')}
            className="px-1.5"
          >
            <ArrowLeft size={18} />
          </Button>
          <h1 className="text-lg font-semibold text-ink">{result.category.name}</h1>
          <SafetyBadge level={result.category.safetyLevel} />
          <span className="nums ml-auto text-sm text-ink-2">
            {items.length} items · {formatSize(result.totalSize)}
          </span>
        </div>
        {result.category.safetyLevel !== 'safe' && result.category.safetyNote && (
          <div className="mt-3 flex items-start gap-2 rounded-control bg-moderate/15 px-3 py-2 text-xs text-moderate">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            {result.category.safetyNote}
          </div>
        )}
        {isDocker ? (
          <div className="mt-3 flex items-start gap-2 rounded-control bg-accent/15 px-3 py-2 text-xs text-accent">
            <Info size={14} className="mt-0.5 shrink-0" />
            Docker space is cleaned all-at-once via docker system prune
          </div>
        ) : (
          <div className="mt-3 flex items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => setItemSelection(activeCategoryId, 'all')}
              className="h-auto border border-hairline px-3 py-1 text-xs"
            >
              Select all
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => setItemSelection(activeCategoryId, invertSelection(allPaths, sel))}
              className="h-auto border border-hairline px-3 py-1 text-xs"
            >
              Invert
            </Button>
          </div>
        )}
      </header>

      {items.length === 0 ? (
        <EmptyState title="Nothing here" subtitle="This category has no cleanable items." />
      ) : grouped ? (
        <ul className="flex-1 space-y-1 overflow-y-auto px-6 py-2">
          {rows.map((row, i) => {
            if (row.type === 'directory-header') {
              const dirPaths = rows
                .filter((r) => r.type === 'file' && r.directoryKey === row.directoryKey && r.path)
                .map((r) => r.path as string);
              const dirChecked = dirPaths.length > 0 && dirPaths.every(checked);
              return (
                <li
                  key={`h-${row.directoryKey}`}
                  className="mt-3 flex items-baseline gap-2 px-2 py-1 text-xs font-semibold text-ink-2"
                >
                  <Checkbox
                    aria-label={`Select all in ${row.displayName}`}
                    checked={dirChecked}
                    disabled={dirPaths.length === 0}
                    onCheckedChange={() =>
                      setItemSelection(activeCategoryId, toggleDirectory(allPaths, sel, dirPaths))
                    }
                  />
                  {row.displayName}
                  <span className="nums font-normal">({row.totalFilesInDir})</span>
                </li>
              );
            }
            if (row.type === 'expand-hint') {
              return (
                <li key={`e-${row.directoryKey}-${i}`}>
                  <button
                    type="button"
                    onClick={() => onExpandHint(row.directoryKey)}
                    className="rounded px-2 py-1 text-xs text-accent hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                  >
                    Show {row.hiddenCount} more
                  </button>
                </li>
              );
            }
            // type === 'file'
            const path = row.path ?? '';
            return (
              <li
                key={path}
                className="glass-1 group flex items-center gap-3 rounded-control px-3 py-2 text-sm transition hover:brightness-105"
              >
                <Checkbox
                  aria-label={`Select ${row.name ?? row.displayName}`}
                  checked={checked(path)}
                  onCheckedChange={() => toggleItem(path)}
                />
                <span className="w-64 shrink-0 truncate font-medium text-ink">
                  {row.displayName}
                </span>
                <span className="min-w-0 flex-1 truncate text-xs text-ink-2" title={path}>
                  {middleTruncate(contractHome(path, homeDir()), 50)}
                </span>
                <span className="hidden shrink-0 items-center gap-1 group-hover:flex">
                  <Button
                    type="button"
                    variant="ghost"
                    title="Reveal in Finder"
                    onClick={() => RevealInFinder(path)}
                    className="h-auto w-auto rounded p-1 text-ink-2 hover:bg-hairline hover:text-ink"
                  >
                    <Folder size={14} />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    title="Copy path"
                    onClick={() => CopyPath(path)}
                    className="h-auto w-auto rounded p-1 text-ink-2 hover:bg-hairline hover:text-ink"
                  >
                    <Copy size={14} />
                  </Button>
                </span>
                <span className="nums w-20 shrink-0 text-right text-ink-2">
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
