# Backup Details + Regression Audit + Coverage Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expandable per-session backup details (manifest-backed with legacy fallback), a verified regression test for every bug fixed to date, and coverage raised to the spec's targets with a committed coverage report.

**Architecture:** Engine first: `BackupItems` writes `items.json` (moved items only), `Restore` skips it, new `Manager.Details(sessionDir, home)` reads manifest-first with a capped file-walk fallback. Then one Wails-bound passthrough (`GetBackupDetails`), then expandable rows in `Backups.tsx`. Parts 2–3 are verification passes: audit fixed-bug regression tests, then measure and raise coverage per the spec's targets.

**Tech Stack:** Go (packages/engine, apps/desktop, apps/cli), Wails v2 bindings (hand-maintained committed files under `apps/desktop/frontend/wailsjs/`), React 18 + zustand + vitest.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-18-backup-details-and-coverage-design.md`.
- All slices crossing the Wails bridge non-nil Go-side AND `?? []`-normalized frontend-side (nil→JSON-null crash class).
- Manifest filename exactly `items.json` at the session root; write failure non-fatal; not deleted on restore.
- Fallback walk: files only, sorted by path, capped at 200, `Truncated` = omitted count.
- Details containment error reuses `resolveSession` (exact existing error string).
- Frontend copy: `…and N more files`, `No details recorded for this backup`, `Couldn't read backup details`.
- Coverage targets: engine + CLI packages ≥ 80% lines; `apps/desktop` = every pure helper and headless-safe bound method covered, residual uncovered functions individually listed; frontend ≥ 80% lines on `src/` (excluding `wailsjs/`). Exclusions documented in `docs/superpowers/coverage-2026-07-18.md`, never silent.
- No new runtime dependencies. Tooling exception: `@vitest/coverage-v8` may be added as a devDependency if missing.
- Never `go get`/`go mod tidy` in packages/engine or apps/desktop (plist/wails MVS trap).
- Test commands: engine `cd packages/engine && go test ./...`; desktop `cd apps/desktop && go test ./...`; frontend `cd apps/desktop/frontend && npx vitest run`.
- Commit trailers on every commit:
  `Co-Authored-By: WOZCODE <contact@withwoz.com>`
  `Claude-Session: https://claude.ai/code/session_01MrKaJcom7uSdmXnRxiAjMd`

---

### Task 1: Engine — manifest write + Restore skip

**Files:**
- Modify: `packages/engine/backup/backup.go`
- Test: `packages/engine/backup/backup_test.go`

**Interfaces:**
- Consumes: existing `Manager`, `BackupItems`, `Restore`.
- Produces: exported `Item{Path string; Name string; Size int64}` (json tags `path`,`name`,`size`), unexported `const manifestName = "items.json"`; `BackupItems` writes the manifest; `Restore` skips it. Task 2 reads the manifest via `manifestName` and `Item`.

