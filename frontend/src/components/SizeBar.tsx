interface Props {
  size: number;
  maxSize: number; // largest category size on screen; bar width is proportional
}

export default function SizeBar({ size, maxSize }: Props) {
  const pct = maxSize > 0 ? Math.max((size / maxSize) * 100, size > 0 ? 2 : 0) : 0;
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-neutral-200 dark:bg-neutral-800">
      <div
        className="h-full rounded-full bg-blue-500 dark:bg-blue-400"
        style={{ width: `${Math.min(pct, 100)}%` }}
      />
    </div>
  );
}
