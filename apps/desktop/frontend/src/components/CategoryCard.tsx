import { ChevronRight } from 'lucide-react';
import { Badge } from './ui/badge';
import SizeBar from './SizeBar';
import { formatSize } from '../lib/format';
import type { ScanResult, SafetyLevel } from '../lib/types';

interface Props {
  result: ScanResult;
  itemCount: number;
  selected: boolean;
  maxSize: number;
  onToggle: () => void;
  onOpen: () => void; // click-through to the CategoryDetail view
}

const SAFETY_LABELS: Record<SafetyLevel, string> = {
  safe: 'Safe',
  moderate: 'Moderate',
  risky: 'Risky',
};

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
    <div className="glass-1 flex items-center gap-3 rounded-control px-3 py-2.5">
      <input
        type="checkbox"
        aria-label={`Select ${category.name}`}
        checked={selected}
        onChange={onToggle}
        className="h-4 w-4 accent-accent"
      />
      <button
        type="button"
        onClick={onOpen}
        className="flex min-w-0 flex-1 items-center gap-3 text-left"
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium text-ink">{category.name}</span>
            <Badge variant={category.safetyLevel}>{SAFETY_LABELS[category.safetyLevel]}</Badge>
          </div>
          <div className="mt-1 flex items-center gap-2">
            <div className="w-40 shrink-0">
              <SizeBar size={result.totalSize} maxSize={maxSize} />
            </div>
            <span className="nums text-xs text-ink-2">
              {itemCount} items · {formatSize(result.totalSize)}
            </span>
          </div>
          {result.error && <div className="mt-1 text-xs text-danger">{result.error}</div>}
        </div>
        <ChevronRight size={16} className="shrink-0 text-ink-2" />
      </button>
    </div>
  );
}
