const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

/**
 * Mirrors internal/core FormatSize: base-1024; "512 B" (0 decimals),
 * "1.5 KB" / "2.0 GB" (exactly 1 decimal above bytes). Index clamped to TB.
 */
export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  let i = Math.floor(Math.log(bytes) / Math.log(1024));
  if (i < 0) i = 0;
  if (i > UNITS.length - 1) i = UNITS.length - 1;
  const value = bytes / Math.pow(1024, i);
  return `${value.toFixed(i > 0 ? 1 : 0)} ${UNITS[i]}`;
}
