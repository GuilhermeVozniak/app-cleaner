package fsx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func mkItem(path string, size int64) core.CleanableItem {
	return core.CleanableItem{Path: path, Size: size, Name: filepath.Base(path)}
}

func TestRemoveItemsDryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, 10)
	writeFile(t, b, 20)
	items := []core.CleanableItem{
		mkItem(a, 10),
		mkItem(b, 20),
		// dry-run skips even safety validation (CLI parity): counted as cleaned
		mkItem("/System/Library/CoreServices", 999),
	}
	var calls [][2]int
	out := RemoveItems(context.Background(), items, true, func(cur, total int, item core.CleanableItem) {
		calls = append(calls, [2]int{cur, total})
	})
	if out.Cleaned != 3 || out.Freed != 1029 || len(out.Failures) != 0 {
		t.Errorf("dry-run outcome = %+v, want Cleaned=3 Freed=1029 no failures", out)
	}
	for _, p := range []string{a, b} {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("dry-run touched disk: %s is gone", p)
		}
	}
	wantCalls := [][2]int{{1, 3}, {2, 3}, {3, 3}}
	if len(calls) != len(wantCalls) {
		t.Fatalf("progress called %d times, want %d", len(calls), len(wantCalls))
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Errorf("progress call %d = %v, want %v (1-based)", i, calls[i], wantCalls[i])
		}
	}
}

func TestRemoveItemsProgressCalledBeforeDeletion(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeFile(t, a, 10)
	RemoveItems(context.Background(), []core.CleanableItem{mkItem(a, 10)}, false,
		func(cur, total int, item core.CleanableItem) {
			if _, err := os.Lstat(item.Path); err != nil {
				t.Errorf("progress fired after %s was already deleted", item.Path)
			}
		})
	if _, err := os.Lstat(a); !os.IsNotExist(err) {
		t.Errorf("item %s was not deleted", a)
	}
}

func TestRemoveItemsSymlinkRemovesLinkOnly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link")
	writeFile(t, target, 100)
	mustSymlink(t, target, link)
	out := RemoveItems(context.Background(), []core.CleanableItem{mkItem(link, 5)}, false, nil)
	if out.Cleaned != 1 || out.Freed != 5 {
		t.Fatalf("outcome = %+v, want Cleaned=1 Freed=5", out)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("symlink still exists")
	}
	if _, err := os.Lstat(target); err != nil {
		t.Errorf("symlink target was deleted — links must never be followed: %v", err)
	}
}

func TestRemoveItemsDirectoryUsesScanTimeSize(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	mustMkdir(t, filepath.Join(victim, "nested"))
	writeFile(t, filepath.Join(victim, "nested", "f.txt"), 100)
	out := RemoveItems(context.Background(), []core.CleanableItem{mkItem(victim, 12345)}, false, nil)
	if out.Cleaned != 1 || out.Freed != 12345 {
		t.Errorf("outcome = %+v, want Cleaned=1 Freed=12345 (scan-time size, never re-measured)", out)
	}
	if _, err := os.Lstat(victim); !os.IsNotExist(err) {
		t.Errorf("directory %s was not removed recursively", victim)
	}
}

func TestRemoveItemsProtectedPath(t *testing.T) {
	out := RemoveItems(context.Background(),
		[]core.CleanableItem{mkItem("/System/Library/CoreServices", 1)}, false, nil)
	if out.Cleaned != 0 || out.Freed != 0 {
		t.Errorf("outcome = %+v, want nothing cleaned", out)
	}
	if len(out.Failures) != 1 || out.Failures[0].Code != "PROTECTED" {
		t.Fatalf("failures = %+v, want one PROTECTED failure", out.Failures)
	}
	if out.Failures[0].Path != "/System/Library/CoreServices" {
		t.Errorf("failure path = %q, want the item path", out.Failures[0].Path)
	}
}

func TestRemoveItemsMissingPathENOENT(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.txt")
	out := RemoveItems(context.Background(), []core.CleanableItem{mkItem(missing, 10)}, false, nil)
	if out.Cleaned != 0 || len(out.Failures) != 1 {
		t.Fatalf("outcome = %+v, want one failure", out)
	}
	if out.Failures[0].Code != "ENOENT" {
		t.Errorf("code = %q, want ENOENT (re-Lstat found the item gone)", out.Failures[0].Code)
	}
}

