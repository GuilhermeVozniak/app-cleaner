import type { SafetyLevel } from '../lib/types';
import { Badge } from './ui/badge';

const LABELS: Record<SafetyLevel, string> = {
  safe: 'Safe',
  moderate: 'Moderate',
  risky: 'Risky',
};

export default function SafetyBadge({ level }: { level: SafetyLevel }) {
  return <Badge variant={level}>{LABELS[level]}</Badge>;
}
