import { useEffect } from 'react';
import { HardDrive, Mail, ShieldCheck, Trash2 } from 'lucide-react';
import { OpenFDASettings } from '../../wailsjs/go/main/App';
import { useUiStore } from '../stores/uiStore';

export default function FirstRun() {
  const fda = useUiStore((s) => s.fda);
  const setView = useUiStore((s) => s.setView);
  const refreshFda = useUiStore((s) => s.refreshFda);

  // Re-check whenever the window regains focus (user returning from System Settings).
  useEffect(() => {
    const onFocus = () => void refreshFda();
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, [refreshFda]);

  // Granted while on this screen -> continue automatically.
  useEffect(() => {
    if (fda === true) setView('smart-scan');
  }, [fda, setView]);

  return (
    <div className="flex h-full flex-col items-center justify-center gap-6 px-10 text-center">
      <ShieldCheck size={56} className="text-blue-600 dark:text-blue-400" />
      <h1 className="text-2xl font-bold text-neutral-900 dark:text-neutral-100">
        Grant Full Disk Access
      </h1>
      <p className="max-w-md text-sm text-neutral-600 dark:text-neutral-400">
        App Cleaner needs Full Disk Access to scan everything it can clean. Without it, some
        locations cannot be read:
      </p>
      <ul className="flex gap-6 text-sm text-neutral-700 dark:text-neutral-300">
        <li className="flex items-center gap-2">
          <Trash2 size={16} /> Trash
        </li>
        <li className="flex items-center gap-2">
          <HardDrive size={16} /> Safari cache
        </li>
        <li className="flex items-center gap-2">
          <Mail size={16} /> Mail attachments
        </li>
      </ul>
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => OpenFDASettings()}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          Open System Settings
        </button>
        <button
          type="button"
          onClick={() => void refreshFda()}
          className="rounded-md border border-neutral-300 px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
        >
          Re-check
        </button>
        <button
          type="button"
          onClick={() => setView('smart-scan')}
          className="px-2 py-2 text-sm text-neutral-500 hover:text-neutral-700 dark:text-neutral-400 dark:hover:text-neutral-200"
        >
          Continue without
        </button>
      </div>
      <p className="text-xs text-neutral-400 dark:text-neutral-500">
        System Settings → Privacy &amp; Security → Full Disk Access → enable App Cleaner, then
        return here.
      </p>
    </div>
  );
}
