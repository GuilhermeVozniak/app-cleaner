import { useEffect, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronRight, X } from 'lucide-react';
import CategoryCard from '../components/CategoryCard';
import EmptyState from '../components/EmptyState';
import { ModuleHero } from '../components/ModuleHero';
import { MODULES } from '../lib/modules';
import { ScanLens } from '../components/ScanLens';
import { ActionBar } from '../components/ActionBar';
import { Button } from '../components/ui/button';
import { Card } from '../components/ui/card';
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
  const scanIds = useScanStore((s) => s.scanIds);
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
    const cleanup = MODULES.find((m) => m.view === 'smart-scan')!;
    return (
      <ModuleHero
        module={cleanup}
        cta={
          <ScanLens
            state="idle"
            hue={cleanup.hue}
            label="Scan"
            onScan={() => void useScanStore.getState().startScan()}
          />
        }
      />
    );
  }

  // ---- Scanning state ----
  if (status === 'scanning') {
    return (
      <div className="flex h-full flex-col items-center gap-6 overflow-y-auto px-8 py-8">
        <ScanLens
          state="scanning"
          hue="var(--color-module-cleanup)"
          completed={progress.completed}
          total={progress.total}
          totalSize={totalSize}
          onScan={() => {}}
        />
        <div className="flex w-full max-w-xl items-center">
          <Button
            type="button"
            variant="glass"
            size="sm"
            onClick={() => useScanStore.getState().cancelScan()}
            className="ml-auto"
          >
            Cancel
          </Button>
        </div>
        <div className="w-full max-w-xl flex-1 space-y-1">
          {(scanIds.length > 0 ? categories.filter((c) => scanIds.includes(c.id)) : categories).map((c) => {
            const r = results[c.id];
            return (
              <div
                key={c.id}
                className="glass-1 flex items-center gap-2 rounded-control px-3 py-1.5 text-sm"
              >
                <span className="min-w-0 flex-1 truncate text-ink">{c.name}</span>
                {r ? (
                  <span className="nums shrink-0 text-ink-2">
                    {itemCounts[c.id] ?? 0} items · {formatSize(r.totalSize)}
                  </span>
                ) : (
                  <span className="shrink-0 text-ink-2">pending…</span>
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
    <div className="animate-[materialize_220ms_var(--ease-glass)] flex h-full flex-col">
      <div className="flex-1 overflow-y-auto px-6 py-4">
        {scanError && !errorDismissed && (
          <div className="glass-1 mb-3 flex items-start gap-2 rounded-control px-3 py-2 text-xs text-moderate">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            <span className="min-w-0 flex-1">{scanError}</span>
            <button
              type="button"
              aria-label="Dismiss scan error"
              onClick={() => setErrorDismissed(true)}
              className="shrink-0 rounded p-0.5 hover:bg-hairline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <X size={14} />
            </button>
          </div>
        )}
        {scanCancelled && (
          <div className="mb-3 text-xs text-ink-2">Scan cancelled — partial results</div>
        )}
        <div className="mb-4 text-lg font-semibold text-ink">
          Found {formatSize(totalSize)} that can be cleaned
        </div>
        {groups.map((g) => (
          <section key={g.group} className="mb-5">
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-ink-2">
              {g.group}
            </h2>
            <Card className="space-y-2 p-3">{g.categories.map(renderCard)}</Card>
          </section>
        ))}
        {riskyCats.length > 0 && (
          <section className="mb-5">
            <button
              type="button"
              onClick={() => setRiskyExpanded(!riskyOpen)}
              className="mb-2 flex items-center gap-1 rounded text-xs font-semibold uppercase tracking-wide text-danger focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              {riskyOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              Risky ({riskyCats.length})
            </button>
            {riskyOpen && <Card className="space-y-2 p-3">{riskyCats.map(renderCard)}</Card>}
          </section>
        )}
      </div>
      <ActionBar>
        <span className="nums text-sm text-ink-2">
          {totals.items} items · {formatSize(totals.size)} selected
        </span>
        <Button
          type="button"
          variant="primary"
          disabled={totals.items === 0}
          onClick={() => useCleanStore.getState().openConfirm()}
          className="ml-auto"
        >
          Clean
        </Button>
      </ActionBar>
    </div>
  );
}
