package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
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
