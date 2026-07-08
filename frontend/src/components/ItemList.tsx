import { Copy, Folder } from 'lucide-react';
import { CopyPath, RevealInFinder } from '../../wailsjs/go/main/App';
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
    <ul className="divide-y divide-neutral-100 dark:divide-neutral-800">
      {items.map((item) => (
        <li key={item.path} className="group flex items-center gap-3 px-2 py-1.5 text-sm">
          {selectable && (
            <input
              type="checkbox"
              aria-label={`Select ${item.name}`}
              checked={isChecked(item.path)}
              onChange={() => onToggle(item.path)}
              className="h-4 w-4 accent-blue-600"
            />
          )}
          <span className="w-64 shrink-0 truncate font-medium text-neutral-900 dark:text-neutral-100">
            {item.name}
          </span>
          <span
            className="min-w-0 flex-1 truncate text-xs text-neutral-400 dark:text-neutral-500"
            title={item.path}
          >
            {middleTruncate(contractHome(item.path, homeDir()), 50)}
          </span>
          <span className="hidden shrink-0 items-center gap-1 group-hover:flex">
            <button
              type="button"
              title="Reveal in Finder"
              onClick={() => RevealInFinder(item.path)}
              className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
            >
              <Folder size={14} />
            </button>
            <button
              type="button"
              title="Copy path"
              onClick={() => CopyPath(item.path)}
              className="rounded p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700 dark:hover:bg-neutral-800 dark:hover:text-neutral-200"
            >
              <Copy size={14} />
            </button>
          </span>
          <span className="w-20 shrink-0 text-right text-neutral-500 dark:text-neutral-400">
            {formatSize(item.size)}
          </span>
        </li>
      ))}
    </ul>
  );
}