func TestRemoveItemsHeterogeneousBatchReportsPerItemFailures(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.txt")
	writeFile(t, ok, 7)
	nonexistent := filepath.Join(dir, "nonexistent")
	items := []core.CleanableItem{
		mkItem(ok, 7),
		mkItem("/System/Library", 0),
		mkItem(nonexistent, 0),
	}
	out := RemoveItems(context.Background(), items, false, nil)
	if out.Cleaned != 1 {
		t.Errorf("Cleaned = %d, want 1", out.Cleaned)
	}
	if len(out.Failures) != 2 {
		t.Fatalf("Failures = %+v, want 2 entries", out.Failures)
	}
	if out.Freed != 7 {
		t.Errorf("Freed = %d, want 7 (only the succeeded item's size)", out.Freed)
	}
	want := []RemoveFailure{
		{Path: "/System/Library", Code: "PROTECTED"},
		{Path: nonexistent, Code: "ENOENT"},
	}
	for i, w := range want {
		if out.Failures[i] != w {
			t.Errorf("Failures[%d] = %+v, want %+v", i, out.Failures[i], w)
		}
	}
}

func TestRemoveItemsCancellationStopsBetweenItems(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 3)
	items := make([]core.CleanableItem, 3)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		writeFile(t, paths[i], 10)
		items[i] = mkItem(paths[i], 10)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := RemoveItems(ctx, items, false, func(cur, total int, item core.CleanableItem) {
		if cur == 2 {
			cancel() // cancel while item 2 is in flight
		}
	})
	if out.Cleaned != 2 {
		t.Errorf("Cleaned = %d, want 2 (in-flight item finishes, later items stop)", out.Cleaned)
	}
	for i := 0; i < 2; i++ {
		if _, err := os.Lstat(paths[i]); !os.IsNotExist(err) {
			t.Errorf("item %d should have been deleted before cancellation", i+1)
		}
	}
	if _, err := os.Lstat(paths[2]); err != nil {
		t.Errorf("item 3 must survive cancellation, got lstat error %v", err)
	}
}

func TestErrnoCode(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&os.PathError{Op: "remove", Path: "/x", Err: syscall.EPERM}, "EPERM"},
		{&os.PathError{Op: "remove", Path: "/x", Err: syscall.EACCES}, "EACCES"},
		{&os.PathError{Op: "lstat", Path: "/x", Err: syscall.ENOENT}, "ENOENT"},
		{&os.PathError{Op: "remove", Path: "/x", Err: syscall.EROFS}, "UNKNOWN"},
		{errors.New("boom"), "UNKNOWN"},
	}
	for _, c := range cases {
		if got := errnoCode(c.err); got != c.want {
			t.Errorf("errnoCode(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestAggregateFailures(t *testing.T) {
	if got := AggregateFailures(nil); got != nil {
		t.Errorf("AggregateFailures(nil) = %v, want nil", got)
	}
	if got := AggregateFailures([]RemoveFailure{}); got != nil {
		t.Errorf("AggregateFailures(empty) = %v, want nil", got)
	}
	mk := func(code string, n int) []RemoveFailure {
		fs := make([]RemoveFailure, n)
		for i := range fs {
			fs[i] = RemoveFailure{Path: fmt.Sprintf("/x/%s-%d", code, i), Code: code}
		}
		return fs
	}
	cases := []struct {
		name     string
		failures []RemoveFailure
		want     string
	}{
		{"single code", mk("ENOENT", 1), "Failed to remove 1 items (1 ENOENT)"},
		{"count desc", append(mk("EACCES", 8), mk("EPERM", 32)...), "Failed to remove 40 items (32 EPERM, 8 EACCES)"},
		{"tie broken alphabetically", append(append(mk("EPERM", 3), mk("EACCES", 3)...), mk("ENOENT", 1)...), "Failed to remove 7 items (3 EACCES, 3 EPERM, 1 ENOENT)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AggregateFailures(c.failures)
			if len(got) != 1 || got[0] != c.want {
				t.Errorf("AggregateFailures = %v, want [%q]", got, c.want)
			}
		})
	}
}
