package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// swapBackupSeams points backupManagerFor at a Manager rooted at root
// (ignoring whatever home is passed) and backupHome at home, restoring both
// on cleanup. This is how tests use a temp backup root instead of touching
// the real $HOME.
func swapBackupSeams(t *testing.T, root, home string) {
	t.Helper()
	origMgr, origHome := backupManagerFor, backupHome
	backupManagerFor = func(string) *backup.Manager { return &backup.Manager{Root: root} }
	backupHome = func() string { return home }
	t.Cleanup(func() {
		backupManagerFor = origMgr
		backupHome = origHome
	})
}

func TestBackupsListEmpty(t *testing.T) {
	swapBackupSeams(t, t.TempDir(), t.TempDir())
	var buf bytes.Buffer
	if err := runBackupsList(&buf); err != nil {
		t.Fatalf("err = %v", err)
	}
	if buf.String() != "No backups found.\n" {
		t.Fatalf("out = %q", buf.String())
	}
}

func TestBackupsListShowsNewestFirstWithFormattedSizes(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "session-old")
	newer := filepath.Join(root, "session-new")
	mustWriteFile(t, filepath.Join(older, "f"), strings.Repeat("x", 1024)) // 1.0 KB
	mustWriteFile(t, filepath.Join(newer, "f"), strings.Repeat("x", 2048)) // 2.0 KB
	oldTime := time.Now().Add(-48 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	swapBackupSeams(t, root, t.TempDir())
	var buf bytes.Buffer
	if err := runBackupsList(&buf); err != nil {
		t.Fatalf("err = %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3 (header + 2 rows): %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[1], newer) || !strings.Contains(lines[1], core.FormatSize(2048)) {
		t.Fatalf("row 1 = %q, want the newer session with %s", lines[1], core.FormatSize(2048))
	}
	if !strings.Contains(lines[2], older) || !strings.Contains(lines[2], core.FormatSize(1024)) {
		t.Fatalf("row 2 = %q, want the older session with %s", lines[2], core.FormatSize(1024))
	}
}

func TestBackupsRestoreYesFlagSkipsPromptAndRestores(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	mgr := &backup.Manager{Root: root}
	src := filepath.Join(home, "Documents", "note.txt")
	mustWriteFile(t, src, "hello")
	outcome := mgr.BackupItems(context.Background(), home, []core.CleanableItem{{Path: src, Size: 5, Name: "note.txt"}}, nil)
	if outcome.BackedUp != 1 {
		t.Fatalf("setup: BackedUp = %d, want 1", outcome.BackedUp)
	}

	swapBackupSeams(t, root, home)
	var buf bytes.Buffer
	if err := runBackupsRestore(strings.NewReader(""), &buf, outcome.SessionDir, true); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "Restored: 1") {
		t.Fatalf("out = %q", buf.String())
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("file must be restored to %s: %v", src, err)
	}
}

func TestBackupsRestoreDefaultNoDeclines(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	mgr := &backup.Manager{Root: root}
	src := filepath.Join(home, "Documents", "note.txt")
	mustWriteFile(t, src, "hello")
	outcome := mgr.BackupItems(context.Background(), home, []core.CleanableItem{{Path: src, Size: 5, Name: "note.txt"}}, nil)

	swapBackupSeams(t, root, home)
	var buf bytes.Buffer
	// Bare Enter ("\n") on a default-no prompt must decline.
	if err := runBackupsRestore(strings.NewReader("\n"), &buf, outcome.SessionDir, false); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "Restore cancelled.") {
		t.Fatalf("out = %q, want cancellation on bare Enter (default no)", buf.String())
	}
	if _, err := os.Stat(src); err == nil {
		t.Fatal("file must NOT be restored when the confirm is declined")
	}
}

func TestBackupsCleanOldRemovesSessionsPastRetention(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir() // no config.json here -> config.Load returns Default() (7-day retention)
	oldSession := filepath.Join(root, "old")
	newSession := filepath.Join(root, "new")
	mustMkdirAll(t, oldSession)
	mustMkdirAll(t, newSession)
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	newTime := time.Now().Add(-1 * 24 * time.Hour)
	if err := os.Chtimes(oldSession, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newSession, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	swapBackupSeams(t, root, home)
	var buf bytes.Buffer
	if err := runBackupsCleanOld(&buf); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "Removed 1 old backup session(s).") {
		t.Fatalf("out = %q", buf.String())
	}
	if _, err := os.Stat(oldSession); !os.IsNotExist(err) {
		t.Fatal("old session must be removed")
	}
	if _, err := os.Stat(newSession); err != nil {
		t.Fatal("new session must be kept")
	}
}
