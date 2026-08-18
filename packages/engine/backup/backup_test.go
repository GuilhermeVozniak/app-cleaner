package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// withFixedNow pins the package's now() seam for deterministic session names.
func withFixedNow(t *testing.T, fixed time.Time) {
	t.Helper()
	orig := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = orig })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewManagerRoot(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	want := filepath.Join(home, "Library", "Application Support", "AppCleaner", "Backups")
	if m.Root != want {
		t.Fatalf("Root = %q, want %q", m.Root, want)
	}
}

func TestSessionNameIsFilenameSafe(t *testing.T) {
	withFixedNow(t, time.Date(2026, 7, 7, 18, 45, 45, 0, time.UTC))
	home := t.TempDir()
	m := NewManager(home)
	src := filepath.Join(home, "Library", "Caches", "a.txt")
	writeFile(t, src, "x")
	out := m.BackupItems(context.Background(), home,
		[]core.CleanableItem{{Path: src, Size: 1, Name: "a.txt"}}, nil)
	want := filepath.Join(m.Root, "2026-07-07T18-45-45Z")
	if out.SessionDir != want {
		t.Fatalf("SessionDir = %q, want %q (':' and '.' must become '-')", out.SessionDir, want)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	src := filepath.Join(home, "Library", "Caches", "com.example", "blob.bin")
	writeFile(t, src, "payload-123")
	item := core.CleanableItem{Path: src, Size: 11, Name: "blob.bin"}

	var progressCalls []int
	out := m.BackupItems(context.Background(), home, []core.CleanableItem{item},
		func(current, total int, it core.CleanableItem) {
			progressCalls = append(progressCalls, current)
		})

	if out.BackedUp != 1 || len(out.NotBackedUp) != 0 {
		t.Fatalf("outcome = %+v", out)
	}
	if len(progressCalls) != 1 || progressCalls[0] != 1 {
		t.Fatalf("progress calls = %v, want [1] (1-based, before each item)", progressCalls)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Fatal("source must be MOVED away by backup")
	}
	moved := filepath.Join(out.SessionDir, "HOME", "Library", "Caches", "com.example", "blob.bin")
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("backup layout wrong, want %s: %v", moved, err)
	}

	res := m.Restore(out.SessionDir, home)
	if res.Restored != 1 || res.Failed != 0 || len(res.Errors) != 0 {
		t.Fatalf("restore = %+v", res)
	}
	got, err := os.ReadFile(src)
	if err != nil || string(got) != "payload-123" {
		t.Fatalf("restored content = %q, err = %v", got, err)
	}
}

func TestBackupItemsCancelMidBatchLeavesRemainingUntouched(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	items := make([]core.CleanableItem, 3)
	for i, n := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(home, "Library", "Caches", n)
		writeFile(t, p, "x")
		items[i] = core.CleanableItem{Path: p, Size: 1, Name: n}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// BackupItems fires progress BEFORE the ctx.Err() check and BEFORE the
	// item's own rename, so cancelling during the 2nd item's progress call
	// stops the loop before that item (or the 3rd) is ever touched.
	out := m.BackupItems(ctx, home, items, func(current, total int, it core.CleanableItem) {
		if current == 2 {
			cancel()
		}
	})

	if len(out.Moved) != 1 || out.Moved[0] != items[0].Path {
		t.Fatalf("Moved = %v, want only the first item (renamed before cancellation)", out.Moved)
	}
	if len(out.NotBackedUp) != 0 {
		t.Fatalf("NotBackedUp = %v, want empty: b.txt/c.txt were never reached, not refused", out.NotBackedUp)
	}
	// b.txt and c.txt must still be at their original location: cancellation
	// must leave them completely untouched (neither moved nor recorded).
	for _, it := range items[1:] {
		if _, err := os.Stat(it.Path); err != nil {
			t.Fatalf("item %s must remain untouched after cancellation: %v", it.Path, err)
		}
	}
}

func TestBackupItemNonExistentGoesToNotBackedUp(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	missing := filepath.Join(home, "Library", "Caches", "missing.txt")

	out := m.BackupItems(context.Background(), home,
		[]core.CleanableItem{{Path: missing, Size: 0, Name: "missing.txt"}}, nil)

	if out.BackedUp != 0 || len(out.NotBackedUp) != 1 || out.NotBackedUp[0] != missing {
		t.Fatalf("outcome = %+v, want missing item in NotBackedUp", out)
	}
	if len(out.Moved) != 0 {
		t.Fatalf("Moved = %v, want empty: rename of a non-existent item must fail", out.Moved)
	}
}

