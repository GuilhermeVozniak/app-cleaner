/**
 * Dev-only: drive a flow to a given state right after boot so it can be
 * screenshotted headlessly (`?mock&flow=<name>`). Pairs with mock.ts.
 */
import { useCleanStore } from '../stores/cleanStore'
import { selectedPaths, useScanStore } from '../stores/scanStore'
import { useUiStore } from '../stores/uiStore'
import { useUninstallerStore } from '../stores/uninstallerStore'

function whenScanDone(): Promise<void> {
  return new Promise((resolve) => {
    if (useScanStore.getState().status === 'done') return resolve()
    const unsub = useScanStore.subscribe((s) => {
      if (s.status === 'done') {
        unsub()
        resolve()
      }
    })
  })
}

function whenAppsLoaded(): Promise<void> {
  return new Promise((resolve) => {
    if (useUninstallerStore.getState().apps.length > 0 && !useUninstallerStore.getState().scanning) return resolve()
    const unsub = useUninstallerStore.subscribe((s) => {
      if (s.apps.length > 0 && !s.scanning) {
        unsub()
        resolve()
      }
    })
  })
}

export async function runDevFlow(flow: string): Promise<void> {
  switch (flow) {
    case 'scan':
      useUiStore.getState().setView('smart-scan')
      void useScanStore.getState().startScan()
      return
    case 'category':
      useUiStore.getState().setView('smart-scan')
      void useScanStore.getState().startScan()
      await whenScanDone()
      useUiStore.getState().setView('category', 'downloads')
      return
    case 'confirm':
    case 'clean': {
      useUiStore.getState().setView('smart-scan')
      void useScanStore.getState().startScan()
      await whenScanDone()
      useCleanStore.getState().openConfirm()
      if (flow === 'clean') {
        const { results, selected } = useScanStore.getState()
        void useCleanStore.getState().startClean(selectedPaths(results, selected), { dryRun: false, backup: true })
      }
      return
    }
    case 'uninstall':
    case 'uninstalling': {
      useUiStore.getState().setView('uninstaller')
      await whenAppsLoaded()
      const apps = useUninstallerStore.getState().apps
      for (const a of apps.slice(0, 3)) if (!a.running) useUninstallerStore.getState().toggle(a.path)
      useUninstallerStore.getState().openConfirm()
      if (flow === 'uninstalling') useUninstallerStore.getState().requestUninstall(false)
      return
    }
    default:
      console.warn('[dev] unknown flow', flow)
  }
}