- [ ] **Step 1: Write the failing tests** — append to `backup_test.go` (reuse the file's existing helpers for making items; if none fit, use these as written):

```go
func TestBackupItemsWritesManifestOfMovedItemsOnly(t *testing.T) {
	home := t.TempDir()
	inHome := filepath.Join(home, "Library", "Caches", "a.log")
	if err := os.MkdirAll(filepath.Dir(inHome), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inHome, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "b.log") // not under home -> NotBackedUp
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(home)
	out := m.BackupItems(context.Background(), home, []core.CleanableItem{
		{Path: inHome, Size: 5, Name: "a.log"},
		{Path: outside, Size: 1, Name: "b.log"},
	}, nil)
	if out.BackedUp != 1 || len(out.NotBackedUp) != 1 {
		t.Fatalf("outcome = %+v, want 1 moved / 1 not backed up", out)
	}
	data, err := os.ReadFile(filepath.Join(out.SessionDir, "items.json"))
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("manifest not valid JSON: %v", err)
	}
	want := []Item{{Path: inHome, Name: "a.log", Size: 5}}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("manifest = %+v, want moved items only %+v", items, want)
	}
}

func TestBackupItemsManifestRecordsMovedSoFarOnCancel(t *testing.T) {
	home := t.TempDir()
	mk := func(name string) core.CleanableItem {
		p := filepath.Join(home, "Library", "Caches", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return core.CleanableItem{Path: p, Size: 1, Name: name}
	}
	items := []core.CleanableItem{mk("one"), mk("two")}
	ctx, cancel := context.WithCancel(context.Background())
	m := NewManager(home)
	out := m.BackupItems(ctx, home, items, func(current, total int, it core.CleanableItem) {
		if current == 2 {
			cancel() // fires BEFORE item 2 is processed -> only item 1 moves
		}
	})
	if out.BackedUp != 1 {
		t.Fatalf("BackedUp = %d, want 1 (cancelled before second item)", out.BackedUp)
	}
	data, err := os.ReadFile(filepath.Join(out.SessionDir, "items.json"))
	if err != nil {
		t.Fatalf("manifest not written on cancel: %v", err)
	}
	var got []Item
	if err := json.Unmarshal(data, &got); err != nil || len(got) != 1 || got[0].Name != "one" {
		t.Fatalf("manifest = %+v (err %v), want exactly the moved-so-far item 'one'", got, err)
	}
}

func TestBackupItemsNoManifestWhenNothingMoved(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(home)
	out := m.BackupItems(context.Background(), home, []core.CleanableItem{{Path: outside, Size: 1, Name: "x.log"}}, nil)
	if _, err := os.Stat(filepath.Join(out.SessionDir, "items.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest must not be written when nothing moved (stat err = %v)", err)
	}
}

func TestRestoreSkipsManifestWithoutCountingIt(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, "Library", "Caches", "c.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(home)
	out := m.BackupItems(context.Background(), home, []core.CleanableItem{{Path: p, Size: 4, Name: "c.log"}}, nil)
	res := m.Restore(out.SessionDir, home)
	if res.Failed != 0 || res.Restored != 1 {
		t.Fatalf("RestoreResult = %+v, want 1 restored / 0 failed (manifest skipped, not counted)", res)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out.SessionDir, "items.json")); err != nil {
		t.Fatal("manifest must remain in the session after restore")
	}
}
```

Add `"encoding/json"` and `"reflect"` to the test file's imports if absent.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd packages/engine && go test ./backup/ -run 'Manifest|SkipsManifest'`
Expected: FAIL — `undefined: Item` (compile error) is acceptable RED evidence.

- [ ] **Step 3: Implement** — in `backup.go`:

Add near the top (after the existing type declarations):

```go
// manifestName is the session-root manifest listing the moved items.
// Restore must skip it; Details reads it. Never under HOME/.
const manifestName = "items.json"

// Item is one moved item as recorded in the session manifest.
type Item struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}
```

In `BackupItems`, accumulate and write the manifest. After the successful-rename lines

```go
		out.BackedUp++
		out.Moved = append(out.Moved, it.Path)
```

add `manifest = append(manifest, Item{Path: it.Path, Name: it.Name, Size: it.Size})` (declare `var manifest []Item` before the loop). After the loop, before `return out`:

```go
	if len(manifest) > 0 {
		// Non-fatal: the moves already succeeded; a missing manifest only
		// degrades Details to the walk fallback.
		if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(sessionDir, manifestName), data, 0o644)
		}
	}
```

Add `"encoding/json"` to imports. In `Restore`'s WalkDir callback, immediately after the `rel, rerr := filepath.Rel(sd, path)` error check, add:

```go
		if rel == manifestName {
			return nil // session metadata, not user data — never restored, never counted
		}
```

- [ ] **Step 4: Run the package suite**

Run: `cd packages/engine && go test ./backup/`
Expected: PASS (all, including the 4 new).

- [ ] **Step 5: Commit**

```bash
git add packages/engine/backup/backup.go packages/engine/backup/backup_test.go
git commit -m "feat(backup): session manifest of moved items; Restore skips it"
```

---

### Task 2: Engine — Manager.Details

**Files:**
- Modify: `packages/engine/backup/backup.go`
- Test: `packages/engine/backup/backup_test.go`

**Interfaces:**
- Consumes: Task 1's `Item`, `manifestName`, existing `resolveSession`.
- Produces (Task 3 relies on these exact names): `type Details struct { Items []Item; FromManifest bool; Truncated int }` (json tags `items`,`fromManifest`,`truncated`) and `func (m *Manager) Details(sessionDir, home string) (Details, error)`. `Items` is ALWAYS non-nil.

- [ ] **Step 1: Write the failing tests** — append to `backup_test.go`:

```go
func TestDetailsFromManifest(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, "Library", "Caches", "d.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(home)
	out := m.BackupItems(context.Background(), home, []core.CleanableItem{{Path: p, Size: 8, Name: "d.log"}}, nil)
	d, err := m.Details(out.SessionDir, home)
	if err != nil {
		t.Fatal(err)
	}
	if !d.FromManifest || d.Truncated != 0 {
		t.Fatalf("Details = %+v, want FromManifest=true Truncated=0", d)
	}
	want := []Item{{Path: p, Name: "d.log", Size: 8}}
	if !reflect.DeepEqual(d.Items, want) {
		t.Fatalf("Items = %+v, want %+v", d.Items, want)
	}
}

func TestDetailsFallbackWalkSortedAndCapped(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	// Legacy session: build by hand, NO manifest.
	sd := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	for i := 0; i < 205; i++ {
		p := filepath.Join(sd, "HOME", "Library", "Caches", fmt.Sprintf("f%03d.log", i))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("xy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := m.Details(sd, home)
	if err != nil {
		t.Fatal(err)
	}
	if d.FromManifest {
		t.Fatal("legacy session must not report FromManifest")
	}
	if len(d.Items) != 200 || d.Truncated != 5 {
		t.Fatalf("len=%d Truncated=%d, want 200/5", len(d.Items), d.Truncated)
	}
	first := d.Items[0]
	if first.Path != filepath.Join(home, "Library", "Caches", "f000.log") || first.Name != "f000.log" || first.Size != 2 {
		t.Fatalf("first item = %+v, want reconstructed ~ path, base name, on-disk size", first)
	}
	if !sort.SliceIsSorted(d.Items, func(i, j int) bool { return d.Items[i].Path < d.Items[j].Path }) {
		t.Fatal("fallback items must be sorted by path")
	}
}

func TestDetailsContainmentAndEmptySessions(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	if _, err := m.Details(filepath.Join(t.TempDir(), "elsewhere"), home); err == nil {
		t.Fatal("session outside Root must be rejected")
	}
	// Session dir that does not exist under Root: no error, empty non-nil Items.
	d, err := m.Details(filepath.Join(m.Root, "missing-session"), home)
	if err != nil {
		t.Fatal(err)
	}
	if d.Items == nil || len(d.Items) != 0 || d.FromManifest || d.Truncated != 0 {
		t.Fatalf("Details = %+v, want empty non-nil Items and zero flags", d)
	}
}
```

Add `"fmt"` and `"sort"` to the test imports if absent.

- [ ] **Step 2: Run to verify RED**

Run: `cd packages/engine && go test ./backup/ -run 'Details'`
Expected: FAIL — `m.Details undefined` compile error.

- [ ] **Step 3: Implement** — append to `backup.go`:

```go
// detailsCap bounds the legacy fallback listing; the manifest path is uncapped
// (it holds items, not files, and is naturally small).
const detailsCap = 200

// Details describes a session's contents for display.
type Details struct {
	Items        []Item `json:"items"`
	FromManifest bool   `json:"fromManifest"`
	Truncated    int    `json:"truncated"`
}

// Details returns what a session contains: the manifest's items when present,
// otherwise a sorted, capped walk of the session's HOME/ files with original
// paths reconstructed under home. Items is never nil (JSON bridge: []).
func (m *Manager) Details(sessionDir, home string) (Details, error) {
	d := Details{Items: []Item{}}
	sd, err := m.resolveSession(sessionDir)
	if err != nil {
		return d, err
	}
	if data, rerr := os.ReadFile(filepath.Join(sd, manifestName)); rerr == nil {
		var items []Item
		if jerr := json.Unmarshal(data, &items); jerr == nil && items != nil {
			d.Items = items
			d.FromManifest = true
			return d, nil
		}
		// Corrupt manifest: fall through to the walk.
	}
	home = filepath.Clean(home)
	root := filepath.Join(sd, "HOME")
	var files []Item
	_ = filepath.WalkDir(root, func(path string, de fs.DirEntry, werr error) error {
		if werr != nil || de.IsDir() {
			return nil // unreadable entries tolerated, like List()
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		var size int64
		if st, serr := de.Info(); serr == nil {
			size = st.Size()
		}
		files = append(files, Item{Path: filepath.Join(home, rel), Name: de.Name(), Size: size})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if len(files) > detailsCap {
		d.Truncated = len(files) - detailsCap
		files = files[:detailsCap]
	}
	if files != nil {
		d.Items = files
	}
	return d, nil
}
```

(`"sort"`, `"io/fs"` are already imported by backup.go; add if the compiler disagrees.)

- [ ] **Step 4: Run the package suite**

Run: `cd packages/engine && go test ./backup/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/engine/backup/backup.go packages/engine/backup/backup_test.go
git commit -m "feat(backup): Manager.Details — manifest-first session contents with capped walk fallback"
```

---

### Task 3: Bridge — GetBackupDetails + bindings

**Files:**
- Modify: `apps/desktop/app.go` (next to `ListBackups`/`RestoreBackup`)
- Modify: `apps/desktop/frontend/wailsjs/go/main/App.js`, `apps/desktop/frontend/wailsjs/go/main/App.d.ts`, `apps/desktop/frontend/wailsjs/go/models.ts` (hand-maintained generated files — mirror the exact style of the existing `ListBackups`/`backup.Info` entries; read them first)
- Test: `apps/desktop/app_test.go`

**Interfaces:**
- Consumes: Task 2's `backup.Details` / `backup.Item`.
- Produces: bound method `GetBackupDetails(path string) backup.Details`; JS binding `GetBackupDetails(arg1)`; Task 4 calls `GetBackupDetails` from `../../wailsjs/go/main/App`.

- [ ] **Step 1: Write the failing test** — append to `apps/desktop/app_test.go` (it already constructs `App` values directly for pure-function tests; follow that pattern):

```go
func TestGetBackupDetailsHeadless(t *testing.T) {
	home := t.TempDir()
	a := &App{home: home, backupMgr: backup.NewManager(home)}
	// Invalid (outside Root) -> empty, NON-NIL items, no panic.
	d := a.GetBackupDetails(filepath.Join(t.TempDir(), "nope"))
	if d.Items == nil || len(d.Items) != 0 {
		t.Fatalf("invalid path: Details = %+v, want empty non-nil Items", d)
	}
	// Valid session with manifest -> items pass through.
	p := filepath.Join(home, "Library", "Caches", "z.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := a.backupMgr.BackupItems(context.Background(), home, []core.CleanableItem{{Path: p, Size: 3, Name: "z.log"}}, nil)
	d = a.GetBackupDetails(out.SessionDir)
	if !d.FromManifest || len(d.Items) != 1 || d.Items[0].Name != "z.log" {
		t.Fatalf("Details = %+v, want 1 manifest item z.log", d)
	}
}
```

Add imports (`context`, `os`, `path/filepath`, engine `backup`, `core`) as needed.

- [ ] **Step 2: RED**

Run: `cd apps/desktop && go test ./ -run TestGetBackupDetailsHeadless`
Expected: FAIL — `a.GetBackupDetails undefined`.

- [ ] **Step 3: Implement** — in `app.go`, after `RestoreBackup`:

```go
// GetBackupDetails returns a backup session's contents for the Backups view.
// Containment errors yield an empty (non-nil) list — the UI cannot produce an
// invalid path, and the view's empty state covers the degenerate case.
func (a *App) GetBackupDetails(path string) backup.Details {
	d, err := a.backupMgr.Details(path, a.home)
	if err != nil {
		return backup.Details{Items: []backup.Item{}}
	}
	return d
}
```

Then bindings — READ each file first and mirror existing entries exactly:
- `App.js`: alphabetical position — after `GetAppIcon`:

```js
export function GetBackupDetails(arg1) {
  return window['go']['main']['App']['GetBackupDetails'](arg1);
}
```

- `App.d.ts`: mirror the `ListBackups` declaration style, e.g. `export function GetBackupDetails(arg1:string):Promise<backup.Details>;` (match the file's actual import/namespace form).
- `models.ts`: add `Details` and `Item` classes in the `backup` namespace mirroring how `Info` is declared there (fields + `createFrom`/constructor in the file's own idiom). If `models.ts` declares `Info` with `static createFrom`, replicate for both new classes.

- [ ] **Step 4: GREEN + build**

Run: `cd apps/desktop && go test ./ && go build ./...` and `cd apps/desktop/frontend && npx tsc --noEmit`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/app.go apps/desktop/app_test.go apps/desktop/frontend/wailsjs
git commit -m "feat(backups): GetBackupDetails bound method + bindings"
```

---

### Task 4: Frontend — expandable session details in Backups.tsx

**Files:**
- Modify: `apps/desktop/frontend/src/lib/types.ts`, `apps/desktop/frontend/src/views/Backups.tsx`
- Create: `apps/desktop/frontend/src/views/Backups.test.tsx`

**Interfaces:**
- Consumes: Task 3's `GetBackupDetails`; existing `contractHome` (`../lib/uninstallMath`), `formatSize`, `ListBackups`, `RestoreBackup`, `DeleteBackup`, `CleanOldBackups`.
- Produces: `BackupItem`/`BackupDetails` types in `lib/types.ts`; no API changes for other views.

- [ ] **Step 1: Add types** — in `lib/types.ts` after `BackupInfo`:

```ts
export interface BackupItem {
  path: string;
  name: string;
  size: number;
}

export interface BackupDetails {
  items: BackupItem[];
  fromManifest: boolean;
  truncated: number;
}
```

- [ ] **Step 2: Write the failing tests** — create `Backups.test.tsx`:

```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), EventsOff: vi.fn() }))
vi.mock('../../wailsjs/go/main/App', () => ({
  ListBackups: vi.fn().mockResolvedValue([]),
  RestoreBackup: vi.fn(),
  DeleteBackup: vi.fn(),
  CleanOldBackups: vi.fn().mockResolvedValue(0),
  GetBackupDetails: vi.fn().mockResolvedValue({ items: [], fromManifest: false, truncated: 0 }),
  GetConfig: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn(),
  CheckFDA: vi.fn().mockResolvedValue(null),
}))

