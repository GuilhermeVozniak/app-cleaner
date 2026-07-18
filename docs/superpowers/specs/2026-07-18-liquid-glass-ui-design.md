# Liquid Glass UI revamp — design

Date: 2026-07-18
Status: approved (design discussion in session)
Scope: `apps/desktop/frontend` only. Engine, bridge, CLI, web untouched.

## Goal

Revamp the desktop UI to Apple's Liquid Glass design language — premium,
native-to-macOS-Tahoe feel — while reorganizing the component layer into an
idiomatic shadcn-style system. Zero behavior change: every copy string,
aria-label, role, store interaction, and flow stays identical; the 155-test
suite keeps passing throughout.

## Non-goals

- No window-level translucency (no Go/main.go changes; in-app glass only).
- No webfonts (native SF stack; app is offline).
- No copy/flow/store changes; no engine or binding changes.
- No SVG-displacement refraction (Chromium-only; Wails renders in WKWebView).

## Dependencies (frontend package only; all small, MIT)

`class-variance-authority`, `clsx`, `tailwind-merge`,
`@radix-ui/react-checkbox`, `@radix-ui/react-dialog`,
`@radix-ui/react-progress`, `@radix-ui/react-switch`,
`@radix-ui/react-tooltip`, `@radix-ui/react-slot`.

## Token system (Tailwind v4, CSS-first in `src/style.css` `@theme`)

Colors (light / dark):
- Canvas: `--color-canvas` `#F5F6F8` / `#0B0E14`.
- Aurora tints (backdrop blobs): sky `#7CB8FF`, mint `#7FE0C3`, violet
  `#C6B3FF` — same hues both modes, opacity ~0.35 light / ~0.16 dark.
- Glass fills: `--color-glass-1` `rgb(255 255 255 / 0.55)` /
  `rgb(23 26 34 / 0.55)`; `--color-glass-2` `rgb(255 255 255 / 0.68)` /
  `rgb(28 32 42 / 0.68)`.
- Text: primary `#1D1D1F` / `#F5F5F7`; secondary `#6E6E73` / `#98989D`.
- Accent `#0A84FF`; destructive `#FF453A`; safety: safe `#30D158`,
  moderate `#FF9F0A`, risky `#FF453A`.
- Hairline: `rgb(0 0 0 / 0.08)` / `rgb(255 255 255 / 0.10)`.

Radii (concentric): shell 20px → card 16px → control 10px → pill 9999px.
Inner radius = outer − padding wherever nested.

Motion: `--dur-fast` 150ms, `--dur-med` 220ms; easing
`cubic-bezier(0.32, 0.72, 0, 1)`.

## Glass material recipe (exact, shared via CSS utilities)

Two elevations, applied with Tailwind v4 `@utility glass-1` / `@utility
glass-2` (usable as classes):

```css
background: var(--color-glass-1|2);
backdrop-filter: blur(20px) saturate(1.4);
-webkit-backdrop-filter: blur(20px) saturate(1.4);
border: 1px solid transparent;
background-clip: padding-box;
box-shadow:
  inset 0 1px 0 rgb(255 255 255 / 0.25),   /* specular top edge */
  0 8px 24px rgb(0 0 0 / 0.10);            /* depth (0.35 dark) */
```

plus a 1px gradient ring (white/25 → white/5; dark: white/12 → white/3) via
`::after` or border-image in the utility. glass-2 gets blur(28px) and the
larger depth shadow — it is the chrome layer (sidebar rail, floating action
bars, dialogs/overlays); glass-1 is content (cards, list containers).

Rules: at most TWO stacked backdrop-filter regions anywhere (canvas has
none — the aurora is plain, pre-blurred gradients); `@media
(prefers-reduced-transparency: reduce)` swaps both glass fills for solid
surface colors and drops backdrop-filter; `@media (prefers-reduced-motion:
reduce)` disables all non-essential animation.

## Aurora canvas

`src/components/Backdrop.tsx`: a fixed, `pointer-events-none`, `z-0` layer
behind the shell rendering 3 large radial-gradient blobs (the aurora hues)
on the canvas color — static (no filters, no animation). Light/dark via the
existing `dark:` scheme.

## Type & numerals

SF stack unchanged. View titles: 20px/semibold, −0.01em tracking. Section
eyebrows: 11px/semibold/uppercase/0.06em, secondary color. ALL sizes and
counts render with `tabular-nums` (utility class `nums`). Paths stay mono
(SF Mono via `font-mono`).

## shadcn layer — `src/components/ui/`

