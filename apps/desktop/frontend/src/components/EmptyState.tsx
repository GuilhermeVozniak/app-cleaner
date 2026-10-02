import type { LucideIcon } from 'lucide-react';
import { Sparkles } from 'lucide-react';
import type { ReactNode } from 'react';
import { ModuleIcon } from './ModuleIcon';

interface Props {
  title: string;
  subtitle?: string;
  icon?: LucideIcon;
  /** Optional action under the copy (e.g. Scan again). */
  action?: ReactNode;
}

/** Centred "nothing here" moment in the module's colour. */
export default function EmptyState({ title, subtitle, icon: Icon = Sparkles, action }: Props) {
  return (
    <div className="materialize flex h-full flex-col items-center justify-center gap-4 px-10 text-center">
      <ModuleIcon Icon={Icon} size="lg" />
      <div>
        <div className="text-headline font-semibold text-ink">{title}</div>
        {subtitle && <div className="mt-1.5 text-card text-ink-2">{subtitle}</div>}
      </div>
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
