// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), EventsOff: vi.fn() }))
vi.mock('../../wailsjs/go/main/App', () => ({
  ListApps: vi.fn().mockResolvedValue([]),
  StartUninstall: vi.fn().mockResolvedValue(undefined),
}))

import { ListApps, StartUninstall } from '../../wailsjs/go/main/App'
import {
  useUninstallerStore,
  handleUninstallDone,
  handleUninstallProgress,
} from './uninstallerStore'
import type { AppInfo } from '../lib/types'

const ListAppsMock = ListApps as unknown as ReturnType<typeof vi.fn>
const StartUninstallMock = StartUninstall as unknown as ReturnType<typeof vi.fn>

function app(name: string, path: string): AppInfo {
  return { name, path, bundleId: `com.x.${name}`, appSize: 10, relatedPaths: [], totalSize: 10, running: false }
}

function resetStore() {
  useUninstallerStore.setState({
    apps: [], lastScanAt: null, scanning: false, selected: new Set<string>(),
    icons: {}, phase: 'list', pending: null, lastRequested: [], lastDryRun: false,
    skippedApps: [], progress: { current: 0, total: 0, appName: '' }, done: null, startError: null,
  })
}

beforeEach(() => {
  resetStore()
  localStorage.clear()
  ListAppsMock.mockReset().mockResolvedValue([])
  StartUninstallMock.mockReset().mockResolvedValue(undefined)
})

describe('refresh', () => {
  it('keeps the cached list visible while scanning, then replaces it', async () => {
    useUninstallerStore.setState({ apps: [app('Old', '/Applications/Old.app')] })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    expect(useUninstallerStore.getState().scanning).toBe(true)
    expect(useUninstallerStore.getState().apps).toHaveLength(1) // cached list untouched
    resolve([app('New', '/Applications/New.app')])
    await p
    const s = useUninstallerStore.getState()
    expect(s.scanning).toBe(false)
    expect(s.apps.map((a) => a.name)).toEqual(['New'])
    expect(s.lastScanAt).toBeTruthy()
  })

  it('normalises null relatedPaths and prunes vanished selections', async () => {
    useUninstallerStore.setState({ selected: new Set(['/Applications/Gone.app', '/Applications/Kept.app']) })
    ListAppsMock.mockResolvedValueOnce([
      { ...app('Kept', '/Applications/Kept.app'), relatedPaths: null as unknown as AppInfo['relatedPaths'] },
    ])
    await useUninstallerStore.getState().refresh()
    const s = useUninstallerStore.getState()
    expect(s.apps[0].relatedPaths).toEqual([])
    expect([...s.selected]).toEqual(['/Applications/Kept.app'])
  })

  it('on failure keeps the cached list and does not clear lastScanAt', async () => {
    useUninstallerStore.setState({ apps: [app('Old', '/Applications/Old.app')], lastScanAt: '2026-07-18T00:00:00Z' })
    ListAppsMock.mockRejectedValueOnce(new Error('boom'))
    await useUninstallerStore.getState().refresh()
    const s = useUninstallerStore.getState()
    expect(s.scanning).toBe(false)
    expect(s.apps).toHaveLength(1)
    expect(s.lastScanAt).toBe('2026-07-18T00:00:00Z')
  })

  it('persists only apps and lastScanAt', async () => {
    ListAppsMock.mockResolvedValueOnce([app('A', '/Applications/A.app')])
    await useUninstallerStore.getState().refresh()
    useUninstallerStore.setState({ icons: { '/Applications/A.app': 'xxx' }, phase: 'confirm' })
    const raw = localStorage.getItem('app-cleaner.uninstaller')
    expect(raw).toBeTruthy()
    const persisted = JSON.parse(raw as string)
    expect(Object.keys(persisted.state).sort()).toEqual(['apps', 'lastScanAt'])
    expect(persisted.version).toBe(1)
  })
})

