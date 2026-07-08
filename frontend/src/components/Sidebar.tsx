import { Archive, Settings, ShieldAlert, Sparkles, Trash2, Wrench } from 'lucide-react';
import { useUiStore, type View } from '../stores/uiStore';

const NAV: Array<{ view: View; label: string; Icon: typeof Sparkles }> = [
  { view: 'smart-scan', label: 'Smart Scan', Icon: Sparkles },
  { view: 'uninstaller', label: 'Uninstaller', Icon: Trash2 },
  { view: 'maintenance', label: 'Maintenance', Icon: Wrench },
  { view: 'backups', label: 'Backups', Icon: Archive },
  { view: 'settings', label: 'Settings', Icon: Settings },
];

export default function Sidebar() {
  const view = useUiStore((s) => s.view);
  const fda = useUiStore((s) => s.fda);
  const setView = useUiStore((s) => s.setView);

  return (
    <aside className="flex w-52 shrink-0 flex-col border-r border-neutral-200 bg-neutral-50 pt-10 dark:border-neutral-800 dark:bg-neutral-950">
      <div className="px-4 pb-4 text-sm font-semibold text-neutral-500 dark:text-neutral-400">
        App Cleaner
      </div>
      <nav className="flex flex-col gap-1 px-2">
        {NAV.map(({ view: v, label, Icon }) => {
          const active = view === v || (v === 'smart-scan' && view === 'category');
          return (
            <button
              key={v}
              type="button"
              onClick={() => setView(v)}
              className={`flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
                active
                  ? 'bg-blue-600 text-white'
                  : 'text-neutral-700 hover:bg-neutral-200 dark:text-neutral-300 dark:hover:bg-neutral-800'
              }`}
            >
              <Icon size={16} />
              <span className="flex-1">{label}</span>
            </button>
          );
        })}
      </nav>
      {fda === false && (
        <button
          type="button"
          onClick={() => setView('first-run')}
          className="mx-2 mt-auto mb-3 flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs text-amber-600 hover:bg-neutral-200 dark:text-amber-400 dark:hover:bg-neutral-800"
        >
          <span className="h-2 w-2 shrink-0 rounded-full bg-amber-500" />
          <span className="flex-1">Limited disk access</span>
          <ShieldAlert size={14} />
        </button>
      )}
    </aside>
  );
}
