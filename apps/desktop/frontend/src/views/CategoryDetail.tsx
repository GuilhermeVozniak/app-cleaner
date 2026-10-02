import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, ArrowLeft, Info } from 'lucide-react';
import { GroupItems } from '../../wailsjs/go/main/App';
import EmptyState from '../components/EmptyState';
import ItemList, { FileRow } from '../components/ItemList';
import SafetyBadge from '../components/SafetyBadge';
import { StartOver } from '../components/StartOver';
import { Button } from '../components/ui/button';
import { Checkbox } from '../components/ui/checkbox';
import { formatSize } from '../lib/format';
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
    <div className="materialize flex h-full flex-col">
      <StartOver label="Cleanup" icon={ArrowLeft} onClick={() => setView('smart-scan')} />
      <header className="mx-auto w-full max-w-5xl px-10 pb-3 pt-3">
        <div className="flex items-center gap-3">
          <h1 className="text-title font-semibold text-ink">{result.category.name}</h1>
          <SafetyBadge level={result.category.safetyLevel} />
          <span className="nums ml-auto text-body text-ink-2">
            {items.length} {items.length === 1 ? 'item' : 'items'}, {formatSize(result.totalSize)}
          </span>
        </div>
        {result.category.description ? (
          <p className="mt-1 text-body text-ink-2">{result.category.description}</p>
        ) : null}
        {result.category.safetyLevel !== 'safe' && result.category.safetyNote && (
          <div className="mt-3 flex items-start gap-2 rounded-control bg-moderate/15 px-3 py-2 text-caption text-moderate">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            {result.category.safetyNote}
          </div>
        )}
        {isDocker ? (
          <div className="mt-3 flex items-start gap-2 rounded-control bg-fill px-3 py-2 text-caption text-ink-2">
            <Info size={14} className="mt-0.5 shrink-0" />
            Docker space is cleaned all-at-once via docker system prune
          </div>
        ) : (
          <div className="mt-3 flex items-center gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setItemSelection(activeCategoryId, 'all')}
            >
              Select all
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setItemSelection(activeCategoryId, invertSelection(allPaths, sel))}
            >
              Invert
            </Button>
          </div>
        )}
      </header>

      {items.length === 0 ? (
        <EmptyState title="Nothing here" subtitle="This category has no cleanable items." />
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto px-10 pb-6">
          <div className="glass-1 mx-auto max-w-5xl rounded-card py-1">
            {grouped ? (
              <ul className="divide-y divide-hairline">
                {rows.map((row, i) => {
                  if (row.type === 'directory-header') {
                    const dirPaths = rows
                      .filter((r) => r.type === 'file' && r.directoryKey === row.directoryKey && r.path)
                      .map((r) => r.path as string);
                    const dirChecked = dirPaths.length > 0 && dirPaths.every(checked);
                    return (
                      <li
                        key={`h-${row.directoryKey}`}
                        className="flex h-10 items-center gap-3 bg-[rgb(255_255_255/0.04)] px-4 text-caption font-semibold text-ink-2"
                      >
                        <Checkbox
                          aria-label={`Select all in ${row.displayName}`}
                          checked={dirChecked}
                          disabled={dirPaths.length === 0}
                          onCheckedChange={() =>
                            setItemSelection(activeCategoryId, toggleDirectory(allPaths, sel, dirPaths))
                          }
                        />
                        <span className="truncate text-ink">{row.displayName}</span>
                        <span className="nums font-normal">({row.totalFilesInDir})</span>
                      </li>
                    );
                  }
                  if (row.type === 'expand-hint') {
                    return (
                      <li key={`e-${row.directoryKey}-${i}`} className="px-4 py-1.5">
                        <button
                          type="button"
                          onClick={() => onExpandHint(row.directoryKey)}
                          className="focus-ring rounded px-1 text-caption font-medium text-ink-2 hover:text-ink"
                        >
                          Show {row.hiddenCount} more
                        </button>
                      </li>
                    );
                  }
                  const path = row.path ?? '';
                  return (
                    <FileRow
                      key={path}
                      name={row.displayName}
                      path={path}
                      size={row.size ?? 0}
                      selectable
                      checked={checked(path)}
                      onToggle={() => toggleItem(path)}
                      ariaLabel={`Select ${row.name ?? row.displayName}`}
                    />
                  );
                })}
              </ul>
            ) : (
              <ItemList
                items={[...items].sort((a, b) => b.size - a.size)}
                selectable={!isDocker}
                isChecked={checked}
                onToggle={toggleItem}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
