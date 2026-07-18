import { useEffect, useState } from 'react'
import { useUiStore } from '../stores/uiStore'
import { toForm, fromForm } from '../lib/settings'
import type { SettingsForm } from '../lib/settings'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Switch } from '../components/ui/switch'

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
    <div className="max-w-xl p-6">
      <h1 className="text-xl font-semibold text-ink">Settings</h1>

      <div className="mt-4 divide-y divide-hairline">
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
      </div>

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
