import type { AppInfo } from './types'

// Display-only ~ contraction: the engine only ever returns $HOME-contained
// related paths, so a leading /Users/<name> segment is the home dir.
export function contractHome(p: string): string {
  return p.replace(/^\/Users\/[^/]+(?=\/|$)/, '~')
}

// Selection is keyed by bundle path, not display name: two installed apps
// can share a display name (e.g. the same app under /Applications and
// ~/Applications) but never a path.
export function selectionTotals(
  apps: AppInfo[],
  selectedPaths: ReadonlySet<string>,
): { size: number; paths: number } {
  let size = 0
  let paths = 0
  for (const app of apps) {
    if (!selectedPaths.has(app.path)) continue
    size += app.appSize + app.relatedPaths.reduce((acc, r) => acc + r.size, 0)
    paths += 1 + app.relatedPaths.length
  }
  return { size, paths }
}

export function anySelectedRunning(apps: AppInfo[], selectedPaths: ReadonlySet<string>): boolean {
  return apps.some((a) => selectedPaths.has(a.path) && a.running)
}
