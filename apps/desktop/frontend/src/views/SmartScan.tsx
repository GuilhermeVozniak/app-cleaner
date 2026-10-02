import { useEffect, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronRight, X } from 'lucide-react';
import CategoryCard from '../components/CategoryCard';
import EmptyState from '../components/EmptyState';
import { ModuleHero } from '../components/ModuleHero';
import { ModuleIcon } from '../components/ModuleIcon';
import { ScanLens } from '../components/ScanLens';
import { StartOver } from '../components/StartOver';
import { StateMark } from '../components/Stage';
import { Button } from '../components/ui/button';
import { MODULES } from '../lib/modules';
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

const plural = (n: number, one: string) => (n === 1 ? one : `${one}s`);

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
  const cleanup = MODULES.find((m) => m.view === 'smart-scan')!;

  useEffect(() => {
    void useScanStore.getState().loadCategories();
  }, []);

  // A fresh scan gets a fresh (undismissed) banner.
  useEffect(() => {
    if (status === 'scanning') setErrorDismissed(false);
  }, [status]);

  const startScan = () => void useScanStore.getState().startScan();
  const startOver = () => useScanStore.getState().reset();

  // ---- Hero ----
  if (status === 'idle') {
    return (
      <ModuleHero
        module={cleanup}
        cta={<ScanLens state="idle" hue={cleanup.hue} label="Scan" onScan={startScan} />}
      />
    );
  }

  // ---- Scanning ----
  if (status === 'scanning') {
    const list = scanIds.length > 0 ? categories.filter((c) => scanIds.includes(c.id)) : categories;
    return (
      <div className="materialize flex h-full flex-col items-center px-10 pb-2 pt-3">
        <ModuleIcon Icon={cleanup.Icon} size="xl" />
        <h1 className="mt-5 text-headline font-semibold text-ink">Looking for junk…</h1>
        <p className="nums mt-1.5 text-card text-ink-2">{formatSize(totalSize)} found so far</p>
        <div className="glass-1 mt-5 w-full max-w-2xl min-h-0 flex-1 overflow-y-auto rounded-card px-5 py-1">
          <ul className="divide-y divide-hairline">
            {list.map((c) => {
              const r = results[c.id];
              return (
                <li key={c.id} className="flex items-center gap-3 py-2.5">
                  <span className="min-w-0 flex-1 truncate text-body font-medium text-ink">{c.name}</span>
                  {r ? (
                    <span className="nums shrink-0 text-body text-ink-2">
                      {itemCounts[c.id] ?? 0} {plural(itemCounts[c.id] ?? 0, 'item')}, {formatSize(r.totalSize)}
                    </span>
                  ) : (
                    <span className="shrink-0 text-body text-ink-3">pending…</span>
                  )}
                  <StateMark state={r ? 'done' : 'pending'} />
                </li>
              );
            })}
          </ul>
        </div>
        <div className="pt-4">
          <ScanLens
            state="scanning"
            hue={cleanup.hue}
            completed={progress.completed}
            total={progress.total}
            totalSize={totalSize}
            onScan={() => {}}
            onStop={() => useScanStore.getState().cancelScan()}
            caption={
              <span className="text-ink">
                {progress.completed}/{progress.total}
              </span>
            }
          />
        </div>
      </div>
    );
  }

  // ---- Results (status === 'done') ----
  if (totalSize === 0) {
    return (
      <>
        <StartOver onClick={startOver} />
        <EmptyState
          icon={cleanup.Icon}
          title="Your Mac is already clean!"
          subtitle="Nothing to remove was found."
          action={
            <Button type="button" variant="secondary" onClick={startScan}>
              Scan again
            </Button>
          }
        />
      </>
    );
  }

  const hasContent = (c: Category) => {
    const r = results[c.id];
    return !!r && ((r.items?.length ?? 0) > 0 || !!r.error);
  };
  const groups = groupCategories(categories)
    .map((g) => ({ ...g, categories: g.categories.filter(hasContent) }))
    .filter((g) => g.categories.length > 0);
  const riskyCats = categories.filter((c) => c.safetyLevel === 'risky').filter(hasContent);
  const totals = selectionTotals(results, selected);
  const sizeOf = (cats: Category[]) => cats.reduce((n, c) => n + (results[c.id]?.totalSize ?? 0), 0);
  const totalItems = Object.values(itemCounts).reduce((n, c) => n + c, 0);
  const categoryCount = groups.reduce((n, g) => n + g.categories.length, 0) + riskyCats.length;

  const toggle = (id: string) => useScanStore.getState().toggleCategory(id);
  const openCategory = (id: string) => useUiStore.getState().setView('category', id);

  const renderCard = (c: Category) => (
    <CategoryCard
      key={c.id}
      result={results[c.id]}
      itemCount={itemCounts[c.id] ?? results[c.id].items?.length ?? 0}
      selected={isCategorySelected(selected[c.id])}
      onToggle={() => toggle(c.id)}
      onOpen={() => openCategory(c.id)}
    />
  );

  return (
    <div className="materialize flex h-full flex-col">
      <StartOver onClick={startOver} />
      <div className="min-h-0 flex-1 overflow-y-auto px-10 pb-4 pt-3">
        {scanError && !errorDismissed && (
          <div className="glass-1 mx-auto mb-4 flex max-w-5xl items-start gap-2 rounded-control px-3 py-2 text-caption text-moderate">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            <span className="min-w-0 flex-1">{scanError}</span>
            <button
              type="button"
              aria-label="Dismiss scan error"
              onClick={() => setErrorDismissed(true)}
              className="focus-ring shrink-0 rounded p-0.5 hover:bg-fill"
            >
              <X size={14} />
            </button>
          </div>
        )}
        <div className="text-center">
          <h1 className="text-headline font-semibold text-ink">
            We've found {formatSize(totalSize)} you can clean
          </h1>
          <p className="nums mt-1.5 text-card text-ink-2">
            {totalItems} {plural(totalItems, 'item')} in {categoryCount} {plural(categoryCount, 'category').replace('categorys', 'categories')}. Safe items are already selected.
          </p>
          {scanCancelled && (
            <p className="mt-1.5 text-caption text-moderate">Scan cancelled — partial results</p>
          )}
        </div>

        <div className="mx-auto mt-6 grid max-w-5xl grid-cols-2 gap-4 lg:grid-cols-3">
          {groups.map((g) => (
            <section key={g.group} className="glass-1 flex flex-col rounded-card">
              <header className="flex items-baseline justify-between gap-3 px-5 pb-1 pt-4">
                <h2 className="text-card font-semibold text-ink">{g.group}</h2>
                <span className="nums text-caption text-ink-2">{formatSize(sizeOf(g.categories))}</span>
              </header>
              <ul className="divide-y divide-hairline px-2 pb-2">{g.categories.map(renderCard)}</ul>
            </section>
          ))}
        </div>

        {riskyCats.length > 0 && (
          <section className="glass-1 mx-auto mt-4 max-w-5xl rounded-card">
            <button
              type="button"
              onClick={() => setRiskyExpanded(!riskyOpen)}
              aria-expanded={riskyOpen}
              className="focus-ring flex w-full items-center gap-2 rounded-card px-5 py-4 text-left"
            >
              {riskyOpen ? (
                <ChevronDown size={16} className="text-ink-2" />
              ) : (
                <ChevronRight size={16} className="text-ink-2" />
              )}
              <span className="text-card font-semibold text-ink">Risky ({riskyCats.length})</span>
              <span className="text-body text-ink-2">
                Review these before cleaning. They are never selected for you.
              </span>
              <span className="nums ml-auto text-caption text-ink-2">{formatSize(sizeOf(riskyCats))}</span>
            </button>
            {riskyOpen && (
              <ul className="divide-y divide-hairline border-t border-hairline px-2 pb-2">
                {riskyCats.map(renderCard)}
              </ul>
            )}
          </section>
        )}
      </div>

      <div className="flex flex-col items-center gap-2 pb-2 pt-1">
        <span className="nums text-caption text-ink-2">
          {totals.items} {plural(totals.items, 'item')} selected, {formatSize(totals.size)}
        </span>
        <ScanLens
          state="idle"
          hue={cleanup.hue}
          label="Clean"
          disabled={totals.items === 0}
          onScan={() => useCleanStore.getState().openConfirm()}
        />
      </div>
    </div>
  );
}
