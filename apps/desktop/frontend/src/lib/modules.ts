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
  Settings,
  Sparkles,
  Telescope,
  Timer,
  Trash2,
} from 'lucide-react'
import type { View } from '../stores/uiStore'

/**
 * Module registry: every screen reachable from the icon rail, with its
 * identity — accent hue, canvas gradient, icon and hero copy. The hue is
 * applied via the `--module` CSS indirection var so utilities (orb, gem,
 * module-glow) and Tailwind arbitrary values can all read one source.
 */
export interface ModuleCanvas {
  /** Gradient stops top-left → bottom-right. */
  bg1: string
  bg2: string
  bg3: string
  /** The bright radial glow in the top-right corner. */
  glow: string
}

export interface ModuleDef {
  view: View
  label: string
  /** CSS color value, normally a var(--color-module-*) reference. */
  hue: string
  canvas: ModuleCanvas
  Icon: LucideIcon
  description: string
  /** Hero feature bullets (module screens only). */
  features: Array<{ label: string; Icon: LucideIcon }>
}

const CANVAS = {
  care: { bg1: '#4b17a6', bg2: '#2a0e6e', bg3: '#150836', glow: '#b05cff' },
  cleanup: { bg1: '#1a9a44', bg2: '#0f6a2e', bg3: '#063a1a', glow: '#4ade80' },
  apps: { bg1: '#1e63e6', bg2: '#11398f', bg3: '#081a4a', glow: '#4aa3ff' },
  perf: { bg1: '#c2501a', bg2: '#7a2a0c', bg3: '#3d1206', glow: '#ff9a3d' },
  lens: { bg1: '#6a2bd9', bg2: '#3e168f', bg3: '#1c0940', glow: '#b26bff' },
  tools: { bg1: '#2f1a6e', bg2: '#221352', bg3: '#140b33', glow: '#6b46d6' },
  backups: { bg1: '#12807c', bg2: '#0b5250', bg3: '#052a2a', glow: '#3ee0d6' },
  settings: { bg1: '#2a2150', bg2: '#1d1740', bg3: '#110c28', glow: '#5b4fb3' },
} as const satisfies Record<string, ModuleCanvas>

export const MODULES: ModuleDef[] = [
  {
    view: 'dashboard',
    label: 'Smart Care',
    hue: 'var(--color-module-care)',
    canvas: CANVAS.care,
    Icon: HeartPulse,
    description: 'Everything that keeps your Mac in shape, in one place.',
    features: [],
  },
  {
    view: 'smart-scan',
    label: 'Cleanup',
    hue: 'var(--color-module-cleanup)',
    canvas: CANVAS.cleanup,
    Icon: Sparkles,
    description: 'Clear caches, logs and leftover junk to get your space back.',
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
    canvas: CANVAS.apps,
    Icon: AppWindow,
    description: 'Uninstall apps completely, leftovers included.',
    features: [
      { label: 'App Uninstaller', Icon: Trash2 },
      { label: 'File Leftovers', Icon: Files },
    ],
  },
  {
    view: 'maintenance',
    label: 'Performance',
    hue: 'var(--color-module-perf)',
    canvas: CANVAS.perf,
    Icon: Gauge,
    description: 'Run maintenance tasks and see what starts with your Mac.',
    features: [
      { label: 'Maintenance Tasks', Icon: Timer },
      { label: 'Login Items', Icon: ListChecks },
    ],
  },
  {
    view: 'space-lens',
    label: 'Space Lens',
    hue: 'var(--color-module-lens)',
    canvas: CANVAS.lens,
    Icon: Telescope,
    description: 'See what is taking up your disk, folder by folder.',
    features: [
      { label: 'Visual Storage Map', Icon: HardDrive },
      { label: 'Large Folders', Icon: FileSearch },
    ],
  },
  {
    view: 'my-tools',
    label: 'My Tools',
    hue: 'var(--color-module-tools)',
    canvas: CANVAS.tools,
    Icon: LayoutGrid,
    description: 'Every tool in one place. Pick a task and go.',
    features: [],
  },
  {
    view: 'backups',
    label: 'Backups',
    hue: 'var(--color-module-backups)',
    canvas: CANVAS.backups,
    Icon: Archive,
    description: 'Everything App Cleaner set aside before cleaning, ready to restore.',
    features: [],
  },
]

/** Settings is reachable from the rail but is not a module tile. */
export const SETTINGS_MODULE: ModuleDef = {
  view: 'settings',
  label: 'Settings',
  hue: 'var(--color-module-settings)',
  canvas: CANVAS.settings,
  Icon: Settings,
  description: 'How App Cleaner scans, backs up and updates.',
  features: [],
}

/** Rail highlighting: which module owns a (possibly nested) view. */
export function moduleForView(view: View): ModuleDef | undefined {
  if (view === 'category') return MODULES.find((m) => m.view === 'smart-scan')
  if (view === 'login-items') return MODULES.find((m) => m.view === 'maintenance')
  return MODULES.find((m) => m.view === view)
}

/** The module (or settings) whose identity paints a view. */
export function identityForView(view: View): ModuleDef {
  if (view === 'settings') return SETTINGS_MODULE
  return moduleForView(view) ?? MODULES[0]
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
    description: 'Find files over your size threshold and review them.',
    hue: 'var(--color-module-clutter)',
    Icon: HardDrive,
    action: { kind: 'scan', categoryId: 'large-files' },
  },
  {
    id: 'mail-attachments',
    name: 'Mail Attachments',
    description: 'Clear attachments Mail has downloaded locally.',
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
    description: 'Flush DNS, free purgeable space, clear Time Machine snapshots.',
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
