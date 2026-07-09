package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/scanners"

	"github.com/spf13/cobra"
)

func resetCleanFlags(t *testing.T) {
	t.Helper()
	origCats, origDry, origYes, origBackup, origNoBackup, origJSON := cleanCategories, cleanDryRun, cleanYes, cleanBackup, cleanNoBackup, cleanJSON
	cleanCategories, cleanDryRun, cleanYes, cleanBackup, cleanNoBackup, cleanJSON = nil, false, false, false, false, false
	t.Cleanup(func() {
		cleanCategories, cleanDryRun, cleanYes, cleanBackup, cleanNoBackup, cleanJSON = origCats, origDry, origYes, origBackup, origNoBackup, origJSON
	})
}

// newCleanTestCmd builds a throwaway *cobra.Command carrying only the
// backup/no-backup bool flags resolveBackupFlag inspects. Integration
// tests use this instead of the package-singleton cleanCmd because
// pflag's Flags().Changed() is sticky (never resets to false once Set),
// which would leak --backup/--no-backup state across test cases sharing
// one FlagSet.
func newCleanTestCmd() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().Bool("backup", false, "")
	c.Flags().Bool("no-backup", false, "")
	return c
}

func writeTempFile(t *testing.T, dir, name string, size int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, bytes.Repeat([]byte{'a'}, size), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	return path
}

func TestResolveBackupFlagPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		backup     bool
		noBackup   bool
		cfgDefault bool
		want       bool
	}{
		{"neither set falls back to config default true", false, false, true, true},
		{"neither set falls back to config default false", false, false, false, false},
		{"explicit --backup wins over config default false", true, false, false, true},
		{"explicit --no-backup wins over config default true", false, true, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := newCleanTestCmd()
			if c.backup {
				if err := cmd.Flags().Set("backup", "true"); err != nil {
					t.Fatalf("Set(backup) error = %v", err)
				}
			}
			if c.noBackup {
				if err := cmd.Flags().Set("no-backup", "true"); err != nil {
					t.Fatalf("Set(no-backup) error = %v", err)
				}
			}
			got, err := resolveBackupFlag(cmd, c.cfgDefault)
			if err != nil {
				t.Fatalf("resolveBackupFlag() error = %v", err)
			}
			if got != c.want {
				t.Fatalf("resolveBackupFlag() = %v, want %v", got, c.want)
			}
		})
	}

	t.Run("both set is an error", func(t *testing.T) {
		cmd := newCleanTestCmd()
		_ = cmd.Flags().Set("backup", "true")
		_ = cmd.Flags().Set("no-backup", "true")
		if _, err := resolveBackupFlag(cmd, true); err == nil {
			t.Fatal("resolveBackupFlag() error = nil, want a mutually-exclusive error")
		}
	})
}

func TestCleanCmdDryRunNeverTouchesDiskOrBackup(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.log", 100)

	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 100, Name: "cache.log"}}, TotalSize: 100}},
			TotalSize:  100,
			TotalItems: 1,
		}
	})
	cleanDryRun = true

	cmd := newCleanTestCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("os.Stat(%s) error = %v, want the file untouched by a dry run", filePath, err)
	}
	backupRoot := filepath.Join(dir, "Library", "Application Support", "AppCleaner", "Backups")
	if _, err := os.Stat(backupRoot); !os.IsNotExist(err) {
		t.Fatalf("backup dir exists after a dry run (err=%v), want it never created", err)
	}
	text := buf.String()
	if !strings.Contains(text, "[DRY RUN]") {
		t.Fatalf("output = %q, want the [DRY RUN] prefix", text)
	}
	if !strings.Contains(text, core.FormatSize(100)) {
		t.Fatalf("output = %q, want the formatted would-free size", text)
	}
}

func TestCleanCmdDeclinedConfirmDoesNothing(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.log", 50)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 50, Name: "cache.log"}}, TotalSize: 50}},
			TotalSize:  50,
			TotalItems: 1,
		}
	})

	cmd := newCleanTestCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader("n\n"))

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file removed despite a declined confirm: %v", err)
	}
	if !strings.Contains(buf.String(), "Cleaning cancelled.") {
		t.Fatalf("output = %q, want the cancellation message", buf.String())
	}
}

func TestCleanCmdYesNoBackupDeletesDirectly(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.log", 200)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 200, Name: "cache.log"}}, TotalSize: 200}},
			TotalSize:  200,
			TotalItems: 1,
		}
	})
	cleanYes = true

	cmd := newCleanTestCmd()
	if err := cmd.Flags().Set("no-backup", "true"); err != nil {
		t.Fatalf("Set(no-backup) error = %v", err)
	}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("file still exists after clean (err=%v)", err)
	}
	text := buf.String()
	if !strings.Contains(text, "✓ Trash:") || !strings.Contains(text, core.FormatSize(200)) {
		t.Fatalf("output = %q, want a Trash freed line with the formatted size", text)
	}
}

