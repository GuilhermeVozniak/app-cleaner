import { useEffect } from 'react';
import { HardDrive, Mail, ShieldCheck, Trash2 } from 'lucide-react';
import { OpenFDASettings } from '../../wailsjs/go/main/App';
import { useUiStore } from '../stores/uiStore';
import { Button } from '../components/ui/button';
import { Card } from '../components/ui/card';

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
    <div className="flex h-full flex-col items-center justify-center px-10">
      <Card className="flex max-w-md flex-col items-center gap-6 p-8 text-center">
        <ShieldCheck size={56} className="text-accent" />
        <h1 className="text-2xl font-bold text-ink">
          Grant Full Disk Access
        </h1>
        <p className="text-sm text-ink-2">
          App Cleaner needs Full Disk Access to scan everything it can clean. Without it, some
          locations cannot be read:
        </p>
        <ul className="flex gap-6 text-sm text-ink">
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
          <Button type="button" variant="primary" onClick={() => OpenFDASettings()}>
            Open System Settings
          </Button>
          <Button type="button" variant="ghost" onClick={() => void refreshFda()}>
            Re-check
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={() => setView('smart-scan')}
            className="h-auto px-2 py-2 text-ink-2 hover:bg-transparent hover:text-ink"
          >
            Continue without
          </Button>
        </div>
        <p className="text-xs text-ink-2">
          System Settings → Privacy &amp; Security → Full Disk Access → enable App Cleaner, then
          return here.
        </p>
      </Card>
    </div>
  );
}
