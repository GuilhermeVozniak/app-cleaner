import { useEffect, type CSSProperties } from 'react';
import { HardDrive, Mail, ShieldCheck, Trash2 } from 'lucide-react';
import { OpenFDASettings } from '../../wailsjs/go/main/App';
import { useUiStore } from '../stores/uiStore';
import { ModuleIcon } from '../components/ModuleIcon';
import { Button } from '../components/ui/button';

const LOCATIONS = [
  { Icon: Trash2, label: 'Trash' },
  { Icon: HardDrive, label: 'Safari cache' },
  { Icon: Mail, label: 'Mail attachments' },
];

/**
 * First-run gate: a full-height hero in the Smart Care hue asking for Full
 * Disk Access. The shell paints the canvas and title strip; there is no rail.
 */
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
    if (fda === true) setView('dashboard');
  }, [fda, setView]);

  return (
    <div
      className="relative flex h-full flex-col overflow-hidden"
      style={{ '--module': 'var(--color-module-care)' } as CSSProperties}
    >
      <div className="materialize flex flex-1 items-center justify-center gap-20 px-16 pb-16">
        <ModuleIcon Icon={ShieldCheck} size="hero" hue="var(--color-module-care)" />
        <div className="max-w-[26rem]">
          <h1 className="text-display font-normal text-ink">Grant Full Disk Access</h1>
          <p className="mt-3 text-card text-ink-2">
            App Cleaner needs Full Disk Access to scan everything it can clean. Without it, some
            locations cannot be read:
          </p>
          <ul className="mt-7 space-y-4">
            {LOCATIONS.map(({ Icon, label }) => (
              <li key={label} className="flex items-center gap-3 text-card font-semibold text-ink">
                <ModuleIcon Icon={Icon} size="sm" />
                {label}
              </li>
            ))}
          </ul>
          <div className="mt-8 flex items-center gap-3">
            <Button type="button" variant="primary" onClick={() => OpenFDASettings()}>
              Open System Settings
            </Button>
            <Button type="button" variant="secondary" onClick={() => void refreshFda()}>
              Re-check
            </Button>
          </div>
          <div className="mt-3">
            <Button
              type="button"
              variant="ghost"
              onClick={() => setView('dashboard')}
              className="-ml-2 px-2 text-ink-3 hover:bg-transparent hover:text-ink"
            >
              Continue without
            </Button>
          </div>
          <p className="mt-5 text-caption text-ink-2">
            System Settings, then Privacy &amp; Security, then Full Disk Access: enable App Cleaner and
            come back here.
          </p>
        </div>
      </div>
    </div>
  );
}
