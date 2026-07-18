import type { LucideIcon } from 'lucide-react';
import { Sparkles } from 'lucide-react';

interface Props {
  title: string;
  subtitle?: string;
  icon?: LucideIcon;
}

export default function EmptyState({ title, subtitle, icon: Icon = Sparkles }: Props) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
      <Icon size={48} className="text-safe" />
      <div className="text-lg font-semibold text-ink">{title}</div>
      {subtitle && <div className="text-sm text-ink-2">{subtitle}</div>}
    </div>
  );
}
