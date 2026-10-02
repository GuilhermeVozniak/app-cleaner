import { useEffect, useState, type ReactNode } from 'react'
import { ArrowDownToLine } from 'lucide-react'
import { DownloadUpdate, GetVersion, InstallUpdate, OpenDownloadedUpdate } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useUiStore } from '../stores/uiStore'
import { toForm, fromForm } from '../lib/settings'
import type { SettingsForm } from '../lib/settings'
import { ActionBar } from '../components/ActionBar'
import { ModuleIcon } from '../components/ModuleIcon'
import { PageHeader } from '../components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Progress } from '../components/ui/progress'
import { Switch } from '../components/ui/switch'

type UpdatePhase =
  | { step: 'idle' }
  | { step: 'downloading'; percent: number | null }
  | { step: 'installing' }
  | { step: 'install-failed'; error: string }
  | { step: 'download-failed'; error: string }

/** A preference group: glass card, title row, hairline-separated rows. */
function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="glass-1 rounded-card">
      <h2 className="px-5 pb-2 pt-4 text-card font-semibold text-ink">{title}</h2>
      <div className="divide-y divide-hairline">{children}</div>
    </section>
  )
}

/** Label + helper on the left, control on the right. */
function Row({
  id,
  label,
  helper,
  children,
}: {
  id: string
  label: string
  helper?: string
  children: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-6 px-5 py-3.5">
      <div className="min-w-0">
        <label htmlFor={id} className="block text-body font-medium text-ink">
          {label}
        </label>
        {helper ? (
          <p id={`${id}-help`} className="mt-0.5 text-caption text-ink-2">
            {helper}
          </p>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-2">{children}</div>
    </div>
  )
}

/** Label + helper on top, a full-width control underneath. */
function StackedRow({
  id,
  label,
  helper,
  children,
}: {
  id: string
  label: string
  helper?: string
  children: ReactNode
}) {
  return (
    <div className="px-5 py-3.5">
      <label htmlFor={id} className="block text-body font-medium text-ink">
        {label}
      </label>
      {helper ? (
        <p id={`${id}-help`} className="mt-0.5 text-caption text-ink-2">
          {helper}
        </p>
      ) : null}
      {children}
    </div>
  )
}

function Unit({ children }: { children: ReactNode }) {
  return <span className="w-9 text-caption text-ink-2">{children}</span>
}

const textareaClass =
  'focus-ring mt-2 w-full rounded-control border border-hairline bg-[rgb(255_255_255/0.08)] px-3 py-2 font-mono text-caption text-ink'

/** Software-update group: check, then download + install in place. */
function UpdateGroup() {
  const update = useUiStore((s) => s.update)
  const [version, setVersion] = useState('')
  const [checking, setChecking] = useState(false)
  const [phase, setPhase] = useState<UpdatePhase>({ step: 'idle' })

  useEffect(() => {
    GetVersion()
      .then((v) => setVersion(v ?? ''))
      .catch(() => setVersion(''))
  }, [])

  const check = async () => {
    setChecking(true)
    await useUiStore.getState().checkUpdate()
    setChecking(false)
  }

  const updateNow = async () => {
    if (!update?.latest) return
    setPhase({ step: 'downloading', percent: null })
    const off = EventsOn('update:progress', (p: { done: number; total: number }) => {
      setPhase({ step: 'downloading', percent: p.total > 0 ? Math.round((p.done / p.total) * 100) : null })
    })
    try {
      await DownloadUpdate(update.latest)
    } catch (e) {
      off()
      setPhase({ step: 'download-failed', error: String(e) })
      return
    }
    off()
    setPhase({ step: 'installing' })
    try {
      await InstallUpdate()
      // Success quits + relaunches the app; nothing left to render.
    } catch (e) {
      setPhase({ step: 'install-failed', error: String(e) })
    }
  }

  const busy = phase.step === 'downloading' || phase.step === 'installing'

  return (
    <Group title="Software Update">
      <div className="flex items-center gap-4 px-5 py-4">
        <ModuleIcon Icon={ArrowDownToLine} size="md" hue="var(--color-module-care)" />
        <div className="min-w-0 flex-1">
          <p className="text-body font-medium text-ink">
            Current version: {version || '…'}
            {update?.available ? (
              <span className="ml-2 text-safe">New version {update.latest} available</span>
            ) : update && !update.error ? (
              <span className="ml-2 text-ink-2">— up to date</span>
            ) : null}
          </p>
          {update?.error ? (
            <p className="mt-0.5 text-caption text-moderate">Could not check for updates: {update.error}</p>
          ) : null}
          {phase.step === 'downloading' ? (
            <div className="mt-2 flex items-center gap-3">
              <Progress value={phase.percent ?? 0} className="w-40 text-safe" />
              <span className="nums text-caption text-ink-2">
                {phase.percent === null ? 'Downloading…' : `Downloading… ${phase.percent}%`}
              </span>
            </div>
          ) : phase.step === 'installing' ? (
            <p className="mt-1 text-caption text-ink-2">Installing…</p>
          ) : phase.step === 'install-failed' ? (
            <p className="mt-1 text-caption text-moderate">Couldn't install automatically: {phase.error}</p>
          ) : phase.step === 'download-failed' ? (
            <p className="mt-1 text-caption text-moderate">Update failed: {phase.error}</p>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {phase.step === 'install-failed' ? (
            <Button type="button" variant="primary" onClick={() => void OpenDownloadedUpdate()}>
              Open downloaded update
            </Button>
          ) : update?.available && !busy ? (
            <Button type="button" variant="primary" onClick={() => void updateNow()}>
              Update now
            </Button>
          ) : null}
          <Button type="button" variant="secondary" disabled={checking || busy} onClick={() => void check()}>
            {checking ? 'Checking…' : 'Check for updates'}
          </Button>
        </div>
      </div>
    </Group>
  )
}

export function Settings() {
  const config = useUiStore((s) => s.config)
  const [form, setForm] = useState<SettingsForm | null>(null)
  const [toast, setToast] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null)

  useEffect(() => {
    if (!config) void useUiStore.getState().loadConfig()
    else if (form === null) setForm(toForm(config))
  }, [config, form])

  if (!form) {
    return <p className="px-10 pt-6 text-body text-ink-2">Loading settings…</p>
  }

  const patch = (p: Partial<SettingsForm>) => setForm({ ...form, ...p })
  const num = (v: number) => (Number.isFinite(v) ? v : '')

  const save = async () => {
    const cleaned = fromForm(form) // client-side clamping mirrors internal/config bounds
    try {
      await useUiStore.getState().saveConfig(cleaned)
      setForm(toForm(cleaned)) // show the clamped values back
      setToast({ kind: 'ok', text: 'Settings saved' })
    } catch (e) {
      setToast({ kind: 'error', text: `Save failed: ${String(e)}` })
    }
    window.setTimeout(() => setToast(null), 2500)
  }

  return (
    <div className="flex h-full flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="materialize mx-auto flex max-w-3xl flex-col gap-5 px-10 pb-6 pt-6">
      <PageHeader title="Settings" subtitle="How App Cleaner scans, backs up and updates." />

      <UpdateGroup />

      <Group title="Cleanup">
        <Row
          id="downloads-days"
          label="Downloads considered old after"
          helper="Files in Downloads you have not opened for this long show up as Old Downloads. 1 to 365 days."
        >
          <Input
            id="downloads-days"
            aria-describedby="downloads-days-help"
            className="nums w-24 text-right"
            type="number"
            min={1}
            max={365}
            value={num(form.downloadsDaysOld)}
            onChange={(e) => patch({ downloadsDaysOld: Number(e.target.value) })}
          />
          <Unit>days</Unit>
        </Row>

        <Row
          id="large-files-mb"
          label="Large-file threshold"
          helper="Files above this size appear in Large Files. 1 to 102400 MB."
        >
          <Input
            id="large-files-mb"
            aria-describedby="large-files-mb-help"
            className="nums w-24 text-right"
            type="number"
            min={1}
            max={102400}
            value={num(form.largeFilesMB)}
            onChange={(e) => patch({ largeFilesMB: Number(e.target.value) })}
          />
          <Unit>MB</Unit>
        </Row>

        <Row
          id="show-risky"
          label="Show risky categories by default"
          helper="Risky categories stay collapsed in scan results unless you turn this on."
        >
          <Switch
            id="show-risky"
            aria-label="Show risky categories by default"
            checked={form.showRisky}
            onCheckedChange={(checked) => patch({ showRisky: checked })}
          />
        </Row>

        <StackedRow
          id="keep-languages"
          label="Extra languages to keep"
          helper="Comma-separated, for example pt-BR, de. Language Files never removes these."
        >
          <Input
            id="keep-languages"
            aria-describedby="keep-languages-help"
            className="mt-2 w-full"
            type="text"
            value={form.keepLanguages}
            onChange={(e) => patch({ keepLanguages: e.target.value })}
          />
        </StackedRow>
      </Group>

      <Group title="Backups">
        <Row
          id="backup-default"
          label="Back up by default"
          helper="Moderate and risky items are moved to a backup before deletion so you can undo."
        >
          <Switch
            id="backup-default"
            aria-label="Back up by default (moderate/risky categories)"
            checked={form.backupByDefault}
            onCheckedChange={(checked) => patch({ backupByDefault: checked })}
          />
        </Row>

        <Row
          id="backup-retention"
          label="Backup retention"
          helper="Backups older than this are removed automatically. 1 to 365 days."
        >
          <Input
            id="backup-retention"
            aria-describedby="backup-retention-help"
            className="nums w-24 text-right"
            type="number"
            min={1}
            max={365}
            value={num(form.backupRetentionDays)}
            onChange={(e) => patch({ backupRetentionDays: Number(e.target.value) })}
          />
          <Unit>days</Unit>
        </Row>
      </Group>

      <Group title="Scanning">
        <Row id="concurrency" label="Scan concurrency" helper="Categories scanned in parallel. 1 to 16.">
          <Input
            id="concurrency"
            aria-describedby="concurrency-help"
            className="nums w-24 text-right"
            type="number"
            min={1}
            max={16}
            value={num(form.concurrency)}
            onChange={(e) => patch({ concurrency: Number(e.target.value) })}
          />
        </Row>

        <StackedRow
          id="node-modules-roots"
          label="Extra node_modules scan roots"
          helper="One folder per line, up to 50. node_modules inside them are offered for cleanup."
        >
          <textarea
            id="node-modules-roots"
            aria-describedby="node-modules-roots-help"
            className={textareaClass}
            rows={3}
            value={form.nodeModulesPaths}
            onChange={(e) => patch({ nodeModulesPaths: e.target.value })}
          />
        </StackedRow>

        <StackedRow id="project-roots" label="Extra project scan roots" helper="One folder per line, up to 50.">
          <textarea
            id="project-roots"
            aria-describedby="project-roots-help"
            className={textareaClass}
            rows={3}
            value={form.projectsPaths}
            onChange={(e) => patch({ projectsPaths: e.target.value })}
          />
        </StackedRow>
      </Group>

        </div>
      </div>
      <div className="mx-auto w-full max-w-3xl px-10 pb-5">
        <ActionBar className="mx-0 mb-0">
          <span
            role="status"
            className={toast?.kind === 'error' ? 'text-body text-danger' : 'text-body text-safe'}
          >
            {toast?.text ?? ''}
          </span>
          <Button type="button" variant="primary" className="ml-auto" onClick={() => void save()}>
            Save
          </Button>
        </ActionBar>
      </div>
    </div>
  )
}

export default Settings
