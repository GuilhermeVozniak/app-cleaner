# Canonical Contract — Monorepo + CLI Implementation Plans (2026-07-08)

Binding for every plan task. Spec: `docs/superpowers/specs/2026-07-08-monorepo-cli-release-design.md`. Where this contract and free-form prose in a task disagree, the contract governs.

## Module & package paths

| Module | Path (go.mod `module`) | Location |
|---|---|---|
| engine | `github.com/GuilhermeVozniak/app-cleaner/packages/engine` | `packages/engine/` |
| desktop | `github.com/GuilhermeVozniak/app-cleaner/apps/desktop` | `apps/desktop/` |
| cli | `github.com/GuilhermeVozniak/app-cleaner/apps/cli` | `apps/cli/` |

- Engine packages (moved 1:1 from today's `internal/`): `core`, `fsx`, `scanners`, `backup`, `uninstall`, `maintenance`, `fda`, `grouping`, `config`. Import as `github.com/GuilhermeVozniak/app-cleaner/packages/engine/<pkg>`.
- desktop/cli `go.mod`: `require github.com/GuilhermeVozniak/app-cleaner/packages/engine v0.0.0` + `replace github.com/GuilhermeVozniak/app-cleaner/packages/engine => ../../packages/engine`.
- `go.work` at repo root: `go 1.26` + `use (./packages/engine ./apps/desktop)`; Plan 2 Task 1 appends `./apps/cli`.
- All three modules declare `go 1.26`.
- **Forbidden in `apps/desktop` and `packages/engine`:** any `go get` (the plist@v1.0.1/wails MVS trap). Engine keeps `howett.net/plist v1.0.2-0.20250314012144-ee69052608d9`; desktop keeps `wails/v2 v2.13.0`. In `apps/cli`, `go get <dep>@latest` is allowed for cobra/charm deps only.

## Product & release-asset contract (TS `packages/shared`, npm name `@app-cleaner/shared`)

```ts
export const PRODUCT = {
  name: "app-cleaner",
  displayName: "App Cleaner",
  repo: "https://github.com/GuilhermeVozniak/app-cleaner",
  site: "https://app-cleaner.vozniak.dev",
} as const;
export type AssetKind = "gui" | "cli";
export function releaseAssetName(kind: AssetKind, version: string): string;
// gui -> `app-cleaner_${version}_darwin_universal.dmg`
// cli -> `app-cleaner-cli_${version}_darwin_universal.tar.gz`
export function downloadUrl(kind: AssetKind, version: string): string; // `${PRODUCT.repo}/releases/download/v${version}/${releaseAssetName(kind, version)}`
export function latestReleaseUrl(): string; // `${PRODUCT.repo}/releases/latest`
```

CLI binary name: **`app-cleaner`**. Tarball contains exactly `app-cleaner` at its root. Release tag format: `v<semver>` (single tag; no nested-module tags).

## JS workspace & tooling versions

- Root `package.json`: name `app-cleaner`, private, `packageManager: "bun@1.1.38"`, workspaces `["apps/*", "apps/desktop/frontend", "packages/*"]`, scripts `build|lint|test|dev` → `turbo run <x>`, `prepare` → `lefthook install`. Dev deps: `@biomejs/biome ^2.0.0`, `lefthook ^1.7.0`, `turbo ^2.0.0`, `typescript ^5.6.0`.
- `turbo.json`: copy option-tab's verbatim (`~/Dev/pessoal/option-tab/turbo.json`).
- `biome.json`: option-tab's, adapted — **exclude** `apps/desktop/frontend/**` (it keeps its own vitest/tsc gates; no biome churn on existing code) and `apps/desktop/frontend/wailsjs/**`.
- `apps/web`: Next.js `^15.0.0`, React `^19.0.0`, Tailwind `^4.3.2` via `@tailwindcss/postcss`, Vitest `^3.0.0`; `next.config.ts` with `output: "export"`, `images: { unoptimized: true }`, `basePath: process.env.NEXT_PUBLIC_BASE_PATH ?? ""`; `public/CNAME` = `app-cleaner.vozniak.dev`. npm name `@app-cleaner/web`, scripts `dev|build|lint(biome check .)|test(vitest run)`.
- `apps/desktop/frontend/package.json` gains workspace-compatible scripts: `lint` → `tsc --noEmit`, `test` → `vitest run`, `build` stays as-is. Do NOT add biome to it.
- `packages/shared`: npm name `@app-cleaner/shared`, `src/index.ts` + `src/index.test.ts`, scripts `build` (tsc to dist/, composite of option-tab's), `lint` (biome), `test` (vitest).

## Taskfile task names (root `Taskfile.yml`, task v3)

`lint`, `test`, `build`, `dev:web`, `dev:desktop`, `dev:cli`, `build:cli` (builds `apps/cli` → `apps/cli/bin/app-cleaner`), `release:prep` (asserts clean tree + green tests, then prints tag instructions). Go steps iterate modules explicitly: `packages/engine`, `apps/desktop`, `apps/cli` (guard `apps/cli` with existence check until Plan 2 lands).

## Workflows (`.github/workflows/`)

- `ci.yml`: jobs `js` (ubuntu: bun install --frozen-lockfile → `bun run lint` → `bun run test` → `bun run build`) and `go` (macos-latest: setup-go 1.26 + bun; build desktop frontend first because `main.go` embeds `frontend/dist`; then per-module `go test -race ./...`; then `gofmt -l` check + `go vet`). golangci-lint: option-tab's `.golangci.yml` adapted, run for each Go module.
- `deploy-web.yml`: option-tab's verbatim, paths `apps/web/**` + `packages/shared/**`, artifact `apps/web/out`.
- `release.yml`: on tag `v*`; job `test` (same gates as ci) → job `build-release` on `macos-latest`: wails build `-platform darwin/universal` in `apps/desktop`; keychain-import/sign/appdmg/notarize/staple steps copied from option-tab (secrets `MACOS_CERT_P12`, `MACOS_CERT_PASSWORD`, `APPLE_ID`, `APPLE_TEAM_ID`, `APPLE_APP_PASSWORD`; `HAS_MACOS_SIGNING` guard so forks build unsigned); CLI: `CGO_ENABLED=0 go build` for amd64+arm64 → `lipo -create` → codesign → tar.gz; asset names EXACTLY per the shared contract; upload via `softprops/action-gh-release@v2`.
- `appdmg.json` at `apps/desktop/build/darwin/dmg/appdmg.json`: option-tab's layout, title "App Cleaner Installer", paths to `App Cleaner.app`, **no `background` key**.

## Engine API surface the CLI consumes (read the code; signatures are authoritative there)

- `core`: `Category`, `CategoryID`, `CleanableItem`, `ScanResult`, `ScanSummary`, `CleanResult`, `FormatSize`, `CategoriesInOrder()`, category registry.
- `scanners`: `RunScans(ctx, ids, opts, concurrency, onResult)`, `Options`/`Roots`, `Get`, `All`.
- `fsx`: `RemoveItems` (+ dry-run), outcome/failure types, `ValidatePathSafety`.
- `backup`: `Manager` (`BackupItems`, `Restore`, `List`, `CleanOld`, `Delete`), `DefaultRoot`.
- `maintenance`: `FlushDNS`, `FreePurgeable`, `ClearTMSnapshots`, `ExecRunner`, `OsaElevator`, `Runner`/`Elevator` interfaces.
- `uninstall`: `ListApps`, `IsAppRunning`, `Uninstall`, `AppInfo`.
- `grouping`: `GroupItems`, `DisplayRow` (types `directory-header|file|expand-hint`), `TruncateDirectoryPath`.
- `config`: `Load`, `Save`, `Config` (DownloadsDaysOld, LargeFilesMinSize, BackupByDefault, BackupRetentionDays, Concurrency, ShowRisky, KeepLanguages, ExtraPaths).
- `fda`: `Check` (tri-state), `SettingsURL`.

## CLI structure (`apps/cli`)

```
apps/cli/
├── go.mod  main.go            # main calls cmd.Execute()
├── cmd/                       # cobra: root (interactive), scan, clean, uninstall, maintenance, backups
├── internal/selection/        # pure selection state machine + ported file-picker tests
├── internal/tui/              # bubbletea models: categorypicker, filepicker, confirm, progress, results
└── internal/output/           # text formatting: rules, [DRY RUN], ~ contraction, errno lines, JSON encode
```

- Deps: `spf13/cobra`, `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `charmbracelet/bubbles` — all `@latest` at execution time.
- Version: `var version = "dev"` in `main.go`, ldflags `-X main.version=` in release workflow and `build:cli`.
- Constants (from the original, binding): `FILES_PAGE_SIZE=6`, `DIR_VIS_CHILD_LIMIT=5`, `EXPAND_INCREMENT=10`, `FILE_NAME_WIDTH=35`. Keymaps — categories pane: `space` toggle, `a` all, `i` invert, `→`/`enter` drill/confirm; files pane: `space/a/i/d/m/h/c/←/backspace/enter` (`d` = toggle visible files of caret's directory; `m` = show more; `h` = toggle hidden; `c` = copy path via pbcopy).
- Selection invariants: selecting a file auto-selects its category; deselecting the last file deselects the category; category deselect clears its file selections; per-category caret/expand state persists across pane switches; cross-category isolation. Normative tests: original clone `/private/tmp/claude-501/-Users-guilherme-Dev-pessoal-app-cleaner/408cddb2-bb70-490e-b401-299ae99d3ad0/scratchpad/mac-cleaner-cli/src/pickers/file-picker.test.ts` (and `.ts` source next to it).
- Confirm defaults: interactive-mode "Proceed with cleaning?" = **yes**; non-interactive `clean` without `--yes` = **no**; `--dry-run` never prompts.
- JSON shapes for `scan --json`/`clean --json`: reproduce the original per `docs/reference/porting-notes.json` §commands; golden-file tests pin them.
- Behavior always comes from the engine — the CLI never reimplements scanning/deletion/backup logic.

## Global constraints (every task)

- TDD: failing test → implement → pass → commit. Frequent commits ending with `Co-Authored-By: WOZCODE <contact@withwoz.com>`.
- All 244 existing Go tests + 80 frontend tests must stay green through every task; restructure tasks run them explicitly.
- `t.TempDir()` only; never touch the real `$HOME` in tests; never shell out in tests except via existing fake runners.
- gofmt/go vet clean at every commit; TS: `tsc --noEmit` clean.
- Reference material: option-tab repo at `~/Dev/pessoal/option-tab` (read-only), original CLI clone at the scratchpad path above (read-only), porting-notes at `docs/reference/porting-notes.json`.
- Secrets (§7 of spec) are controller-run steps: plans document them as instructions for the session controller, never as subagent actions, and no secret value ever appears in any file or transcript.