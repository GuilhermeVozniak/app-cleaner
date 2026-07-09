import { describe, it, expect } from 'vitest'
import {
  clampInt,
  mbToBytes,
  bytesToMb,
  parseKeepLanguages,
  formatKeepLanguages,
  parseLines,
  toForm,
  fromForm,
} from './settings'
import type { Config } from './types'

describe('MB <-> bytes conversion', () => {
  it('converts and clamps to the config bounds (1..102400 MB)', () => {
    expect(mbToBytes(500)).toBe(524288000)
    expect(bytesToMb(524288000)).toBe(500)
    expect(mbToBytes(0)).toBe(1048576) // clamped up to 1 MB
    expect(mbToBytes(999999)).toBe(102400 * 1048576) // clamped to 100 GiB
    expect(mbToBytes(Number.NaN)).toBe(500 * 1048576) // fallback = 500 MB default
  })

  it('round-trips', () => {
    expect(bytesToMb(mbToBytes(123))).toBe(123)
  })
})

describe('clampInt', () => {
  it('clamps, rounds and falls back on non-finite input', () => {
    expect(clampInt(0, 1, 365, 30)).toBe(1)
    expect(clampInt(400, 1, 365, 30)).toBe(365)
    expect(clampInt(4.6, 1, 16, 4)).toBe(5)
    expect(clampInt(Number.NaN, 1, 16, 4)).toBe(4)
  })
})

describe('keepLanguages parsing', () => {
  it('parses comma-separated tags, trimming, dropping empties and deduping', () => {
    expect(parseKeepLanguages('pt-BR, de , ,fr,de')).toEqual(['pt-BR', 'de', 'fr'])
    expect(parseKeepLanguages('')).toEqual([])
  })

  it('round-trips through format + parse', () => {
    const langs = ['pt-BR', 'de', 'fr']
    expect(parseKeepLanguages(formatKeepLanguages(langs))).toEqual(langs)
  })
})

describe('form mapping', () => {
  const cfg: Config = {
    downloadsDaysOld: 45,
    largeFilesMinSize: 524288000,
    backupByDefault: true,
    backupRetentionDays: 14,
    concurrency: 8,
    showRisky: true,
    keepLanguages: ['pt-BR'],
    extraPaths: { nodeModules: ['/Users/me/dev'], projects: ['/Users/me/work'] },
  }

  it('fromForm(toForm(cfg)) round-trips an in-bounds config', () => {
    expect(fromForm(toForm(cfg))).toEqual(cfg)
  })

  it('parseLines splits textarea input, dropping blanks and capping at 50', () => {
    expect(parseLines('/a\n\n  /b  \n')).toEqual(['/a', '/b'])
    expect(parseLines(Array.from({ length: 60 }, (_, i) => `/p${i}`).join('\n'))).toHaveLength(50)
  })
})
