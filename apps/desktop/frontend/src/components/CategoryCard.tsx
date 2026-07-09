import { ChevronRight } from 'lucide-react';
import SafetyBadge from './SafetyBadge';
import SizeBar from './SizeBar';
import { formatSize } from '../lib/format';
import type { ScanResult } from '../lib/types';

interface Props {
  result: ScanResult;
  itemCount: number;
  selected: boolean;
  maxSize: number;
  onToggle: () => void;
  onOpen: () => void; // click-through to the CategoryDetail view
}

export default function CategoryCard({
  result,
  itemCount,
  selected,
  maxSize,
  onToggle,
  onOpen,
}: Props) {
  const { category } = result;
  return (
    <div className="flex items-center gap-3 rounded-lg border border-neutral-200 bg-white px-3 py-2.5 hover:border-neutral-300 dark:border-neutral-800 dark:bg-neutral-900 dark:hover:border-neutral-700">
      <input
        type="checkbox"
        aria-label={`Select ${category.name}`}
        checked={selected}
        onChange={onToggle}
        className="h-4 w-4 accent-blue-600"
      />
      <button
        type="button"
        onClick={onOpen}
        className="flex min-w-0 flex-1 items-center gap-3 text-left"
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium text-neutral-900 dark:text-neutral-100">
              {category.name}
            </span>
            <SafetyBadge level={category.safetyLevel} />
          </div>
          <div className="mt-1 flex items-center gap-2">
            <div className="w-40 shrink-0">
              <SizeBar size={result.totalSize} maxSize={maxSize} />
            </div>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">
              {itemCount} items · {formatSize(result.totalSize)}
            </span>
          </div>
          {result.error && (
            <div className="mt-1 text-xs text-amber-600 dark:text-amber-400">{result.error}</div>
          )}
        </div>
        <ChevronRight size={16} className="shrink-0 text-neutral-400" />
      </button>
    </div>
  );
}
