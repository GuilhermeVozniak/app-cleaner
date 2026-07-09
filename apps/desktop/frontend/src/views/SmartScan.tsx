import { useEffect, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronRight, Loader2, Search, X } from 'lucide-react';
import CategoryCard from '../components/CategoryCard';
import EmptyState from '../components/EmptyState';
import { formatSize } from '../lib/format';
import type { Category } from '../lib/types';
import { useCleanStore } from '../stores/cleanStore';
import { isCategorySelected, selectionTotals, useScanStore } from '../stores/scanStore';
import { useUiStore } from '../stores/uiStore';

/** Non-risky categories bucketed by CategoryGroup; group order = first appearance in `categories` (spec §5 order via GetCategories). */
export function groupCategories(
  categories: Category[],
): Array<{ group: string; categories: Category[] }> {
  const out: Array<{ group: string; categories: Category[] }> = [];
  const index = new Map<string, number>();
  for (const c of categories) {
    if (c.safetyLevel === 'risky') continue;
    let i = index.get(c.group);
    if (i === undefined) {
      i = out.length;
      index.set(c.group, i);
      out.push({ group: c.group, categories: [] });
    }
    out[i].categories.push(c);
  }
  return out;
}

export default function SmartScan() {
  const status = useScanStore((s) => s.status);
  const progress = useScanStore((s) => s.progress);
  const categories = useScanStore((s) => s.categories);
  const results = useScanStore((s) => s.results);
  const itemCounts = useScanStore((s) => s.itemCounts);
  const totalSize = useScanStore((s) => s.totalSize);
  const selected = useScanStore((s) => s.selected);
  const scanError = useScanStore((s) => s.scanError);
  const scanCancelled = useScanStore((s) => s.scanCancelled);
  const config = useUiStore((s) => s.config);
  // null = untouched -> fall back to config.showRisky for the initial expanded state.
  const [riskyExpanded, setRiskyExpanded] = useState<boolean | null>(null);
  const riskyOpen = riskyExpanded ?? config?.showRisky ?? false;
  const [errorDismissed, setErrorDismissed] = useState(false);

  useEffect(() => {
    void useScanStore.getState().loadCategories();
  }, []);

  // A fresh scan gets a fresh (undismissed) banner.
  useEffect(() => {
    if (status === 'scanning') setErrorDismissed(false);
  }, [status]);

  // ---- Hero state ----
  if (status === 'idle') {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-6">
        <h1 className="text-3xl font-bold text-neutral-900 dark:text-neutral-100">App Cleaner</h1>
        <p className="text-sm text-neutral-500 dark:text-neutral-400">
          Find caches, logs, and leftover junk taking up space on your Mac.
        </p>
        <button
          type="button"
          onClick={() => void useScanStore.getState().startScan()}
          className="flex items-center gap-2 rounded-full bg-blue-600 px-8 py-4 text-lg font-semibold text-white shadow-lg hover:bg-blue-700"
        >
          <Search size={20} /> Smart Scan
        </button>
      </div>
    );
  }

  // ---- Scanning state ----
  if (status === 'scanning') {
    return (
      <div className="flex h-full flex-col px-8 py-8">
        <div className="mb-4 flex items-center gap-3">
          <Loader2 size={18} className="animate-spin text-blue-600 dark:text-blue-400" />
          <span className="text-sm font-medium">
            Scanning… {progress.completed}/{progress.total}
          </span>
          <button
            type="button"
            onClick={() => useScanStore.getState().cancelScan()}
            className="ml-auto rounded-md border border-neutral-300 px-3 py-1 text-sm text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
          >
            Cancel
          </button>
        </div>
        <div className="flex-1 space-y-1 overflow-y-auto">
          {categories.map((c) => {
            const r = results[c.id];
            return (
              <div key={c.id} className="flex items-center gap-2 rounded px-2 py-1.5 text-sm">
                <span className="w-56 truncate text-neutral-800 dark:text-neutral-200">
                  {c.name}
                </span>
                {r ? (
                  <span className="text-neutral-500 dark:text-neutral-400">
                    {itemCounts[c.id] ?? 0} items · {formatSize(r.totalSize)}
                  </span>
                ) : (
                  <span className="text-neutral-400 dark:text-neutral-600">pending…</span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    );
  }

  // ---- Results state (status === 'done') ----
  if (totalSize === 0) {
    return (
      <EmptyState title="Your Mac is already clean!" subtitle="Nothing to remove was found." />
    );
  }

  const hasContent = (c: Category) => {
    const r = results[c.id];
    return !!r && ((r.items?.length ?? 0) > 0 || !!r.error);
  };
  const maxSize = Math.max(0, ...Object.values(results).map((r) => r.totalSize));
  const groups = groupCategories(categories)
    .map((g) => ({ ...g, categories: g.categories.filter(hasContent) }))
    .filter((g) => g.categories.length > 0);
  const riskyCats = categories.filter((c) => c.safetyLevel === 'risky').filter(hasContent);
  const totals = selectionTotals(results, selected);

  const toggle = (id: string) => useScanStore.getState().toggleCategory(id);
  const openCategory = (id: string) => useUiStore.getState().setView('category', id);

  const renderCard = (c: Category) => (
    <CategoryCard
      key={c.id}
      result={results[c.id]}
      itemCount={itemCounts[c.id] ?? results[c.id].items?.length ?? 0}
      selected={isCategorySelected(selected[c.id])}
      maxSize={maxSize}
      onToggle={() => toggle(c.id)}
      onOpen={() => openCategory(c.id)}
    />
  );

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-y-auto px-6 py-4">
        {scanError && !errorDismissed && (
          <div className="mb-3 flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950 dark:text-amber-300">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            <span className="min-w-0 flex-1">{scanError}</span>
            <button
              type="button"
              aria-label="Dismiss scan error"
              onClick={() => setErrorDismissed(true)}
              className="shrink-0 rounded p-0.5 hover:bg-amber-100 dark:hover:bg-amber-900"
            >
              <X size={14} />
            </button>
          </div>
        )}
        {scanCancelled && (
          <div className="mb-3 text-xs text-neutral-500 dark:text-neutral-400">
            Scan cancelled — partial results
          </div>
        )}
        <div className="mb-4 text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          Found {formatSize(totalSize)} that can be cleaned
        </div>
        {groups.map((g) => (
          <section key={g.group} className="mb-5">
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
              {g.group}
            </h2>
            <div className="space-y-2">{g.categories.map(renderCard)}</div>
          </section>
        ))}
        {riskyCats.length > 0 && (
          <section className="mb-5">
            <button
              type="button"
              onClick={() => setRiskyExpanded(!riskyOpen)}
              className="mb-2 flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-red-600 dark:text-red-400"
            >
              {riskyOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              Risky ({riskyCats.length})
            </button>
            {riskyOpen && <div className="space-y-2">{riskyCats.map(renderCard)}</div>}
          </section>
        )}
      </div>
      <footer className="flex items-center gap-4 border-t border-neutral-200 px-6 py-3 dark:border-neutral-800">
        <span className="text-sm text-neutral-600 dark:text-neutral-400">
          {totals.items} items · {formatSize(totals.size)} selected
        </span>
        <button
          type="button"
          disabled={totals.items === 0}
          onClick={() => useCleanStore.getState().openConfirm()}
          className="ml-auto rounded-md bg-blue-600 px-6 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Clean
        </button>
      </footer>
    </div>
  );
}