import { ListBackups, GetBackupDetails } from '../../wailsjs/go/main/App'
import Backups from './Backups'

const ListBackupsMock = ListBackups as unknown as ReturnType<typeof vi.fn>
const DetailsMock = GetBackupDetails as unknown as ReturnType<typeof vi.fn>

const session = { path: '/Users/me/Library/Application Support/AppCleaner/Backups/2026-07-18T00-00-00Z', date: '2026-07-18T00:00:00Z', size: 1024 }

beforeEach(() => {
  ListBackupsMock.mockClear()
  ListBackupsMock.mockResolvedValue([session])
  DetailsMock.mockReset()
  DetailsMock.mockResolvedValue({ items: [], fromManifest: false, truncated: 0 })
})
afterEach(() => cleanup())

describe('<Backups /> details', () => {
  it('expanding a session fetches details once and renders contracted paths with sizes', async () => {
    DetailsMock.mockResolvedValue({
      items: [{ path: '/Users/me/Library/Caches/Foo', name: 'Foo', size: 2048 }],
      fromManifest: true,
      truncated: 0,
    })
    render(<Backups />)
    const toggle = await screen.findByRole('button', { name: /toggle details/i })
    fireEvent.click(toggle)
    expect(await screen.findByText(/~\/Library\/Caches\/Foo \(2\.0 KB\)/)).toBeDefined()
    expect(DetailsMock).toHaveBeenCalledTimes(1)
    expect(DetailsMock).toHaveBeenCalledWith(session.path)
    // collapse + re-expand: cached, no second fetch
    fireEvent.click(toggle)
    fireEvent.click(toggle)
    expect(await screen.findByText(/~\/Library\/Caches\/Foo/)).toBeDefined()
    expect(DetailsMock).toHaveBeenCalledTimes(1)
  })

  it('shows the truncation tail', async () => {
    DetailsMock.mockResolvedValue({
      items: [{ path: '/Users/me/a', name: 'a', size: 1 }],
      fromManifest: false,
      truncated: 42,
    })
    render(<Backups />)
    fireEvent.click(await screen.findByRole('button', { name: /toggle details/i }))
    expect(await screen.findByText('…and 42 more files')).toBeDefined()
  })

  it('shows the empty state for sessions with no recorded details', async () => {
    render(<Backups />)
    fireEvent.click(await screen.findByRole('button', { name: /toggle details/i }))
    expect(await screen.findByText('No details recorded for this backup')).toBeDefined()
  })

  it('tolerates a null items payload (nil Go slice)', async () => {
    DetailsMock.mockResolvedValue({ items: null, fromManifest: false, truncated: 0 })
    render(<Backups />)
    fireEvent.click(await screen.findByRole('button', { name: /toggle details/i }))
    expect(await screen.findByText('No details recorded for this backup')).toBeDefined()
  })

  it('shows an error on fetch failure and retries on re-expand', async () => {
    DetailsMock.mockRejectedValueOnce(new Error('boom'))
    DetailsMock.mockResolvedValueOnce({ items: [{ path: '/Users/me/b', name: 'b', size: 1 }], fromManifest: true, truncated: 0 })
    render(<Backups />)
    const toggle = await screen.findByRole('button', { name: /toggle details/i })
    fireEvent.click(toggle)
    expect(await screen.findByText("Couldn't read backup details")).toBeDefined()
    fireEvent.click(toggle) // collapse
    fireEvent.click(toggle) // re-expand -> refetch
    expect(await screen.findByText(/~\/b \(1 B\)/)).toBeDefined()
    expect(DetailsMock).toHaveBeenCalledTimes(2)
  })
})
```

- [ ] **Step 3: RED**

Run: `cd apps/desktop/frontend && npx vitest run src/views/Backups.test.tsx`
Expected: FAIL — no toggle button rendered.

- [ ] **Step 4: Implement** — modify `Backups.tsx`:

Add imports:

```ts
import { ChevronDown, ChevronRight } from 'lucide-react'
import { GetBackupDetails } from '../../wailsjs/go/main/App'
import { contractHome } from '../lib/uninstallMath'
import type { BackupDetails, BackupInfo } from '../lib/types'
```

Add state + handler inside the component:

```ts
const [expanded, setExpanded] = useState<string | null>(null)
const [details, setDetails] = useState<Record<string, BackupDetails>>({})
const [detailsError, setDetailsError] = useState<string | null>(null)

