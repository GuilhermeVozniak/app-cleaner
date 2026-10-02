import { ArrowDownToLine, ShieldAlert } from 'lucide-react';
import type { CSSProperties } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cn } from '../lib/cn';
import { MODULES, SETTINGS_MODULE, moduleForView } from '../lib/modules';
import { useUiStore } from '../stores/uiStore';
import { Tooltip } from './ui/tooltip';

/**
 * Icon-only module rail (92px, the width of the macOS traffic-light cluster
 * plus its margins, so the divider falls clear of the green button and the
 * tiles sit centred under the lights). Smart Care sits alone at the top, the
 * four modules below it, My Tools + Backups and Settings in a bottom cluster.
 * The active tile is a filled rounded square with a halo in its module's hue;
 * labels live in tooltips. Top padding clears the traffic lights.
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
    Icon: LucideIcon;
    onClick: () => void;
    dot?: boolean;
  }) => (
    <Tooltip key={opts.key} content={opts.label} side="right">
      <button
        type="button"
        aria-label={opts.label}
        aria-current={opts.active ? 'page' : undefined}
        onClick={opts.onClick}
        style={{ '--module': opts.hue } as CSSProperties}
        className={cn(
          'focus-ring relative flex h-11 w-11 items-center justify-center rounded-[13px] transition-colors duration-150',
          opts.active
            ? 'module-glow bg-fill text-ink'
            : 'text-ink-2 hover:bg-glass-1 hover:text-ink',
        )}
      >
        <opts.Icon size={21} strokeWidth={1.9} />
        {opts.dot ? (
          <span
            aria-hidden
            className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-safe shadow-[0_0_8px_var(--color-safe)]"
          />
        ) : null}
      </button>
    </Tooltip>
  );

  const [care, ...rest] = MODULES;
  const main = rest.filter((m) => m.view !== 'my-tools' && m.view !== 'backups');
  const cluster = rest.filter((m) => m.view === 'my-tools' || m.view === 'backups');
  const renderModule = (m: (typeof MODULES)[number]) =>
    tile({
      key: m.view,
      label: m.label,
      hue: m.hue,
      active: activeModule?.view === m.view,
      Icon: m.Icon,
      onClick: () => setView(m.view),
    });

  return (
    <aside className="relative z-20 flex w-[92px] shrink-0 flex-col items-center gap-1.5 border-r border-hairline pb-3 pt-[52px]">
      {renderModule(care)}
      <Hairline />
      {main.map(renderModule)}
      <div className="mt-auto flex flex-col items-center gap-1.5">
        {cluster.map(renderModule)}
        <Hairline />
        {update?.available && (
          <Tooltip content={`Update available: version ${update.latest}`} side="right">
            <button
              type="button"
              aria-label={`Update available: version ${update.latest}`}
              onClick={() => setView('settings')}
              className="focus-ring flex h-11 w-11 items-center justify-center rounded-[13px] text-safe transition-colors hover:bg-glass-1"
            >
              <ArrowDownToLine size={20} strokeWidth={2} />
            </button>
          </Tooltip>
        )}
        {fda === false && (
          <Tooltip content="Limited disk access. Click to fix." side="right">
            <button
              type="button"
              aria-label="Limited disk access"
              onClick={() => setView('first-run')}
              className="focus-ring flex h-11 w-11 items-center justify-center rounded-[13px] text-moderate transition-colors hover:bg-glass-1"
            >
              <ShieldAlert size={20} strokeWidth={2} />
            </button>
          </Tooltip>
        )}
        {tile({
          key: 'settings',
          label: SETTINGS_MODULE.label,
          hue: SETTINGS_MODULE.hue,
          active: view === 'settings',
          Icon: SETTINGS_MODULE.Icon,
          onClick: () => setView('settings'),
        })}
      </div>
    </aside>
  );
}

function Hairline() {
  return <span aria-hidden className="my-1 h-px w-7 bg-hairline" />;
}
