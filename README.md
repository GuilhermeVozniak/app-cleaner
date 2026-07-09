# App Cleaner

A native macOS cleaning suite — a Wails desktop app, a full-parity terminal CLI, and a
landing page, all built on one shared Go engine. Scan and remove caches, logs, old
downloads, orphaned `node_modules`, duplicate files and more; uninstall apps with their
leftovers; run system maintenance; undo cleans from move-based backups. 100% offline, no
telemetry.

A monorepo restructure of [mac-cleaner-cli](https://github.com/guhcostan/mac-cleaner-cli)
with full feature parity, splitting the original desktop-only port into one shared engine
reused by both a GUI and a TUI.

## Monorepo layout

```
app-cleaner/
├── packages/
│   ├── engine/            # Go module: scanners, backup, uninstall, maintenance, fsx,
│   │                      #   grouping, config, fda — shared by desktop + cli
│   └── shared/             # @app-cleaner/shared — release-asset naming contract (TS)
├── apps/
│   ├── desktop/            # Wails desktop app — see apps/desktop/README.md
│   │   └── frontend/       # React/TypeScript UI (Vite + Tailwind)
│   ├── cli/                 # Terminal UI (Bubble Tea) — full parity with the desktop app
│   └── web/                 # Next.js landing page → app-cleaner.vozniak.dev
├── .github/workflows/       # ci.yml · deploy-web.yml · release.yml
├── Taskfile.yml  turbo.json  biome.json  lefthook.yml  .golangci.yml
└── docs/                    # specs, plans, porting notes
```

## Apps at a glance

| App | What it is |
|---|---|
| `apps/desktop` | The GUI: Wails v2 + React. 16 scan categories, app uninstaller, maintenance tasks, backups/undo. |
| `apps/cli` | The terminal: full-parity Bubble Tea TUI plus non-interactive flags (`scan`, `clean`, `uninstall`, `maintenance`, `backups`), sharing the same `~/Library/Application Support/AppCleaner/` backup store as the GUI. |
| `apps/web` | The landing page: a static Next.js site with OS-aware download links, deployed to GitHub Pages. |

## Terminal CLI

- **`apps/cli`** — terminal CLI, full parity with the desktop app. See [`apps/cli/README.md`](apps/cli/README.md) for the command reference and keymap. Install via a release tarball or `git clone && task build:cli`.

## Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | 1.26+ | <https://go.dev/dl> |
| Bun | 1.1+ | <https://bun.sh> |
| Wails CLI | v2.13 | `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0` |
| Task | latest | <https://taskfile.dev/installation> |
| golangci-lint | v2 | <https://golangci-lint.run/welcome/install> |
| gofumpt | latest | `go install mvdan.cc/gofumpt@latest` |

## Quickstart

```bash
bun install          # installs JS/TS deps for every workspace, then `lefthook install`
task dev:desktop     # Wails dev mode (hot reload)
task dev:web         # landing page dev server
task dev:cli         # CLI from source (once apps/cli exists)
```

## Available tasks

```bash
task lint           # Biome (JS/TS, every workspace) + golangci-lint (Go, per module)
task test           # Vitest (JS/TS, every workspace) + go test -race -cover (Go, per module)
task build          # web build + wails build (+ cli build once apps/cli exists)
task build:cli       # build apps/cli/bin/app-cleaner
task dev:web         # Next.js dev server
task dev:desktop     # Wails dev mode
task dev:cli         # go run apps/cli from source
task release:prep    # assert a clean tree + green tests, print tag instructions
```

## Release flow

Tags of the form `vX.Y.Z` trigger `.github/workflows/release.yml`: it re-runs the full
test suite, then builds a signed and notarized universal `.dmg` for the desktop app and a
signed universal tarball for the CLI, and uploads both to the GitHub release. Asset names
follow `packages/shared`'s contract — `app-cleaner_<version>_darwin_universal.dmg` and
`app-cleaner-cli_<version>_darwin_universal.tar.gz`. Run `task release:prep` locally
before tagging.

## Further reading

- [apps/desktop/README.md](apps/desktop/README.md) — desktop app build/dev notes
- `docs/superpowers/specs/` and `docs/superpowers/plans/` — design + implementation plans
- `docs/reference/porting-notes.json` — the original CLI's normative behavior reference

## License

[MIT](LICENSE)
