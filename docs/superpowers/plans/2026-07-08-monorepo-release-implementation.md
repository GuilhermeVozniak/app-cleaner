# App Cleaner Monorepo + Release Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the App Cleaner repo into a monorepo (engine extracted for reuse), add a landing page deployed to GitHub Pages, and wire CI plus a signed/notarized tag-triggered release pipeline — spec §2–§7.

**Architecture:** `git mv` restructure preserving history; three-module Go workspace (`packages/engine`, `apps/desktop`, later `apps/cli`) resolved via require+replace; bun/turbo JS workspace with `packages/shared` (release-asset contract) and `apps/web` (Next.js 15 static export); GitHub Actions for CI, Pages deploy, and release (option-tab's proven patterns).

**Tech Stack:** Go 1.26, Wails v2.13, bun + turbo 2 + biome 2 + lefthook, Next.js 15 + React 19 + Tailwind 4, Taskfile, GitHub Actions + appdmg + notarytool.

## Global Constraints

Binding contract for every task: `docs/superpowers/plans/2026-07-08-monorepo-cli-contract.md` (module paths, asset names, tooling versions, task names, secrets rules). Spec: `docs/superpowers/specs/2026-07-08-monorepo-cli-release-design.md`. Additional hard rules:
- Never run `go get` in `apps/desktop` or `packages/engine` (plist/wails MVS trap); the single sanctioned exception is `go mod tidy` for the new engine module in Task 2, exactly as that task documents.
- All 244 Go + 80 frontend tests stay green through every task.
- Task 8 is controller-run only (secrets); never dispatch it to a subagent.
- Commits end with `Co-Authored-By: WOZCODE <contact@withwoz.com>`.
- No v1.0.0 tag in this plan — tagging waits for Plan 2 (`2026-07-08-cli-implementation.md`).

---
## Repo-root inventory (checked live before writing this plan)

`git ls-files` + `ls -la` at repo root today shows exactly these top-level entries:

| Entry | Tracked by git? | Destination |
|---|---|---|
| `.git/` | n/a | untouched |
| `.gitignore` | yes | stays at root, **content edited** (paths below) |
| `.superpowers/` | **no** (gitignored, `.superpowers/` pattern) | stays at root, untouched (nothing to `git mv`) |
| `LICENSE` | yes | stays at root, untouched |
| `README.md` | yes | stays at root, untouched (monorepo rewrite is a later Plan 1 task, not this one) |
| `app.go` | yes | → `apps/desktop/app.go` |
| `app_test.go` | yes | → `apps/desktop/app_test.go` |
| `build/` | yes | → `apps/desktop/build/` |
| `docs/` | yes | stays at root, untouched |
| `frontend/` | yes | → `apps/desktop/frontend/` |
| `go.mod` | yes | → `apps/desktop/go.mod` |
| `go.sum` | yes | → `apps/desktop/go.sum` |
| `internal/` (9 subpackages) | yes | → `apps/desktop/internal/` (stays here for Task 1; Task 2 extracts it to `packages/engine/`) |
| `mac-cleaner-cli/` | **no** (gitignored, "reference clone ... not part of this repo") | stays at root, untouched (nothing to `git mv`) |
| `main.go` | yes | → `apps/desktop/main.go` |
| `tools/` (`tools/genicon/main.go`) | yes | → `apps/desktop/tools/` |
| `wails.json` | yes | → `apps/desktop/wails.json` |

Confirmed facts that shape the steps below:
- `internal/*.go` files import `github.com/guhcostan/app-cleaner/internal/<pkg>` (verified across all 9 packages). Because Task 1 moves `go.mod` (module `github.com/guhcostan/app-cleaner`, unchanged) together with `internal/` as one atomic unit into `apps/desktop/`, every import path still resolves correctly — **zero import edits are needed in Task 1**. The module rename and import rewrite happens in Task 2.
- `main.go` has `//go:embed all:frontend/dist` and `//go:embed build/appicon.png` — both paths are relative to `apps/desktop/` after the move and move together with it, so the embeds keep working unchanged.
- `frontend/dist/` and `frontend/node_modules/` already exist on disk locally (pre-built) even though gitignored; `git mv frontend apps/desktop/frontend` moves the whole directory tree on disk (git mv performs a physical directory rename, carrying untracked contents along), so nothing needs reinstalling/rebuilding to run the frontend test suite after the move.
- `tools/genicon/main.go` imports only stdlib (`image`, `image/color`, `image/png`, `log`, `math`, `os`) — no module-path-sensitive imports, moves with zero edits.
- `.gitignore` today:
  ```
  # reference clone of the original CLI (not part of this repo)
  mac-cleaner-cli/

  # Go / Wails
  build/bin/
  *.log

  # Node
  node_modules/
  frontend/dist/

  # OS
  .DS_Store

  # SDD scratch (ledger, briefs)
  .superpowers/
  ```
  Per gitignore semantics, a pattern with a slash in the **middle** (not just a trailing slash) is anchored to the directory of the `.gitignore` file itself. `build/bin/` and `frontend/dist/` both have a middle slash, so they are currently anchored to repo-root and would **stop matching** once those directories live under `apps/desktop/`. `mac-cleaner-cli/` and `.superpowers/` have only a *trailing* slash (no middle slash), so they already match at any depth and need no change; `*.log`, `node_modules/`, `.DS_Store` are depth-independent and also need no change. Only `build/bin/` and `frontend/dist/` need rewriting to `apps/desktop/build/bin/` and `apps/desktop/frontend/dist/`.

---

### Task 1: Move the desktop app to `apps/desktop`

#### Files
- **Modify:** `.gitignore`
- **Move (`git mv`, history-preserving), no content edits:** `main.go`, `app.go`, `app_test.go`, `wails.json`, `go.mod`, `go.sum`, `frontend/` (incl. `frontend/wailsjs/`, `frontend/dist/`, `frontend/node_modules/`), `build/` (incl. `build/bin/`), `tools/` (incl. `tools/genicon/main.go`), `internal/` (all 9 subpackages, staying nested — Task 2 extracts them)
- **Untouched:** `docs/`, `LICENSE`, `README.md`, `.superpowers/` (gitignored, not tracked), `mac-cleaner-cli/` (gitignored, not tracked)
- **Test:** no test files change; existing `apps/desktop/*_test.go` (moved) and `apps/desktop/internal/**/*_test.go` (moved) and `apps/desktop/frontend/src/**/*.test.tsx` (moved) are the verification surface — they must all still pass, unmodified.

#### Interfaces
- **Consumes:** nothing (first task of Plan 1).
- **Produces:** `apps/desktop/` containing `main.go`, `app.go`, `app_test.go`, `wails.json`, `go.mod` (module `github.com/guhcostan/app-cleaner`, **unchanged** — renamed in Task 2), `go.sum` (byte-identical to today's), `frontend/`, `build/`, `tools/genicon/`, and `internal/{backup,config,core,fda,fsx,grouping,maintenance,scanners,uninstall}/` (still present here — Task 2's move source is `apps/desktop/internal/<pkg>`). Repo root after this task contains only `docs/`, `LICENSE`, `README.md`, `.gitignore`, `apps/desktop/` as git-tracked entries (plus the untracked `.superpowers/`, `mac-cleaner-cli/`).

#### Steps

- [ ] **Step 1: Confirm the pre-move baseline is green**
  Run:
  ```
  go build ./... && go test ./...
  ```
  Expected: `ok` for every package, all 244 tests pass (this is the "before" snapshot — nothing to fix yet, just a checkpoint so any later failure is attributable to the move).

- [ ] **Step 2: Create the destination directory and move every desktop file/dir into it in one commit-sized batch**
  ```
  mkdir -p apps/desktop
  git mv main.go app.go app_test.go wails.json go.mod go.sum frontend build tools internal apps/desktop/
  ```
  Expected: `git status` shows all moved paths as renames (`R`), e.g. `renamed: internal/core/categories.go -> apps/desktop/internal/core/categories.go`, with no content diff. Repo root (`ls`) now shows only `.git`, `.gitignore`, `.superpowers` (untracked), `LICENSE`, `README.md`, `apps/desktop`, `docs`, `mac-cleaner-cli` (untracked).

- [ ] **Step 3: Update `.gitignore` for the new layout**
  Edit `/Users/guilherme/Dev/pessoal/app-cleaner/.gitignore` to:
  ```
  # reference clone of the original CLI (not part of this repo)
  mac-cleaner-cli/

  # Go / Wails
  apps/desktop/build/bin/
  *.log

  # Node
  node_modules/
  apps/desktop/frontend/dist/

  # OS
  .DS_Store

  # SDD scratch (ledger, briefs)
  .superpowers/
  ```
  (Only the two anchored lines change; `mac-cleaner-cli/`, `node_modules/`, `*.log`, `.DS_Store`, `.superpowers/` are untouched because they already match at any depth.)

- [ ] **Step 4: Verify the Go module builds and tests from its new location, unmodified**
  ```
  cd apps/desktop && go build ./... && go test ./...
  ```
  Expected: identical result to Step 1 — `ok` for every package (`apps/desktop`, `apps/desktop/internal/backup`, `.../config`, `.../core`, `.../fda`, `.../fsx`, `.../grouping`, `.../maintenance`, `.../scanners`, `.../uninstall`), all 244 tests pass, zero import edits were needed (module path and internal package layout moved together, so `github.com/guhcostan/app-cleaner/internal/...` imports still resolve).

- [ ] **Step 5: Verify the frontend test suite still passes from its new location**
  ```
  cd apps/desktop/frontend && npx vitest run
  ```
  (`node_modules/` moved with the directory in Step 2, so no `npm install` should be needed; if `vitest` isn't resolvable, fall back to `npm ci && npx vitest run`.)
  Expected: all 80 frontend tests pass, unmodified.

- [ ] **Step 6: gofmt/vet sanity check on the moved tree**
  ```
  cd apps/desktop && gofmt -l . && go vet ./...
  ```
  Expected: `gofmt -l .` prints nothing (no files need reformatting — pure move, no content changed); `go vet ./...` is clean.

- [ ] **Step 7: Commit**
  ```
  git add -A
  git status   # confirm only the expected renames + .gitignore edit are staged
  git commit -m "$(cat <<'EOF'
  Move desktop app into apps/desktop for the monorepo restructure

  Relocates main.go, app.go, app_test.go, wails.json, go.mod/go.sum,
  frontend/, build/, tools/, and internal/ under apps/desktop/ via git mv
  (history preserved, zero import changes needed). docs/, LICENSE,
  README.md, and .gitignore stay at the repo root; .gitignore's
  directory-anchored patterns are updated for the new paths.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  git status
  ```
  Expected: commit succeeds; `git status` reports a clean tree.

---

### Task 2: Extract the engine to `packages/engine` and rename module paths

#### Files
- **Create:** `packages/engine/go.mod`, `packages/engine/go.sum` (generated by `go mod tidy`, new module — no prior file existed), root `go.work`
- **Move (`git mv`):** `apps/desktop/internal/{backup,config,core,fda,fsx,grouping,maintenance,scanners,uninstall}` → `packages/engine/{backup,config,core,fda,fsx,grouping,maintenance,scanners,uninstall}` (package names unchanged, only location changes)
- **Modify:** `apps/desktop/go.mod` (module line + add engine `require`+`replace`; every other require line — wails, plist, all indirect deps — stays byte-for-byte identical); every `*.go` file in `packages/engine/**` and `apps/desktop/{app.go,app_test.go}` (import-path rewrite only, no logic changes)
- **Not modified:** `apps/desktop/go.sum` (byte-identical to Task 1's output — see Step 8 rationale), `apps/desktop/frontend/wailsjs/**` (verified below — no regeneration needed)
- **Test:** all moved `*_test.go` files under `packages/engine/**` (unmodified except import lines) and `apps/desktop/app_test.go` (unmodified) are the verification surface.

#### Interfaces
- **Consumes:** Task 1's output — `apps/desktop/internal/<pkg>` as the move source; `apps/desktop/go.mod`/`go.sum` as the base to edit.
- **Produces:**
  - Module `github.com/GuilhermeVozniak/app-cleaner/packages/engine` at `packages/engine/`, containing packages `core`, `fsx`, `scanners`, `backup`, `uninstall`, `maintenance`, `fda`, `grouping`, `config` — this is the exact API surface listed in the contract's "Engine API surface" section, now importable as `github.com/GuilhermeVozniak/app-cleaner/packages/engine/<pkg>` by both `apps/desktop` (already wired here) and, in Plan 2, `apps/cli`.
  - Module `github.com/GuilhermeVozniak/app-cleaner/apps/desktop` at `apps/desktop/`, with `require github.com/GuilhermeVozniak/app-cleaner/packages/engine v0.0.0` + `replace github.com/GuilhermeVozniak/app-cleaner/packages/engine => ../../packages/engine`.
  - Root `go.work` (`go 1.26`, `use (./packages/engine ./apps/desktop)`) — Plan 2 Task 1 appends `./apps/cli` to it.
  - `packages/engine/go.mod` requiring exactly `howett.net/plist v1.0.2-0.20250314012144-ee69052608d9` — this is the pin every later Go task (Plan 2's `apps/cli`) must not disturb via `go get`.

#### Why `go mod tidy` is safe here (and only here)

`packages/engine/go.mod` doesn't exist yet — there is no prior state a `go mod tidy` could corrupt, and the general "no `go get`/`go mod tidy`" caution in this project exists specifically to avoid the plist-vs-wails MVS trap (an ordinary `go get` on a module that also (transitively) requires `howett.net/plist` can bump it from the pinned pseudo-version `v1.0.2-0.20250314012144-ee69052608d9` to the newer tagged `v1.0.1`-family release, which is incompatible with the Wails-vendored fork this repo actually needs). `packages/engine` has **no wails dependency at all** (confirmed: none of the 9 packages import anything under `github.com/wailsapp/...`; the only external, non-stdlib import anywhere in this package tree is `howett.net/plist` in `uninstall/related.go`), so there is no wails requirement in its module graph to conflict with and nothing MVS can bump the plist requirement against — `go mod tidy` here has exactly one external module to resolve and it's the one we hand-wrote into `go.mod` ourselves. `packages/engine/go.sum` is a brand-new file that must be generated somehow — `tidy` is the correct, standard way to generate it for a module that has never had one, as opposed to `go get` (which is what's actually forbidden — it re-resolves versions; `tidy` on an already-fully-specified `go.mod` only fills in `go.sum` and indirect bookkeeping for what's already declared). Step 6 below explicitly greps the resulting `go.mod` to confirm the pin didn't move.

Separately, `apps/desktop/go.mod` is **not** tidied in this task: its plist requirement is only ever used transitively (through `packages/engine/uninstall`, which `app.go` imports indirectly) — the existing `go.sum` (moved byte-for-byte from today's, per Task 1) already carries the correct hash entries for `howett.net/plist v1.0.2-0.20250314012144-ee69052608d9`, and those entries stay valid regardless of how `go.mod`'s require block is worded, because `go.sum` is keyed by `module@version`, not by which `go.mod` declares it or whether it's marked `// indirect`. The new `packages/engine` module itself needs **no** `go.sum` entry in `apps/desktop/go.sum` at all — it's resolved via a local filesystem `replace` directive, and Go does not checksum-verify locally-replaced modules. So Step 8 is a hand-edit of two lines (module name + engine require/replace) with the rest of the require block copied verbatim, and `go build`/`go test` under the default `-mod=readonly` succeed without ever invoking `go get` or `go mod tidy` in `apps/desktop`.

#### Import-path rewrite — exact old → new prefixes

Every one of the 9 packages was imported under a single uniform prefix, so **one** substitution covers all of them (this also means the `guhcostan`→`GuilhermeVozniak` module rename and the `internal/`→`packages/engine/` relocation collapse into a single find/replace — there is no per-package variation):

| Old prefix (any of the 9 suffixes: `core fsx scanners backup uninstall maintenance fda grouping config`) | New prefix |
|---|---|
| `github.com/guhcostan/app-cleaner/internal/` | `github.com/GuilhermeVozniak/app-cleaner/packages/engine/` |

This string appears only in Go import statements (`internal/*.go` files that import sibling internal packages, and `apps/desktop/app.go`/`app_test.go`). It does **not** appear in `frontend/wailsjs/**` (verified below), `wails.json` (branding strings only, e.g. `"author": {"name": "guhcostan"}` — cosmetic, fixed by Task 9 Step 1's branding pass, not here), or `tools/genicon/main.go` (stdlib-only imports).

#### wailsjs bindings — verified, no regeneration needed

Read `apps/desktop/frontend/wailsjs/go/main/App.d.ts`, `App.js`, and `frontend/wailsjs/go/models.ts` directly (post-Task-1 paths): the generated bindings reference Go types purely by **short package name** — `import {core} from '../models'`, `import {config} from '../models'`, `import {grouping} from '../models'`, `import {uninstall} from '../models'`, `import {backup} from '../models'`, `import {maintenance} from '../models'`, `import {main} from '../models'` — and at runtime dispatch via `window['go']['main']['App']['<Method>']()`. None of the generated TypeScript contains the Go module import path string (`github.com/guhcostan/...` or `internal/...`) anywhere — Wails' binding generator keys namespaces off each imported package's short identifier, not its full import path. Since this task renames only the module path and the packages' *location* (not their package names, which stay `core`, `config`, `grouping`, `uninstall`, `backup`, `maintenance`), **`wails generate module` is not required** by this task. (`wails build` itself — which would also regenerate bindings as a side effect — is deliberately deferred; see Step 10.)

#### Steps

- [ ] **Step 1: Create the engine directory and move the 9 packages into it**
  ```
  mkdir -p packages/engine
  git mv apps/desktop/internal/backup apps/desktop/internal/config apps/desktop/internal/core \
         apps/desktop/internal/fda apps/desktop/internal/fsx apps/desktop/internal/grouping \
         apps/desktop/internal/maintenance apps/desktop/internal/scanners apps/desktop/internal/uninstall \
         packages/engine/
  rmdir apps/desktop/internal   # now-empty dir; git never tracked the dir itself, only its files
  ```
  Expected: `git status` shows all files as renames into `packages/engine/<pkg>/...`; `ls apps/desktop` no longer lists `internal`.

- [ ] **Step 2: Confirm the expected "no module context" failure**
  ```
  cd packages/engine && go build ./...
  ```
  Expected failure: `go: go.mod file not found in current directory or any parent directory` (or similar) — proves `packages/engine` has no module of its own yet and the move is what's blocking the build, not a code issue.

- [ ] **Step 3: Write `packages/engine/go.mod` (hand-written, before tidy)**
  Create `/Users/guilherme/Dev/pessoal/app-cleaner/packages/engine/go.mod`:
  ```
  module github.com/GuilhermeVozniak/app-cleaner/packages/engine

  go 1.26

  require howett.net/plist v1.0.2-0.20250314012144-ee69052608d9
  ```

- [ ] **Step 4: Confirm the expected "stale import path" failure**
  ```
  cd packages/engine && go build ./...
  ```
  Expected failure: import errors on every package, e.g. `package github.com/guhcostan/app-cleaner/internal/core is not in std ...` / `no required module provides package github.com/guhcostan/app-cleaner/internal/core` — proves the import strings (still pointing at the old module+location) are the remaining blocker.

- [ ] **Step 5: Rewrite import paths across both modules in one global substitution**
  macOS ships BSD sed, where `-i` takes a **mandatory** in-place suffix argument (use `''` for "no backup", passed as its own shell argument — writing `-i ''` as two separate tokens; `-i''` glued together or a bare `-i` will either error out or, worse on some BSD builds, silently treat the next argument as the backup suffix instead of the script). Run from repo root:
  ```
  find apps/desktop packages/engine -type f -name '*.go' -print0 \
    | xargs -0 sed -i '' 's#github\.com/guhcostan/app-cleaner/internal/#github.com/GuilhermeVozniak/app-cleaner/packages/engine/#g'
  ```
  Expected: no output (silent success). Verify no stale references remain:
  ```
  grep -rl 'guhcostan/app-cleaner/internal' apps/desktop packages/engine
  ```
  Expected: no matches (empty output / exit code 1).

- [ ] **Step 6: Generate `packages/engine`'s go.sum via `go mod tidy` and verify the plist pin survived**
  ```
  cd packages/engine && go mod tidy
  cat go.mod
  grep plist go.sum
  ```
  Expected `go.mod` still reads:
  ```
  module github.com/GuilhermeVozniak/app-cleaner/packages/engine

  go 1.26

  require howett.net/plist v1.0.2-0.20250314012144-ee69052608d9
  ```
  (version pin unchanged — `tidy` had only this one require line to satisfy and nothing else in the graph to bump it against, since there is no wails dependency in this module). `go.sum` contains exactly two `howett.net/plist v1.0.2-0.20250314012144-ee69052608d9` lines (the module hash and the go.mod hash) — no `github.com/jessevdk/go-flags` or `gopkg.in/yaml.v3` entries (those are only imports of plist's own `cmd/ply` tool, which `packages/engine` never imports).

- [ ] **Step 7: Build and test the engine module standalone**
  ```
  cd packages/engine && go build ./... && go test ./...
  ```
  Expected: `ok` for `packages/engine/backup`, `.../config`, `.../core`, `.../fda`, `.../fsx`, `.../grouping`, `.../maintenance`, `.../scanners`, `.../uninstall` — all tests that used to live under `apps/desktop/internal/**` now pass here, unmodified except their import lines.

- [ ] **Step 8: Rewrite `apps/desktop/go.mod` — module line + engine wiring only**
  Edit `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/go.mod` so the top reads:
  ```
  module github.com/GuilhermeVozniak/app-cleaner/apps/desktop

  go 1.26

  require (
  	github.com/GuilhermeVozniak/app-cleaner/packages/engine v0.0.0
  	github.com/wailsapp/wails/v2 v2.13.0
  	howett.net/plist v1.0.2-0.20250314012144-ee69052608d9
  )

  replace github.com/GuilhermeVozniak/app-cleaner/packages/engine => ../../packages/engine

  require (
  	git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3 // indirect
  	github.com/bep/debounce v1.2.1 // indirect
  	github.com/go-ole/go-ole v1.3.0 // indirect
  	github.com/godbus/dbus/v5 v5.1.0 // indirect
  	github.com/google/uuid v1.6.0 // indirect
  	github.com/gorilla/websocket v1.5.3 // indirect
  	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
  	github.com/labstack/echo/v4 v4.13.3 // indirect
  	github.com/labstack/gommon v0.4.2 // indirect
  	github.com/leaanthony/go-ansi-parser v1.6.1 // indirect
  	github.com/leaanthony/gosod v1.0.4 // indirect
  	github.com/leaanthony/slicer v1.6.0 // indirect
  	github.com/leaanthony/u v1.1.1 // indirect
  	github.com/mattn/go-colorable v0.1.13 // indirect
  	github.com/mattn/go-isatty v0.0.20 // indirect
  	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
  	github.com/pkg/errors v0.9.1 // indirect
  	github.com/rivo/uniseg v0.4.7 // indirect
  	github.com/samber/lo v1.49.1 // indirect
  	github.com/tkrajina/go-reflector v0.5.8 // indirect
  	github.com/valyala/bytebufferpool v1.0.0 // indirect
  	github.com/valyala/fasttemplate v1.2.2 // indirect
  	github.com/wailsapp/go-webview2 v1.0.22 // indirect
  	github.com/wailsapp/mimetype v1.4.1 // indirect
  	golang.org/x/crypto v0.51.0 // indirect
  	golang.org/x/net v0.54.0 // indirect
  	golang.org/x/sys v0.44.0 // indirect
  	golang.org/x/text v0.37.0 // indirect
  )
  ```
  Only the `module` line changed and the `github.com/GuilhermeVozniak/app-cleaner/packages/engine v0.0.0` require + its `replace` line were added; the `wails/v2`, `howett.net/plist`, and every indirect line are copied verbatim from the pre-existing `go.mod` — **do not run `go get` or `go mod tidy` here**; `apps/desktop/go.sum` is not touched at all in this step.

- [ ] **Step 9: Write root `go.work`**
  Create `/Users/guilherme/Dev/pessoal/app-cleaner/go.work`:
  ```
  go 1.26

  use (
  	./packages/engine
  	./apps/desktop
  )
  ```

- [ ] **Step 10: Verify `apps/desktop` builds and tests against the local engine replace, gofmt/vet clean in both modules**
  ```
  cd apps/desktop && go build ./... && go test ./...
  cd apps/desktop && gofmt -l . && go vet ./...
  cd packages/engine && gofmt -l . && go vet ./...
  ```
  Expected: `apps/desktop` build/test succeed with **zero** changes to `apps/desktop/go.sum` (confirm with `git diff --stat apps/desktop/go.sum` — empty); all 244 tests are green across the two modules combined (however many now live in `packages/engine/**` vs. `apps/desktop/app_test.go`); `gofmt -l .` prints nothing in both modules; `go vet ./...` clean in both. A full `wails build` (which produces the signed/notarized `.app` and would also be Wails' own opportunity to regenerate `frontend/wailsjs/**`) is **not** run in this task — Task 1's `go build ./...` plus this task's `go build ./...`/`go test ./...` are the build-correctness gate for the restructure; the end-to-end `wails build` check is deferred to Plan 1's final task (per contract, once CI/release workflows exist to actually exercise it).

- [ ] **Step 11: Confirm no stray old-module references remain anywhere in Go source**
  ```
  grep -rn 'github.com/guhcostan' apps/desktop packages/engine --include='*.go'
  ```
  Expected: no matches. The grep is deliberately scoped to the module-import prefix `github.com/guhcostan` (not a bare `guhcostan`): after Step 5's rewrite no Go import path references the old module, but `main.go` still carries two cosmetic branding strings — `SingleInstanceLock.UniqueId: "com.guhcostan.appcleaner"` and the About-dialog copyright `"© 2026 guhcostan — MIT"` — and the `com.guhcostan.appcleaner` bundle ID also lives in `wails.json` and `build/darwin/Info*.plist`. Those are product branding, not import paths (none contain the `github.com/guhcostan` import prefix), so this import-scoped grep correctly ignores them; a bare `grep 'guhcostan' --include='*.go'` here would match `main.go` and wrongly report a stale reference. Task 9 Step 1's branding-consistency pass rewrites all of them; leave them untouched in this module-rename task.

- [ ] **Step 12: Commit**
  ```
  git add -A
  git status   # confirm: packages/engine/** (new + moved), apps/desktop/go.mod, apps/desktop/internal removed,
               # go.work (new); apps/desktop/go.sum should show NO diff
  git commit -m "$(cat <<'EOF'
  Extract engine to packages/engine; rename module paths off guhcostan

  Moves the 9 internal packages (core, fsx, scanners, backup, uninstall,
  maintenance, fda, grouping, config) from apps/desktop/internal to a new
  standalone Go module packages/engine (github.com/GuilhermeVozniak/app-
  cleaner/packages/engine), resolved by apps/desktop via a local replace
  directive. Renames apps/desktop's module to github.com/GuilhermeVozniak/
  app-cleaner/apps/desktop. Adds root go.work for editor/tooling
  convenience. All import paths rewritten mechanically (one global
  find/sed substitution); no behavior change, no wails dependency added
  to the engine, plist pin unchanged, apps/desktop/go.sum untouched.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  git status
  ```
  Expected: commit succeeds; `git status` reports a clean tree.

---

### Task 3: `packages/shared` — product + release-asset contract

Reference read: `~/Dev/pessoal/option-tab/packages/shared/{package.json,src/index.ts,src/index.test.ts,tsconfig.json,vitest.config.ts}`. Contract: `docs/superpowers/plans/2026-07-08-monorepo-cli-contract.md` §"Product & release-asset contract" and §"JS workspace & tooling versions".

Every command below runs with `cd /Users/guilherme/Dev/pessoal/app-cleaner/packages/shared` (or an absolute path — do not rely on shell state persisting). This package has **no root workspace to join yet** (Task 5 creates root `package.json`/workspaces) — `bun install` here must work standalone, and it does: verified end-to-end in a scratch copy of this exact layout (`bun install` → 52 packages, `bun run test` → 5 passed, `bun run build` → `dist/index.js` + `dist/index.d.ts` only).

Do **not** add a `biome.json` here and do not run `bun run lint` during this task — the `lint` script (`biome check .`) needs the root `biome.json` and hoisted `@biomejs/biome` devDependency that Task 5 provides. The script is declared now (per contract) but stays unexercised until Task 5 lands; Task 5's plan section must run it once the root tooling exists.

#### Files
- Create: `packages/shared/package.json`
- Create: `packages/shared/tsconfig.json`
- Create: `packages/shared/vitest.config.ts`
- Create: `packages/shared/src/index.ts`
- Create: `packages/shared/src/index.test.ts` (Test)

#### Interfaces
- **Consumes:** nothing (leaf package; no dependency on engine or any other workspace package).
- **Produces:** npm package `@app-cleaner/shared` at `packages/shared`, resolved via its `exports` field to `./src/index.ts` (source, not `dist/`) for any workspace consumer — Task 4 (`apps/web`) and Plan 2's CLI tooling (if ever ported to TS, N/A here) import from it as `@app-cleaner/shared`. Exported surface, exact signatures:
  - `PRODUCT: { name: "app-cleaner"; displayName: "App Cleaner"; repo: "https://github.com/GuilhermeVozniak/app-cleaner"; site: "https://app-cleaner.vozniak.dev" }`
  - `type AssetKind = "gui" | "cli"`
  - `releaseAssetName(kind: AssetKind, version: string): string` — `"gui"` → `` `app-cleaner_${version}_darwin_universal.dmg` ``; `"cli"` → `` `app-cleaner-cli_${version}_darwin_universal.tar.gz` ``
  - `downloadUrl(kind: AssetKind, version: string): string` — `` `${PRODUCT.repo}/releases/download/v${version}/${releaseAssetName(kind, version)}` ``
  - `latestReleaseUrl(): string` — `` `${PRODUCT.repo}/releases/latest` ``
  - `build` script emits type-checked JS+`.d.ts` to `packages/shared/dist/` (verification artifact only; nothing in this plan consumes `dist/` — all workspace imports resolve through `exports` → `src/index.ts`).
- Task 5 must add `"packages/shared"` build/lint/test to the root `turbo.json` pipeline and confirm `bun run lint` (biome) is clean here once the root `biome.json` exists — no source changes expected in this package for that to pass.

#### Steps

- [ ] **Step 1: Scaffold package config (no tests yet)**
  Create `packages/shared/package.json`:
  ```json
  {
    "name": "@app-cleaner/shared",
    "version": "0.0.0",
    "private": true,
    "type": "module",
    "main": "./src/index.ts",
    "types": "./src/index.ts",
    "exports": {
      ".": "./src/index.ts"
    },
    "scripts": {
      "lint": "biome check .",
      "test": "vitest run",
      "build": "tsc"
    },
    "devDependencies": {
      "typescript": "^5.6.0",
      "vitest": "^3.0.0"
    }
  }
  ```
  Create `packages/shared/tsconfig.json`:
  ```json
  {
    "compilerOptions": {
      "target": "ES2022",
      "module": "ESNext",
      "moduleResolution": "Bundler",
      "strict": true,
      "declaration": true,
      "skipLibCheck": true,
      "outDir": "dist",
      "rootDir": "src"
    },
    "include": ["src"],
    "exclude": ["src/**/*.test.ts"]
  }
  ```
  Create `packages/shared/vitest.config.ts`:
  ```ts
  import { defineConfig } from "vitest/config";

  export default defineConfig({
    test: { environment: "node", include: ["src/**/*.test.ts"] },
  });
  ```
  Run: `mkdir -p /Users/guilherme/Dev/pessoal/app-cleaner/packages/shared/src` (creates the `src/` dir the two files above will land in). No test run yet — these are config files, not behavior.

- [ ] **Step 2: Install deps standalone**
  Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner/packages/shared && bun install`
  Expected: resolves and installs `typescript` + `vitest` (and transitive deps) with no errors, writes `packages/shared/bun.lock`. No workspace-resolution error is possible here since this package has zero dependencies on other workspace packages.

- [ ] **Step 3: Write the failing contract test**
  Create `packages/shared/src/index.test.ts`:
  ```ts
  import { describe, expect, it } from "vitest";
  import { downloadUrl, latestReleaseUrl, PRODUCT, releaseAssetName } from "./index";

  describe("releaseAssetName", () => {
    it("builds the exact gui asset name for version 1.0.0", () => {
      expect(releaseAssetName("gui", "1.0.0")).toBe("app-cleaner_1.0.0_darwin_universal.dmg");
    });

    it("builds the exact cli asset name for version 1.0.0", () => {
      expect(releaseAssetName("cli", "1.0.0")).toBe(
        "app-cleaner-cli_1.0.0_darwin_universal.tar.gz",
      );
    });
  });

  describe("downloadUrl", () => {
    it("builds a tagged gui release asset URL", () => {
      expect(downloadUrl("gui", "1.0.0")).toBe(
        `${PRODUCT.repo}/releases/download/v1.0.0/app-cleaner_1.0.0_darwin_universal.dmg`,
      );
    });

    it("builds a tagged cli release asset URL", () => {
      expect(downloadUrl("cli", "1.0.0")).toBe(
        `${PRODUCT.repo}/releases/download/v1.0.0/app-cleaner-cli_1.0.0_darwin_universal.tar.gz`,
      );
    });
  });

  describe("latestReleaseUrl", () => {
    it("points at the latest release page", () => {
      expect(latestReleaseUrl()).toBe(`${PRODUCT.repo}/releases/latest`);
    });
  });
  ```
  Run: `bun run test`
  Expected: fails — `src/index.ts` does not exist yet, vitest reports a module-resolution error (`Cannot find module './index'` / transform error), 0 tests run.

- [ ] **Step 4: Implement the contract**
  Create `packages/shared/src/index.ts`:
  ```ts
  export const PRODUCT = {
    name: "app-cleaner",
    displayName: "App Cleaner",
    repo: "https://github.com/GuilhermeVozniak/app-cleaner",
    site: "https://app-cleaner.vozniak.dev",
  } as const;

  export type AssetKind = "gui" | "cli";

  const ASSET_NAME_BUILDERS: Record<AssetKind, (version: string) => string> = {
    gui: (version) => `app-cleaner_${version}_darwin_universal.dmg`,
    cli: (version) => `app-cleaner-cli_${version}_darwin_universal.tar.gz`,
  };

  export function releaseAssetName(kind: AssetKind, version: string): string {
    return ASSET_NAME_BUILDERS[kind](version);
  }

  export function downloadUrl(kind: AssetKind, version: string): string {
    return `${PRODUCT.repo}/releases/download/v${version}/${releaseAssetName(kind, version)}`;
  }

  export function latestReleaseUrl(): string {
    return `${PRODUCT.repo}/releases/latest`;
  }
  ```

- [ ] **Step 5: Run the test to green**
  Run: `bun run test`
  Expected: `Test Files 1 passed (1)`, `Tests 5 passed (5)`.

- [ ] **Step 6: Verify the typecheck+emit build**
  Run: `bun run build`
  Expected: exits 0; `packages/shared/dist/index.js` and `packages/shared/dist/index.d.ts` are created (only these two files — the `test.ts` file is excluded from the compile via `tsconfig.json`'s `exclude`).

- [ ] **Step 7: Commit**
  Run:
  ```
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add packages/shared
  git commit -m "$(cat <<'EOF'
  feat(shared): add @app-cleaner/shared product + release-asset contract

  PRODUCT metadata and releaseAssetName/downloadUrl/latestReleaseUrl give
  apps/web and the release workflow one source of truth for asset names.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```
  Expected: commit created; `git status` shows a clean tree (only `packages/shared/**` tracked, `node_modules/` and `bun.lock` ignored by the root `.gitignore` already present in this repo — if `bun.lock` is not currently gitignored, add it to the commit too, matching whatever the rest of the repo does for lockfiles at this point in the restructure).

---

### Task 4: `apps/web` — landing page

Reference read: `~/Dev/pessoal/option-tab/apps/web/{package.json,next.config.ts,app/layout.tsx,app/page.tsx,app/globals.css,postcss.config.mjs,tsconfig.json,vitest.config.ts,public/CNAME}` (structure only — page content below is original for App Cleaner). Category copy sourced from `/Users/guilherme/Dev/pessoal/app-cleaner/internal/core/categories.go` (will be `packages/engine/core/categories.go` after Task 2 — the marketing copy is a hand-transcribed literal copy of the 16 entries' `Name`, `Group`, `SafetyLevel`, `Description`, not a build-time import — the engine is Go and cannot be imported by the Next.js app). Contract: `docs/superpowers/plans/2026-07-08-monorepo-cli-contract.md` §"JS workspace & tooling versions" (apps/web row) and §"Product & release-asset contract".

Every command below runs with `cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/web`. Depends on Task 3 (`packages/shared`) existing at `../../packages/shared` — **not** on Task 5's root workspaces. This entire task (scaffold → install → failing test → implement → passing test → static build) was verified end-to-end in a scratch copy of this exact layout: `bun install` → 211 packages, `bun run test` → 4 passed, `bun run build` → static export to `out/` (`index.html` + `_next/`) with zero type errors.

**Standalone dependency resolution — read before Step 1.** `apps/web/package.json` depends on `@app-cleaner/shared` via `"file:../../packages/shared"`, **not** `"workspace:*"`. This was verified empirically: `bun install` in a directory with no ancestor `package.json` declaring `workspaces` fails hard on `workspace:*` (`error: Workspace dependency "@app-cleaner/shared" not found`), but `"file:../../packages/shared"` installs cleanly (bun symlinks the sibling directory into `node_modules/@app-cleaner/shared` and resolves its `exports` field normally). **Task 5, when it creates the root `package.json` with `workspaces: ["apps/*", "apps/desktop/frontend", "packages/*"]`, must change this one line in `apps/web/package.json` from `"file:../../packages/shared"` to `"workspace:*"`** (both `apps/web` and `packages/shared` fall under those workspace globs, so `workspace:*` will resolve correctly once the root config exists) and re-run `bun install` + `bun run test` + `bun run build` in `apps/web` to confirm nothing regressed. Task 5 Step 3 performs exactly this flip and re-verification.

Do **not** add `biome.json` and do not run `bun run lint` during this task, same reasoning as Task 3 — the script is declared per contract but unexercised until Task 5's root `biome.json` exists.

#### Files
- Create: `apps/web/package.json`
- Create: `apps/web/next.config.ts`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/postcss.config.mjs`
- Create: `apps/web/vitest.config.ts`
- Create: `apps/web/vitest.setup.ts`
- Create: `apps/web/public/CNAME`
- Create: `apps/web/lib/version.ts`
- Create: `apps/web/app/layout.tsx`
- Create: `apps/web/app/globals.css`
- Create: `apps/web/app/page.tsx`
- Create: `apps/web/app/page.test.tsx` (Test)

#### Interfaces
- **Consumes:** `@app-cleaner/shared` (Task 3) — `PRODUCT`, `downloadUrl`, `latestReleaseUrl`; `apps/web/lib/version.ts`'s own `APP_VERSION` constant (this package's single source of truth for which release tag it advertises — bump in lockstep with a tagged release, same pattern as option-tab's `lib/download.ts` `APP_VERSION`).
- **Produces:** static-exported site at `apps/web/out/` (from `next build`, `output: "export"`) — consumed by `deploy-web.yml` (later Plan 1 task) which uploads `apps/web/out` as the Pages artifact. `public/CNAME` (`app-cleaner.vozniak.dev`) ships into `out/` unchanged (Next copies `public/` verbatim) and is what GitHub Pages reads for the custom domain. No other package imports from `apps/web`.
- Depends on **Task 5** only for: (a) the `"file:../../packages/shared"` → `"workspace:*"` dependency-spec change noted above, (b) hoisting the root `biome.json`/`@biomejs/biome` so `bun run lint` becomes runnable, (c) wiring `apps/web` into the root `turbo.json` pipeline and `deploy-web.yml`'s path filter (already specified in the contract as `apps/web/**` + `packages/shared/**` — no action needed here, just confirming the filter will see this task's files).

#### Steps

- [ ] **Step 1: Scaffold package + build config (no tests yet)**
  Create `apps/web/package.json`:
  ```json
  {
    "name": "@app-cleaner/web",
    "private": true,
    "type": "module",
    "scripts": {
      "dev": "next dev",
      "build": "next build",
      "start": "next start",
      "lint": "biome check .",
      "test": "vitest run"
    },
    "dependencies": {
      "@app-cleaner/shared": "file:../../packages/shared",
      "next": "^15.0.0",
      "react": "^19.0.0",
      "react-dom": "^19.0.0"
    },
    "devDependencies": {
      "@tailwindcss/postcss": "^4.3.2",
      "@testing-library/jest-dom": "^6.6.0",
      "@testing-library/react": "^16.0.0",
      "@types/node": "^20.0.0",
      "@types/react": "^19.0.0",
      "@types/react-dom": "^19.0.0",
      "@vitejs/plugin-react": "^4.3.0",
      "jsdom": "^25.0.0",
      "postcss": "^8.5.16",
      "tailwindcss": "^4.3.2",
      "typescript": "^5.6.0",
      "vitest": "^3.0.0"
    }
  }
  ```
  Create `apps/web/next.config.ts`:
  ```ts
  import type { NextConfig } from "next";

  // Served from the root of the custom domain (app-cleaner.vozniak.dev), so
  // the base path stays empty. NEXT_PUBLIC_BASE_PATH can still override it if
  // the site is ever published under a project path instead.
  const nextConfig: NextConfig = {
    output: "export",
    images: { unoptimized: true },
    basePath: process.env.NEXT_PUBLIC_BASE_PATH ?? "",
  };

  export default nextConfig;
  ```
  Create `apps/web/tsconfig.json`:
  ```json
  {
    "compilerOptions": {
      "target": "ES2022",
      "lib": ["ES2022", "DOM", "DOM.Iterable"],
      "module": "ESNext",
      "moduleResolution": "Bundler",
      "jsx": "preserve",
      "strict": true,
      "skipLibCheck": true,
      "noEmit": true,
      "plugins": [{ "name": "next" }],
      "allowJs": true,
      "incremental": true,
      "esModuleInterop": true,
      "resolveJsonModule": true,
      "isolatedModules": true,
      "baseUrl": ".",
      "paths": { "@/*": ["./*"] }
    },
    "include": ["**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
    "exclude": ["node_modules"]
  }
  ```
  Create `apps/web/postcss.config.mjs`:
  ```js
  const config = {
    plugins: {
      "@tailwindcss/postcss": {},
    },
  };

  export default config;
  ```
  Create `apps/web/vitest.config.ts`:
  ```ts
  import react from "@vitejs/plugin-react";
  import { defineConfig } from "vitest/config";

  export default defineConfig({
    plugins: [react()],
    test: {
      environment: "jsdom",
      include: ["app/**/*.test.tsx"],
      setupFiles: ["./vitest.setup.ts"],
    },
  });
  ```
  Create `apps/web/vitest.setup.ts`:
  ```ts
  import { cleanup } from "@testing-library/react";
  import "@testing-library/jest-dom/vitest";
  import { afterEach } from "vitest";

  // RTL's automatic afterEach(cleanup) only self-registers when the test
  // runner exposes global `afterEach` (Vitest's `test.globals` is off here,
  // matching the rest of this repo's explicit-import style) — without this,
  // DOM trees from earlier tests in the same file accumulate and later
  // getByRole/getByTestId queries fail with "found multiple elements".
  afterEach(() => {
    cleanup();
  });
  ```
  Create `apps/web/public/CNAME`:
  ```
  app-cleaner.vozniak.dev
  ```
  (no trailing content beyond the single line; match option-tab's `public/CNAME` format exactly — one line, no trailing newline concerns, just the hostname.)

- [ ] **Step 2: Install deps standalone**
  Run: `cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/web && bun install`
  Expected: resolves `@app-cleaner/shared` via the `file:../../packages/shared` specifier (symlinked into `node_modules/@app-cleaner/shared`), installs `next`, `react`, `react-dom`, Tailwind 4 + PostCSS, Vitest + jsdom + Testing Library — no errors, `apps/web/bun.lock` written.

- [ ] **Step 3: Write the failing smoke test**
  Create `apps/web/app/page.test.tsx`:
  ```tsx
  import { downloadUrl, latestReleaseUrl } from "@app-cleaner/shared";
  import { render, screen } from "@testing-library/react";
  import { describe, expect, it } from "vitest";
  import { APP_VERSION } from "../lib/version";
  import Home from "./page";

  describe("Home", () => {
    it("renders the product name as the main heading", () => {
      render(<Home />);
      expect(screen.getByRole("heading", { level: 1, name: "App Cleaner" })).toBeInTheDocument();
    });

    it("links the primary download button to the shared gui asset URL", () => {
      render(<Home />);
      expect(screen.getByTestId("download-gui")).toHaveAttribute(
        "href",
        downloadUrl("gui", APP_VERSION),
      );
    });

    it("links 'All releases' to the shared latest-release URL", () => {
      render(<Home />);
      expect(screen.getByRole("link", { name: "All releases" })).toHaveAttribute(
        "href",
        latestReleaseUrl(),
      );
    });

    it("renders all 16 scan categories", () => {
      render(<Home />);
      expect(screen.getAllByRole("heading", { level: 3 }).length).toBeGreaterThanOrEqual(16);
    });
  });
  ```
  Run: `bun run test`
  Expected: fails — neither `./page` nor `../lib/version` exist yet; vitest reports a module-resolution error, 0 tests run.

- [ ] **Step 4: Implement `lib/version.ts` and `app/page.tsx`**
  Create `apps/web/lib/version.ts`:
  ```ts
  // Single source of truth for the version the landing page advertises.
  // Bump this in lockstep with a tagged release (release.yml builds v<version>).
  export const APP_VERSION = "1.0.0";
  ```
  Create `apps/web/app/page.tsx`:
  ```tsx
  import { downloadUrl, latestReleaseUrl, PRODUCT } from "@app-cleaner/shared";
  import { APP_VERSION } from "../lib/version";

  type SafetyLevel = "safe" | "moderate" | "risky";

  interface CategoryCopy {
    name: string;
    group: string;
    safety: SafetyLevel;
    description: string;
  }

  // Verbatim from packages/engine/core/categories.go — keep in sync (16 total).
  const CATEGORIES: CategoryCopy[] = [
    { name: "User Cache Files", group: "System Junk", safety: "moderate", description: "Application caches stored in ~/Library/Caches" },
    { name: "System Log Files", group: "System Junk", safety: "moderate", description: "System and application logs" },
    { name: "Temporary Files", group: "System Junk", safety: "safe", description: "Temporary files in /tmp and /var/folders" },
    { name: "Trash", group: "Storage", safety: "safe", description: "Files in the Trash bin" },
    { name: "Old Downloads", group: "Storage", safety: "risky", description: "Downloads older than 30 days" },
    { name: "Browser Cache", group: "Browsers", safety: "safe", description: "Cache from Chrome, Safari, Firefox, and Arc" },
    { name: "Development Cache", group: "Development", safety: "moderate", description: "npm, yarn, pip, Xcode DerivedData, CocoaPods cache" },
    { name: "Homebrew Cache", group: "Development", safety: "safe", description: "Homebrew download cache and old versions" },
    { name: "Docker", group: "Development", safety: "safe", description: "Unused Docker images, containers, and build cache" },
    { name: "iOS Backups", group: "Storage", safety: "risky", description: "iPhone and iPad backup files" },
    { name: "Mail Attachments", group: "Storage", safety: "risky", description: "Downloaded email attachments from Mail.app" },
    { name: "Language Files", group: "System Junk", safety: "risky", description: "Unused language localizations in applications" },
    { name: "Large Files", group: "Large Files", safety: "risky", description: "Files larger than 500MB for review" },
    { name: "Node Modules", group: "Development", safety: "moderate", description: "Orphaned node_modules in old projects" },
    { name: "Duplicate Files", group: "Storage", safety: "risky", description: "Files with identical content" },
    { name: "Orphaned Launch Agents", group: "System Junk", safety: "moderate", description: "Launch agents pointing to non-existent applications" },
  ];

  const SAFETY_SECTIONS: { level: SafetyLevel; title: string; blurb: string }[] = [
    { level: "safe", title: "Safe to clean", blurb: "Regenerated automatically — remove anytime with no downside." },
    { level: "moderate", title: "Review recommended", blurb: "Reclaimable, but a few apps may need to rebuild state." },
    { level: "risky", title: "Review carefully", blurb: "May contain files you actually want — inspect before deleting." },
  ];

  function CategoryCard({ category }: { category: CategoryCopy }) {
    return (
      <div className="rounded-2xl border border-card-border bg-card p-5">
        <div className="mb-1 flex items-center justify-between gap-2">
          <h3 className="m-0 text-base font-semibold tracking-tight">{category.name}</h3>
          <span className="shrink-0 rounded-full border border-card-border px-2 py-0.5 text-xs text-muted-foreground">
            {category.group}
          </span>
        </div>
        <p className="m-0 text-sm leading-relaxed text-muted-foreground">{category.description}</p>
      </div>
    );
  }

  function TerminalBlock() {
    return (
      <pre className="overflow-x-auto rounded-2xl border border-card-border bg-terminal-bg p-6 text-left text-[13px] leading-relaxed text-terminal-fg">
        <code>{`# Download the CLI tarball for macOS (universal binary)
  curl -L ${downloadUrl("cli", APP_VERSION)} | tar xz
  ./app-cleaner --help

  # ...or build it yourself from source
  git clone ${PRODUCT.repo}.git
  cd app-cleaner
  task build:cli`}</code>
      </pre>
    );
  }

  export default function Home() {
    return (
      <main className="mx-auto max-w-[1080px] px-6">
        <section className="pb-16 pt-24 text-center">
          <span className="inline-block rounded-full border border-card-border bg-card px-4 py-1.5 text-sm text-muted-foreground">
            Native macOS app · open source · no account, no paywall
          </span>
          <h1 className="mx-auto mb-4 mt-7 text-[clamp(48px,9vw,84px)] font-bold leading-[1.02] tracking-tight">
            {PRODUCT.displayName}
          </h1>
          <p className="mx-auto mb-9 max-w-[640px] text-xl text-muted-foreground">
            Reclaim disk space on your Mac. Scan 16 categories of junk, review every file before it
            goes, and undo any cleanup from a full backup — all from a native app or the terminal.
          </p>
          <div className="mb-5 flex flex-wrap items-center justify-center gap-3">
            <a
              data-testid="download-gui"
              className="inline-flex h-12 cursor-pointer items-center justify-center gap-2 rounded-xl bg-primary px-7 text-[17px] font-semibold text-primary-foreground no-underline transition-opacity hover:opacity-90"
              href={downloadUrl("gui", APP_VERSION)}
            >
              Download for macOS (.dmg)
            </a>
            <a
              className="inline-flex h-12 cursor-pointer items-center justify-center gap-2 rounded-xl border border-card-border px-7 text-[17px] font-semibold no-underline transition-colors hover:bg-card"
              href={latestReleaseUrl()}
            >
              All releases
            </a>
          </div>
          <p className="m-0 text-sm text-muted-foreground">macOS 13+ (Apple Silicon &amp; Intel) · signed &amp; notarized</p>
        </section>

        <section className="border-t border-card-border py-14" id="scan-categories">
          <h2 className="m-0 mb-2 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
            16 scan categories, ranked by safety
          </h2>
          <p className="mx-auto mb-10 max-w-[640px] text-center text-muted-foreground">
            Every category shows what it finds and how risky it is to remove before you touch
            anything.
          </p>
          {SAFETY_SECTIONS.map((section) => (
            <div className="mb-10" key={section.level}>
              <h3 className="m-0 mb-1 text-lg font-semibold">{section.title}</h3>
              <p className="m-0 mb-4 text-sm text-muted-foreground">{section.blurb}</p>
              <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-4">
                {CATEGORIES.filter((c) => c.safety === section.level).map((c) => (
                  <CategoryCard category={c} key={c.name} />
                ))}
              </div>
            </div>
          ))}
        </section>

        <section className="border-t border-card-border py-14">
          <h2 className="m-0 mb-10 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
            More than a junk scanner
          </h2>
          <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-5">
            <div className="rounded-2xl border border-card-border bg-card p-6">
              <h3 className="m-0 mb-2 text-lg font-semibold">Undo with backups</h3>
              <p className="m-0 text-sm text-muted-foreground">
                Every cleanup can back up what it removes first. Restore any item from the Backups
                view — nothing is gone for good by accident.
              </p>
            </div>
            <div className="rounded-2xl border border-card-border bg-card p-6">
              <h3 className="m-0 mb-2 text-lg font-semibold">App uninstaller</h3>
              <p className="m-0 text-sm text-muted-foreground">
                Remove an app and its leftover caches, preferences, and support files together,
                with a running-app guard so you never uninstall from under yourself.
              </p>
            </div>
            <div className="rounded-2xl border border-card-border bg-card p-6">
              <h3 className="m-0 mb-2 text-lg font-semibold">Maintenance tasks</h3>
              <p className="m-0 text-sm text-muted-foreground">
                Flush DNS cache, free purgeable disk space, and clear local Time Machine
                snapshots in one click.
              </p>
            </div>
            <div className="rounded-2xl border border-card-border bg-card p-6">
              <h3 className="m-0 mb-2 text-lg font-semibold">Native &amp; open source</h3>
              <p className="m-0 text-sm text-muted-foreground">
                Built with Go &amp; Wails — no Electron, no telemetry. Read every line on GitHub.
              </p>
            </div>
          </div>
        </section>

        <section className="border-t border-card-border py-14" id="cli">
          <h2 className="m-0 mb-2 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
            Prefer the terminal?
          </h2>
          <p className="mx-auto mb-8 max-w-[640px] text-center text-muted-foreground">
            App Cleaner ships a full-parity terminal CLI with the same interactive picker,
            backups, and maintenance tasks as the GUI — built on the same engine.
          </p>
          <TerminalBlock />
        </section>

        <footer className="border-t border-card-border py-12 pb-16 text-center text-muted-foreground">
          <p className="m-0 mb-2">
            <a className="text-primary no-underline hover:underline" href={PRODUCT.repo}>
              Source on GitHub
            </a>{" "}
            · MIT license
          </p>
          <p className="m-0 text-sm opacity-80">
            A native macOS port of{" "}
            <a className="text-primary no-underline hover:underline" href="https://github.com/gabrielmaialva33/mac-cleaner-cli">
              mac-cleaner-cli
            </a>
            .
          </p>
        </footer>
      </main>
    );
  }
  ```
  (Note: the `TerminalBlock` template literal is written with a 2-space indent inside this plan's code fence for readability; when creating the actual file, dedent its contents to column 0 as ordinary top-level TS/TSX — the surrounding markdown indentation here is a plan-formatting artifact, not part of the file. The literal text of the terminal snippet — the `curl -L ... | tar xz`, `./app-cleaner --help`, `git clone ...`, `cd app-cleaner`, `task build:cli` lines — must be reproduced exactly.)

- [ ] **Step 5: Run the test to green**
  Run: `bun run test`
  Expected: `Test Files 1 passed (1)`, `Tests 4 passed (4)`.

- [ ] **Step 6: Add supporting files needed for a full build (layout + global styles)**
  Create `apps/web/app/layout.tsx`:
  ```tsx
  import { PRODUCT } from "@app-cleaner/shared";
  import type { Metadata } from "next";
  import type { ReactNode } from "react";
  import "./globals.css";

  const title = "App Cleaner — reclaim disk space on your Mac";
  const description =
    "A native, open-source Mac cleaner: 16 scan categories, undoable backups, an app uninstaller, and maintenance tools. Also available as a full-parity terminal CLI.";

  export const metadata: Metadata = {
    metadataBase: new URL(PRODUCT.site),
    title,
    description,
    alternates: { canonical: "/" },
    openGraph: {
      type: "website",
      url: PRODUCT.site,
      siteName: PRODUCT.displayName,
      title,
      description,
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
    },
  };

  export default function RootLayout({ children }: { children: ReactNode }) {
    return (
      <html lang="en">
        <body>{children}</body>
      </html>
    );
  }
  ```
  Create `apps/web/app/globals.css`:
  ```css
  @import "tailwindcss";

  :root {
    --background: #f7f8fb;
    --foreground: #0b0f19;
    --muted-foreground: rgba(11, 15, 25, 0.62);
    --primary: #2563eb;
    --primary-foreground: #ffffff;
    --accent: #059669;
    --card-bg: rgba(11, 15, 25, 0.035);
    --card-border: rgba(11, 15, 25, 0.1);
    --terminal-bg: #0b0f19;
    --terminal-fg: #d7ecff;
  }

  @media (prefers-color-scheme: dark) {
    :root {
      --background: #0a0e14;
      --foreground: #f2f5fa;
      --muted-foreground: rgba(226, 233, 255, 0.62);
      --primary: #60a5fa;
      --primary-foreground: #0a0e14;
      --accent: #34d399;
      --card-bg: rgba(255, 255, 255, 0.045);
      --card-border: rgba(255, 255, 255, 0.1);
      --terminal-bg: #05070c;
      --terminal-fg: #b8e6c8;
    }
  }

  @theme inline {
    --color-background: var(--background);
    --color-foreground: var(--foreground);
    --color-muted-foreground: var(--muted-foreground);
    --color-primary: var(--primary);
    --color-primary-foreground: var(--primary-foreground);
    --color-accent: var(--accent);
    --color-card: var(--card-bg);
    --color-card-border: var(--card-border);
    --color-terminal-bg: var(--terminal-bg);
    --color-terminal-fg: var(--terminal-fg);
  }

  html,
  body {
    margin: 0;
    padding: 0;
  }

  body {
    min-height: 100vh;
    background: var(--background);
    color: var(--foreground);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    line-height: 1.6;
  }
  ```
  These two files are not separately unit-tested (no visual/DOM assertions beyond Step 3's smoke test, which already exercises `page.tsx`'s markup); their correctness is verified by the static build in Step 7.

- [ ] **Step 7: Verify the static export build**
  Run: `bun run build`
  Expected: exits 0. Output includes `✓ Compiled successfully`, `✓ Generating static pages (4/4)` (`/`, `/_not-found`, plus their data files), `✓ Exporting (2/2)`, and a route table showing `┌ ○ /` prerendered as static content. `apps/web/out/index.html`, `apps/web/out/_next/`, and `apps/web/out/404.html` exist on disk afterward. No TypeScript errors are reported during the "Linting and checking validity of types" phase.

- [ ] **Step 8: Commit**
  Run:
  ```
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add apps/web
  git commit -m "$(cat <<'EOF'
  feat(web): add App Cleaner landing page (Next.js 15 static export)

  Hero, 16-category safety-ranked feature grid, backup/uninstall/maintenance
  cards, and a CLI download section — all download links derive from
  @app-cleaner/shared so the site and release workflow can never drift.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```
  Expected: commit created; `git status` clean except for whatever lockfile policy the rest of the in-progress restructure is using (match it — if `apps/desktop`'s `bun.lock`-equivalent, if any, is tracked, track `apps/web/bun.lock` too; if gitignored, leave it out).

---

### Task 5: Root workspace tooling

Starting state (after Tasks 1–4): `go.work` exists (`use ./packages/engine ./apps/desktop`); `packages/engine` is a standalone Go module; `apps/desktop` is a Go module at repo root path `apps/desktop` with its own `go.mod` + `frontend/` (the former root `frontend/`, moved via `git mv`, still has the original `package.json` with `test: "vitest run"`); `apps/web` (`@app-cleaner/web`) and `packages/shared` (`@app-cleaner/shared`) each already have their own `package.json` with `build`/`lint`/`test` scripts, per the contract. No root `package.json` exists yet — this task creates the workspace root that ties all four JS/TS packages and three Go modules together, plus the Taskfile that is the single cross-language entrypoint. `apps/cli` does not exist yet (Plan 2); every Go-iterating step here must skip it via an existence guard, never hard-fail on its absence. This task also performs Task 4's documented handoff: flipping `apps/web`'s `@app-cleaner/shared` dependency from `file:../../packages/shared` to `workspace:*` once the root workspaces exist (Step 3).

#### Files

- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/package.json`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/turbo.json`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/biome.json`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/lefthook.yml`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/Taskfile.yml`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/README.md`
- Create (generated by `bun install`, then committed): `/Users/guilherme/Dev/pessoal/app-cleaner/bun.lock`
- Modify: `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/frontend/package.json` (add `lint` script)
- Modify: `/Users/guilherme/Dev/pessoal/app-cleaner/apps/web/package.json` (flip `@app-cleaner/shared` dep from `file:../../packages/shared` to `workspace:*` — Task 4's documented handoff)
- Delete (if present/tracked): `/Users/guilherme/Dev/pessoal/app-cleaner/apps/web/bun.lock`, `/Users/guilherme/Dev/pessoal/app-cleaner/packages/shared/bun.lock` (standalone lockfiles superseded by the root `bun.lock`)
- Modify: `/Users/guilherme/Dev/pessoal/app-cleaner/README.md` (full rewrite for the monorepo)
- Modify: `/Users/guilherme/Dev/pessoal/app-cleaner/.gitignore` (add turbo/build-output entries)
- Test: none new — this task is tooling-only and is verified by running the *existing* suites through the new entrypoints (packages/engine + apps/desktop Go tests — 244 at time of writing; apps/desktop/frontend + apps/web + packages/shared JS/TS tests — 80+ at time of writing). No test file is created or modified.

#### Interfaces

**Consumes**
- `apps/desktop` — Go module `github.com/GuilhermeVozniak/app-cleaner/apps/desktop`, with `frontend/` subdir carrying its own `package.json` (Task 1).
- `packages/engine` — Go module `github.com/GuilhermeVozniak/app-cleaner/packages/engine` (Task 2).
- `packages/shared` — npm package `@app-cleaner/shared` with `build`/`lint`/`test` scripts (Task 3).
- `apps/web` — npm package `@app-cleaner/web` with `dev`/`build`/`lint`/`test` scripts (Task 4).
- Exact names/values from `docs/superpowers/plans/2026-07-08-monorepo-cli-contract.md` (workspace globs, dev-dep versions, Taskfile task names).

**Produces** (relied on by later Plan 1 tasks — CI/release workflow authoring — and by Plan 2's CLI task)
- Root `package.json` scripts: `bun run build|lint|test|dev` (each → `turbo run <name>`), `bun run prepare` (→ `lefthook install`). CI's `js` job runs `bun install --frozen-lockfile` then these three scripts verbatim.
- `turbo.json` pipeline task names: `build`, `lint`, `test`, `e2e`, `dev`.
- `biome.json`: js/ts lint+format config at repo root, with `files.includes` excluding `apps/desktop/frontend/**` and `apps/desktop/frontend/wailsjs/**`.
- `lefthook.yml`: hooks `pre-commit` (biome/gofumpt/golangci, parallel), `pre-push` (`task test`), `commit-msg` (Conventional Commits).
- `.golangci.yml` at repo root, shared by every Go module (each module's `golangci-lint run ./...` is invoked from inside that module's directory, so config auto-discovery finds this file by walking up to the git root; no `module-path` override is set, so gofumpt auto-detects the local module per invocation).
- `Taskfile.yml` task names, **exact, relied on by name**: `lint`, `test`, `build`, `dev:web`, `dev:desktop`, `dev:cli`, `build:cli` (produces `apps/cli/bin/app-cleaner`, version via `-ldflags "-X main.version={{.VERSION}}"`), `release:prep`. Every Go-iterating task (`lint`, `test`, `build`) walks `packages/engine`, `apps/desktop`, `apps/cli` and skips `apps/cli` if the directory does not exist.
- `apps/desktop/frontend/package.json` gains a `lint` script (`tsc --noEmit`); its `test` script (`vitest run`) is unchanged (already present).
- Root `README.md` (monorepo map + quickstart + release-flow overview) and `apps/desktop/README.md` (short, points back to root).

---

- [ ] **Step 1: Create the root `package.json` (contract values verbatim)**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/package.json`:

  ```json
  {
    "name": "app-cleaner",
    "private": true,
    "type": "module",
    "packageManager": "bun@1.1.38",
    "engines": {
      "bun": ">=1.1",
      "node": ">=20"
    },
    "workspaces": [
      "apps/*",
      "apps/desktop/frontend",
      "packages/*"
    ],
    "scripts": {
      "build": "turbo run build",
      "lint": "turbo run lint",
      "test": "turbo run test",
      "dev": "turbo run dev",
      "prepare": "lefthook install"
    },
    "devDependencies": {
      "@biomejs/biome": "^2.0.0",
      "lefthook": "^1.7.0",
      "turbo": "^2.0.0",
      "typescript": "^5.6.0"
    }
  }
  ```

  Note: the `apps/*` glob also matches `apps/desktop`, which has no `package.json` of its own (it is a pure Go module directory) — Bun silently skips workspace entries without a `package.json`, so this is not an error (same layout as option-tab's `apps/desktop`).

- [ ] **Step 2: Add turbo/build-output entries to `.gitignore`**

  After Task 1, the root `.gitignore`'s `# Node` block reads `node_modules/` + `apps/desktop/frontend/dist/`; it predates turbo/bun. Add the workspace-wide cache/output dirs turbo and the new packages generate.

  Modify `/Users/guilherme/Dev/pessoal/app-cleaner/.gitignore` so the `# Node` block becomes:

  ```
  # Node
  node_modules/
  apps/desktop/frontend/dist/
  .turbo/
  out/
  .next/
  coverage/
  ```

  (Leave every other existing line untouched — `mac-cleaner-cli/`, `apps/desktop/build/bin/`, `*.log`, `.DS_Store`, `.superpowers/` stay exactly as they are.)

- [ ] **Step 3: Flip `apps/web`'s shared dep to `workspace:*`, drop stale standalone lockfiles, and `bun install` at root**

  Task 4 declared `"@app-cleaner/shared": "file:../../packages/shared"` only because no workspace root existed yet, and explicitly flagged this flip for this task. Now that Step 1's root `package.json` declares the workspaces:

  1. Edit `/Users/guilherme/Dev/pessoal/app-cleaner/apps/web/package.json` — change one line in `dependencies`:

  ```
  "@app-cleaner/shared": "file:../../packages/shared"   →   "@app-cleaner/shared": "workspace:*"
  ```

  2. Remove the standalone lockfiles Tasks 3–4 generated (the root `bun.lock` created below becomes the single lockfile for every workspace member):

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git rm --cached --ignore-unmatch apps/web/bun.lock packages/shared/bun.lock
  rm -f apps/web/bun.lock packages/shared/bun.lock
  ```

  3. Install from the root:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && bun install
  ```

  Expected outcome: exit code 0; Bun resolves and links all three workspace packages (`apps/web`, `apps/desktop/frontend`, `packages/shared`) plus the four root dev deps, with `@app-cleaner/shared` now resolved via `workspace:*`; a `bun.lock` file is generated at the repo root; because `prepare` runs automatically on install, the output includes a lefthook line similar to:

  ```
  🥊 lefthook has been installed and set up (no hooks defined yet — lefthook.yml not present)
  ```

  (Lefthook installs its git hook shims even before `lefthook.yml` exists; hooks become active once Step 8 adds the config.) If `bun install` reports any workspace as unresolved, stop — it means `apps/web` or `packages/shared` is missing its own `package.json` from an earlier task; fix that dependency before continuing.

  4. Re-run the `apps/web` suite and static build against the workspace-resolved dependency (Task 4's explicit handoff requirement):

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/web && bun run test && bun run build
  ```

  Expected outcome: vitest reports `Tests 4 passed (4)`; `next build` completes the static export to `apps/web/out/` exactly as it did in Task 4 — proving the flip changed dependency-resolution mechanics only, not behavior.

- [ ] **Step 4: Create `turbo.json` (copied verbatim from option-tab)**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/turbo.json`:

  ```json
  {
    "$schema": "https://turbo.build/schema.json",
    "tasks": {
      "build": { "dependsOn": ["^build"], "outputs": ["dist/**", ".next/**", "out/**"] },
      "lint": { "dependsOn": ["^build"] },
      "test": { "dependsOn": ["^build"] },
      "e2e": { "dependsOn": ["build"] },
      "dev": { "cache": false, "persistent": true }
    }
  }
  ```

- [ ] **Step 5: Create `biome.json` (adapted — desktop frontend excluded)**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/biome.json`:

  ```json
  {
    "$schema": "https://biomejs.dev/schemas/2.5.0/schema.json",
    "vcs": { "enabled": true, "clientKind": "git", "useIgnoreFile": true },
    "files": {
      "ignoreUnknown": true,
      "includes": ["**", "!apps/desktop/frontend/**", "!apps/desktop/frontend/wailsjs/**"]
    },
    "formatter": { "enabled": true, "indentStyle": "space", "indentWidth": 2, "lineWidth": 100 },
    "linter": { "enabled": true, "rules": { "preset": "none" } },
    "javascript": { "formatter": { "quoteStyle": "double" } },
    "css": { "parser": { "tailwindDirectives": true } }
  }
  ```

  This drops option-tab's `overlay/**` a11y-rule override (that path doesn't exist in this repo, and `apps/desktop/frontend/**` is now excluded from Biome entirely anyway — it keeps its own `tsc --noEmit` + vitest gates instead, per the contract).

- [ ] **Step 6: Sanity-check Biome config**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && bunx biome check .
  ```

  Expected outcome: exit code 0, no config-parse errors, no lint/format diagnostics against `apps/web` and `packages/shared` sources (both already lint-clean from their own tasks); output ends with a line of the shape `Checked N files in <time>. No fixes applied.` `apps/desktop/frontend/**` is not scanned (excluded by `files.includes`).

- [ ] **Step 7: Commit — workspace root, turbo, biome**

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add package.json bun.lock turbo.json biome.json .gitignore apps/web/package.json
  git commit -m "$(cat <<'EOF'
  chore(repo): add root workspace package.json, turbo pipeline, biome config

  Wires apps/web, apps/desktop/frontend, and packages/* into one Bun
  workspace with a shared turbo build/lint/test pipeline; biome covers
  every JS/TS package except apps/desktop/frontend, which keeps its own
  tsc/vitest gates. apps/web's @app-cleaner/shared dep flips from
  file:../../packages/shared to workspace:* now that the workspace root
  exists, and the standalone per-package lockfiles are dropped in favor
  of the root bun.lock (removed via git rm in this same change).

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```

  Expected outcome: commit created; `git status` clean except for files touched in later steps.

- [ ] **Step 8: Create `lefthook.yml` (adapted — golangci loop over existing Go modules)**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/lefthook.yml`:

  ```yaml
  pre-commit:
    parallel: true
    commands:
      biome:
        glob: "*.{js,jsx,ts,tsx,json,jsonc}"
        run: bunx biome check --write --no-errors-on-unmatched {staged_files}
        stage_fixed: true
      gofumpt:
        glob: "*.go"
        run: gofumpt -w {staged_files}
        stage_fixed: true
      golangci:
        glob: "*.go"
        run: |
          for m in packages/engine apps/desktop apps/cli; do
            if [ -d "$m" ]; then
              (cd "$m" && golangci-lint run ./...) || exit 1
            fi
          done

  pre-push:
    commands:
      test:
        run: task test

  commit-msg:
    commands:
      conventional:
        run: |
          head -n1 {1} | grep -qE '^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?!?: .+' \
            || { echo 'Commit message must follow Conventional Commits (e.g. "feat: add X")'; exit 1; }
  ```

  The `golangci` hook iterates all three module directories but only runs in ones that exist, so it is a no-op for `apps/cli` until Plan 2 lands, and it does not swallow a real failure once a module does exist (the `|| exit 1` inside the loop propagates).

- [ ] **Step 9: Reinstall hooks with the new config**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && bun run prepare
  ```

  Expected outcome: exit code 0; output lists the three hook names now wired, e.g.:

  ```
  🥊 lefthook has been installed and set up.
  ┌
  │ pre-commit
  │ pre-push
  │ commit-msg
  └
  ```

- [ ] **Step 10: Create `.golangci.yml` (adapted — no hardcoded `module-path`)**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml`:

  ```yaml
  version: "2"
  run:
    timeout: 5m
  linters:
    default: standard
    enable:
      - errcheck
      - ineffassign
      - misspell
      - staticcheck
      - unused
  formatters:
    enable:
      - gofmt
      - gofumpt
    settings:
      gofumpt:
        extra-rules: true
  ```

  option-tab's version hardcodes `gofumpt.module-path: option-tab` because that repo has a single Go module whose `go.mod` literally declares `module option-tab`. This repo has three modules with three different long import paths (`.../packages/engine`, `.../apps/desktop`, `.../apps/cli`), and `golangci-lint` is always invoked from inside the target module's own directory (see Taskfile, Step 12), so gofumpt auto-detects the correct local-module prefix from that directory's own `go.mod` — no override needed, and a single shared root config stays correct for all three modules.

- [ ] **Step 11: Sanity-check golangci config against existing modules**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner/packages/engine && golangci-lint run ./...
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop && golangci-lint run ./...
  ```

  Expected outcome: both exit code 0 with output `0 issues.` (the existing code from Tasks 1–4 is already gofmt/govet clean per the contract's global constraints).

- [ ] **Step 12: Commit — lefthook + golangci config**

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add lefthook.yml .golangci.yml
  git commit -m "$(cat <<'EOF'
  chore(repo): add lefthook git hooks and shared golangci-lint config

  pre-commit runs biome/gofumpt/golangci-lint (looping over every Go
  module that currently exists); pre-push runs the full test task;
  commit-msg enforces Conventional Commits.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```

- [ ] **Step 13: Create `Taskfile.yml` — every contract task name, existence-guarded Go loops**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/Taskfile.yml`:

  ```yaml
  version: "3"

  tasks:
    default:
      cmds: [task --list]

    lint:
      desc: Lint all code (Biome across JS/TS workspaces + golangci-lint per Go module)
      cmds:
        - bun run lint
        - cd packages/engine && golangci-lint run ./...
        - cd apps/desktop && golangci-lint run ./...
        - |
          if [ -d apps/cli ]; then
            (cd apps/cli && golangci-lint run ./...)
          fi

    test:
      desc: Run all unit/integration tests (Vitest across JS/TS workspaces + go test per Go module)
      cmds:
        - bun run test
        - cd packages/engine && go test ./... -race -cover
        - cd apps/desktop && go test ./... -race -cover
        - |
          if [ -d apps/cli ]; then
            (cd apps/cli && go test ./... -race -cover)
          fi

    build:
      desc: Build web + desktop (+ cli once apps/cli exists)
      cmds:
        - bun run build
        - cd apps/desktop && wails build
        - |
          if [ -d apps/cli ]; then
            task build:cli
          fi

    build:cli:
      desc: Build the CLI binary (apps/cli/bin/app-cleaner)
      vars:
        VERSION:
          sh: git describe --tags --always --dirty 2>/dev/null || echo dev
      cmds:
        - mkdir -p apps/cli/bin
        - cd apps/cli && go build -ldflags "-X main.version={{.VERSION}}" -o bin/app-cleaner .

    dev:web:
      desc: Run the landing page dev server
      cmds: [cd apps/web && bun run dev]

    dev:desktop:
      desc: Run the desktop app in Wails dev mode
      cmds: [cd apps/desktop && wails dev]

    dev:cli:
      desc: Run the CLI in interactive mode from source (requires apps/cli)
      cmds: [cd apps/cli && go run .]

    release:prep:
      desc: Assert a clean working tree and green tests, then print tag instructions
      cmds:
        - |
          if [ -n "$(git status --porcelain)" ]; then
            echo "Working tree is not clean — commit or stash first." >&2
            exit 1
          fi
        - task test
        - 'echo "Tree is clean and tests are green. Tag with: git tag vX.Y.Z && git push origin vX.Y.Z"'
  ```

  Every Go-iterating task (`lint`, `test`, `build`) guards `apps/cli` with `if [ -d apps/cli ]; then …; fi` so the failing branch of the `if` (not the guard) is what propagates a real exit code once Plan 2 adds the module — an unguarded `&& … || true` would have silently swallowed a genuine lint/test failure in `apps/cli` after it exists, which is why the guard uses `if`/`fi` instead.

- [ ] **Step 14: Verify `task --list` shows every contract task name**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task --list
  ```

  Expected outcome: exit code 0; output lists exactly these task names (order may vary, names must match): `default`, `lint`, `test`, `build`, `build:cli`, `dev:web`, `dev:desktop`, `dev:cli`, `release:prep`.

- [ ] **Step 15: `task lint` must pass**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task lint
  ```

  Expected outcome: exit code 0. `bun run lint` (via turbo) reports every JS/TS workspace's `lint` script passing (`apps/web`: `biome check .`; `packages/shared`: `biome check .`; `apps/desktop/frontend`: not yet wired — see Step 18 — until then turbo's `lint` task simply has no `lint` script to run there and skips it); `golangci-lint run ./...` reports `0 issues.` for `packages/engine` and `apps/desktop`; the `apps/cli` guard is skipped silently (directory absent).

- [ ] **Step 16: `task test` must pass**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task test
  ```

  Expected outcome: exit code 0. `bun run test` (via turbo) reports `apps/desktop/frontend`'s vitest suite green (the pre-existing 80 frontend tests), plus `apps/web` and `packages/shared`'s vitest suites green; `go test ./... -race -cover` reports `ok` for every package under `packages/engine` and `apps/desktop` (the pre-existing 244 Go tests, now split across the two modules); the `apps/cli` guard is skipped silently.

- [ ] **Step 17: Commit — Taskfile**

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add Taskfile.yml
  git commit -m "$(cat <<'EOF'
  chore(repo): add Taskfile as the cross-language build entrypoint

  task lint/test/build fan out across every JS/TS workspace (via bun run)
  and every existing Go module (packages/engine, apps/desktop); dev:web,
  dev:desktop, dev:cli, build:cli, and release:prep round out the
  contract's task surface. apps/cli steps are existence-guarded until
  Plan 2 lands.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```

- [ ] **Step 18: Add the `lint` script to `apps/desktop/frontend/package.json`**

  Modify `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/frontend/package.json`:

  ```json
  {
    "name": "frontend",
    "private": true,
    "version": "0.0.0",
    "type": "module",
    "scripts": {
      "dev": "vite",
      "build": "tsc && vite build",
      "preview": "vite preview",
      "typecheck": "tsc --noEmit",
      "lint": "tsc --noEmit",
      "test": "vitest run"
    },
    "dependencies": {
      "lucide-react": "^1.23.0",
      "react": "^18.3.1",
      "react-dom": "^18.3.1",
      "zustand": "^5.0.14"
    },
    "devDependencies": {
      "@tailwindcss/vite": "^4.3.2",
      "@testing-library/dom": "^10.4.1",
      "@testing-library/jest-dom": "^6.9.1",
      "@testing-library/react": "^16.3.2",
      "@types/react": "^18.3.31",
      "@types/react-dom": "^18.3.7",
      "@vitejs/plugin-react": "^4.7.0",
      "jsdom": "^29.1.1",
      "tailwindcss": "^4.3.2",
      "typescript": "^5.9.3",
      "vite": "^6.4.3",
      "vitest": "^3.2.7"
    }
  }
  ```

  Only the new `"lint": "tsc --noEmit"` line is added (kept alongside the pre-existing `"typecheck"` script, which other tooling/docs may still reference); `"test"` is unchanged — it was already `"vitest run"`. Do not add Biome here; the contract deliberately excludes this workspace from Biome (Step 5).

- [ ] **Step 19: Re-run `task lint`/`task test` — desktop frontend now participates via its own scripts**

  Run:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task lint && task test
  ```

  Expected outcome: exit code 0 for both; turbo's `lint` run now additionally reports `apps/desktop/frontend`'s `lint` script (`tsc --noEmit`) passing with no output (clean); `test` output is unchanged from Step 16 (the `test` script already existed).

- [ ] **Step 20: Commit — desktop frontend workspace scripts**

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add apps/desktop/frontend/package.json
  git commit -m "$(cat <<'EOF'
  chore(desktop): wire frontend into the turbo lint pipeline

  Adds a lint script (tsc --noEmit) so turbo run lint covers the desktop
  frontend the same way it covers every other JS/TS workspace, without
  adding Biome to a package that intentionally keeps its own gates.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```

- [ ] **Step 21: Rewrite the root `README.md`**

  Overwrite `/Users/guilherme/Dev/pessoal/app-cleaner/README.md`:

  ```markdown
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
  ```

- [ ] **Step 22: Create `apps/desktop/README.md`**

  Create `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/README.md`:

  ```markdown
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
  ```

- [ ] **Step 23: Full green-run verification**

  Run, in order:

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && bun install
  cd /Users/guilherme/Dev/pessoal/app-cleaner && turbo run lint test build
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task lint
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task test
  ```

  Expected outcome for all four: exit code 0.
  - `bun install`: no changes to `bun.lock` (already up to date from Step 3), lefthook hooks already installed.
  - `turbo run lint test build`: turbo prints a per-package summary table ending with a line of the shape `Tasks: N successful, N total` for each of `lint`, `test`, and `build` (three such summary blocks, one per task), zero failures — covers `apps/web`, `apps/desktop/frontend`, `packages/shared`.
  - `task lint`: `0 issues.` from golangci-lint for `packages/engine` and `apps/desktop`; Biome/tsc clean for every JS/TS workspace.
  - `task test`: `ok` for every Go package under `packages/engine` and `apps/desktop`; vitest green for every JS/TS workspace with a `test` script.

- [ ] **Step 24: Commit — README rewrite**

  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add README.md apps/desktop/README.md
  git commit -m "$(cat <<'EOF'
  docs: rewrite root README for the monorepo, add apps/desktop README

  Root README now documents the monorepo map, app summaries, task
  commands, and release flow; apps/desktop gets a short app-specific
  README (wails dev/build/test) that points back to the root doc.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```

  Expected outcome: commit created; `git status` clean; `git log --oneline -6` shows the five commits from this task (Steps 7, 12, 17, 20, 24) in order.

---

## Plan 1 — Tasks 6–7: CI workflow, deploy-web, release pipeline, appdmg config

Starting state: Tasks 1–5 done. The repo is already restructured per the contract:
`packages/engine` (module `github.com/GuilhermeVozniak/app-cleaner/packages/engine`), `apps/desktop`
(module `github.com/GuilhermeVozniak/app-cleaner/apps/desktop`, the former repo root — `wails.json`
`outputfilename: "App Cleaner"`), `apps/web`, `packages/shared`, root `package.json`/`turbo.json`/
`biome.json`/`Taskfile.yml`/`go.work` all exist. `apps/cli` does **not** exist yet (that's Plan 2) —
every step below that touches `apps/cli` must degrade gracefully when the directory is absent.

---

### Task 6: `.github/workflows/ci.yml`

#### Files
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/.github/workflows/ci.yml`
- Verify only (owned by Task 5 Step 10, already committed there — do NOT create or overwrite here): `/Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml`
- Test: none (infrastructure config — verified via `actionlint`, `gofmt`, `go vet`, and a local
  golangci-lint run against the real `packages/engine`/`apps/desktop` modules, not a unit test)

#### Interfaces
**Consumes**
- Root `package.json` scripts `lint`, `test`, `build` (turbo-driven, from Tasks 1–5) — invoked by the `js` job.
- Go modules `packages/engine` and `apps/desktop` (from Tasks 1–5) — invoked by the `go` job's
  per-module `go test -race ./...`, `gofmt -l`, `go vet ./...` steps.
- `apps/desktop/frontend` — `bun run build` produces `frontend/dist`, which `apps/desktop/main.go`
  `//go:embed`s; must run before any `go test`/`go vet`/`go build` touching `apps/desktop`.
- `.golangci.yml` at repo root (created and committed by Task 5 Step 10) — picked up via golangci-lint's
  upward config discovery by this workflow's per-module lint steps, and later by Plan 2's `apps/cli`
  lint step.

**Produces**
- `.github/workflows/ci.yml` — jobs `js` (ubuntu-latest; includes spec §9's actionlint workflow-lint
  step) and `go` (macos-latest).

#### Steps

- [ ] **Step 1: Confirm starting state**
  Run:
  ```bash
  test -f /Users/guilherme/Dev/pessoal/app-cleaner/packages/engine/go.mod && \
  test -f /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/go.mod && \
  test ! -d /Users/guilherme/Dev/pessoal/app-cleaner/apps/cli && \
  echo "OK: engine + desktop modules present, apps/cli absent as expected"
  ```
  Expected outcome: prints `OK: engine + desktop modules present, apps/cli absent as expected`. If
  `apps/cli` already exists, Plan 2 has landed early — keep the `hashFiles('apps/cli/go.mod')`
  guards below anyway; they are no-ops once the directory is present.

- [ ] **Step 2: Verify actionlint would fail on a missing file (sanity check the verification step itself)**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && actionlint .github/workflows/ci.yml
  ```
  Expected outcome: `actionlint: open .github/workflows/ci.yml: no such file or directory` (nonzero
  exit) — confirms the file does not yet exist and that `actionlint` is on `PATH` (installed via
  `brew install actionlint`; already present in this environment as v1.7.12).

- [ ] **Step 3: Verify the root `.golangci.yml` (owned by Task 5) is present and correct**
  Task 5 Step 10 created and committed `/Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml`; this
  task only consumes it. Do NOT create or overwrite it, and do NOT add a `gofumpt.module-path`
  override: option-tab pins `module-path: option-tab` because its single module is literally named
  `option-tab`, but this repo has three modules with three different import paths, and golangci-lint
  always runs from inside the target module's directory here, so gofumpt auto-detects the local
  module from that directory's `go.mod` — Task 5's file (no `module-path` key) is correct as-is.
  Run:
  ```bash
  test -f /Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml && \
  grep -q 'version: "2"' /Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml && \
  ! grep -q 'module-path' /Users/guilherme/Dev/pessoal/app-cleaner/.golangci.yml && \
  echo "OK: root .golangci.yml present, v2 config, no module-path override"
  ```
  Expected outcome: prints the OK line. If the file is missing, Task 5 did not complete — stop and
  finish Task 5 rather than recreating the file here.
  This single root file is picked up by golangci-lint's upward config discovery regardless of which
  module's directory the linter is invoked from (`working-directory: packages/engine`,
  `apps/desktop`, or later `apps/cli`), exactly as option-tab's does for its one module.

- [ ] **Step 4: Create `.github/workflows/ci.yml`**
  Create `/Users/guilherme/Dev/pessoal/app-cleaner/.github/workflows/ci.yml`:
  ```yaml
  name: CI
  on:
    pull_request:
    push:
      branches: [main]

  jobs:
    js:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - uses: oven-sh/setup-bun@v2
          with: { bun-version: latest }
        - run: bun install --frozen-lockfile
        - run: bun run lint
        - run: bun run test
        - run: bun run build
        - name: Lint workflows (actionlint, spec §9)
          run: |
            bash <(curl -fsSL https://raw.githubusercontent.com/rhysd/actionlint/main/scripts/download-actionlint.bash)
            ./actionlint -color

    go:
      runs-on: macos-latest
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with: { go-version: "1.26" }
        - uses: oven-sh/setup-bun@v2
          with: { bun-version: latest }
        - name: Build desktop frontend (main.go embeds frontend/dist)
          run: bun install --frozen-lockfile && cd apps/desktop/frontend && bun run build
        - name: Test packages/engine
          run: cd packages/engine && go test ./... -race -cover
        - name: Test apps/desktop
          run: cd apps/desktop && go test ./... -race -cover
        - name: Test apps/cli
          if: hashFiles('apps/cli/go.mod') != ''
          run: cd apps/cli && go test ./... -race -cover
        - name: gofmt check (packages/engine)
          run: test -z "$(gofmt -l packages/engine)"
        - name: gofmt check (apps/desktop)
          run: test -z "$(gofmt -l apps/desktop)"
        - name: gofmt check (apps/cli)
          if: hashFiles('apps/cli/go.mod') != ''
          run: test -z "$(gofmt -l apps/cli)"
        - name: go vet (packages/engine)
          run: cd packages/engine && go vet ./...
        - name: go vet (apps/desktop)
          run: cd apps/desktop && go vet ./...
        - name: go vet (apps/cli)
          if: hashFiles('apps/cli/go.mod') != ''
          run: cd apps/cli && go vet ./...
        - name: golangci-lint (packages/engine)
          uses: golangci/golangci-lint-action@v7
          with:
            version: v2.12
            working-directory: packages/engine
        - name: golangci-lint (apps/desktop)
          uses: golangci/golangci-lint-action@v7
          with:
            version: v2.12
            working-directory: apps/desktop
        - name: golangci-lint (apps/cli)
          if: hashFiles('apps/cli/go.mod') != ''
          uses: golangci/golangci-lint-action@v7
          with:
            version: v2.12
            working-directory: apps/cli
  ```

- [ ] **Step 5: actionlint passes**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && actionlint .github/workflows/ci.yml
  ```
  Expected outcome: no output, exit code 0. (`actionlint` shells out to `shellcheck`, also present
  in this environment at v0.11.0, so the `run:` blocks are shellcheck-clean too — every variable
  above is quoted, `$(...)` command substitutions are used only in `test -z "..."` contexts.)

- [ ] **Step 6: Dry-run the real per-module commands locally**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner/packages/engine && go test ./... -race -cover && gofmt -l . && go vet ./...
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop && go test ./... -race -cover && gofmt -l . && go vet ./...
  ```
  Expected outcome: both `go test` runs print `ok` for every package (the 244 existing Go tests,
  now split across the two modules by Tasks 1–5) and exit 0; both `gofmt -l` calls print nothing;
  both `go vet` calls exit 0 with no output.

- [ ] **Step 7: golangci-lint runs clean against the real modules**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner/packages/engine && golangci-lint run
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop && golangci-lint run
  ```
  (Install golangci-lint locally first if not present: `brew install golangci-lint`.)
  Expected outcome: `0 issues.` for both modules. If pre-existing issues surface (e.g. `errcheck` on
  a call the desktop/engine port left unchecked), fix them in this step before committing — do not
  weaken `.golangci.yml` to paper over a real finding.

- [ ] **Step 8: Commit**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add .github/workflows/ci.yml
  git commit -m "$(cat <<'EOF'
  ci: add ci.yml (js + go jobs)

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```
  Expected outcome: commit succeeds; `git status` shows a clean tree.

---

### Task 7: `deploy-web.yml` + `release.yml` + appdmg config

#### Files
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/.github/workflows/deploy-web.yml`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/.github/workflows/release.yml`
- Create: `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/darwin/dmg/appdmg.json`
- Test: none (infrastructure config — verified via `actionlint` and a local JSON parse; the release
  workflow's signing/notarization branches only execute in CI where `HAS_MACOS_SIGNING` and the
  five secrets exist, per the contract's controller-run §7 setup — never exercised locally)

#### Interfaces
**Consumes**
- `apps/web` — `bun run build` producing `apps/web/out` (from the earlier Plan 1 web task).
- `packages/shared` (`@app-cleaner/shared`) — the `releaseAssetName`/`downloadUrl` contract whose
  exact strings this workflow's asset filenames must match byte-for-byte:
  `app-cleaner_<version>_darwin_universal.dmg`, `app-cleaner-cli_<version>_darwin_universal.tar.gz`.
- `apps/desktop` — wails app, `wails.json` `outputfilename: "App Cleaner"` (so `wails build` emits
  `apps/desktop/build/bin/App Cleaner.app`), plus its `frontend/` and Go module (from Tasks 1–5).
- `apps/cli` — absent until Plan 2; the CLI build/sign/package steps below are guarded by
  `hashFiles('apps/cli/go.mod') != ''` so a tag cut before Plan 2 lands still releases the GUI alone.
- Secrets `MACOS_CERT_P12`, `MACOS_CERT_PASSWORD`, `APPLE_ID`, `APPLE_TEAM_ID`,
  `APPLE_APP_PASSWORD` — provisioned by the controller per spec §7, never created or read by this
  task; referenced here only by name.
- `.golangci.yml` (Task 6) — not directly referenced by these two workflows, but the `test` job
  below runs the same `go test -race` gates Task 6 established per module.

**Produces**
- `.github/workflows/deploy-web.yml` — jobs `build`, `deploy` (Pages).
- `.github/workflows/release.yml` — jobs `test`, `build-release`; triggers on tag `v*`; publishes
  release assets whose names Plan 2 and the web app's download links depend on verbatim.
- `apps/desktop/build/darwin/dmg/appdmg.json` — consumed by `release.yml`'s `npx --yes appdmg` step.

#### Steps

- [ ] **Step 1: actionlint fails on missing files (sanity check)**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && actionlint .github/workflows/deploy-web.yml .github/workflows/release.yml
  ```
  Expected outcome: `actionlint: open .github/workflows/deploy-web.yml: no such file or directory`
  (nonzero exit) — confirms neither file exists yet.

- [ ] **Step 2: Create `.github/workflows/deploy-web.yml`**
  This is option-tab's `deploy-web.yml` copied verbatim — its trigger paths (`apps/web/**`,
  `packages/shared/**`) and artifact path (`apps/web/out`) already match this repo's layout exactly,
  per the contract, so no adaptation is needed beyond the copy itself.
  Create `/Users/guilherme/Dev/pessoal/app-cleaner/.github/workflows/deploy-web.yml`:
  ```yaml
  name: Deploy Web
  on:
    push:
      branches: [main]
      paths: ["apps/web/**", "packages/shared/**"]
    workflow_dispatch:

  permissions:
    contents: read
    pages: write
    id-token: write

  # Serialize deploys; never run two at once (avoids racing/duplicate artifacts).
  concurrency:
    group: pages
    cancel-in-progress: false

  jobs:
    build:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - uses: oven-sh/setup-bun@v2
          with: { bun-version: latest }
        - run: bun install --frozen-lockfile
        - run: cd apps/web && bun run build
        - uses: actions/configure-pages@v5
          with: { enablement: true }
        - uses: actions/upload-pages-artifact@v3
          with: { path: apps/web/out }

    deploy:
      needs: build
      environment:
        name: github-pages
        url: ${{ steps.deployment.outputs.page_url }}
      runs-on: ubuntu-latest
      steps:
        - id: deployment
          uses: actions/deploy-pages@v4
  ```

- [ ] **Step 3: Create `apps/desktop/build/darwin/dmg/appdmg.json`**
  Layout and positions copied from option-tab's `apps/desktop/build/darwin/dmg/appdmg.json`, adapted:
  title, icon path, and app name to "App Cleaner"; the `background` key is **omitted entirely** (no
  background art for this product, per contract — do not include the key at all, not even empty).
  Create `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/darwin/dmg/appdmg.json`:
  ```json
  {
    "title": "App Cleaner Installer",
    "icon": "../../bin/App Cleaner.app/Contents/Resources/iconfile.icns",
    "icon-size": 128,
    "window": {
      "position": { "x": 360, "y": 220 },
      "size": { "width": 660, "height": 400 }
    },
    "contents": [
      {
        "x": 165,
        "y": 195,
        "type": "file",
        "path": "../../bin/App Cleaner.app",
        "name": "App Cleaner.app"
      },
      { "x": 495, "y": 195, "type": "link", "path": "/Applications" }
    ]
  }
  ```
  The relative paths (`../../bin/...`) resolve from this file's location
  (`apps/desktop/build/darwin/dmg/`) up to `apps/desktop/build/bin/`, where `wails build` writes
  `App Cleaner.app` (per `apps/desktop/wails.json`'s `outputfilename: "App Cleaner"`); wails
  generates `Contents/Resources/iconfile.icns` inside the bundle automatically, same as option-tab's.

- [ ] **Step 4: JSON parses cleanly**
  Run:
  ```bash
  jq . /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/darwin/dmg/appdmg.json
  ```
  Expected outcome: pretty-printed JSON echoed back, exit code 0 (confirms valid JSON syntax before
  it's consumed by the `npx --yes appdmg` step in `release.yml`).

- [ ] **Step 5: Create `.github/workflows/release.yml`**
  Create `/Users/guilherme/Dev/pessoal/app-cleaner/.github/workflows/release.yml`:
  ```yaml
  name: Release
  on:
    push:
      tags: ["v*"]

  permissions:
    contents: write

  jobs:
    # Gate the release on the full test suite: engine, desktop, cli (if present),
    # frontend, and the web build must all be green before any signing happens.
    test:
      runs-on: macos-latest
      steps:
        - uses: actions/checkout@v4
        - uses: oven-sh/setup-bun@v2
          with: { bun-version: latest }
        - uses: actions/setup-go@v5
          with: { go-version: "1.26" }
        - run: bun install --frozen-lockfile
        - name: Build desktop frontend (main.go embeds frontend/dist)
          run: cd apps/desktop/frontend && bun run build
        - name: Test packages/engine
          run: cd packages/engine && go test ./... -race -cover
        - name: Test apps/desktop
          run: cd apps/desktop && go test ./... -race -cover
        - name: Test apps/cli
          if: hashFiles('apps/cli/go.mod') != ''
          run: cd apps/cli && go test ./... -race -cover
        - name: Frontend vitest
          run: cd apps/desktop/frontend && bun run test
        - name: Web build
          run: cd apps/web && bun run build

    build-release:
      needs: test
      runs-on: macos-latest
      env:
        # secrets are not readable in step-level `if:`; surface presence here.
        HAS_MACOS_SIGNING: ${{ secrets.MACOS_CERT_P12 != '' }}
      steps:
        - uses: actions/checkout@v4
        - uses: oven-sh/setup-bun@v2
          with: { bun-version: latest }
        - uses: actions/setup-go@v5
          with: { go-version: "1.26" }
        - name: Install Wails CLI
          run: go install github.com/wailsapp/wails/v2/cmd/wails@latest
        - run: bun install --frozen-lockfile
        - name: Build desktop app (universal binary)
          run: cd apps/desktop && wails build -platform darwin/universal
        # Signing/notarization steps are skipped when the signing secrets are
        # absent (e.g. forks), so unsigned releases still build.
        - name: Import signing certificate
          if: env.HAS_MACOS_SIGNING == 'true'
          env:
            MACOS_CERT_P12: ${{ secrets.MACOS_CERT_P12 }}
            MACOS_CERT_PASSWORD: ${{ secrets.MACOS_CERT_PASSWORD }}
          run: |
            KEYCHAIN="$RUNNER_TEMP/signing.keychain-db"
            KEYCHAIN_PASSWORD="$(openssl rand -base64 24)"
            echo "$MACOS_CERT_P12" | base64 --decode > "$RUNNER_TEMP/cert.p12"
            security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN"
            security set-keychain-settings -lut 21600 "$KEYCHAIN"
            security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN"
            security import "$RUNNER_TEMP/cert.p12" -k "$KEYCHAIN" -P "$MACOS_CERT_PASSWORD" -T /usr/bin/codesign
            security set-key-partition-list -S apple-tool:,apple: -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN"
            security list-keychains -d user -s "$KEYCHAIN" login.keychain
            rm "$RUNNER_TEMP/cert.p12"
            # fail here, not at codesign, if the p12 lacks a Developer ID identity.
            security find-identity -v -p codesigning "$KEYCHAIN" | grep "Developer ID Application"
        - name: Sign app
          if: env.HAS_MACOS_SIGNING == 'true'
          run: |
            codesign --force --deep --options runtime --timestamp \
              --sign "Developer ID Application" "apps/desktop/build/bin/App Cleaner.app"
            codesign --verify --deep --strict --verbose=2 "apps/desktop/build/bin/App Cleaner.app"
        - name: Package artifact (.dmg)
          run: |
            cd apps/desktop
            npx --yes appdmg build/darwin/dmg/appdmg.json \
              "build/bin/app-cleaner_${GITHUB_REF_NAME#v}_darwin_universal.dmg"
        - name: Sign, notarize, and staple dmg
          if: env.HAS_MACOS_SIGNING == 'true'
          env:
            APPLE_ID: ${{ secrets.APPLE_ID }}
            APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
            APPLE_APP_PASSWORD: ${{ secrets.APPLE_APP_PASSWORD }}
          run: |
            DMG=(apps/desktop/build/bin/*.dmg)
            codesign --force --timestamp --sign "Developer ID Application" "${DMG[0]}"
            xcrun notarytool submit "${DMG[0]}" --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" \
              --password "$APPLE_APP_PASSWORD" --wait
            # staple fails if notarization did not fully succeed, failing the job.
            xcrun stapler staple "${DMG[0]}"
            xcrun stapler validate "${DMG[0]}"
        # Guarded so a tag cut before Plan 2 lands still releases the GUI alone;
        # softprops/action-gh-release does not fail on an unmatched glob by
        # default (fail_on_unmatched_files is false), so no CLI tarball simply
        # means the release ships with the .dmg only.
        - name: Build CLI (universal binary)
          if: hashFiles('apps/cli/go.mod') != ''
          run: |
            cd apps/cli
            VERSION="${GITHUB_REF_NAME#v}"
            CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o bin/app-cleaner-amd64 .
            CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" -o bin/app-cleaner-arm64 .
            lipo -create -output bin/app-cleaner bin/app-cleaner-amd64 bin/app-cleaner-arm64
            rm bin/app-cleaner-amd64 bin/app-cleaner-arm64
        - name: Sign CLI binary
          if: hashFiles('apps/cli/go.mod') != '' && env.HAS_MACOS_SIGNING == 'true'
          run: codesign --force --options runtime --timestamp --sign "Developer ID Application" apps/cli/bin/app-cleaner
        - name: Package CLI tarball
          if: hashFiles('apps/cli/go.mod') != ''
          run: |
            cd apps/cli/bin
            tar -czf "app-cleaner-cli_${GITHUB_REF_NAME#v}_darwin_universal.tar.gz" app-cleaner
        - uses: softprops/action-gh-release@v2
          with:
            files: |
              apps/desktop/build/bin/*.dmg
              apps/cli/bin/*.tar.gz
  ```
  Note the CLI tarball step `cd apps/cli/bin && tar -czf ... app-cleaner` — this puts exactly the
  binary `app-cleaner` at the tarball root, per the contract ("Tarball contains exactly
  `app-cleaner` at its root"). The `.dmg` and `.tar.gz` filenames above are byte-identical to
  `packages/shared`'s `releaseAssetName("gui", version)` / `releaseAssetName("cli", version)`
  output with `version = ${GITHUB_REF_NAME#v}` — verify this against that package's test in whatever
  task implements `packages/shared` before tagging a release.

- [ ] **Step 6: actionlint passes on both files**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner && actionlint .github/workflows/deploy-web.yml .github/workflows/release.yml
  ```
  Expected outcome: no output, exit code 0. (Both files' `run:` blocks are shellcheck-clean at
  v0.11.0: every expansion is quoted, the `DMG=(...)` glob-to-array assignment mirrors option-tab's
  already-passing pattern.)

- [ ] **Step 7: Dry-parse both workflow files as YAML**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in ['.github/workflows/deploy-web.yml','.github/workflows/release.yml']]; print('OK')"
  ```
  Expected outcome: prints `OK`. (If `pyyaml` isn't available locally, `actionlint` in Step 6 already
  performs a stricter structural parse — this step is a redundant belt-and-suspenders check, not a
  hard gate; skip it if `python3 -c "import yaml"` errors with `ModuleNotFoundError` and rely on
  Step 6 alone.)

- [ ] **Step 8: Confirm the guarded CLI steps are inert today**
  Run:
  ```bash
  test ! -d /Users/guilherme/Dev/pessoal/app-cleaner/apps/cli && echo "apps/cli absent — CLI release steps will no-op via hashFiles() guard"
  ```
  Expected outcome: prints the confirmation line, demonstrating that tagging `v*` today would build
  and publish only the signed `.dmg`, exactly as required by "a tag cut before Plan 2 still releases
  the GUI alone."

- [ ] **Step 9: Commit**
  Run:
  ```bash
  cd /Users/guilherme/Dev/pessoal/app-cleaner
  git add .github/workflows/deploy-web.yml .github/workflows/release.yml apps/desktop/build/darwin/dmg/appdmg.json
  git commit -m "$(cat <<'EOF'
  ci: add deploy-web.yml, release.yml, and appdmg packaging config

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```
  Expected outcome: commit succeeds; `git status` shows a clean tree.

---

## Plan 1 — Tasks 8–9

Starting state assumed: Tasks 1–7 done and committed locally on `main` (monorepo restructure, `packages/engine`, `apps/desktop`, `apps/web`, `packages/shared`, `Taskfile.yml`, `.github/workflows/{ci,deploy-web,release}.yml` all authored and passing locally). No git remote exists yet (`git remote -v` is empty). All 244 Go + 80 frontend tests plus the new web/shared unit tests are green locally.

---

### Task 8: Create GitHub repo, push, configure secrets + Pages

**⚠ Controller-run — do NOT dispatch to a subagent; secrets are involved.** Every command below must be run directly by the session controller in its own shell. No subagent, background task, or logged tool transcript may see `$P12PASS`, the base64 p12 body, or the notarization password. Per contract §"Global constraints" and spec §7, secrets are documented here only as controller instructions — no secret value is ever written to a file, committed, or printed to stdout by any command in this task.

**Files:**
- Create: none (this task only calls `gh`, `security`, `openssl`, `dig` against GitHub/macOS; it writes no repository files)
- Modify: none
- Test: none (verification is via `gh secret list` / `gh run watch` / `dig`, run inline as part of the steps below)

**Interfaces:**
- Consumes: an authenticated `gh` CLI session (`gh auth status` succeeds) with admin rights over the intended `GuilhermeVozniak` GitHub account/org; the local `main` branch from Tasks 1–7 with a clean working tree; the macOS login Keychain containing Developer ID Application identity `BA63827BD3FD3C6C80904C50ACC02A1E786B0D1F`; the file `~/Documents/Certificates/notorize_tool_password.txt` (app-specific password, single line); DNS control at the `vozniak.dev` provider (exercised by the user, not the controller).
- Produces: GitHub repo `https://github.com/GuilhermeVozniak/app-cleaner` (public), with `origin` pointing at it and `main` pushed and tracked; exactly 5 Actions secrets present — `MACOS_CERT_P12`, `MACOS_CERT_PASSWORD`, `APPLE_APP_PASSWORD`, `APPLE_TEAM_ID`, `APPLE_ID` — consumed by `release.yml`'s `build-release` job (already authored in an earlier task per contract §Workflows); GitHub Pages enabled with `build_type=workflow` and custom domain `app-cleaner.vozniak.dev` (consumed by `deploy-web.yml`, already authored); a first successful `deploy-web.yml` run. Consumed by Task 9 (which verifies CI + Pages against this remote and these secrets).

- [ ] **Step 1: Verify preconditions.**
  ```
  gh auth status
  git -C /Users/guilherme/Dev/pessoal/app-cleaner status --short
  git -C /Users/guilherme/Dev/pessoal/app-cleaner remote -v
  git -C /Users/guilherme/Dev/pessoal/app-cleaner log --oneline -1
  ```
  Expected: `gh auth status` reports "Logged in to github.com"; `git status --short` prints nothing (clean tree); `git remote -v` prints nothing (no remote yet); `git log --oneline -1` shows the last Task 1–7 commit.

- [ ] **Step 2: Create the GitHub repo from the local checkout and push `main`.**
  ```
  gh repo create GuilhermeVozniak/app-cleaner --public --source . --push \
    --description "App Cleaner — native macOS disk cleaner with a GUI and terminal CLI, ported from mac-cleaner-cli."
  ```
  Expected: repo created at `https://github.com/GuilhermeVozniak/app-cleaner`; command output ends with a line confirming the push; `git -C /Users/guilherme/Dev/pessoal/app-cleaner remote -v` now shows `origin  https://github.com/GuilhermeVozniak/app-cleaner.git (fetch/push)` (exact host casing as `gh` writes it) and `main` tracks `origin/main`.

- [ ] **Step 3: Confirm the push landed.**
  ```
  git -C /Users/guilherme/Dev/pessoal/app-cleaner ls-remote --heads origin main
  ```
  Expected: one line, `<sha>	refs/heads/main`, where `<sha>` matches `git rev-parse main` locally.

- [ ] **Step 4: Generate a fresh p12 password and export the signing identity from the login Keychain.**
  ```
  P12PASS=$(openssl rand -base64 24)
  TMP_DIR="$(mktemp -d -t app-cleaner-signing)"
  TMP_P12="$TMP_DIR/cert.p12"
  security export -k login.keychain -t identities -f pkcs12 -P "$P12PASS" -o "$TMP_P12"
  ```
  (`mktemp -d` avoids the stray zero-byte file that a bare `mktemp -t name` followed by appending `.p12` to the *name* would leave behind; everything secret-adjacent lives in one directory that Step 5 removes wholesale.)
  Note: macOS will show a Keychain access-control dialog asking permission for `security` to export the private key for identity `BA63827BD3FD3C6C80904C50ACC02A1E786B0D1F` — the user must click "Always Allow" or "Allow" for the export to proceed. `$P12PASS` never appears in the command's stdout (it is only referenced by variable name). Expected: exit code 0; `ls -l "$TMP_P12"` shows a non-empty file.

- [ ] **Step 5: Push the two certificate secrets, then delete the temp file and unset the password.**
  ```
  base64 -i "$TMP_P12" | gh secret set MACOS_CERT_P12 --repo GuilhermeVozniak/app-cleaner
  printf '%s' "$P12PASS" | gh secret set MACOS_CERT_PASSWORD --repo GuilhermeVozniak/app-cleaner
  rm -rf "$TMP_DIR"
  unset P12PASS TMP_P12 TMP_DIR
  ```
  Both secrets are fed to `gh secret set` over stdin (a pipe from `base64` / the shell-builtin `printf`), never as `--body` arguments — so no secret value ever appears in any process's argv (visible to `ps`), in the shell history file (which records only the unexpanded command text), or on stdout. Expected: two lines of the form `✓ Set secret MACOS_CERT_P12 for GuilhermeVozniak/app-cleaner` / `MACOS_CERT_PASSWORD`; the temp directory is gone (a follow-up `ls` of its former path reports "No such file or directory") and no shell variable still holds the password (`echo "${P12PASS:-unset}"` prints `unset`).

- [ ] **Step 6: Push the notarization identity secrets.**
  ```
  gh secret set APPLE_TEAM_ID --repo GuilhermeVozniak/app-cleaner --body CT22R575UG
  gh secret set APPLE_ID --repo GuilhermeVozniak/app-cleaner --body gui336699@gmail.com
  tr -d '\n' < ~/Documents/Certificates/notorize_tool_password.txt | gh secret set APPLE_APP_PASSWORD --repo GuilhermeVozniak/app-cleaner
  ```
  Expected: three `✓ Set secret ...` confirmation lines; the `tr -d '\n'` strips the trailing newline from the password file before it reaches `gh secret set` (which reads the secret body from stdin when no `--body` is given), so no newline is embedded in the stored secret.

- [ ] **Step 7: Verify all 5 secret names are present (values are never retrievable or printed).**
  ```
  gh secret list --repo GuilhermeVozniak/app-cleaner
  ```
  Expected: exactly 5 rows, names `APPLE_APP_PASSWORD`, `APPLE_ID`, `APPLE_TEAM_ID`, `MACOS_CERT_P12`, `MACOS_CERT_PASSWORD` (order may vary; values are never shown).

- [ ] **Step 8: Enable GitHub Pages with the Actions-workflow build type.**
  ```
  gh api -X POST repos/GuilhermeVozniak/app-cleaner/pages -f build_type=workflow
  ```
  Expected: HTTP 201 with the new Pages site JSON. If `deploy-web.yml`'s `enablement: true` already auto-created the Pages site on push (Step 2's push may have triggered the workflow), this call instead returns HTTP 409 ("Pages already enabled for this repository") — in that case skip directly to Step 9; the site's `build_type` is already `workflow` in that path.

- [ ] **Step 9: Set the custom domain.**
  ```
  gh api -X PUT repos/GuilhermeVozniak/app-cleaner/pages -f cname=app-cleaner.vozniak.dev
  ```
  Expected: HTTP 204 (no body). Follow-up `gh api repos/GuilhermeVozniak/app-cleaner/pages -q .cname` prints `app-cleaner.vozniak.dev`.

- [ ] **Step 10: USER ACTION — add the DNS record (not runnable by the controller).**
  At the `vozniak.dev` DNS provider, add:
  ```
  CNAME  app-cleaner  ->  guilhermevozniak.github.io.
  ```
  This step is manual; the controller only prompts the user to do it and waits.

- [ ] **Step 11: Check DNS propagation.**
  ```
  dig +short CNAME app-cleaner.vozniak.dev
  ```
  Expected: `guilhermevozniak.github.io.`. If empty, propagation hasn't finished yet — re-run periodically (do not loop-sleep more than a few checks in one sitting; this can take minutes to hours). Task 9's Pages-live check depends on this having resolved.

- [ ] **Step 12: Confirm the first `deploy-web.yml` run published successfully.**
  ```
  gh run list --repo GuilhermeVozniak/app-cleaner --workflow=deploy-web.yml --limit 1
  gh run watch --repo GuilhermeVozniak/app-cleaner \
    "$(gh run list --repo GuilhermeVozniak/app-cleaner --workflow=deploy-web.yml --limit 1 --json databaseId -q '.[0].databaseId')" \
    --exit-status
  ```
  Expected: `gh run watch` exits 0 with the run's conclusion "success" (jobs "build" and "deploy" both green).

---

### Task 9: Final verification + docs

**Files:**
- Create: none
- Modify:
  - `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/wails.json` (branding: `author.name`, `info.companyName`, `info.copyright`)
  - `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/darwin/Info.plist` and `.../Info.dev.plist` (bundle id + copyright)
  - `/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/main.go` (About copyright + `SingleInstanceLock` UniqueId)
  - `/Users/guilherme/Dev/pessoal/app-cleaner/docs/superpowers/specs/2026-07-08-monorepo-cli-release-design.md` (status line, line 3)
  - `/Users/guilherme/Dev/pessoal/app-cleaner/.superpowers/sdd/progress.md` (ledger note — this file is gitignored per `.gitignore`'s `.superpowers/` entry, so this edit is local-only and is never staged/committed)
- Test: none new (this task re-runs the existing full suites as gates; no test files are added)

**Interfaces:**
- Consumes: the GitHub remote, secrets, and Pages config produced by Task 8; `Taskfile.yml`'s `lint`/`test` targets and `.github/workflows/ci.yml` authored in an earlier Task 1–7; `apps/desktop` (Wails project) and `apps/web` (`NEXT_PUBLIC_BASE_PATH` env-driven `basePath`) from Tasks 1–7; the spec file's existing status line (`**Date:** 2026-07-08 · **Status:** Approved · ...`).
- Produces: a branding-consistency commit (bundle id `com.guilhermevozniak.appcleaner`, author/copyright strings) plus a docs commit on `main` (pushed) recording that Plan 1 (Tasks 1–9) shipped; confirmed-green `ci.yml` run on GitHub for that push; confirmed-live Pages site; explicit confirmation that **no `v1.0.0` tag exists** — tagging is deferred to Plan 2 per contract/spec §10–11. This is the state Plan 2 Task 1 (append `./apps/cli` to `go.work`) starts from.

- [ ] **Step 1: Branding-consistency pass — bundle id and author strings off `guhcostan`.**
  Task 2 Step 11 deliberately left the cosmetic `guhcostan` branding strings in place; fix them now so the release build ships a consistent product identity. Exact edits (current values verified against the tree):
  1. `apps/desktop/wails.json` — `"author": { "name": "guhcostan" }` → `"author": { "name": "Guilherme Vozniak" }`; `"companyName": "guhcostan"` → `"companyName": "Guilherme Vozniak"`; `"copyright": "© 2026 guhcostan — MIT"` → `"copyright": "© 2026 Guilherme Vozniak — MIT"`.
  2. `apps/desktop/build/darwin/Info.plist` **and** `apps/desktop/build/darwin/Info.dev.plist` — `<key>CFBundleIdentifier</key><string>com.guhcostan.appcleaner</string>` → `<string>com.guilhermevozniak.appcleaner</string>`; `<key>NSHumanReadableCopyright</key><string>© 2026 guhcostan — MIT</string>` → `<string>© 2026 Guilherme Vozniak — MIT</string>`.
  3. `apps/desktop/main.go` — `SingleInstanceLock` `UniqueId: "com.guhcostan.appcleaner"` → `"com.guilhermevozniak.appcleaner"` (keep it identical to the bundle id), and the About message `"Version 1.0.0\n© 2026 guhcostan — MIT\n\nA macOS cleaning app — Go/Wails port of mac-cleaner-cli."` → `"Version 1.0.0\n© 2026 Guilherme Vozniak — MIT\n\nA macOS cleaning app — Go/Wails port of mac-cleaner-cli."`.

  **FDA note (user-visible, expected):** macOS keys Full Disk Access grants to the bundle identifier. After this change the FDA grant previously given to `com.guhcostan.appcleaner` no longer applies — the rebuilt app shows the FDA onboarding screen again, and FDA must be re-granted once for `com.guilhermevozniak.appcleaner` (System Settings → Privacy & Security → Full Disk Access). This is expected, not a regression; say so in the commit body.

  Verify and commit:
  ```
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop && gofmt -l . && go vet ./... && go test ./...
  grep -rn guhcostan /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/wails.json \
    /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/darwin/Info.plist \
    /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/darwin/Info.dev.plist \
    /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/main.go || echo "branding clean"
  git -C /Users/guilherme/Dev/pessoal/app-cleaner add apps/desktop/wails.json apps/desktop/build/darwin/Info.plist apps/desktop/build/darwin/Info.dev.plist apps/desktop/main.go
  git -C /Users/guilherme/Dev/pessoal/app-cleaner commit -m "$(cat <<'EOF'
  chore(desktop): rebrand bundle id and author strings to Guilherme Vozniak

  CFBundleIdentifier (and SingleInstanceLock UniqueId) change from
  com.guhcostan.appcleaner to com.guilhermevozniak.appcleaner; wails.json
  author/companyName/copyright and main.go's About copyright follow. Full
  Disk Access is keyed to the bundle id, so FDA must be re-granted once
  for the new identifier — expected, not a regression.

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```
  Expected: `gofmt -l` prints nothing, `go vet`/`go test` green (no test asserts these strings); the grep prints `branding clean`; commit succeeds with exactly those four files.

- [ ] **Step 2: Run the full lint gate.**
  ```
  cd /Users/guilherme/Dev/pessoal/app-cleaner && task lint
  ```
  Expected: exit 0. Covers: biome over `apps/web` + `packages/shared` + root; `tsc --noEmit` for `apps/desktop/frontend`; `gofmt -l` (empty output) + `go vet ./...` for `packages/engine` and `apps/desktop`.

- [ ] **Step 3: Run the full test gate.**
  ```
  task test
  ```
  Expected: exit 0. Covers: `go test -race ./...` in `packages/engine` and `apps/desktop` (244 tests, unchanged from pre-restructure per contract's "no behavior change" guarantee), `apps/desktop/frontend` vitest (80 tests), `apps/web` vitest, `packages/shared` vitest.

- [ ] **Step 4: Build the desktop app as a universal binary.**
  ```
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop && wails build -platform darwin/universal
  ```
  Expected: exit 0; produces `apps/desktop/build/bin/App Cleaner.app`.

- [ ] **Step 5: Verify the embedded binary is truly universal.**
  ```
  lipo -archs "/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/bin/App Cleaner.app/Contents/MacOS/App Cleaner"
  ```
  Expected: output is exactly `x86_64 arm64` (order may vary).

- [ ] **Step 6: Smoke-launch the built app.**
  ```
  open "/Users/guilherme/Dev/pessoal/app-cleaner/apps/desktop/build/bin/App Cleaner.app"
  sleep 3
  pgrep -f "App Cleaner.app/Contents/MacOS/App Cleaner"
  ```
  Expected: `pgrep` prints a PID (the process launched without crashing); visually confirm the window renders the FDA/smart-scan screen (not a blank/crashed window). After Step 1's bundle-id change macOS treats this as a brand-new app for Full Disk Access, so the FDA onboarding screen being shown again is the expected first render — re-grant FDA for `com.guilhermevozniak.appcleaner` if you want to exercise a scan, though launching and rendering is all this step gates on. Then quit it:
  ```
  osascript -e 'tell application "App Cleaner" to quit'
  ```
  Expected: exits 0; a follow-up `pgrep -f "App Cleaner.app/Contents/MacOS/App Cleaner"` finds nothing.

- [ ] **Step 7: Build the web app with an empty base path and confirm no path prefix leaked in.**
  ```
  cd /Users/guilherme/Dev/pessoal/app-cleaner/apps/web && NEXT_PUBLIC_BASE_PATH= bun run build
  ```
  Expected: exit 0; static export written to `apps/web/out/`. Then:
  ```
  cat /Users/guilherme/Dev/pessoal/app-cleaner/apps/web/out/CNAME
  grep -o 'href="[^"]*"' /Users/guilherme/Dev/pessoal/app-cleaner/apps/web/out/index.html | head -5
  ```
  Expected: `CNAME` contains exactly `app-cleaner.vozniak.dev`; the sampled `href`s are root-relative (no `/app-cleaner` or other prefix segment before the resource path).

- [ ] **Step 8: Update the spec's status line.**
  Change line 3 of `docs/superpowers/specs/2026-07-08-monorepo-cli-release-design.md`:

  Old:
  ```
  **Date:** 2026-07-08 · **Status:** Approved · **Reference implementation:** `~/Dev/pessoal/option-tab` (same author, same patterns)
  ```

  New:
  ```
  **Date:** 2026-07-08 · **Status:** Plan 1 shipped 2026-07-08 (monorepo, web, CI/release workflows, repo live at https://github.com/GuilhermeVozniak/app-cleaner); Plan 2 (§8 Terminal CLI) pending · **Reference implementation:** `~/Dev/pessoal/option-tab` (same author, same patterns)
  ```

- [ ] **Step 9: Add a ledger note (local-only, gitignored).**
  Prepend to the top of `.superpowers/sdd/progress.md`:
  ```
  # Plan 2026-07-08: monorepo-cli-release (Plan 1 of 2)
  # plan: docs/superpowers/plans/2026-07-08-monorepo-cli-contract.md
  Plan 1 (Tasks 1-9) SHIPPED 2026-07-08: monorepo restructure (packages/engine, apps/desktop, apps/web, packages/shared), ci.yml/deploy-web.yml/release.yml authored, repo created and pushed to https://github.com/GuilhermeVozniak/app-cleaner (public), 5 release secrets set (MACOS_CERT_P12, MACOS_CERT_PASSWORD, APPLE_APP_PASSWORD, APPLE_TEAM_ID, APPLE_ID), GitHub Pages live at https://app-cleaner.vozniak.dev (custom domain, DNS CNAME confirmed). Verified: task lint/task test green, wails build -platform darwin/universal produces a true x86_64+arm64 App Cleaner.app that launches cleanly, web builds clean with empty NEXT_PUBLIC_BASE_PATH. Branding: bundle id com.guhcostan.appcleaner -> com.guilhermevozniak.appcleaner (wails.json/Info.plists/main.go author strings updated; FDA re-grant required for the new id, expected). v1.0.0 tag intentionally NOT cut — deferred to Plan 2 (apps/cli TUI) per release-design spec §10-11.
  Task 9: complete (branding-consistency commit + final verification + docs commit)
  Task 8: complete (controller-run: gh repo create, 5 secrets set, Pages configured with custom domain; DNS CNAME added by user, propagation confirmed)
  ```
  This file is untracked by git (`.gitignore` excludes all of `.superpowers/`), so this edit needs no `git add`.

- [ ] **Step 10: Commit the docs-only change.**
  ```
  git -C /Users/guilherme/Dev/pessoal/app-cleaner add docs/superpowers/specs/2026-07-08-monorepo-cli-release-design.md
  git -C /Users/guilherme/Dev/pessoal/app-cleaner commit -m "$(cat <<'EOF'
  docs: Plan 1 shipped — monorepo, CI/release, and repo live at GuilhermeVozniak/app-cleaner

  Co-Authored-By: WOZCODE <contact@withwoz.com>
  EOF
  )"
  ```
  Expected: commit succeeds (only the spec file is staged — `.superpowers/sdd/progress.md` is gitignored and must not appear in `git status`'s staged list); `git -C /Users/guilherme/Dev/pessoal/app-cleaner status --short` shows a clean tree afterward (progress.md shows as untracked-and-ignored, i.e. invisible under default `git status`).

- [ ] **Step 11: Push `main`.**
  ```
  git -C /Users/guilherme/Dev/pessoal/app-cleaner push origin main
  ```
  Expected: push succeeds, `main -> main`.

- [ ] **Step 12: Confirm `ci.yml` is green for this push.**
  ```
  gh run list --repo GuilhermeVozniak/app-cleaner --workflow=ci.yml --limit 1
  gh run watch --repo GuilhermeVozniak/app-cleaner \
    "$(gh run list --repo GuilhermeVozniak/app-cleaner --workflow=ci.yml --limit 1 --json databaseId -q '.[0].databaseId')" \
    --exit-status
  ```
  Expected: exits 0; both `js` and `go` jobs report "success".

- [ ] **Step 13: Confirm Pages is live over the custom domain.**
  ```
  curl -sI https://app-cleaner.vozniak.dev
  ```
  Expected: first response line `HTTP/2 200` (requires Task 8 Step 11's DNS check to have already resolved — if this still 404s/times out, re-check `dig +short CNAME app-cleaner.vozniak.dev` before treating this as a failure).

- [ ] **Step 14: Confirm no release tag exists yet.**
  ```
  git -C /Users/guilherme/Dev/pessoal/app-cleaner tag -l 'v*'
  ```
  Expected: empty output. `v1.0.0` is intentionally not tagged here — it waits for Plan 2 (the `apps/cli` TUI) to land, per contract/spec §10–11.
