package scanners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// scriptedRunner is a CmdRunner fake: records every call, replays scripted
// responses in order, and errors on unexpected extra calls.
type scriptedCall struct {
	Timeout time.Duration
	Bin     string
	Args    []string
}

type scriptedResponse struct {
	Stdout string
	Err    error
}

type scriptedRunner struct {
	calls     []scriptedCall
	responses []scriptedResponse
}

func (r *scriptedRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	r.calls = append(r.calls, scriptedCall{Timeout: timeout, Bin: bin, Args: args})
	if len(r.responses) == 0 {
		return "", fmt.Errorf("scriptedRunner: unexpected call: %s %v", bin, args)
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	return resp.Stdout, resp.Err
}

// mkExec drops an executable (0755) fake binary and returns its path.
func mkExec(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// hbFixture creates a fake brew binary and a populated allowed cache dir.
func hbFixture(t *testing.T) (brew, home, cache string) {
	t.Helper()
	brew = mkExec(t, t.TempDir(), "brew")
	home = t.TempDir()
	cache = filepath.Join(home, "Library", "Caches", "Homebrew")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "pkg.tar.gz"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	return brew, home, cache
}

func TestFindExecutableOrdering(t *testing.T) {
	dir := t.TempDir()
	second := mkExec(t, dir, "second")
	third := mkExec(t, dir, "third")
	nonExec := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(nonExec, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// First existing+executable candidate wins; missing and non-executable skipped.
	if got := findExecutable([]string{filepath.Join(dir, "missing"), nonExec, second, third}); got != second {
		t.Errorf("findExecutable = %q, want %q", got, second)
	}
	if got := findExecutable([]string{filepath.Join(dir, "nope")}); got != "" {
		t.Errorf("findExecutable = %q, want \"\"", got)
	}
}

func TestHomebrewScan(t *testing.T) {
	brew, home, cache := hbFixture(t)
	r := &scriptedRunner{responses: []scriptedResponse{{Stdout: cache + "\n"}}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}

	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})
	if res.Error != "" || len(res.Items) != 1 {
		t.Fatalf("result = %+v, want exactly one item and no error", res)
	}
	it := res.Items[0]
	if it.Name != "Homebrew Download Cache" || it.Path != cache || !it.IsDirectory || it.Size <= 0 {
		t.Errorf("unexpected item %+v", it)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %+v, want exactly one brew --cache call", r.calls)
	}
	c := r.calls[0]
	if c.Bin != brew || !reflect.DeepEqual(c.Args, []string{"--cache"}) || c.Timeout != 30*time.Second {
		t.Errorf("--cache call = %+v, want [--cache] @ 30s on %s", c, brew)
	}
}

func TestHomebrewScanEmptyCases(t *testing.T) {
	// No brew binary in the allowlist → empty result, no commands run.
	r := &scriptedRunner{}
	s := newHomebrewScanner()
	s.candidates = []string{filepath.Join(t.TempDir(), "missing-brew")}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" || len(r.calls) != 0 {
		t.Fatalf("no-binary case: items=%v error=%q calls=%v, want all empty", res.Items, res.Error, r.calls)
	}

	// Cache path outside the allowlisted roots → warn + empty.
	brew := mkExec(t, t.TempDir(), "brew")
	evil := t.TempDir()
	if err := os.WriteFile(filepath.Join(evil, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = &scriptedRunner{responses: []scriptedResponse{{Stdout: evil + "\n"}}}
	s = newHomebrewScanner()
	s.candidates = []string{brew}
	res = s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 {
		t.Fatalf("out-of-allowlist cache: items = %+v, want none", res.Items)
	}

	// brew --cache failing → empty (scan errors are swallowed).
	r = &scriptedRunner{responses: []scriptedResponse{{Err: errors.New("brew broke")}}}
	s = newHomebrewScanner()
	s.candidates = []string{brew}
	res = s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" {
		t.Fatalf("command-failure case: %+v, want silent empty", res)
	}
}

func TestHomebrewCleanCacheRootUsesBrewCleanup(t *testing.T) {
	brew, home, cache := hbFixture(t)
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: cache + "\n"}, // scan: brew --cache
		{Stdout: cache + "\n"}, // clean: brew --cache (re-resolved)
		{Stdout: "cleaned"},    // clean: brew cleanup --prune=all
	}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if len(cr.Errors) != 0 || cr.CleanedItems != 1 || cr.FreedSpace != res.Items[0].Size {
		t.Fatalf("CleanResult = %+v, want 1 cleaned / freed %d", cr, res.Items[0].Size)
	}
	if cr.Errors == nil {
		t.Fatal("success Errors is nil; must be non-nil so the Wails bridge emits \"errors\":[] not null")
	}
	if len(r.calls) != 3 {
		t.Fatalf("calls = %+v, want 3 (scan --cache, clean --cache, cleanup)", r.calls)
	}
	cleanup := r.calls[2]
	if cleanup.Bin != brew || !reflect.DeepEqual(cleanup.Args, []string{"cleanup", "--prune=all"}) || cleanup.Timeout != 60*time.Second {
		t.Errorf("cleanup call = %+v, want [cleanup --prune=all] @ 60s", cleanup)
	}
	// brew owns the deletion — the scanner must NOT rm the cache itself.
	if _, err := os.Stat(filepath.Join(cache, "pkg.tar.gz")); err != nil {
		t.Errorf("brew-cleanup path deleted files directly: %v", err)
	}
}

