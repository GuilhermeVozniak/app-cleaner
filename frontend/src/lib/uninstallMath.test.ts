import { describe, it, expect } from 'vitest'
import { contractHome, selectionTotals, anySelectedRunning } from './uninstallMath'
import type { AppInfo } from './types'

const apps: AppInfo[] = [
  {
    name: 'OldApp',
    path: '/Applications/OldApp.app',
    bundleId: 'com.old.app',
    appSize: 100_000_000,
    relatedPaths: [
      { path: '/Users/me/Library/Caches/OldApp', size: 30_000_000 },
      { path: '/Users/me/Library/Preferences/com.old.app.plist', size: 20_000_000 },
    ],
    totalSize: 150_000_000,
    running: false,
  },
  {
    name: 'BusyApp',
    path: '/Applications/BusyApp.app',
    bundleId: 'com.busy.app',
    appSize: 200_000_000,
    relatedPaths: [],
    totalSize: 200_000_000,
    running: true,
  },
]

describe('selectionTotals', () => {
  it('total = Σ(bundle + related) per selected app; paths = Σ(1 + related count)', () => {
    const one = selectionTotals(apps, new Set(['OldApp']))
    expect(one.size).toBe(100_000_000 + 30_000_000 + 20_000_000) // bundle + related
    expect(one.size).toBe(apps[0].totalSize) // consistent with engine-computed totalSize
    expect(one.paths).toBe(3) // 1 bundle + 2 related

    const both = selectionTotals(apps, new Set(['OldApp', 'BusyApp']))
    expect(both.size).toBe(350_000_000)
    expect(both.paths).toBe(4)

    expect(selectionTotals(apps, new Set())).toEqual({ size: 0, paths: 0 })
  })
})

describe('anySelectedRunning', () => {
  it('is true only when a selected app is running', () => {
    expect(anySelectedRunning(apps, new Set(['OldApp']))).toBe(false)
    expect(anySelectedRunning(apps, new Set(['OldApp', 'BusyApp']))).toBe(true)
    expect(anySelectedRunning(apps, new Set())).toBe(false)
  })
})

describe('contractHome', () => {
  it('contracts /Users/<name> to ~ only as a leading segment', () => {
    expect(contractHome('/Users/me/Library/Caches/OldApp')).toBe('~/Library/Caches/OldApp')
    expect(contractHome('/Applications/OldApp.app')).toBe('/Applications/OldApp.app')
    expect(contractHome('/Volumes/Users/me/x')).toBe('/Volumes/Users/me/x')
  })
})
