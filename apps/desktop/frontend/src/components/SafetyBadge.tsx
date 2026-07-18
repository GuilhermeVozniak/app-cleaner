import type { SafetyLevel } from '../lib/types';

const STYLES: Record<SafetyLevel, string> = {
  safe: 'bg-safe/15 text-safe',
  moderate: 'bg-moderate/15 text-moderate',
  risky: 'bg-risky/15 text-risky',
};

const LABELS: Record<SafetyLevel, string> = {
  safe: 'Safe',
  moderate: 'Moderate',
  risky: 'Risky',
};

export default function SafetyBadge({ level }: { level: SafetyLevel }) {
  return (
    <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${STYLES[level]}`}>
      {LABELS[level]}
    </span>
  );
}
