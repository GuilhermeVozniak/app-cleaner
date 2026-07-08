package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	out   string
	err   error
	calls [][]string
}

func (f *fakeRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{bin}, args...))
	return f.out, f.err
}

// swapRunner replaces the package-level pgrep seam for one test.
func swapRunner(t *testing.T, r Runner) {
	t.Helper()
	orig := runner
	runner = r
	t.Cleanup(func() { runner = orig })
}

func TestIsAppRunning(t *testing.T) {
	f := &fakeRunner{out: "123\n"}
	swapRunner(t, f)
	if !IsAppRunning("/Applications/Foo.app") {
		t.Fatal("want running=true when pgrep prints a pid")
	}
	want := "/usr/bin/pgrep -f /Applications/Foo.app"
	if len(f.calls) != 1 || strings.Join(f.calls[0], " ") != want {
		t.Fatalf("pgrep call = %v, want %q", f.calls, want)
	}

	swapRunner(t, &fakeRunner{err: errors.New("exit status 1")})
	if IsAppRunning("/Applications/Foo.app") {
		t.Fatal("want running=false when pgrep exits non-zero")
	}

	swapRunner(t, &fakeRunner{out: "  \n"})
	if IsAppRunning("/Applications/Foo.app") {
		t.Fatal("want running=false on blank pgrep output")
	}
}

func TestListAppsDiscoversAndSorts(t *testing.T) {
	swapRunner(t, &fakeRunner{err: errors.New("exit status 1")}) // nothing running
	appsDir := t.TempDir()
	home := t.TempDir()

	big := makeApp(t, appsDir, "Big", []byte(xmlInfoPlist("com.example.big")))
	write(t, filepath.Join(big, "Contents", "payload.bin"), strings.Repeat("x", 100))
	small := makeApp(t, appsDir, "Small", []byte(xmlInfoPlist("com.example.small")))
	write(t, filepath.Join(small, "Contents", "payload.bin"), "x")
	write(t, filepath.Join(appsDir, "NotABundle.app"), "plain file") // not a dir → skipped
	mkdir(t, appsDir, "NotAnApp")                                    // no .app suffix → skipped

	apps := ListApps(context.Background(), []string{appsDir, filepath.Join(appsDir, "missing")}, home)
	if len(apps) != 2 {
		t.Fatalf("len(apps) = %d, want 2: %+v", len(apps), apps)
	}
	if apps[0].Name != "Big" || apps[1].Name != "Small" {
		t.Fatalf("order = %s,%s want Big,Small (TotalSize desc)", apps[0].Name, apps[1].Name)
	}
	if apps[0].BundleID != "com.example.big" {
		t.Fatalf("BundleID = %q", apps[0].BundleID)
	}
	if apps[0].Running {
		t.Fatal("Running must be false when pgrep matches nothing")
	}
	if apps[0].AppSize <= apps[1].AppSize {
		t.Fatalf("AppSize ordering wrong: %d <= %d", apps[0].AppSize, apps[1].AppSize)
	}
	if apps[0].Path != big {
		t.Fatalf("Path = %q, want %q", apps[0].Path, big)
	}
}

func TestUninstallFreedSpaceUsesPreRecordedSizes(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "MyApp.app")
	write(t, filepath.Join(bundle, "Contents", "bin"), "1234567890") // 10 real bytes
	related := filepath.Join(base, "Library", "Caches", "MyApp")
	write(t, filepath.Join(related, "c.dat"), "abc") // 3 real bytes

	app := AppInfo{
		Name:         "MyApp",
		Path:         bundle,
		AppSize:      1000, // deliberately != real size: recorded at scan time
		RelatedPaths: []RelatedPath{{Path: related, Size: 5000}},
		TotalSize:    6000,
	}
	var prog []string
	s := Uninstall(context.Background(), []AppInfo{app}, false, func(cur, tot int, name string) {
		prog = append(prog, fmt.Sprintf("%d/%d %s", cur, tot, name))
	})
	if len(s.Errors) != 0 || s.Uninstalled != 1 {
		t.Fatalf("summary = %+v", s)
	}
	if s.FreedSpace != 6000 {
		t.Fatalf("FreedSpace = %d, want 6000 (pre-recorded sizes, never re-measured)", s.FreedSpace)
	}
	if _, err := os.Lstat(bundle); !os.IsNotExist(err) {
		t.Fatal("bundle not removed")
	}
	if _, err := os.Lstat(related); !os.IsNotExist(err) {
		t.Fatal("related path not removed")
	}
	if len(prog) != 1 || prog[0] != "1/1 MyApp" {
		t.Fatalf("progress = %v, want [1/1 MyApp]", prog)
	}
}

func TestUninstallBundleFailureSkipsRelated(t *testing.T) {
	base := t.TempDir()
	related := filepath.Join(base, "Library", "Caches", "GhostApp")
	write(t, filepath.Join(related, "c.dat"), "abc")
	app := AppInfo{
		Name:         "GhostApp",
		Path:         filepath.Join(base, "GhostApp.app"), // never created → ENOENT failure
		AppSize:      100,
		RelatedPaths: []RelatedPath{{Path: related, Size: 3}},
		TotalSize:    103,
	}
	s := Uninstall(context.Background(), []AppInfo{app}, false, nil)
	if s.Uninstalled != 0 || s.FreedSpace != 0 {
		t.Fatalf("summary = %+v", s)
	}
	wantErr := "GhostApp: Failed to remove (security check failed or permission denied)"
	if len(s.Errors) != 1 || s.Errors[0] != wantErr {
		t.Fatalf("Errors = %v, want [%q]", s.Errors, wantErr)
	}
	if _, err := os.Stat(related); err != nil {
		t.Fatal("related paths must be left untouched when the bundle removal fails")
	}
}

func TestUninstallDryRun(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "DryApp.app")
	write(t, filepath.Join(bundle, "Contents", "bin"), "x")
	app := AppInfo{Name: "DryApp", Path: bundle, AppSize: 10, TotalSize: 10}
	s := Uninstall(context.Background(), []AppInfo{app}, true, nil)
	if s.Uninstalled != 1 || s.FreedSpace != 10 || len(s.Errors) != 0 {
		t.Fatalf("summary = %+v", s)
	}
	if _, err := os.Stat(bundle); err != nil {
		t.Fatal("dry run must not touch disk")
	}
}
