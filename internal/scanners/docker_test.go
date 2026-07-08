package scanners

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestDefaultCandidateAllowlists(t *testing.T) {
	wantBrew := []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"}
	if !reflect.DeepEqual(defaultBrewCandidates, wantBrew) {
		t.Errorf("defaultBrewCandidates = %v, want %v", defaultBrewCandidates, wantBrew)
	}
	wantDocker := []string{
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
		"/Applications/Docker.app/Contents/Resources/bin/docker",
	}
	if !reflect.DeepEqual(defaultDockerCandidates, wantDocker) {
		t.Errorf("defaultDockerCandidates = %v, want %v", defaultDockerCandidates, wantDocker)
	}
}

func TestParseDockerSize(t *testing.T) {
	cases := map[string]int64{
		"1.5GB":         1500000000, // SI/decimal — spec §10.5 fixes the CLI's 1024 bug
		"250.5MB (62%)":  250500000,
		"2kB":            2000,
		"2KB":            2000,
		"1TB":            1000000000000,
		"512B":           512,
		"0B":             0,
		"N/A":            0,
	}
	for in, want := range cases {
		if got := parseDockerSize(in); got != want {
			t.Errorf("parseDockerSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestDockerScanParsesDf(t *testing.T) {
	docker := mkExec(t, t.TempDir(), "docker")
	df := "Images\t3.2GB\t1.5GB (46%)\n" +
		"Containers\t400MB\t0B (0%)\n" + // zero reclaimable → skipped
		"Local Volumes\t120GB\t120GB (100%)\n" + // EXCLUDED: prune -af never frees volumes
		"Build Cache\t250.5kB\t250.5kB\n" +
		"Evil Injection\t9GB\t9GB\n" // unknown type → dropped (injection guard)
	r := &scriptedRunner{responses: []scriptedResponse{{Stdout: df}}}
	s := newDockerScanner()
	s.candidates = []string{docker}

	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if res.Error != "" || len(res.Items) != 2 {
		t.Fatalf("result = %+v, want images + build cache only", res)
	}

	byPath := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byPath[it.Path] = it
		if it.IsDirectory {
			t.Errorf("%s: IsDirectory = true, want false", it.Path)
		}
	}
	img := byPath["docker:images"]
	if img.Size != 1500000000 || img.Name != "Docker Images" {
		t.Errorf("images item = %+v, want size 1500000000 / name 'Docker Images'", img)
	}
	bc := byPath["docker:build-cache"]
	if bc.Size != 250500 || bc.Name != "Docker Build Cache" {
		t.Errorf("build-cache item = %+v, want size 250500 / name 'Docker Build Cache'", bc)
	}
	if res.TotalSize != 1500250500 {
		t.Errorf("TotalSize = %d, want 1500250500", res.TotalSize)
	}

	if len(r.calls) != 1 {
		t.Fatalf("calls = %+v, want one df call", r.calls)
	}
	wantArgs := []string{"system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}"}
	if r.calls[0].Bin != docker || !reflect.DeepEqual(r.calls[0].Args, wantArgs) || r.calls[0].Timeout != 30*time.Second {
		t.Errorf("df call = %+v, want args %v @ 30s", r.calls[0], wantArgs)
	}
}

func TestDockerScanSilentlyEmpty(t *testing.T) {
	// No binary in the allowlist → empty, zero commands.
	r := &scriptedRunner{}
	s := newDockerScanner()
	s.candidates = []string{filepath.Join(t.TempDir(), "missing-docker")}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" || len(r.calls) != 0 {
		t.Fatalf("no-binary case: %+v / calls %v, want silent empty", res, r.calls)
	}

	// Daemon down (command error) → empty, still NO ScanResult.Error.
	docker := mkExec(t, t.TempDir(), "docker")
	r = &scriptedRunner{responses: []scriptedResponse{{Err: errors.New("Cannot connect to the Docker daemon")}}}
	s = newDockerScanner()
	s.candidates = []string{docker}
	res = s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})
	if len(res.Items) != 0 || res.Error != "" {
		t.Fatalf("daemon-down case: %+v, want silent empty", res)
	}
}

func TestDockerCleanPrune(t *testing.T) {
	docker := mkExec(t, t.TempDir(), "docker")
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: "Images\t3.2GB\t1.5GB (46%)\n"}, // scan df
		{Stdout: "Total reclaimed space: 1.5GB"}, // prune
	}}
	s := newDockerScanner()
	s.candidates = []string{docker}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if cr.CleanedItems != 1 || cr.FreedSpace != 1500000000 || len(cr.Errors) != 0 {
		t.Fatalf("CleanResult = %+v, want 1 cleaned / freed 1500000000", cr)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls = %+v, want df + prune", r.calls)
	}
	prune := r.calls[1]
	wantArgs := []string{"system", "prune", "-af"} // NEVER --volumes
	if prune.Bin != docker || !reflect.DeepEqual(prune.Args, wantArgs) || prune.Timeout != 60*time.Second {
		t.Errorf("prune call = %+v, want args %v @ 60s", prune, wantArgs)
	}
	for _, a := range prune.Args {
		if a == "--volumes" {
			t.Fatal("prune must never be called with --volumes")
		}
	}
}

func TestDockerCleanFailureAndDryRun(t *testing.T) {
	docker := mkExec(t, t.TempDir(), "docker")
	r := &scriptedRunner{responses: []scriptedResponse{
		{Stdout: "Images\t3.2GB\t1.5GB (46%)\n"},
		{Err: errors.New("daemon down")},
	}}
	s := newDockerScanner()
	s.candidates = []string{docker}
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}, Runner: r})

	cr := s.Clean(context.Background(), res.Items, false, nil)
	if cr.CleanedItems != 0 || cr.FreedSpace != 0 {
		t.Errorf("CleanResult = %+v, want 0 cleaned / 0 freed on failure", cr)
	}
	if len(cr.Errors) != 1 || cr.Errors[0] != "Docker cleanup failed: daemon down" {
		t.Errorf("Errors = %v, want ['Docker cleanup failed: daemon down']", cr.Errors)
	}

	// Dry run: no commands, everything credited.
	r2 := &scriptedRunner{}
	s2 := newDockerScanner()
	cr = s2.Clean(context.Background(), []core.CleanableItem{{Path: "docker:images", Size: 7, Name: "Docker Images"}}, true, nil)
	if cr.CleanedItems != 1 || cr.FreedSpace != 7 || len(cr.Errors) != 0 {
		t.Fatalf("dry-run CleanResult = %+v, want full success", cr)
	}
	if len(r2.calls) != 0 {
		t.Fatalf("dry run must not execute commands: %v", r2.calls)
	}
}

// TestAllScannersRegistered is the final gate of the scanner tasks: homebrew
// and docker are the last two scanner files, so once their init() has run the
// package registry must be complete — all 16 categories, in display order.
func TestAllScannersRegistered(t *testing.T) {
	all := All()
	if len(all) != 16 {
		t.Fatalf("len(All()) = %d, want 16 registered scanners", len(all))
	}
	cats := core.CategoriesInOrder()
	if len(cats) != len(all) {
		t.Fatalf("core.CategoriesInOrder() returned %d categories, All() returned %d scanners", len(cats), len(all))
	}
	for i := range all {
		if got, want := all[i].Category().ID, cats[i].ID; got != want {
			t.Errorf("All()[%d].Category().ID = %q, want %q — All() must follow core.CategoriesInOrder()", i, got, want)
		}
	}
}
