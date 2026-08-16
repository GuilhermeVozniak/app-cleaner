import { Settings, ShieldAlert } from 'lucide-react';
import type { CSSProperties } from 'react';
import { cn } from '../lib/cn';
import { MODULES, moduleForView } from '../lib/modules';
import { useUiStore } from '../stores/uiStore';
import { Tooltip } from './ui/tooltip';

/**
 * CleanMyMac-style icon rail: a narrow strip of module tiles. The active
 * tile is a filled rounded-square tinted with the module's own hue; labels
 * live in tooltips so the rail stays quiet.
 */
export default function Sidebar() {
  const view = useUiStore((s) => s.view);
  const fda = useUiStore((s) => s.fda);
  const setView = useUiStore((s) => s.setView);
  const activeModule = moduleForView(view);

  const tile = (opts: {
    key: string;
    label: string;
    hue: string;
    active: boolean;
    Icon: typeof Settings;
    onClick: () => void;
  }) => (
    <Tooltip key={opts.key} content={opts.label}>
      <button
        type="button"
        aria-label={opts.label}
        aria-current={opts.active ? 'page' : undefined}
        onClick={opts.onClick}
        style={{ '--module': opts.hue } as CSSProperties}
        className={cn(
          'flex h-10 w-10 items-center justify-center rounded-[12px] transition-colors duration-150',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
          opts.active
            ? 'module-glow bg-[color-mix(in_srgb,var(--module)_22%,transparent)] text-[var(--module)]'
            : 'text-ink-2 hover:bg-hairline hover:text-ink',
        )}
      >
        <opts.Icon size={20} />
      </button>
    </Tooltip>
  );

  return (
    <aside className="glass-2 z-10 m-2.5 mr-0 flex w-14 shrink-0 flex-col items-center gap-1.5 rounded-shell pb-3 pt-9">
      {MODULES.map((m) =>
        tile({
          key: m.view,
          label: m.label,
          hue: m.hue,
          active: activeModule?.view === m.view,
          Icon: m.Icon,
          onClick: () => setView(m.view),
        }),
      )}
      <div className="mt-auto flex flex-col items-center gap-1.5">
        {fda === false && (
          <Tooltip content="Limited disk access — click to fix">
            <button
              type="button"
              aria-label="Limited disk access"
              onClick={() => setView('first-run')}
              className="flex h-10 w-10 items-center justify-center rounded-[12px] text-moderate hover:bg-hairline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <ShieldAlert size={18} />
            </button>
          </Tooltip>
        )}
        {tile({
          key: 'settings',
          label: 'Settings',
          hue: 'var(--color-accent)',
          active: view === 'settings',
          Icon: Settings,
          onClick: () => setView('settings'),
        })}
      </div>
    </aside>
  );
}
