// Package backup implements the move-based undo mechanism: before a
// non-dry-run clean, items are os.Rename'd into a timestamped session
// directory and can later be restored, listed, expired, or deleted.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// now is the determinism seam for session naming; tests override it.
var now = time.Now

var sessionNameReplacer = strings.NewReplacer(":", "-", ".", "-")

// Manager owns the backups root:
// <home>/Library/Application Support/AppCleaner/Backups.
type Manager struct{ Root string }

func NewManager(home string) *Manager {
	return &Manager{Root: filepath.Join(home, "Library", "Application Support", "AppCleaner", "Backups")}
}

// Info describes one backup session directory.
type Info struct {
	Path string    `json:"path"`
	Date time.Time `json:"date"`
	Size int64     `json:"size"`
}

// BackupOutcome reports what BackupItems did. Paths in NotBackedUp were NOT
// moved (non-$HOME item, cross-volume EXDEV, or any rename error) — the
// caller permanently deletes those itself. Moved lists the paths actually
// renamed into the backup session. On context cancellation the loop stops
// partway through: items that are neither in Moved nor NotBackedUp were never
// touched and must NOT be credited as cleaned/freed by the caller.
type BackupOutcome struct {
	SessionDir  string   `json:"sessionDir"`
	BackedUp    int      `json:"backedUp"`
	NotBackedUp []string `json:"notBackedUp"`
	Moved       []string `json:"moved"`
}

// RestoreResult reports a Restore run.
type RestoreResult struct {
	Restored int      `json:"restored"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors"`
}

func sessionName() string {
	return sessionNameReplacer.Replace(now().UTC().Format(time.RFC3339))
}

// under reports whether path is strictly inside root (both cleaned/absolute).
func under(path, root string) bool {
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// BackupItems moves each item into a new session directory laid out as
// <session>/HOME/<path relative to home>. Progress fires BEFORE each item,
// 1-based. On context cancellation the loop stops; remaining items are
// neither moved nor added to NotBackedUp.
func (m *Manager) BackupItems(ctx context.Context, home string, items []core.CleanableItem, progress core.ProgressFunc) BackupOutcome {
	out := BackupOutcome{}
	if len(items) == 0 {
		return out
	}
	home = filepath.Clean(home)
	sessionDir := filepath.Join(m.Root, sessionName())
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		for _, it := range items {
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
		}
		return out
	}
	out.SessionDir = sessionDir
	total := len(items)
	for i, it := range items {
		if progress != nil {
			progress(i+1, total, it)
		}
		if ctx.Err() != nil {
			break
		}
		p := filepath.Clean(it.Path)
		if !under(p, home) {
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
			continue
		}
		rel := strings.TrimPrefix(p, home+string(filepath.Separator))
		dest := filepath.Join(sessionDir, "HOME", rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
			continue
		}
		if err := os.Rename(p, dest); err != nil {
			// EXDEV (cross-volume) or any other failure: caller deletes permanently.
			out.NotBackedUp = append(out.NotBackedUp, it.Path)
			continue
		}
		out.BackedUp++
		out.Moved = append(out.Moved, it.Path)
	}
	return out
}

// List returns session directories under Root, newest first by mtime, each
// with its recursive size. Missing/unreadable Root yields nil.
func (m *Manager) List() []Info {
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return nil
	}
	var infos []Info
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(m.Root, e.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		infos = append(infos, Info{Path: p, Date: st.ModTime(), Size: fsx.GetSize(p)})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Date.After(infos[j].Date) })
	return infos
}

// resolveSession validates that sessionDir resolves strictly under Root.
func (m *Manager) resolveSession(sessionDir string) (string, error) {
	root, err := filepath.Abs(m.Root)
	if err != nil {
		return "", err
	}
	sd, err := filepath.Abs(sessionDir)
	if err != nil {
		return "", err
	}
	if !under(sd, root) {
		return "", errors.New("Invalid backup directory: must be within the App Cleaner backups folder")
	}
	return sd, nil
}

// restoreTarget maps a session-relative entry path to its restore
// destination, enforcing the HOME/ prefix, the no-".." rule, and home
// containment.
func restoreTarget(rel, home string) (string, error) {
	if !strings.HasPrefix(rel, "HOME"+string(filepath.Separator)) {
		return "", fmt.Errorf("Skipping file outside HOME structure: %s", rel)
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("Suspicious path pattern detected: %s", rel)
	}
	target := filepath.Join(home, rel[len("HOME/"):])
	abs, err := filepath.Abs(target)
	if err != nil || !under(abs, home) {
		return "", fmt.Errorf("Path traversal detected: %s resolves outside home directory", target)
	}
	return target, nil
}

// Restore moves every file in the session back under home. Directories are
// only traversed (empty dirs are not restored). Per-file failures are
// recorded and the walk continues.
func (m *Manager) Restore(sessionDir, home string) RestoreResult {
	res := RestoreResult{}
	sd, err := m.resolveSession(sessionDir)
	if err != nil {
		res.Failed = 1
		res.Errors = []string{err.Error()}
		return res
	}
	home = filepath.Clean(home)
	_ = filepath.WalkDir(sd, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to read %s: %v", path, werr))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(sd, path)
		if rerr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to resolve %s: %v", path, rerr))
			return nil
		}
		target, terr := restoreTarget(rel, home)
		if terr != nil {
			res.Failed++
			res.Errors = append(res.Errors, terr.Error())
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to restore %s: %v", d.Name(), err))
			return nil
		}
		if err := os.Rename(path, target); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to restore %s: %v", d.Name(), err))
			return nil
		}
		res.Restored++
		return nil
	})
	return res
}

// CleanOld removes session directories whose mtime is older than
// retentionDays days and returns how many were removed.
func (m *Manager) CleanOld(retentionDays int) int {
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return 0
	}
	cutoff := now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(m.Root, e.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.ModTime().Before(cutoff) {
			if err := os.RemoveAll(p); err == nil {
				removed++
			}
		}
	}
	return removed
}

// Delete removes one session directory after containment validation.
func (m *Manager) Delete(sessionDir string) error {
	sd, err := m.resolveSession(sessionDir)
	if err != nil {
		return err
	}
	return os.RemoveAll(sd)
}
