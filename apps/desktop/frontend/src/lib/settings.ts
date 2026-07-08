import type { Config } from './types'

export const MB = 1024 * 1024

// UI mirror of the internal/config bounds: invalid input falls back, out-of-range clamps.
export function clampInt(n: number, min: number, max: number, fallback: number): number {
  if (!Number.isFinite(n)) return fallback
  const i = Math.round(n)
  return Math.min(max, Math.max(min, i))
}

// largeFilesMinSize is edited in whole MB: 1 MB .. 102400 MB (= 100 GiB, the config ceiling).
export function mbToBytes(mb: number): number {
  return clampInt(mb, 1, 102400, 500) * MB
}

export function bytesToMb(bytes: number): number {
  return clampInt(bytes / MB, 1, 102400, 500)
}

export function parseKeepLanguages(input: string): string[] {
  const out: string[] = []
  for (const raw of input.split(',')) {
    const t = raw.trim()
    if (t && !out.includes(t)) out.push(t)
  }
  return out
}

export function formatKeepLanguages(langs: string[]): string {
  return langs.join(', ')
}

// One path per line; config caps extraPaths arrays at 50 entries each.
export function parseLines(input: string): string[] {
  return input
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l.length > 0)
    .slice(0, 50)
}

export interface SettingsForm {
  downloadsDaysOld: number
  largeFilesMB: number
  backupByDefault: boolean
  backupRetentionDays: number
  concurrency: number
  showRisky: boolean
  keepLanguages: string // comma-separated
  nodeModulesPaths: string // one path per line
  projectsPaths: string // one path per line
}

export function toForm(c: Config): SettingsForm {
  return {
    downloadsDaysOld: c.downloadsDaysOld,
    largeFilesMB: bytesToMb(c.largeFilesMinSize),
    backupByDefault: c.backupByDefault,
    backupRetentionDays: c.backupRetentionDays,
    concurrency: c.concurrency,
    showRisky: c.showRisky,
    keepLanguages: formatKeepLanguages(c.keepLanguages ?? []),
    nodeModulesPaths: (c.extraPaths?.nodeModules ?? []).join('\n'),
    projectsPaths: (c.extraPaths?.projects ?? []).join('\n'),
  }
}

export function fromForm(f: SettingsForm): Config {
  return {
    downloadsDaysOld: clampInt(f.downloadsDaysOld, 1, 365, 30),
    largeFilesMinSize: mbToBytes(f.largeFilesMB),
    backupByDefault: f.backupByDefault,
    backupRetentionDays: clampInt(f.backupRetentionDays, 1, 365, 7),
    concurrency: clampInt(f.concurrency, 1, 16, 4),
    showRisky: f.showRisky,
    keepLanguages: parseKeepLanguages(f.keepLanguages),
    extraPaths: {
      nodeModules: parseLines(f.nodeModulesPaths),
      projects: parseLines(f.projectsPaths),
    },
  }
}
