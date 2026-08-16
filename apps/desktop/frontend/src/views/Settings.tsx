import { useEffect, useState } from 'react'
import { GetVersion, OpenReleasePage } from '../../wailsjs/go/main/App'
import { useUiStore } from '../stores/uiStore'
import { toForm, fromForm } from '../lib/settings'
import type { SettingsForm } from '../lib/settings'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Switch } from '../components/ui/switch'

/** Software-update card: current version, latest check result, release link. */
function UpdateCard() {
  const update = useUiStore((s) => s.update)
  const [version, setVersion] = useState('')
  const [checking, setChecking] = useState(false)

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

  return (
    <Card className="mt-4 p-5">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="font-medium text-ink">Software Update</h2>
          <p className="mt-1 text-sm text-ink-2">
            Current version: {version || '…'}
            {update?.available ? (
              <span className="ml-2 text-safe">New version {update.latest} available</span>
            ) : update && !update.error ? (
              <span className="ml-2">— up to date</span>
            ) : null}
          </p>
          {update?.error ? (
            <p className="mt-1 text-xs text-moderate">Could not check for updates: {update.error}</p>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {update?.available ? (
            <Button type="button" variant="primary" onClick={() => void OpenReleasePage()}>
              Get update
            </Button>
          ) : null}
          <Button type="button" variant="glass" disabled={checking} onClick={() => void check()}>
            {checking ? 'Checking…' : 'Check for updates'}
          </Button>
        </div>
      </div>
    </Card>
  )
}

export function Settings() {
  const config = useUiStore((s) => s.config)
  const [form, setForm] = useState<SettingsForm | null>(null)
  const [toast, setToast] = useState<string | null>(null)

  useEffect(() => {
    if (!config) void useUiStore.getState().loadConfig()
    else if (form === null) setForm(toForm(config))
  }, [config, form])

  if (!form) {
    return <p className="p-6 text-sm text-ink-2">Loading settings…</p>
  }

  const patch = (p: Partial<SettingsForm>) => setForm({ ...form, ...p })
  const num = (v: number) => (Number.isFinite(v) ? v : '')

  const save = async () => {
    const cleaned = fromForm(form) // client-side clamping mirrors internal/config bounds
    try {
      await useUiStore.getState().saveConfig(cleaned)
      setForm(toForm(cleaned)) // show the clamped values back
      setToast('Settings saved')
    } catch (e) {
      setToast(`Save failed: ${String(e)}`)
    }
    window.setTimeout(() => setToast(null), 2500)
  }

  const row = 'flex items-center justify-between gap-4 py-3'
  const area =
    'mt-1 w-full rounded-control border border-hairline bg-surface-solid/60 px-2 py-1 font-mono text-xs text-ink'

  return (
    <div className="max-w-2xl p-6">
      <h1 className="text-xl font-semibold text-ink">Settings</h1>

      <UpdateCard />

      <Card className="mt-4 divide-y divide-hairline px-5 py-1">
        <label className={row}>
          <span className="text-sm text-ink">Downloads considered old after (days, 1–365)</span>
          <Input
            className="w-28"
            type="number"
            min={1}
            max={365}
            value={num(form.downloadsDaysOld)}
            onChange={(e) => patch({ downloadsDaysOld: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm text-ink">Large-file threshold (MB, 1–102400)</span>
          <Input
            className="w-28"
            type="number"
            min={1}
            max={102400}
            value={num(form.largeFilesMB)}
            onChange={(e) => patch({ largeFilesMB: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm text-ink">Back up by default (moderate/risky categories)</span>
          <Switch
            aria-label="Back up by default (moderate/risky categories)"
            checked={form.backupByDefault}
            onCheckedChange={(checked) => patch({ backupByDefault: checked })}
          />
        </label>

        <label className={row}>
          <span className="text-sm text-ink">Backup retention (days, 1–365)</span>
          <Input
            className="w-28"
            type="number"
            min={1}
            max={365}
            value={num(form.backupRetentionDays)}
            onChange={(e) => patch({ backupRetentionDays: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm text-ink">Scan concurrency (1–16)</span>
          <Input
            className="w-28"
            type="number"
            min={1}
            max={16}
            value={num(form.concurrency)}
            onChange={(e) => patch({ concurrency: Number(e.target.value) })}
          />
        </label>

        <label className={row}>
          <span className="text-sm text-ink">Show risky categories by default</span>
          <Switch
            aria-label="Show risky categories by default"
            checked={form.showRisky}
            onCheckedChange={(checked) => patch({ showRisky: checked })}
          />
        </label>

        <label className="block py-3">
          <span className="text-sm text-ink">Extra languages to keep (comma-separated, e.g. pt-BR, de)</span>
          <Input
            className="mt-1 w-full"
            type="text"
            value={form.keepLanguages}
            onChange={(e) => patch({ keepLanguages: e.target.value })}
          />
        </label>

        <label className="block py-3">
          <span className="text-sm text-ink">Extra node_modules scan roots (one path per line, max 50)</span>
          <textarea
            className={area}
            rows={3}
            value={form.nodeModulesPaths}
            onChange={(e) => patch({ nodeModulesPaths: e.target.value })}
          />
        </label>

        <label className="block py-3">
          <span className="text-sm text-ink">Extra project scan roots (one path per line, max 50)</span>
          <textarea
            className={area}
            rows={3}
            value={form.projectsPaths}
            onChange={(e) => patch({ projectsPaths: e.target.value })}
          />
        </label>
      </Card>

      <div className="mt-4 flex items-center gap-3">
        <Button type="button" variant="primary" onClick={() => void save()}>
          Save
        </Button>
        {toast ? <span className="text-sm text-safe">{toast}</span> : null}
      </div>
    </div>
  )
}

export default Settings
