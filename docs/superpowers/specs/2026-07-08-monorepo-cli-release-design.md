# App Cleaner — Monorepo, Terminal CLI, and Release Pipeline: Design

**Date:** 2026-07-08 · **Status:** Plan 1 code-complete locally on branch `feat/monorepo-cli` (monorepo, engine extraction, web page, CI/release/pages workflows, universal wails build verified). GitHub publish (repo create, signing secrets, push, Pages, DNS) deferred to a single coordinated go-live after Plan 2 (§8 Terminal CLI) lands, since the cert export and DNS need the user's machine. · **Reference implementation:** `~/Dev/pessoal/option-tab` (same author, same patterns)

Productizes the existing App Cleaner desktop app: restructure into a monorepo mirroring option-tab, extract the engine so a full-parity terminal CLI can reuse it, add a landing page deployed to GitHub Pages, and wire CI + a signed/notarized release pipeline. Two implementation plans execute this spec: **Plan 1** (§2–§7: monorepo + web + CI/release + repo setup) and **Plan 2** (§8: TUI CLI).

## 1. Decisions (user-approved 2026-07-08)

| Decision | Value |
|---|---|
| GitHub repo | `GuilhermeVozniak/app-cleaner`, **public** (GitHub Pages requires public on free plan) |
| Go module rename | `github.com/guhcostan/app-cleaner` → `github.com/GuilhermeVozniak/app-cleaner/...` (per-module paths below) |
| CLI scope | **Full TUI parity** with mac-cleaner-cli (interactive mode + non-interactive flags), not just scriptable flags |
| Landing page domain | `app-cleaner.vozniak.dev` (CNAME; empty basePath) |
| Notarization identity | Team `CT22R575UG`, Apple ID `gui336699@gmail.com`, app-specific password from `~/Documents/Certificates/notorize_tool_password.txt` |
| Signing cert source | Export Developer ID identity `BA63827BD3FD3C6C80904C50ACC02A1E786B0D1F` from the login Keychain to a fresh p12 with a generated password (GitHub secrets are write-only; option-tab's values cannot be read back) |

## 2. Monorepo layout

```
app-cleaner/
├── go.work                        # use ./packages/engine ./apps/desktop (Plan 1); + ./apps/cli (Plan 2)
├── package.json                   # bun workspaces: apps/*, apps/desktop/frontend, packages/*
├── turbo.json  biome.json  lefthook.yml  Taskfile.yml   # copied from option-tab, adapted
├── .github/workflows/             # ci.yml · deploy-web.yml · release.yml (§6)
├── packages/
│   ├── engine/                    # Go module github.com/GuilhermeVozniak/app-cleaner/packages/engine
│   └── shared/                    # TS @app-cleaner/shared: PRODUCT + release-asset contract (§5)
├── apps/
│   ├── desktop/                   # Go module .../apps/desktop — current repo root, moved via git mv
│   ├── cli/                       # Go module .../apps/cli — Plan 2
│   └── web/                       # Next.js 15 landing page (@app-cleaner/web)
├── docs/  LICENSE  README.md      # docs stay at root; README gains monorepo map
```

- **History preserved**: all moves are `git mv` in one commit; no re-init, no squash.
- Root `README.md` rewritten for the monorepo (apps, dev commands, release flow); `apps/desktop` keeps a short app-specific README.
- `.superpowers/`, `docs/reference/porting-notes.json`, and specs/plans stay at root.
- Go version pin: `go 1.26` in all three modules and `go.work`; CI uses `go-version: "1.26"`.

## 3. Engine extraction (`packages/engine`)

Mechanical move of every `internal/*` package plus `internal/config` out of the desktop module so both apps can import them. `internal/` privacy is dropped deliberately — the engine is this repo's shared library.

| From (apps/desktop/internal/…) | To (packages/engine/…) |
|---|---|
| `core`, `fsx`, `scanners`, `backup`, `uninstall`, `maintenance`, `fda`, `grouping`, `config` | same package names, same file contents — only `package`-path imports rewritten |

- **No behavior change**: all 244 Go tests move with their packages and must pass unmodified (only import lines change).
- What stays in `apps/desktop`: `main.go`, `app.go` + `app_test.go` (bridge: bound methods, event emission, `splitByBackup`, `resolveScanIDs`, DisplayRow wiring), `frontend/`, `build/`, `tools/genicon`.
- `grouping.DisplayRow` remains the wire contract for the GUI **and** becomes the row model for the CLI file picker (§8.3).
- Module resolution: `apps/desktop` and `apps/cli` `go.mod`s carry `require .../packages/engine v0.0.0` + a relative `replace` directive; `go.work` at root exists for editor/tooling convenience. Every checkout builds without registry access.
- Engine module has **no** wails dependency; `go.mod` carries only `howett.net/plist` (v1.0.2-0.20250314012144-ee69052608d9, the wails-compatible pseudo-version — the MVS trap no longer applies across modules, but keep the pin to avoid surprises when go.work resolves).

## 4. Landing page (`apps/web`)

Same stack and structure as option-tab's web app: Next.js 15 `output: "export"`, React 19, Tailwind CSS 4, Biome, Vitest. `basePath: process.env.NEXT_PUBLIC_BASE_PATH ?? ""`; `public/CNAME` = `app-cleaner.vozniak.dev`.

Content (single page): hero (name, tagline "Reclaim disk space on your Mac", download button for the dmg), feature grid (16 scan categories with safety levels; undo backups; app uninstaller; maintenance tasks; native + open source), a CLI section (download tarball or `git clone && task build:cli`, one screenshot-style terminal block), footer (GitHub, license, "ported from mac-cleaner-cli"). Download links come from `@app-cleaner/shared` `downloadUrl()` / `latestReleaseUrl()` — never hard-coded.

## 5. Shared release-asset contract (`packages/shared`)

TS mirror of option-tab's, adapted:

```ts
export const PRODUCT = {
  name: "app-cleaner",
  displayName: "App Cleaner",
  repo: "https://github.com/GuilhermeVozniak/app-cleaner",
  site: "https://app-cleaner.vozniak.dev",
} as const;
// macOS-only: assets are
//   app-cleaner_<version>_darwin_universal.dmg          (GUI)
//   app-cleaner-cli_<version>_darwin_universal.tar.gz   (CLI; contains binary `app-cleaner`)
releaseAssetName(kind: "gui" | "cli", version: string): string
downloadUrl(kind, version) · latestReleaseUrl()
```

Release workflow file names and web download links both derive from this contract; a unit test pins the exact strings.

## 6. Workflows

**ci.yml** (push to main + PRs)
- `js` job: bun install → `turbo run lint test build` (biome over web/shared; desktop frontend keeps vitest + `tsc --noEmit`, wired as its `lint`/`test` scripts).
- `go` job: `macos-latest` runner (the engine's tests exercise darwin paths and the desktop build needs Xcode tooling). Steps: build the desktop frontend first (main.go embeds `frontend/dist` — same ordering trick as option-tab's go job), then `go test -race ./...` in each module, then golangci-lint (config adapted from option-tab's `.golangci.yml`).

**deploy-web.yml** — option-tab's verbatim (Pages via actions/deploy-pages, `enablement: true`, path filter `apps/web/**` + `packages/shared/**`, concurrency group `pages`).

**release.yml** (tag `v*`)
1. `test` gate job: full `go test -race` (all modules) + frontend vitest + web build. No Playwright (no e2e suite exists; revisit later).
2. `build-release` job (`macos-latest`, no matrix — macOS-only product):
   - `wails build -platform darwin/universal` in `apps/desktop`.
   - Import p12 from `MACOS_CERT_P12`/`MACOS_CERT_PASSWORD` into a throwaway keychain (option-tab's script verbatim, including the `find-identity | grep "Developer ID Application"` fail-fast).
   - `codesign --force --deep --options runtime --timestamp` the .app; verify.
   - Package dmg with `appdmg` (`build/darwin/dmg/appdmg.json`: app + /Applications link, positions copied from option-tab; **no background image** — omit the key).
   - Sign dmg, `notarytool submit --wait`, `stapler staple` + `validate`.
   - CLI: `CGO_ENABLED=0 go build` per arch (amd64, arm64) in `apps/cli`, `lipo -create` → universal binary `app-cleaner`, `codesign --options runtime --timestamp`, tar.gz. (Tarball itself is not notarized — standard for CLI tools; noted as future work.)
   - `softprops/action-gh-release` with both assets.
   - Steps guarded by `HAS_MACOS_SIGNING` like option-tab so forks build unsigned.

## 7. Repo + secrets setup (controller-run, never delegated to subagents)

1. `gh repo create GuilhermeVozniak/app-cleaner --public --source . --push` after the restructure lands on main.
2. Export identity `BA63827…` to a temp p12 (`security export` — user approves the Keychain prompt), password = freshly generated; `gh secret set MACOS_CERT_P12` (base64) and `MACOS_CERT_PASSWORD`; delete the temp file. No secret value is ever printed.
3. `gh secret set APPLE_ID` (=`gui336699@gmail.com`), `APPLE_TEAM_ID` (=`CT22R575UG`), `APPLE_APP_PASSWORD` (piped from `notorize_tool_password.txt`).
4. Pages: enabled by the first deploy-web run (`enablement: true`); set custom domain `app-cleaner.vozniak.dev`. **User action:** DNS CNAME `app-cleaner` → `guilhermevozniak.github.io` at the vozniak.dev provider.

## 8. Terminal CLI (`apps/cli`) — Plan 2

Full-parity port of mac-cleaner-cli's terminal UX on top of `packages/engine`. Normative behavior source: `docs/reference/porting-notes.json` (commands, interactive flow, file-picker state machine, exact texts) and the original's vitest suites (ported as Go tests, same discipline as the GUI port).

### 8.1 Framework & binary
- Bubble Tea + Lip Gloss + Bubbles (spinner, help); binary name **`app-cleaner`**; module `github.com/GuilhermeVozniak/app-cleaner/apps/cli`; flag/command parsing with `spf13/cobra` (matches the original's commander structure: subcommands + flags).
- `--version` from ldflags (`-X main.version=` set by release workflow; "dev" otherwise).

### 8.2 Command surface (parity with the original + engine-era additions)
| Command | Behavior |
|---|---|
| *(none)* | Interactive mode: scan all → category checkbox picker (space/a/i/→/enter) → file picker for flagged categories → confirm (default **yes**, as the CLI's interactive confirm was) → clean with progress → results |
| `scan [--categories c1,c2] [--json]` | Non-interactive scan; human summary or the original's JSON shape |
| `clean [--categories…] [--dry-run] [--yes] [--backup/--no-backup] [--json]` | Non-interactive clean; `--dry-run` never prompts nor deletes; without `--yes` asks a default-**false** confirm; backup default follows config/safety level (engine rules) |
| `uninstall [--app name]` | App picker (or direct by name); running-app guard; related-paths confirm |
| `maintenance --dns --purgeable --timemachine` | Sequential tasks with spinners; no flags → the original's "No maintenance tasks specified." hint |
| `backups list\|restore <path>\|clean-old` | Engine backup manager; shares `~/Library/Application Support/AppCleaner/` with the GUI — CLI backups appear in the GUI's Backups view and vice versa |

### 8.3 Interactive contracts (the hard parity core)
- **File picker**: dual-pane; keymaps exactly as ported in porting-notes (categories: `space/a/i/→/enter`; files: `space/a/i/d/m/h/c/←/backspace/enter`); FILES_PAGE_SIZE=6, DIR_VIS_CHILD_LIMIT=5, EXPAND_INCREMENT=10, FILE_NAME_WIDTH=35; selection state machine invariants (category–file coupling, cross-category isolation, `d` = visible rows of caret's directory only, per-category caret/expand persistence). Rows come from `engine/grouping.GroupItems` — the same DisplayRow stream the GUI renders.
- **Selection state lives in a pure Go package** (`apps/cli/internal/selection`) with table-driven tests porting file-picker.test.ts — the TUI update loop stays thin.
- **Output formatting**: engine `core.FormatSize`; the original's texts, rules (`─` ×50), `[DRY RUN]` prefix, errno-breakdown error lines, `~` home contraction.
- **Elevation**: engine's osascript admin prompt (GUI dialog) — deliberate deviation from the original's `sudo -n`; documented, consistent across GUI and CLI.
- **No donation prompt** (the original's ko-fi flow is dropped — deliberate deviation).
- Ctrl-C mid-clean: graceful cancel via the engine's context/cancel accounting (same semantics as GUI cancel).

### 8.4 JSON contract
`scan --json` / `clean --json` reproduce the original CLI's JSON shapes (porting-notes §commands) so existing automation ports over; a golden-file test pins each shape.

## 9. Testing
- Everything keeps its current suite; engine tests move untouched.
- New: shared-contract unit tests (TS), web smoke test (renders + download URLs), CLI selection/state + command tests (ported vitest cases + golden JSON), workflow lint (`actionlint`) in CI.
- Release gate = full go + frontend + web suites green.

## 10. Deliberate deviations (this phase)
- No Windows/Linux builds (engine is macOS-specific by design).
- No Playwright e2e (none exists; CI gates on unit suites + builds).
- CLI tarball not notarized (binary signed only).
- ko-fi donation prompt dropped from the CLI.
- No `go install` channel: the desktop/cli modules resolve the engine via `replace` directives (required for reliable unpublished-module builds), which `go install` ignores. Install channels: release tarball (primary) or `git clone && task build:cli`.

## 11. Out of scope
Homebrew tap/cask, auto-update (Sparkle), analytics, Windows/Linux ports, localization.