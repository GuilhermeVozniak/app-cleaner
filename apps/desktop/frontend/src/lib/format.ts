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

/** Middle-elide a string to exactly maxLen chars using '...' when it overflows. */
export function middleTruncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  const keep = maxLen - 3;
  const head = Math.ceil(keep / 2);
  const tail = Math.floor(keep / 2);
  return `${s.slice(0, head)}...${s.slice(s.length - tail)}`;
}

/** Coarse relative time for the "Updated …" hint. '' on unparseable input. */
export function timeAgo(iso: string, now: number = Date.now()): string {
  const t = Date.parse(iso)
  if (!Number.isFinite(t)) return ''
  const s = Math.max(0, Math.floor((now - t) / 1000))
  if (s < 60) return 'just now'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} min ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} h ago`
  return `${Math.floor(h / 24)} d ago`
}
