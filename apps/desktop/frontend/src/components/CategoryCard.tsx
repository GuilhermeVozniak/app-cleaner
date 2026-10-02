import { ChevronRight } from 'lucide-react';
import { Badge } from './ui/badge';
import { Checkbox } from './ui/checkbox';
import { formatSize } from '../lib/format';
import type { ScanResult, SafetyLevel } from '../lib/types';

interface Props {
  result: ScanResult;
  itemCount: number;
  selected: boolean;
  onToggle: () => void;
  onOpen: () => void; // click-through to the CategoryDetail view
}

const SAFETY_LABELS: Record<SafetyLevel, string> = {
  safe: 'Safe',
  moderate: 'Moderate',
  risky: 'Risky',
};

/** One category row inside a result group: checkbox, name, item count, size, chevron. */
export default function CategoryCard({ result, itemCount, selected, onToggle, onOpen }: Props) {
  const { category } = result;
  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      <Checkbox
        aria-label={`Select ${category.name}`}
        checked={selected}
        onCheckedChange={onToggle}
      />
      <button
        type="button"
        onClick={onOpen}
        className="focus-ring group flex min-w-0 flex-1 items-center gap-3 rounded-control text-left"
      >
        <span className="min-w-0 flex-1">
          <span className="block truncate text-body font-semibold text-ink">{category.name}</span>
          <span className="mt-0.5 flex items-center gap-2">
            <span className="nums text-caption text-ink-2">
              {itemCount} {itemCount === 1 ? 'item' : 'items'}
            </span>
            {category.safetyLevel !== 'safe' && (
              <Badge variant={category.safetyLevel}>{SAFETY_LABELS[category.safetyLevel]}</Badge>
            )}
          </span>
          {result.error && <span className="block text-caption text-moderate">{result.error}</span>}
        </span>
        <span className="nums shrink-0 text-body text-ink">{formatSize(result.totalSize)}</span>
        <ChevronRight
          size={16}
          className="shrink-0 text-ink-3 transition-[transform,color] group-hover:translate-x-0.5 group-hover:text-ink"
        />
      </button>
    </li>
  );
}