func TestCleanCmdBackupMovesFileAndReportsNoWarnings(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.log", 300)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 300, Name: "cache.log"}}, TotalSize: 300}},
			TotalSize:  300,
			TotalItems: 1,
		}
	})
	cleanYes = true

	cmd := newCleanTestCmd()
	if err := cmd.Flags().Set("backup", "true"); err != nil {
		t.Fatalf("Set(backup) error = %v", err)
	}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("original file still present after a backup move (err=%v)", err)
	}
	backupRoot := filepath.Join(dir, "Library", "Application Support", "AppCleaner", "Backups")
	entries, err := os.ReadDir(backupRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backup session dirs = %v (err %v), want exactly 1", entries, err)
	}
	text := buf.String()
	if !strings.Contains(text, "✓ Trash:") || !strings.Contains(text, core.FormatSize(300)) {
		t.Fatalf("output = %q, want a Trash freed line crediting the moved item", text)
	}
	if strings.Contains(text, "Warning:") {
		t.Fatalf("output = %q, want no notBackedUp warning when the move succeeds", text)
	}
}

func TestCleanCmdNeverBackupCategoryDeletesDirectlyEvenWithBackupOn(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.tar", 400)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["homebrew"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 400, Name: "cache.tar"}}, TotalSize: 400}},
			TotalSize:  400,
			TotalItems: 1,
		}
	})
	cleanYes = true

	cmd := newCleanTestCmd()
	if err := cmd.Flags().Set("backup", "true"); err != nil {
		t.Fatalf("Set(backup) error = %v", err)
	}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("homebrew item still present (err=%v)", err)
	}
	backupRoot := filepath.Join(dir, "Library", "Application Support", "AppCleaner", "Backups")
	if _, err := os.Stat(backupRoot); !os.IsNotExist(err) {
		t.Fatalf("backup dir was created for a never-backup category (err=%v)", err)
	}
}

func TestCleanCmdJSONShape(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.log", 64)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 64, Name: "cache.log"}}, TotalSize: 64}},
			TotalSize:  64,
			TotalItems: 1,
		}
	})
	cleanYes = true
	cleanJSON = true

	cmd := newCleanTestCmd()
	if err := cmd.Flags().Set("no-backup", "true"); err != nil {
		t.Fatalf("Set(no-backup) error = %v", err)
	}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}

	// clean --json uses Task 3's canonical shape: results[] (not categories[]),
	// and no dryRun/notBackedUp fields (those appear only in the human report).
	var got struct {
		TotalFreedSpace   int64 `json:"totalFreedSpace"`
		TotalCleanedItems int   `json:"totalCleanedItems"`
		TotalErrors       int   `json:"totalErrors"`
		Results           []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			CleanedItems int      `json:"cleanedItems"`
			FreedSpace   int64    `json:"freedSpace"`
			Errors       []string `json:"errors"`
		} `json:"results"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", buf.String(), err)
	}
	if got.TotalFreedSpace != 64 || got.TotalCleanedItems != 1 || got.TotalErrors != 0 {
		t.Fatalf("got = %+v, want freed=64 cleaned=1 errors=0", got)
	}
	if len(got.Results) != 1 || got.Results[0].ID != "trash" || got.Results[0].FreedSpace != 64 {
		t.Fatalf("results = %+v, want one trash entry with freedSpace=64", got.Results)
	}
}

// TestCleanCmdJSONWithoutYesErrorsAndWritesNothing guards the safety-critical
// fix: `clean --json` with neither --yes nor --dry-run must NOT prompt (a human
// y/N prompt on stdout would corrupt the JSON stream) and must NOT auto-delete.
// It must return a non-nil error (routed to stderr / non-zero exit) and write
// nothing to stdout.
func TestCleanCmdJSONWithoutYesErrorsAndWritesNothing(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	filePath := writeTempFile(t, dir, "cache.log", 64)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: filePath, Size: 64, Name: "cache.log"}}, TotalSize: 64}},
			TotalSize:  64,
			TotalItems: 1,
		}
	})
	cleanJSON = true // no --yes, no --dry-run

	cmd := newCleanTestCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	err := runClean(cmd, nil)
	if err == nil {
		t.Fatal("clean --json without --yes/--dry-run must return a non-nil error")
	}
	if buf.Len() != 0 {
		t.Fatalf("clean --json without --yes must write nothing to stdout (no prompt, no partial JSON), got %q", buf.String())
	}
	if _, statErr := os.Stat(filePath); statErr != nil {
		t.Fatalf("clean --json without --yes must not delete anything, stat err = %v", statErr)
	}
}

func TestCleanCmdReportsErrnoBreakdownOnFailure(t *testing.T) {
	resetCleanFlags(t)
	dir := withTempHome(t)
	missing := filepath.Join(dir, "already-gone.log")
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		cat := core.Categories["trash"]
		return core.ScanSummary{
			Results:    []core.ScanResult{{Category: cat, Items: []core.CleanableItem{{Path: missing, Size: 10, Name: "already-gone.log"}}, TotalSize: 10}},
			TotalSize:  10,
			TotalItems: 1,
		}
	})
	cleanYes = true

	cmd := newCleanTestCmd()
	if err := cmd.Flags().Set("no-backup", "true"); err != nil {
		t.Fatalf("Set(no-backup) error = %v", err)
	}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})

	if err := runClean(cmd, nil); err != nil {
		t.Fatalf("runClean() error = %v", err)
	}
	text := buf.String()
	if !strings.Contains(text, "✗ Trash:") || !strings.Contains(text, "ENOENT") {
		t.Fatalf("output = %q, want an errno-breakdown line mentioning ENOENT", text)
	}
	if !strings.Contains(text, "Errors: 1") {
		t.Fatalf("output = %q, want Errors: 1", text)
	}
}
