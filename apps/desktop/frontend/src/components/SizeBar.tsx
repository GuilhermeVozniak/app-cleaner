import { Progress } from './ui/progress';

interface Props {
  size: number;
  maxSize: number; // largest category size on screen; bar width is proportional
}

/** Thin wrapper over the glass-system Progress bar: renders a category's
 * size proportional to the largest category on screen. */
export default function SizeBar({ size, maxSize }: Props) {
  const pct = maxSize > 0 ? Math.max((size / maxSize) * 100, size > 0 ? 2 : 0) : 0;
  return <Progress value={Math.min(pct, 100)} className="h-1.5" />;
}
