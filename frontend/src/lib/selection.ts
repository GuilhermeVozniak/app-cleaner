import type { Selection } from '../stores/scanStore';

/** CLI file-picker parity: 5 files visible per directory by default… */
export const DEFAULT_DIR_LIMIT = 5;
/** …and every expand bumps the per-directory limit by 10 ('m' key / expand-hint). */
export const EXPAND_INCREMENT = 10;

/** Returns a NEW expand map with limit[directoryKey] = (existing ?? 5) + 10. */
export function bumpExpand(
  expand: Record<string, number>,
  directoryKey: string,
): Record<string, number> {
  return {
    ...expand,
    [directoryKey]: (expand[directoryKey] ?? DEFAULT_DIR_LIMIT) + EXPAND_INCREMENT,
  };
}

/**
 * Toggle one path within a category selection.
 * `allPaths` = every selectable path in the category ('all' materializes into a Set).
 */
export function togglePath(
  allPaths: string[],
  sel: Selection | undefined,
  path: string,
): Selection {
  if (sel === 'all') {
    const next = new Set(allPaths);
    next.delete(path);
    return next;
  }
  const next = new Set(sel ?? []);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  return next;
}

/** Invert: every unselected selectable path becomes selected and vice versa. */
export function invertSelection(allPaths: string[], sel: Selection | undefined): Selection {
  if (sel === 'all') return new Set<string>();
  const current = sel ?? new Set<string>();
  return new Set(allPaths.filter((p) => !current.has(p)));
}
