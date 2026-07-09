import type { SafetyLevel } from '../lib/types';

const STYLES: Record<SafetyLevel, string> = {
  safe: 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-400',
  moderate: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400',
  risky: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-400',
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
