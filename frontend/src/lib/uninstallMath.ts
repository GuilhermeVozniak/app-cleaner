import type { AppInfo } from './types'

// Display-only ~ contraction: the engine only ever returns $HOME-contained
// related paths, so a leading /Users/<name> segment is the home dir.
export function contractHome(p: string): string {
  return p.replace(/^\/Users\/[^/]+(?=\/|$)/, '~')
}

export function selectionTotals(
  apps: AppInfo[],
  names: ReadonlySet<string>,
): { size: number; paths: number } {
  let size = 0
  let paths = 0
  for (const app of apps) {
    if (!names.has(app.name)) continue
    size += app.appSize + app.relatedPaths.reduce((acc, r) => acc + r.size, 0)
    paths += 1 + app.relatedPaths.length
  }
  return { size, paths }
}

export function anySelectedRunning(apps: AppInfo[], names: ReadonlySet<string>): boolean {
  return apps.some((a) => names.has(a.name) && a.running)
}
