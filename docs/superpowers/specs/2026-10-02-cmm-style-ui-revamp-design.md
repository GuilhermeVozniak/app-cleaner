# CleanMyMac-style UI revamp (desktop) — design

Date: 2026-10-02. Branch: `feat/ui-revamp-cmm`. Supersedes the visual parts of
`2026-07-18-liquid-glass-ui-design.md`; the behaviour contract and test policy
there still apply.

## Goal

Make the desktop app look and feel like CleanMyMac 5 (reference screenshots
supplied by the owner): a dark canvas that repaints in each module's colour, a
thin icon-only rail, big light display titles, frosted cards, and one glowing
circular call to action per screen. Fix the clunky parts: tiny rail labels,
bold 30px headings, dialog-based progress, and a Settings page made of raw
config-key labels.

## Non-goals

- No light scheme. The module gradient is the brand; `color-scheme: dark`.
- No 3D artwork. Module icons are "gems": a glossy rounded square in the
  module hue with a white glyph.
- No behaviour changes except the two called out below (Run All, Login Items
  card). Every store, binding and event stays as is.

## Token system (`src/style.css`, Tailwind v4 `@theme`)

- Ink: white, 72%, 48%; inverse `#1a1140` for text on white buttons.
- Status: safe `#3ee57f`, moderate `#ffb340`, danger `#ff5b5b`.
- Surfaces: `glass-1` (card: white 10→5% gradient, 12% border, soft shadow),
  `glass-2` (stage/dialog: white 12%, blur 28px), `fill` (white 14%) for
  secondary buttons and inputs, `hairline` (white 10%) between rows.
- Type scale: display 40/1.1 regular, headline 26/1.2 semibold, stat 28,
  title 20, card 15, body 13, caption 12, micro 11. SF Pro throughout; the
  Display optical size kicks in at 20px and gives the light look.
- Radii: shell/card 20, control 10.
- Module hue tokens `--color-module-*` are consumed through the `--module`
  indirection var, which the shell sets on the app root and mirrors onto
  `<html>` so portalled dialogs inherit it.

## Canvas

`lib/modules.ts` carries a `canvas` per module (three gradient stops + a glow
colour). `Backdrop` paints a full-bleed gradient with a bright radial glow
top-right and shade bottom-left. The stops are registered with `@property`
so switching modules cross-fades (700ms) instead of snapping.

Palette: Smart Care violet, Cleanup green, Applications blue, Performance
orange, Space Lens purple, My Tools muted violet, Backups teal, Settings slate.

## Shell

- `TitleBar`: 44px draggable strip, module name centred.
- `StartOver`: title-strip action at top-left ("Start Over", or a Back link
  with `icon={ArrowLeft}`), portalled to `<body>` so a view's entrance
  transform cannot capture its fixed positioning.
- `Sidebar`: 64px icon rail, 44px tiles, labels in tooltips. Smart Care alone
  on top, the four modules, then My Tools / Backups / Settings in a bottom
  cluster; hairlines separate the groups. Active tile = white 14% fill plus a
  halo in the module hue.

## Components

- `ModuleIcon` (gem): xs 24, sm 32, md 40, lg 56, xl 120, hero 176.
- `ScanLens` (orb): 100px circle in the module hue with a soft glow. Idle it is
  the primary action (Scan / Clean / Run All / Smart Scan); busy it spins a
  ring and can become Stop. Caption slot underneath.
- `Stage` / `StageRow` / `StateMark`: the large panel a flow runs inside (gem
  left, title + rows right, actions bottom-right). Dialogs render a Stage
  inside `DialogContent`; the title slot takes a `DialogTitle`.
- `PageHeader`: left-aligned display title + subtitle + right slot.
- `Button`: primary (white), secondary/glass (translucent), ghost,
  destructive, module. `Progress` fills with the current text colour.
- Lists: one `glass-1` card per group with hairline-divided rows. Never nested
  cards.

## Screens

- Smart Care: health headline + thin disk bar; 3×2 tiles (Cleanup,
  Applications, Performance, Space Lens, Storage cleaned, Activity) each with
  the module gem bleeding off the top-right corner, a big stat and a status
  line; Smart Scan orb bottom-centre.
- Cleanup: hero (gem left, title/description/features right, Scan orb);
  scanning (gem, "Looking for junk…", category list with check marks, Stop
  orb); results ("We've found X you can clean", grid of group cards, collapsed
  Risky card, selection caption + Clean orb, Start Over in the title strip);
  Category detail (Back link, title + badge, note banner, hairline file rows).
- Confirm / progress / results for cleaning: Stages inside dialogs.
- Applications: headline "We've found N apps on your Mac", one card list,
  action bar with the Uninstall button; confirm/done as Stages.
- Performance: headline, three task cards + a Login Items row card, Run All
  orb (runs DNS → purge → Time Machine sequentially; new behaviour).
- Login Items, Backups, My Tools, Settings: `PageHeader` pages. Settings is
  macOS-style preference groups (Software Update, Cleanup, Backups, Scanning)
  with a plain label + helper sentence per row and a Save action bar.
- First Run: the hero composition without the rail.

## Dev harness

`vite` + `http://localhost:5173/?mock` installs a fake Wails bridge
(`src/dev/mock.ts`) with realistic data and timed events. `&view=<view>`
picks the first screen, `&flow=scan|category|confirm|clean|uninstall|
uninstalling` drives a flow to a state (`src/dev/flows.ts`), `&fda=0`,
`&empty=1`, `&update=1` flip fixtures. Never bundled: `main.tsx` imports it
only under `import.meta.env.DEV`. Headless Chrome with
`--virtual-time-budget` screenshots these URLs for review.

## Gotchas found while building

- tailwind-merge classifies unknown `text-*` classes as colours, so
  `text-body` + `text-ink` collapsed to one. `lib/cn.ts` registers the type
  scale with `extendTailwindMerge`.
- Any transform on a view root (entrance animation) becomes the containing
  block for `position: fixed` children; hence the `StartOver` portal.
- Tailwind v4 centres with the `translate` property; a keyframe that also
  animates `transform: translate(-50%,-50%)` doubles the offset. Dialog
  keyframes animate scale and opacity only.
