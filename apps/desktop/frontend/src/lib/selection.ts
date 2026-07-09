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

/**
 * CLI file-picker parity: the 'd' key toggles every currently VISIBLE file in
 * one directory group (a DisplayRow.directoryKey). `visiblePaths` must be the
 * paths of the rows actually rendered for that group — files hidden behind an
 * expand-hint are not included, so they are left untouched, exactly like the
 * CLI's toggleDirectoryFiles (file-picker.ts). If every visible file in the
 * group is already selected, they are all deselected; otherwise all selected.
 * Other directory groups, and files outside `visiblePaths`, are never touched.
 *
 * Note: unlike the CLI (which keeps a separate `selectedCategories` set that
 * directory-toggling deliberately never touches), this port has a single
 * per-category Selection, so category membership here is simply whatever the
 * resulting Set implies — there is no separate flag to preserve.
 */
export function toggleDirectory(
  allPaths: string[],
  sel: Selection | undefined,
  visiblePaths: string[],
): Selection {
  const current = sel === 'all' ? new Set(allPaths) : new Set(sel ?? []);
  const allVisibleSelected = visiblePaths.length > 0 && visiblePaths.every((p) => current.has(p));
  if (allVisibleSelected) {
    for (const p of visiblePaths) current.delete(p);
  } else {
    for (const p of visiblePaths) current.add(p);
  }
  return current;
}
