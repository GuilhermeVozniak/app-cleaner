import { Copy, Folder } from 'lucide-react';
import { CopyPath, RevealInFinder } from '../../wailsjs/go/main/App';
import { Button } from './ui/button';
import { Checkbox } from './ui/checkbox';
import { formatSize, middleTruncate } from '../lib/format';
import { contractHome, homeDir } from '../lib/paths';
import type { CleanableItem } from '../lib/types';

interface Props {
  items: CleanableItem[];
  selectable: boolean; // false for docker (rows informational only)
  isChecked: (path: string) => boolean;
  onToggle: (path: string) => void;
}

export default function ItemList({ items, selectable, isChecked, onToggle }: Props) {
  return (
    <ul className="space-y-1">
      {items.map((item) => (
        <li
          key={item.path}
          className="glass-1 group flex items-center gap-3 rounded-control px-3 py-2 text-sm transition hover:brightness-105"
        >
          {selectable && (
            <Checkbox
              aria-label={`Select ${item.name}`}
              checked={isChecked(item.path)}
              onCheckedChange={() => onToggle(item.path)}
            />
          )}
          <span className="w-64 shrink-0 truncate font-medium text-ink">
            {item.name}
          </span>
          <span
            className="min-w-0 flex-1 truncate text-xs text-ink-2"
            title={item.path}
          >
            {middleTruncate(contractHome(item.path, homeDir()), 50)}
          </span>
          <span className="hidden shrink-0 items-center gap-1 group-hover:flex">
            <Button
              type="button"
              variant="ghost"
              title="Reveal in Finder"
              onClick={() => RevealInFinder(item.path)}
              className="h-auto w-auto rounded p-1 text-ink-2 hover:bg-hairline hover:text-ink"
            >
              <Folder size={14} />
            </Button>
            <Button
              type="button"
              variant="ghost"
              title="Copy path"
              onClick={() => CopyPath(item.path)}
              className="h-auto w-auto rounded p-1 text-ink-2 hover:bg-hairline hover:text-ink"
            >
              <Copy size={14} />
            </Button>
          </span>
          <span className="nums w-20 shrink-0 text-right text-ink-2">
            {formatSize(item.size)}
          </span>
        </li>
      ))}
    </ul>
  );
}
