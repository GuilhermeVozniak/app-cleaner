import { ArrowDownToLine, Settings, ShieldAlert } from 'lucide-react';
import type { CSSProperties } from 'react';
import { cn } from '../lib/cn';
import { MODULES, moduleForView } from '../lib/modules';
import { useUiStore } from '../stores/uiStore';
import { Tooltip } from './ui/tooltip';

/**
 * CleanMyMac-style module rail: labeled icon tiles on a dark glass strip.
 * The active tile is a filled rounded-square tinted with the module's own
 * hue. The rail's top padding (pt-12 on top of the m-2.5 offset) keeps the
 * first tile clear of the macOS traffic lights, which overlay the hidden-
 * inset title bar at the window's top-left.
 */
export default function Sidebar() {
  const view = useUiStore((s) => s.view);
  const fda = useUiStore((s) => s.fda);
  const update = useUiStore((s) => s.update);
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
    <button
      key={opts.key}
      type="button"
      aria-label={opts.label}
      aria-current={opts.active ? 'page' : undefined}
      onClick={opts.onClick}
      style={{ '--module': opts.hue } as CSSProperties}
      className={cn(
        'flex w-[74px] flex-col items-center gap-1 rounded-[14px] px-1 py-2 transition-colors duration-150',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
        opts.active
          ? 'module-glow bg-[color-mix(in_srgb,var(--module)_22%,transparent)] text-[var(--module)]'
          : 'text-ink-2 hover:bg-hairline hover:text-ink',
      )}
    >
      <opts.Icon size={22} />
      <span className="w-full truncate text-center text-[10px] font-medium leading-tight">
        {opts.label}
      </span>
    </button>
  );

  return (
    <aside className="glass-2 z-10 m-2.5 mr-0 flex w-[92px] shrink-0 flex-col items-center gap-1 rounded-shell pb-3 pt-12">
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
      <div className="mt-auto flex flex-col items-center gap-1">
        {update?.available && (
          <Tooltip content={`Update available — v${update.latest}`}>
            <button
              type="button"
              aria-label={`Update available: version ${update.latest}`}
              onClick={() => setView('settings')}
              className="flex w-[74px] flex-col items-center gap-1 rounded-[14px] px-1 py-2 text-safe hover:bg-hairline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <ArrowDownToLine size={20} />
              <span className="text-[10px] font-medium leading-tight">Update</span>
            </button>
          </Tooltip>
        )}
        {fda === false && (
          <Tooltip content="Limited disk access — click to fix">
            <button
              type="button"
              aria-label="Limited disk access"
              onClick={() => setView('first-run')}
              className="flex w-[74px] flex-col items-center gap-1 rounded-[14px] px-1 py-2 text-moderate hover:bg-hairline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <ShieldAlert size={18} />
              <span className="text-[10px] font-medium leading-tight">Access</span>
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