describe('requestUninstall', () => {
  it('starts immediately when idle, filtering nothing when all apps exist', () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')],
      selected: new Set(['/Applications/A.app']),
    })
    useUninstallerStore.getState().requestUninstall(false)
    expect(StartUninstallMock).toHaveBeenCalledWith(['/Applications/A.app'], false)
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('running')
    expect(s.skippedApps).toEqual([])
    expect(s.lastRequested).toEqual(['/Applications/A.app'])
  })

  it('queues while scanning and starts after the scan with only still-present apps, noting skipped names', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')],
      selected: new Set(['/Applications/A.app', '/Applications/B.app']),
    })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    expect(useUninstallerStore.getState().phase).toBe('waiting')
    expect(StartUninstallMock).not.toHaveBeenCalled()
    resolve([app('A', '/Applications/A.app')]) // B vanished in the meantime
    await p
    expect(StartUninstallMock).toHaveBeenCalledWith(['/Applications/A.app'], false)
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('running')
    expect(s.skippedApps).toEqual(['B'])
  })

  it('short-circuits to done (no StartUninstall) when every queued app is gone', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    resolve([])
    await p
    expect(StartUninstallMock).not.toHaveBeenCalled()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('done')
    expect(s.done).toEqual({ uninstalled: 0, freedSpace: 0, errors: [] })
    expect(s.skippedApps).toEqual(['A'])
  })

  it('aborts a pending uninstall with a visible error when the verifying scan fails', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    let reject!: (e: Error) => void
    ListAppsMock.mockReturnValueOnce(new Promise((_r, rj) => { reject = rj }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    reject(new Error('scan died'))
    await p
    expect(StartUninstallMock).not.toHaveBeenCalled()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('list')
    expect(s.pending).toBeNull()
    expect(s.startError).toBe("Couldn't verify installed apps — uninstall not started")
  })

  it('surfaces StartUninstall rejection and returns to the list', async () => {
    StartUninstallMock.mockRejectedValueOnce(new Error('an uninstall is already running'))
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    useUninstallerStore.getState().requestUninstall(false)
    await vi.waitFor(() => expect(useUninstallerStore.getState().phase).toBe('list'))
    expect(useUninstallerStore.getState().startError).toContain('already running')
  })

  it('aborts a queued uninstall when the resolved scan shows the app now running', async () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app')],
      selected: new Set(['/Applications/A.app']),
    })
    let resolve!: (v: AppInfo[]) => void
    ListAppsMock.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const p = useUninstallerStore.getState().refresh()
    useUninstallerStore.getState().requestUninstall(false)
    expect(useUninstallerStore.getState().phase).toBe('waiting')
    resolve([{ ...app('A', '/Applications/A.app'), running: true }])
    await p
    expect(StartUninstallMock).not.toHaveBeenCalled()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('list')
    expect(s.startError).toBe('Still running — quit first: A. Uninstall not started.')
  })

  it('aborts immediately (even for a dry run) when the requested app is running in the current list', () => {
    useUninstallerStore.setState({
      apps: [{ ...app('A', '/Applications/A.app'), running: true }],
      selected: new Set(['/Applications/A.app']),
    })
    useUninstallerStore.getState().requestUninstall(true)
    expect(StartUninstallMock).not.toHaveBeenCalled()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('list')
    expect(s.startError).toBe('Still running — quit first: A. Uninstall not started.')
  })
})

describe('uninstall events', () => {
  it('progress drives the running phase', () => {
    handleUninstallProgress({ current: 1, total: 2, appName: 'A' })
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('running')
    expect(s.progress).toEqual({ current: 1, total: 2, appName: 'A' })
  })

  it('done optimistically removes requested apps on a real run', () => {
    useUninstallerStore.setState({
      apps: [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')],
      lastRequested: ['/Applications/A.app'],
      lastDryRun: false,
    })
    handleUninstallDone({ uninstalled: 1, freedSpace: 10, errors: [] })
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('done')
    expect(s.apps.map((a) => a.name)).toEqual(['B'])
  })

  it('dry runs and cancelled runs never mutate the list', () => {
    const both = [app('A', '/Applications/A.app'), app('B', '/Applications/B.app')]
    useUninstallerStore.setState({ apps: both, lastRequested: ['/Applications/A.app'], lastDryRun: true })
    handleUninstallDone({ uninstalled: 1, freedSpace: 10, errors: [] })
    expect(useUninstallerStore.getState().apps).toHaveLength(2)
    useUninstallerStore.setState({ apps: both, lastDryRun: false, phase: 'running' })
    handleUninstallDone({ uninstalled: 0, freedSpace: 0, errors: [], cancelled: true })
    expect(useUninstallerStore.getState().apps).toHaveLength(2)
  })
})

describe('finish', () => {
  it('clears flow state, keeps the list, and kicks a background refresh', async () => {
    useUninstallerStore.setState({
      apps: [app('B', '/Applications/B.app')],
      phase: 'done',
      done: { uninstalled: 1, freedSpace: 10, errors: [] },
      skippedApps: ['A'],
      selected: new Set(['/Applications/B.app']),
    })
    ListAppsMock.mockResolvedValueOnce([app('B', '/Applications/B.app')])
    useUninstallerStore.getState().finish()
    const s = useUninstallerStore.getState()
    expect(s.phase).toBe('list')
    expect(s.done).toBeNull()
    expect(s.skippedApps).toEqual([])
    expect(s.selected.size).toBe(0)
    expect(s.apps).toHaveLength(1) // still visible
    await vi.waitFor(() => expect(useUninstallerStore.getState().scanning).toBe(false))
    expect(ListAppsMock).toHaveBeenCalledTimes(1)
  })
})