`cn()` in `src/lib/cn.ts` (clsx + tailwind-merge). Components (CVA
variants; Radix primitive underneath where named):
- `button.tsx` — variants: `primary` (accent, specular hover sweep),
  `destructive`, `ghost`, `glass` (glass-1 chip); sizes sm/md/lg; asChild
  via Slot.
- `card.tsx` — `Card` with `elevation: 1|2` → glass-1/glass-2 + radius.
- `badge.tsx` — `safety: safe|moderate|risky`, plus `neutral` (replaces
  SafetyBadge's internals; SafetyBadge stays as a thin wrapper to keep its
  API).
- `checkbox.tsx` (Radix Checkbox) — replaces every raw `<input
  type="checkbox">`; keeps each call site's aria-label.
- `dialog.tsx` (Radix Dialog) — glass-2 panel, overlay `bg-black/40` +
  blur(8px); becomes the base of ConfirmModal, UninstallConfirm,
  ResultsPanel, the Uninstaller done dialog, and Backups' inline confirms
  stay inline (row-scoped, not modal).
- `progress.tsx` (Radix Progress) — accent bar on glass track; used by
  ProgressOverlay and CategoryCard's SizeBar (SizeBar becomes a thin
  wrapper).
- `switch.tsx` (Radix Switch) — Settings booleans (replaces checkboxes
  there; semantics/labels unchanged).
- `input.tsx` — text/number inputs for Settings.
- `separator.tsx`, `tooltip.tsx` (Radix) — tooltip replaces the
  title-attribute hint on the blocked Uninstall button (keeps `title` attr
  too, so the existing test still passes).

Idioms enforced across `src/`: typed props interfaces, no inline color
literals (tokens only), zinc/neutral unified to token classes, consistent
named exports + default export per existing file conventions, list rows
extracted where duplicated (Uninstaller/Backups row → shared patterns via
ui primitives, not a premature shared row component).

## Shell & views

- Shell (`App.tsx`): aurora Backdrop; sidebar becomes a floating glass-2
  rail inset 10px (concentric radii), top strip is a drag region
  (`--wails-draggable: drag`) honoring the hidden-inset titlebar; content
  area scrolls, floating glass-2 action bars replace flat footers
  (SmartScan selection bar, Uninstaller uninstall bar).
- Sidebar items: pill selection (glass chip + accent tint), lucide icons
  kept.
- SmartScan: signature **glass lens** hero — a circular layered-glass disc
  (concentric rings, slow specular sweep ~6s) that IS the scan button;
  while scanning it renders live progress (completed/total + total size in
  tabular numerals); results state uses glass Cards per group; risky
  section keeps its disclosure.
- Scan-complete moment: results materialize with blur-fade + scale
  0.98→1 over 220ms (skipped under reduced motion).
- CategoryDetail, Uninstaller, Backups, Maintenance, Settings, FirstRun:
  reskin onto Card/Button/Checkbox/etc.; identical structure, copy, and
  flows.

## Behavior contract & test policy

- All user-visible copy, aria-labels, roles, and store wiring byte-identical.
- Existing tests are the harness: they may only be edited where a query
  depended on incidental DOM structure (e.g. element tag/class), NEVER on
  visible behavior/copy; each such edit is called out in review.
- New tests: ui primitives smoke suite (button variants render + fire,
  dialog open/close + focus trap, checkbox/switch toggle, progress
  value/aria, badge variants); frontend line coverage stays ≥ the current
  93% floor minus at most 3pts, and every suite green per phase.

## Phases (each lands green: vitest + tsc)

1. Foundation: deps, tokens, glass utilities, cn(), Backdrop, ui/ layer +
   its smoke tests.
2. Shell: App/Sidebar/aurora/drag region/action-bar pattern.
3. Views: SmartScan (incl. lens hero) → CategoryDetail → Uninstaller →
   Backups → Maintenance → Settings → FirstRun.
4. Overlays: Dialog migration (ConfirmModal, UninstallConfirm,
   ResultsPanel, done dialog, ProgressOverlay on Progress).
5. Polish: motion pass, reduced-motion/transparency fallbacks, focus-visible
   audit, dead-style cleanup, full-suite + coverage gate, manual visual
   review in `task dev:desktop`.

## Performance & a11y floor

≤ 2 stacked blur regions; aurora static; animations GPU-cheap
(transform/opacity only); visible `:focus-visible` rings (accent, 2px
offset) on every interactive element; text on glass always uses the text
tokens (contrast ≥ 4.5:1 against the worst-case fill — the glass fills'
opacity floors above are chosen for this); `user-select` stays disabled
except inputs/paths.