func TestHomebrewCleanSubpathFallsBackToDirectDelete(t *testing.T) {
	brew, home, cache := hbFixture(t)
	sub := filepath.Join(cache, "old-download.tar.gz")
	if err := os.WriteFile(sub, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: cache + "\n"}, // scan: brew --cache
		{Stdout: cache + "\n"}, // clean: brew --cache — root NOT among selected items
	}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}
	s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})

	items := []core.CleanableItem{{Path: sub, Size: 10, Name: "old-download.tar.gz"}}
	noop := func(current, total int, item core.CleanableItem) {}
	cr := s.Clean(context.Background(), items, false, noop)
	if cr.CleanedItems != 1 || cr.FreedSpace != 10 || len(cr.Errors) != 0 {
		t.Fatalf("CleanResult = %+v, want direct delete of the subpath", cr)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls = %+v, want 2 (no cleanup invocation)", r.calls)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Errorf("subpath not deleted (stat err = %v)", err)
	}
	// The cache root itself must survive.
	if _, err := os.Stat(cache); err != nil {
		t.Errorf("cache root should still exist: %v", err)
	}
}

func TestHomebrewCleanupFailure(t *testing.T) {
	brew, home, cache := hbFixture(t)
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: cache + "\n"},    // scan
		{Stdout: cache + "\n"},    // clean --cache
		{Err: errors.New("boom")}, // cleanup fails
	}}
	s := newHomebrewScanner()
	s.candidates = []string{brew}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if cr.CleanedItems != 0 || cr.FreedSpace != 0 {
		t.Errorf("CleanResult = %+v, want 0 cleaned / 0 freed on failure", cr)
	}
	if len(cr.Errors) != 1 || cr.Errors[0] != "Homebrew cleanup failed: boom" {
		t.Errorf("Errors = %v, want ['Homebrew cleanup failed: boom']", cr.Errors)
	}
}

func TestHomebrewCleanNoBrewBinaryGracefulPerItemError(t *testing.T) {
	s := newHomebrewScanner() // no Scan() run: brewPath is unset ("")
	s.candidates = []string{filepath.Join(t.TempDir(), "missing-brew")}

	items := []core.CleanableItem{{Path: filepath.Join(t.TempDir(), "nonexistent-file"), Size: 10, Name: "ghost"}}
	cr := s.Clean(context.Background(), items, false, nil)

	if cr.CleanedItems != 0 || cr.FreedSpace != 0 {
		t.Fatalf("CleanResult = %+v, want 0 cleaned / 0 freed (item does not exist)", cr)
	}
	if len(cr.Errors) != 1 {
		t.Fatalf("Errors = %v, want exactly one aggregated per-item error, no panic", cr.Errors)
	}
}

func TestHomebrewCleanDryRun(t *testing.T) {
	r := &scriptedRunner{}
	s := newHomebrewScanner() // no Scan needed: dry-run short-circuits
	items := []core.CleanableItem{{Path: "/anything", Size: 42, Name: "Homebrew Download Cache"}}
	cr := s.Clean(context.Background(), items, true, nil)
	if cr.CleanedItems != 1 || cr.FreedSpace != 42 || len(cr.Errors) != 0 {
		t.Fatalf("dry-run CleanResult = %+v, want full success without commands", cr)
	}
	if cr.Errors == nil {
		t.Fatal("dry-run Errors is nil; must be non-nil so the Wails bridge emits \"errors\":[] not null")
	}
	if len(r.calls) != 0 {
		t.Fatalf("dry run must not execute commands: %v", r.calls)
	}
}
