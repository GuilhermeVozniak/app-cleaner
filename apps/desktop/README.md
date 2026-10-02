# App Cleaner — Desktop

The Wails v2 + React GUI. See the [root README](../../README.md) for the monorepo layout,
prerequisites, and the `task` commands shared across every app in this repo.

## Dev

```bash
wails doctor       # verify the toolchain
wails dev          # live-reload development
```

### UI work without the Go side

The frontend can run in a plain browser against a fake Wails bridge:

```bash
cd frontend && bun run dev
open "http://localhost:5173/?mock&view=dashboard"
```

`?mock` installs `src/dev/mock.ts` (realistic data, timed scan/clean/uninstall
events). `&view=<view>` picks the first screen; `&flow=scan|category|confirm|
clean|uninstall|uninstalling` drives a flow to that state; `&fda=0`, `&empty=1`
and `&update=1` flip fixtures. The mock is only imported in dev builds. See
`docs/superpowers/specs/2026-10-02-cmm-style-ui-revamp-design.md` for the
design system.

## Build

```bash
wails build -clean                              # build/bin/App Cleaner.app (host arch)
wails build -clean -platform darwin/universal   # universal release build
```

## Testing

```bash
go test ./... -race -cover                      # from apps/desktop/
cd frontend && bun run lint && bun run test      # tsc --noEmit + vitest
```

Signing and notarization run in CI (`.github/workflows/release.yml`) on tag push — see
the root README's release-flow section for the asset-naming contract.
