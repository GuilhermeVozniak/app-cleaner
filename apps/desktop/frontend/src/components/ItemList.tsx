import { Copy, Folder } from 'lucide-react';
import { CopyPath, RevealInFinder } from '../../wailsjs/go/main/App';
import { Button } from './ui/button';
import { Checkbox } from './ui/checkbox';
import { formatSize, middleTruncate } from '../lib/format';
import { contractHome, homeDir } from '../lib/paths';
import type { CleanableItem } from '../lib/types';

interface RowProps {
  name: string;
  path: string;
  size: number;
  selectable: boolean;
  checked: boolean;
  onToggle: () => void;
  ariaLabel?: string;
}

/** One file/folder line: checkbox, name, contracted path, hover actions, size. */
export function FileRow({ name, path, size, selectable, checked, onToggle, ariaLabel }: RowProps) {
  return (
    <li className="group flex h-11 items-center gap-3 px-4 text-body">
      {selectable && (
        <Checkbox aria-label={ariaLabel ?? `Select ${name}`} checked={checked} onCheckedChange={onToggle} />
      )}
      <span className="w-60 shrink-0 truncate font-semibold text-ink">{name}</span>
      <span className="min-w-0 flex-1 truncate text-caption text-ink-2" title={path}>
        {middleTruncate(contractHome(path, homeDir()), 50)}
      </span>
      <span className="hidden shrink-0 items-center gap-0.5 group-hover:flex">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          title="Reveal in Finder"
          onClick={() => RevealInFinder(path)}
          className="h-7 w-7 px-0"
        >
          <Folder size={14} />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          title="Copy path"
          onClick={() => CopyPath(path)}
          className="h-7 w-7 px-0"
        >
          <Copy size={14} />
        </Button>
      </span>
      <span className="nums w-20 shrink-0 text-right text-ink-2">{formatSize(size)}</span>
    </li>
  );
}

interface Props {
  items: CleanableItem[];
  selectable: boolean; // false for docker (rows informational only)
  isChecked: (path: string) => boolean;
  onToggle: (path: string) => void;
}

export default function ItemList({ items, selectable, isChecked, onToggle }: Props) {
  return (
    <ul className="divide-y divide-hairline">
      {items.map((item) => (
        <FileRow
          key={item.path}
          name={item.name}
          path={item.path}
          size={item.size}
          selectable={selectable}
          checked={isChecked(item.path)}
          onToggle={() => onToggle(item.path)}
        />
      ))}
    </ul>
  );
}