func TestBackupItemsEmptyCreatesNoSessionDir(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)

	out := m.BackupItems(context.Background(), home, nil, nil)

	if out.BackedUp != 0 || len(out.NotBackedUp) != 0 || len(out.Moved) != 0 {
		t.Fatalf("outcome = %+v, want all-zero for an empty batch", out)
	}
	if out.SessionDir != "" {
		t.Fatalf("SessionDir = %q, want empty: an empty batch must not create a session directory", out.SessionDir)
	}
	// Deviation from the CLI (which pre-created the backup root regardless of
	// batch size): the Go port creates nothing on disk for an empty batch.
	if entries, err := os.ReadDir(m.Root); err == nil && len(entries) != 0 {
		t.Fatalf("Root has entries %v, want none: empty batch must not touch disk", entries)
	}
}

func TestBackupItemsCountsSuccessesAndFailures(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	ok := filepath.Join(home, "Library", "Caches", "success.txt")
	writeFile(t, ok, "test")
	fail := filepath.Join(home, "Library", "Caches", "fail.txt")

	out := m.BackupItems(context.Background(), home, []core.CleanableItem{
		{Path: ok, Size: 4, Name: "success.txt"},
		{Path: fail, Size: 0, Name: "fail.txt"},
	}, nil)

	total := 2
	if out.BackedUp+len(out.NotBackedUp) != total {
		t.Fatalf("BackedUp(%d) + NotBackedUp(%d) = %d, want %d", out.BackedUp, len(out.NotBackedUp), out.BackedUp+len(out.NotBackedUp), total)
	}
	if len(out.Moved) != 1 || out.Moved[0] != ok {
		t.Fatalf("Moved = %v, want only the successful item %q", out.Moved, ok)
	}
	if len(out.NotBackedUp) != 1 || out.NotBackedUp[0] != fail {
		t.Fatalf("NotBackedUp = %v, want only the failing item %q", out.NotBackedUp, fail)
	}
}

func TestRestoreEmptySessionDirectory(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	session := filepath.Join(m.Root, "2026-07-08T00-00-00Z")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}

	res := m.Restore(session, home)

	if res.Restored != 0 || res.Failed != 0 || len(res.Errors) != 0 {
		t.Fatalf("restore = %+v, want all-zero for an empty session directory", res)
	}
}

func TestNonHomeItemNotBackedUp(t *testing.T) {
	home := t.TempDir()
	other := t.TempDir()
	m := NewManager(home)
	outside := filepath.Join(other, "app.lproj")
	writeFile(t, outside, "keep")

	var progressCalls []int
	out := m.BackupItems(context.Background(), home,
		[]core.CleanableItem{{Path: outside, Name: "app.lproj"}},
		func(current, total int, it core.CleanableItem) {
			progressCalls = append(progressCalls, current)
		})

	if out.BackedUp != 0 || len(out.NotBackedUp) != 1 || out.NotBackedUp[0] != outside {
		t.Fatalf("outcome = %+v", out)
	}
	if len(progressCalls) != 1 {
		t.Fatalf("progress must fire even for skipped items, got %v", progressCalls)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("non-home item must be left in place for the caller to delete: %v", err)
	}
}

func TestRestoreRejectsSessionOutsideRoot(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	evil := filepath.Join(m.Root, "..", "evil")
	res := m.Restore(evil, home)
	if res.Restored != 0 || res.Failed != 1 || len(res.Errors) != 1 {
		t.Fatalf("restore = %+v", res)
	}
	if !strings.Contains(res.Errors[0], "Invalid backup directory") {
		t.Fatalf("error = %q", res.Errors[0])
	}
}

func TestRestoreRejectsNonHomeEntries(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	session := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	writeFile(t, filepath.Join(session, "etc", "passwd"), "evil")
	res := m.Restore(session, home)
	if res.Restored != 0 || res.Failed != 1 || len(res.Errors) != 1 {
		t.Fatalf("restore = %+v", res)
	}
	if !strings.Contains(res.Errors[0], "outside HOME structure") {
		t.Fatalf("error = %q", res.Errors[0])
	}
	if _, err := os.Stat(filepath.Join(home, "etc", "passwd")); !os.IsNotExist(err) {
		t.Fatal("outside-HOME entry must not be restored")
	}
}

