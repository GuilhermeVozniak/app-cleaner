# App Cleaner — Desktop

The Wails v2 + React GUI. See the [root README](../../README.md) for the monorepo layout,
prerequisites, and the `task` commands shared across every app in this repo.

## Dev

```bash
wails doctor       # verify the toolchain
wails dev          # live-reload development
```

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