const toggleExpand = async (path: string) => {
  if (expanded === path) {
    setExpanded(null)
    return
  }
  setExpanded(path)
  setDetailsError(null)
  if (details[path]) return
  try {
    const d = await GetBackupDetails(path)
    setDetails((prev) => ({
      ...prev,
      // Defense in depth: a Go nil slice arrives as JSON null.
      [path]: { items: d?.items ?? [], fromManifest: Boolean(d?.fromManifest), truncated: d?.truncated ?? 0 },
    }))
  } catch {
    setDetailsError("Couldn't read backup details") // no cache -> re-expand retries
  }
}
```

Restructure each `<li>`: keep the existing action row, prefix it with a chevron toggle, and append the expanded block:

```tsx
<li key={b.path} className="py-3">
  <div className="flex items-center gap-4">
    <button
      aria-label={`Toggle details for backup ${new Date(b.date).toLocaleString()}`}
      onClick={() => void toggleExpand(b.path)}
      className="text-zinc-500 dark:text-zinc-400"
    >
      {expanded === b.path ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
    </button>
    {/* existing date span, size span, pending-confirm / Restore-Delete buttons UNCHANGED */}
  </div>
  {expanded === b.path ? (
    <div className="mt-2 pl-8">
      {!details[b.path] ? (
        <p className="text-xs text-zinc-500 dark:text-zinc-400">{detailsError ?? 'Loading…'}</p>
      ) : details[b.path].items.length === 0 ? (
        <p className="text-xs text-zinc-500 dark:text-zinc-400">No details recorded for this backup</p>
      ) : (
        <ul className="space-y-0.5">
          {details[b.path].items.map((it) => (
            <li key={it.path} className="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400">
              {contractHome(it.path)} ({formatSize(it.size)})
            </li>
          ))}
          {details[b.path].truncated > 0 ? (
            <li className="text-xs text-zinc-400 dark:text-zinc-500">…and {details[b.path].truncated} more files</li>
          ) : null}
        </ul>
      )}
    </div>
  ) : null}
</li>
```

(The existing `flex items-center gap-4 py-3` classes move: `py-3` stays on the `<li>`, the flex classes go on the inner action-row div.)

- [ ] **Step 5: GREEN + full file suite + typecheck**

Run: `cd apps/desktop/frontend && npx vitest run src/views/Backups.test.tsx && npx vitest run && npx tsc --noEmit`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/desktop/frontend/src/lib/types.ts apps/desktop/frontend/src/views/Backups.tsx apps/desktop/frontend/src/views/Backups.test.tsx
git commit -m "feat(backups): expandable per-session details in the Backups view"
```

---

### Task 5: Regression-test audit for fixed bugs

**Files:**
- Possibly modify: test files only (if a gap is found).
- Output: audit table appended to `.superpowers/sdd/progress.md` AND carried into Task 7's coverage report.

**Interfaces:** none — verification task.

- [ ] **Step 1: Run each named regression test and record PASS:**

```bash
cd packages/engine && go test ./uninstall/ -run TestFindRelatedPathsReturnsNonNilWhenNoMatches -v
cd packages/engine && go test ./scanners/ -run 'TestDockerCleanPrune|TestDockerCleanFailureAndDryRun|TestHomebrewCleanCacheRootUsesBrewCleanup|TestHomebrewCleanDryRun' -v
cd packages/engine && go test ./backup/ -run TestRestoreSkipsManifestWithoutCountingIt -v
cd apps/desktop/frontend && npx vitest run src/views/Uninstaller.test.tsx src/components/ResultsPanel.test.tsx src/stores/uninstallerStore.test.ts
```

- [ ] **Step 2: Spot-check discrimination (one per fix, revert-and-observe, then restore):**

For each row below, make the listed temporary edit, run the listed test, confirm it FAILS, then `git checkout -- <file>`:
1. `packages/engine/uninstall/related.go`: change `out := []RelatedPath{}` to `var out []RelatedPath` → `TestFindRelatedPathsReturnsNonNilWhenNoMatches` fails.
2. `packages/engine/scanners/docker.go`: drop `Errors: []string{}` from the success return → `TestDockerCleanPrune` fails.
3. `apps/desktop/frontend/src/stores/uninstallerStore.ts`: remove the `running` check block in `startValidated` → the two running-gate tests fail.
4. `apps/desktop/frontend/src/views/Uninstaller.tsx`: remove the phase guard in the mount effect → the skip-mount-refresh test fails.
5. `packages/engine/backup/backup.go`: remove the `rel == manifestName` skip → `TestRestoreSkipsManifestWithoutCountingIt` fails.

- [ ] **Step 3: Fix any gap found** (missing/non-discriminating test → add a test in the relevant file following that file's patterns; if none found, no code change).

- [ ] **Step 4: Record** — append one line per fix to `.superpowers/sdd/progress.md`: `audit: <fix> -> <test name(s)> PASS, discriminates: yes`. Working tree must be clean afterwards (`git status --short` empty, or only the committed gap-fix).

- [ ] **Step 5: Commit** (only if Step 3 added tests) with message `test: close regression-audit gap — <what>`.

---

### Task 6: Coverage — measure + raise engine & CLI to ≥80%

**Files:**
- Test files only, in `packages/engine/**` and `apps/cli/**`.
- Scratch: coverage profiles under `/tmp` (not committed).

**Interfaces:** Produces before/after `go tool cover -func` numbers for Task 7's report.

- [ ] **Step 1: Measure BEFORE** (record full output for the report):

```bash
cd packages/engine && go test ./... -coverprofile=/tmp/engine-before.out && go tool cover -func=/tmp/engine-before.out | tail -1 && go tool cover -func=/tmp/engine-before.out | awk '$3+0 < 100 {print}' 
cd apps/cli && go test ./... -coverprofile=/tmp/cli-before.out && go tool cover -func=/tmp/cli-before.out | tail -1
```

Known starting points (2026-07-17 run): engine — backup 78.7%, maintenance 81.2%, fsx 94.9%, others 90%+; CLI — cmd 72.9%, tui 70.2%, selection 87.4%, output 96.2%, `main` package 0% (excluded: entrypoint).

- [ ] **Step 2: Raise every package below 80%** — the concrete rule, per package: run `go tool cover -func=<profile> | grep <pkg> | awk '$3+0 < 80'`, and for each listed function write table-driven tests for its uncovered branches (identify them with `go tool cover -html=<profile>`), named `Test<Func><Behavior>`. Priorities: `backup` (List on unreadable Root, CleanOld stat/remove errors, restoreTarget rejects, BackupItems MkdirAll-failure path), `cli/cmd` and `cli/internal/tui` (flag validation, error printing, model update edge cases). Tests must assert real behavior (outputs/filesystem effects), not just "doesn't panic". Respect the MVS trap: no `go get`/`go mod tidy` in engine.

- [ ] **Step 3: Measure AFTER; verify every engine+CLI package (except `main`-only entrypoint packages) ≥ 80%:**

```bash
cd packages/engine && go test ./... -coverprofile=/tmp/engine-after.out && go tool cover -func=/tmp/engine-after.out | tail -1
cd apps/cli && go test ./... -coverprofile=/tmp/cli-after.out && go tool cover -func=/tmp/cli-after.out | tail -1
```

Save the four profile summaries to `/tmp/coverage-notes.md` for Task 7.

- [ ] **Step 4: Full suites green** (`go test ./...` in engine, desktop, cli).

- [ ] **Step 5: Commit** — `test(coverage): raise engine and CLI packages to ≥80% line coverage`.

---

### Task 7: Coverage — desktop + frontend + committed report

**Files:**
- Test files in `apps/desktop` and `apps/desktop/frontend/src/**`.
- Create: `docs/superpowers/coverage-2026-07-18.md`.
- Possibly modify: root `package.json`/workspace devDependencies (ONLY `@vitest/coverage-v8`, only if missing).

**Interfaces:** Consumes Task 6's `/tmp/coverage-notes.md` and Task 5's audit lines from `.superpowers/sdd/progress.md`.

- [ ] **Step 1: Desktop** — `cd apps/desktop && go test ./... -coverprofile=/tmp/desktop.out && go tool cover -func=/tmp/desktop.out`. Add tests (in `app_test.go`, headless `App` construction as in `TestGetBackupDetailsHeadless`) for every UNcovered pure helper and headless-safe bound method — candidates: `resolveSelection`/`splitNeverBackup`/`splitByBackup` (if not fully covered), `GetConfig`/`SaveConfig` round-trip, `ListBackups`/`RestoreBackup`/`DeleteBackup`/`CleanOldBackups` against a temp home, `RunMaintenance` unknown-task branch, `GetAppIcon` with a missing bundle (returns ""). Functions that REQUIRE the Wails runtime (`startup`, `runScan`, `runClean`, `runTMClear`, `StartScan`/`StartClean`/`StartUninstall`/`StartTMSnapshotsClear` goroutine bodies, `CheckFDA`/`OpenFDASettings`/`RevealInFinder`/`CopyPath` shell/clipboard calls) are the exclusion list — name each in the report with one-line justification.
- [ ] **Step 2: Frontend** — `cd apps/desktop/frontend && npx vitest run --coverage` (add `@vitest/coverage-v8` as devDependency if missing — `bun add -d @vitest/coverage-v8` at the workspace root package, matching the existing vitest major). Add tests to reach ≥80% lines on `src/` excluding `wailsjs/`: candidates — `App.tsx` view switching + FDA gate, `FirstRun.tsx`, `Maintenance.tsx` task flows, `Settings.tsx` form round-trip, `Sidebar.tsx`, `CategoryCard`/`ItemList`/`SizeBar`/`SafetyBadge`/`EmptyState` render branches, `cleanStore` startClean-rejection branch, `lib/paths.ts`, `lib/selection.ts`. Follow the mock patterns of existing view tests. No behavior changes to source files — tests only (if a genuine bug surfaces, STOP and report it, do not silently fix).
- [ ] **Step 3: Write `docs/superpowers/coverage-2026-07-18.md`** — sections: (1) before/after per-package table for engine/CLI/desktop/frontend from the recorded profiles; (2) desktop exclusion list with justifications; (3) regression-audit table copied from Task 5's ledger lines; (4) how to re-run (the exact commands above).
- [ ] **Step 4: Everything green** — engine + desktop + cli `go test ./...`, frontend `npx vitest run && npx tsc --noEmit`.
- [ ] **Step 5: Commit** — `test(coverage): desktop + frontend hardening; committed coverage report`.
