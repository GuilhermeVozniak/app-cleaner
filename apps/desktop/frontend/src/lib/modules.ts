import type { LucideIcon } from 'lucide-react'
import {
  Archive,
  AppWindow,
  FileSearch,
  Files,
  Gauge,
  HardDrive,
  HeartPulse,
  LayoutGrid,
  ListChecks,
  Mail,
  Sparkles,
  Telescope,
  Timer,
  Trash2,
} from 'lucide-react'
import type { View } from '../stores/uiStore'

/**
 * Module registry: every screen reachable from the icon rail, with its
 * CleanMyMac-style identity (hue + icon + hero copy). The hue is applied via
 * the `--module` CSS indirection var so glass utilities (module-glow,
 * module-wash) and Tailwind arbitrary values can all read one source.
 */
export interface ModuleDef {
  view: View
  label: string
  /** CSS color value, normally a var(--color-module-*) reference. */
  hue: string
  Icon: LucideIcon
  description: string
  /** Hero feature bullets (module screens only). */
  features: Array<{ label: string; Icon: LucideIcon }>
}

export const MODULES: ModuleDef[] = [
  {
    view: 'dashboard',
    label: 'Smart Care',
    hue: 'var(--color-module-care)',
    Icon: HeartPulse,
    description: 'One place to see and run everything that keeps your Mac in shape.',
    features: [],
  },
  {
    view: 'smart-scan',
    label: 'Cleanup',
    hue: 'var(--color-module-cleanup)',
    Icon: Sparkles,
    description: 'Find caches, logs, and leftover junk taking up space on your Mac.',
    features: [
      { label: 'System Junk', Icon: HardDrive },
      { label: 'Trash Bins', Icon: Trash2 },
      { label: 'Mail Attachments', Icon: Mail },
    ],
  },
  {
    view: 'uninstaller',
    label: 'Applications',
    hue: 'var(--color-module-apps)',
    Icon: AppWindow,
    description: 'Uninstall apps completely — including their caches, preferences, and leftovers.',
    features: [
      { label: 'App Uninstaller', Icon: Trash2 },
      { label: 'File Leftovers', Icon: Files },
    ],
  },
  {
    view: 'maintenance',
    label: 'Performance',
    hue: 'var(--color-module-perf)',
    Icon: Gauge,
    description: 'Run maintenance tasks and review what launches in the background.',
    features: [
      { label: 'Maintenance Tasks', Icon: Timer },
      { label: 'Login Items', Icon: ListChecks },
    ],
  },
  {
    view: 'space-lens',
    label: 'Space Lens',
    hue: 'var(--color-module-lens)',
    Icon: Telescope,
    description: 'Build a size map of any folder and spot what eats your disk.',
    features: [
      { label: 'Visual Storage Map', Icon: HardDrive },
      { label: 'Large Folders', Icon: FileSearch },
    ],
  },
  {
    view: 'my-tools',
    label: 'My Tools',
    hue: 'var(--color-module-clutter)',
    Icon: LayoutGrid,
    description: 'All tools in one place. Pick the task — we handle the rest.',
    features: [],
  },
  {
    view: 'backups',
    label: 'Backups',
    hue: 'var(--color-module-backups)',
    Icon: Archive,
    description: 'Everything App Cleaner backed up before cleaning, ready to restore.',
    features: [],
  },
]

/** Rail highlighting: which module owns a (possibly nested) view. */
export function moduleForView(view: View): ModuleDef | undefined {
  if (view === 'category') return MODULES.find((m) => m.view === 'smart-scan')
  if (view === 'login-items') return MODULES.find((m) => m.view === 'maintenance')
  return MODULES.find((m) => m.view === view)
}

/** A standalone tool card in the My Tools grid. */
export interface ToolDef {
  id: string
  name: string
  description: string
  hue: string
  Icon: LucideIcon
  /** Either start a single-category scan or open a dedicated view. */
  action: { kind: 'scan'; categoryId: string } | { kind: 'view'; view: View }
}

export const TOOLS: ToolDef[] = [
  {
    id: 'trash-bins',
    name: 'Trash Bins',
    description: 'Empty the Trash in one go.',
    hue: 'var(--color-module-cleanup)',
    Icon: Trash2,
    action: { kind: 'scan', categoryId: 'trash' },
  },
  {
    id: 'old-downloads',
    name: 'Old Downloads',
    description: 'Review downloads you have not touched in a while.',
    hue: 'var(--color-module-clutter)',
    Icon: FileSearch,
    action: { kind: 'scan', categoryId: 'downloads' },
  },
  {
    id: 'large-files',
    name: 'Large Files',
    description: 'Find files over your size threshold for review.',
    hue: 'var(--color-module-clutter)',
    Icon: HardDrive,
    action: { kind: 'scan', categoryId: 'large-files' },
  },
  {
    id: 'mail-attachments',
    name: 'Mail Attachments',
    description: 'Clear downloaded Mail.app attachments.',
    hue: 'var(--color-module-cleanup)',
    Icon: Mail,
    action: { kind: 'scan', categoryId: 'mail-attachments' },
  },
  {
    id: 'duplicates',
    name: 'Duplicate Files',
    description: 'Find files with identical content.',
    hue: 'var(--color-module-clutter)',
    Icon: Files,
    action: { kind: 'scan', categoryId: 'duplicates' },
  },
  {
    id: 'space-lens',
    name: 'Space Lens',
    description: 'Map folder sizes across your disk.',
    hue: 'var(--color-module-lens)',
    Icon: Telescope,
    action: { kind: 'view', view: 'space-lens' },
  },
  {
    id: 'login-items',
    name: 'Login Items',
    description: 'See what starts automatically with your Mac.',
    hue: 'var(--color-module-perf)',
    Icon: ListChecks,
    action: { kind: 'view', view: 'login-items' },
  },
  {
    id: 'maintenance',
    name: 'Maintenance',
    description: 'DNS flush, purgeable space, Time Machine snapshots.',
    hue: 'var(--color-module-perf)',
    Icon: Timer,
    action: { kind: 'view', view: 'maintenance' },
  },
]

/** Case-insensitive filter over tool name + description. */
export function filterTools(tools: ToolDef[], query: string): ToolDef[] {
  const q = query.trim().toLowerCase()
  if (!q) return tools
  return tools.filter(
    (t) => t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q),
  )
}