func TestRestoreTargetRejectsTraversal(t *testing.T) {
	home := t.TempDir()
	// Defense-in-depth check on the unexported mapper: filepath.Rel can never
	// emit ".." for a walked entry, but the rule must still hold.
	if _, err := restoreTarget("HOME/../../etc/passwd", home); err == nil {
		t.Fatal("'..' in a session-relative path must be rejected")
	} else if !strings.Contains(err.Error(), "Suspicious path pattern detected") {
		t.Fatalf("error = %v", err)
	}
	if _, err := restoreTarget("NOTHOME/x", home); err == nil {
		t.Fatal("non-HOME prefix must be rejected")
	}
	got, err := restoreTarget("HOME/Library/Caches/x", home)
	if err != nil || got != filepath.Join(home, "Library", "Caches", "x") {
		t.Fatalf("target = %q, err = %v", got, err)
	}
}

func TestCleanOldRemovesOnlyOldSessions(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	oldDir := filepath.Join(m.Root, "2026-06-01T00-00-00Z")
	newDir := filepath.Join(m.Root, "2026-07-07T00-00-00Z")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	if removed := m.CleanOld(7); removed != 1 {
		t.Fatalf("CleanOld = %d, want 1", removed)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("old session must be removed")
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatal("recent session must be kept")
	}
}

func TestListNewestFirstWithRecursiveSize(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	older := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	newer := filepath.Join(m.Root, "2026-02-01T00-00-00Z")
	writeFile(t, filepath.Join(older, "HOME", "f.txt"), "12345") // 5 bytes
	if err := os.MkdirAll(newer, 0o755); err != nil {
		t.Fatal(err)
	}
	tOld := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(older, tOld, tOld); err != nil {
		t.Fatal(err)
	}

	infos := m.List()
	if len(infos) != 2 {
		t.Fatalf("len = %d, want 2", len(infos))
	}
	if infos[0].Path != newer || infos[1].Path != older {
		t.Fatalf("order = [%s, %s], want newest first", infos[0].Path, infos[1].Path)
	}
	if infos[1].Size != 5 {
		t.Fatalf("recursive size = %d, want 5", infos[1].Size)
	}
}

func TestDeleteRemovesReadOnlyEntries(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits are ignored")
	}
	home := t.TempDir()
	m := NewManager(home)
	// A backed-up .app bundle: bundles routinely carry read-only directories,
	// and unlinkat needs write permission on the PARENT dir, so a bare
	// os.RemoveAll fails with EACCES and leaves the session half-deleted.
	session := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	contents := filepath.Join(session, "HOME", "Library", "Caches", "Tool.app", "Contents")
	writeFile(t, filepath.Join(contents, "CodeResources"), "x")
	if err := os.Chmod(contents, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(contents, 0o755) }) // let t.TempDir clean up on failure
	if err := m.Delete(session); err != nil {
		t.Fatalf("Delete must clear read-only dirs before removing: %v", err)
	}
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("session not removed")
	}
}

func TestCleanOldRemovesReadOnlyEntries(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits are ignored")
	}
	home := t.TempDir()
	m := NewManager(home)
	session := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	contents := filepath.Join(session, "HOME", "Library", "Caches", "Tool.app", "Contents")
	writeFile(t, filepath.Join(contents, "CodeResources"), "x")
	if err := os.Chmod(contents, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(contents, 0o755) })
	tOld := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(session, tOld, tOld); err != nil {
		t.Fatal(err)
	}
	if removed := m.CleanOld(1); removed != 1 {
		t.Fatalf("CleanOld = %d, want 1 (read-only dirs must not block expiry)", removed)
	}
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("expired session not removed")
	}
}

func TestDeleteValidatesContainment(t *testing.T) {
	home := t.TempDir()
	m := NewManager(home)
	if err := m.Delete("/tmp/not-a-backup-session"); err == nil {
		t.Fatal("session dir outside Root must be refused")
	}
	if err := m.Delete(m.Root); err == nil {
		t.Fatal("the backups root itself must be refused")
	}
	session := filepath.Join(m.Root, "2026-01-01T00-00-00Z")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("session not removed")
	}
}

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
