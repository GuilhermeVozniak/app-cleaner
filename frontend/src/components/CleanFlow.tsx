import { useCleanStore } from '../stores/cleanStore'
import { ConfirmModal } from './ConfirmModal'
import { ProgressOverlay } from './ProgressOverlay'
import { ResultsPanel } from './ResultsPanel'

export function CleanFlow() {
  const status = useCleanStore((s) => s.status)
  const progress = useCleanStore((s) => s.progress)
  const lastDryRun = useCleanStore((s) => s.lastDryRun)
  const backingUp = useCleanStore((s) => s.backingUp)
  const cancelClean = useCleanStore((s) => s.cancelClean)

  if (status === 'confirming') return <ConfirmModal />
  if (status === 'cleaning')
    return (
      <ProgressOverlay
        title={lastDryRun ? 'Dry run…' : 'Cleaning…'}
        current={progress.current}
        total={progress.total}
        itemName={backingUp ? `Backing up: ${progress.itemName}` : progress.itemName}
        onCancel={cancelClean}
      />
    )
  if (status === 'done') return <ResultsPanel />
  return null
}
