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
    const one = selectionTotals(apps, new Set(['/Applications/OldApp.app']))
    expect(one.size).toBe(100_000_000 + 30_000_000 + 20_000_000) // bundle + related
    expect(one.size).toBe(apps[0].totalSize) // consistent with engine-computed totalSize
    expect(one.paths).toBe(3) // 1 bundle + 2 related

    const both = selectionTotals(apps, new Set(['/Applications/OldApp.app', '/Applications/BusyApp.app']))
    expect(both.size).toBe(350_000_000)
    expect(both.paths).toBe(4)

    expect(selectionTotals(apps, new Set())).toEqual({ size: 0, paths: 0 })
  })

  it('keys selection by path, not display name: same-named apps at different paths are independent', () => {
    const dup: AppInfo[] = [
      { ...apps[0], path: '/Applications/OldApp.app', totalSize: 150_000_000 },
      { ...apps[0], path: '/Users/me/Applications/OldApp.app', appSize: 999, relatedPaths: [], totalSize: 999 },
    ]
    // Both share name "OldApp" but only the first path is selected.
    const selected = selectionTotals(dup, new Set(['/Applications/OldApp.app']))
    expect(selected.size).toBe(150_000_000)
    expect(selected.paths).toBe(3)

    // Selecting both distinct paths credits both independently.
    const both = selectionTotals(dup, new Set(['/Applications/OldApp.app', '/Users/me/Applications/OldApp.app']))
    expect(both.size).toBe(150_000_000 + 999)
    expect(both.paths).toBe(4)
  })
})

describe('anySelectedRunning', () => {
  it('is true only when a selected app is running', () => {
    expect(anySelectedRunning(apps, new Set(['/Applications/OldApp.app']))).toBe(false)
    expect(anySelectedRunning(apps, new Set(['/Applications/OldApp.app', '/Applications/BusyApp.app']))).toBe(true)
    expect(anySelectedRunning(apps, new Set())).toBe(false)
  })

  it('distinguishes same-named apps by path', () => {
    const dup: AppInfo[] = [
      { ...apps[0], path: '/Applications/OldApp.app', running: false },
      { ...apps[0], path: '/Users/me/Applications/OldApp.app', running: true },
    ]
    expect(anySelectedRunning(dup, new Set(['/Applications/OldApp.app']))).toBe(false)
    expect(anySelectedRunning(dup, new Set(['/Users/me/Applications/OldApp.app']))).toBe(true)
  })
})

describe('contractHome', () => {
  it('contracts /Users/<name> to ~ only as a leading segment', () => {
    expect(contractHome('/Users/me/Library/Caches/OldApp')).toBe('~/Library/Caches/OldApp')
    expect(contractHome('/Applications/OldApp.app')).toBe('/Applications/OldApp.app')
    expect(contractHome('/Volumes/Users/me/x')).toBe('/Volumes/Users/me/x')
  })
})
