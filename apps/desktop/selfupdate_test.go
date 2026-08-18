package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// updateAssetURL: deterministic, semver-validated — never frontend-supplied.
// ---------------------------------------------------------------------------

func TestUpdateAssetURL(t *testing.T) {
	want := "https://github.com/GuilhermeVozniak/app-cleaner/releases/download/v1.6.0/app-cleaner_1.6.0_darwin_universal.dmg"
	for _, in := range []string{"1.6.0", "v1.6.0"} {
		got, err := updateAssetURL(in)
		if err != nil || got != want {
			t.Fatalf("updateAssetURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "dev", "1.6", "../../etc/passwd", "1.6.0/../evil"} {
		if got, err := updateAssetURL(in); err == nil {
			t.Fatalf("updateAssetURL(%q) = %q, want error", in, got)
		}
	}
}

// ---------------------------------------------------------------------------
// downloadFile: streams to dest, reports byte progress, fails on non-200.
// ---------------------------------------------------------------------------

func TestDownloadFileReportsProgressAndWritesDest(t *testing.T) {
	payload := strings.Repeat("x", 256*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "u.dmg")
	var lastDone, lastTotal int64
	err := downloadFile(context.Background(), srv.Client(), srv.URL, dest, func(done, total int64) {
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatal(err)
	}
	if lastDone != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Fatalf("progress ended at %d/%d, want %d/%d", lastDone, lastTotal, len(payload), len(payload))
	}
	data, err := os.ReadFile(dest)
	if err != nil || len(data) != len(payload) {
		t.Fatalf("dest = %d bytes, %v; want %d", len(data), err, len(payload))
	}
}

func TestDownloadFileNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "u.dmg")
	if err := downloadFile(context.Background(), srv.Client(), srv.URL, dest, nil); err == nil {
		t.Fatal("expected error on 404")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failed download must not leave a partial dest file")
	}
}

// ---------------------------------------------------------------------------
// parseHdiutilMount / findAppBundle / bundlePathFrom: pure helpers.
// ---------------------------------------------------------------------------

func TestParseHdiutilMount(t *testing.T) {
	out := "/dev/disk4          \tGUID_partition_scheme          \t\n" +
		"/dev/disk4s1        \tApple_HFS                      \t/Volumes/App Cleaner 1.6.0\n"
	got, err := parseHdiutilMount(out)
	if err != nil || got != "/Volumes/App Cleaner 1.6.0" {
		t.Fatalf("parseHdiutilMount = %q, %v", got, err)
	}
	if _, err := parseHdiutilMount("no volumes here"); err == nil {
		t.Fatal("expected error when no /Volumes mount is present")
	}
}

func TestFindAppBundle(t *testing.T) {
	dir := t.TempDir()
	if _, err := findAppBundle(dir); err == nil {
		t.Fatal("expected error for a mount with no .app")
	}
	if err := os.MkdirAll(filepath.Join(dir, "App Cleaner.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findAppBundle(dir)
	if err != nil || got != filepath.Join(dir, "App Cleaner.app") {
		t.Fatalf("findAppBundle = %q, %v", got, err)
	}
}

func TestBundlePathFrom(t *testing.T) {
	got, err := bundlePathFrom("/Applications/App Cleaner.app/Contents/MacOS/App Cleaner")
	if err != nil || got != "/Applications/App Cleaner.app" {
		t.Fatalf("bundlePathFrom = %q, %v", got, err)
	}
	if _, err := bundlePathFrom("/usr/local/bin/app-cleaner"); err == nil {
		t.Fatal("non-bundle executable must be refused (dev build)")
	}
	translocated := "/private/var/folders/sq/xx/T/AppTranslocation/ABC-DEF/d/App Cleaner.app/Contents/MacOS/App Cleaner"
	if _, err := bundlePathFrom(translocated); err == nil {
		t.Fatal("translocated app must be refused (swap would not stick)")
	}
}

// ---------------------------------------------------------------------------
// installUpdate orchestration with a scripted runner + real temp dirs.
// ---------------------------------------------------------------------------

// scriptRunner routes each Run call through fn and records the binaries run.
type scriptRunner struct {
	fn    func(bin string, args ...string) (string, error)
	calls []string
}

func (s *scriptRunner) Run(_ context.Context, _ time.Duration, bin string, args ...string) (string, error) {
	s.calls = append(s.calls, bin)
	return s.fn(bin, args...)
}

func (s *scriptRunner) ran(bin string) bool {
	for _, c := range s.calls {
		if c == bin {
			return true
		}
	}
	return false
}

// marker reads the Contents/marker file identifying which bundle sits at path.
func marker(t *testing.T, appPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "marker"))
	if err != nil {
		return ""
	}
	return string(data)
}

func writeBundle(t *testing.T, appPath, tag string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(appPath, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "marker"), []byte(tag), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newInstallFixture prepares an installed "old" bundle and a mounted "new"
// bundle, returning a runner whose ditto actually copies.
func newInstallFixture(t *testing.T, codesignErr error) (r *scriptRunner, appPath, mount string) {
	t.Helper()
	root := t.TempDir()
	appPath = filepath.Join(root, "Applications", "App Cleaner.app")
	writeBundle(t, appPath, "old")
	mount = filepath.Join(root, "mount")
	writeBundle(t, filepath.Join(mount, "App Cleaner.app"), "new")
	r = &scriptRunner{fn: func(bin string, args ...string) (string, error) {
		switch bin {
		case "/usr/bin/hdiutil":
			if args[0] == "attach" {
				return "/dev/disk4s1\tApple_HFS\t" + mount + "\n", nil
			}
			return "", nil // detach
		case "/usr/bin/codesign":
			if codesignErr != nil {
				return "", codesignErr
			}
			return "", nil
		case "/usr/bin/ditto":
			src, dst := args[0], args[1]
			return "", os.CopyFS(dst, os.DirFS(src))
		default:
			return "", fmt.Errorf("unexpected binary %s", bin)
		}
	}}
	return r, appPath, mount
}

func TestInstallUpdateSwapsBundleAndDetaches(t *testing.T) {
	r, appPath, _ := newInstallFixture(t, nil)
	if err := installUpdate(context.Background(), r, "/tmp/u.dmg", appPath); err != nil {
		t.Fatal(err)
	}
	if got := marker(t, appPath); got != "new" {
		t.Fatalf("installed bundle marker = %q, want new", got)
	}
	if !r.ran("/usr/bin/codesign") {
		t.Fatal("must verify the downloaded bundle's signature")
	}
	// no staging or .previous leftovers
	entries, err := os.ReadDir(filepath.Dir(appPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftovers next to the app: %v", entries)
	}
	// detach must run even on success (deferred)
	if n := len(r.calls); r.calls[n-1] != "/usr/bin/hdiutil" {
		t.Fatalf("last call = %s, want hdiutil detach", r.calls[n-1])
	}
}

func TestInstallUpdateBadSignatureLeavesAppUntouched(t *testing.T) {
	r, appPath, _ := newInstallFixture(t, fmt.Errorf("code object is not signed at all"))
	err := installUpdate(context.Background(), r, "/tmp/u.dmg", appPath)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("err = %v, want signature verification failure", err)
	}
	if got := marker(t, appPath); got != "old" {
		t.Fatalf("bundle marker = %q, want old (untouched)", got)
	}
	if !r.ran("/usr/bin/hdiutil") {
		t.Fatal("dmg must still be detached after a failed verify")
	}
}

func TestInstallUpdateSwapFailureRollsBack(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits are ignored")
	}
	r, appPath, _ := newInstallFixture(t, nil)
	parent := filepath.Dir(appPath)
	// Read+exec only: staging (ditto) into the parent must fail.
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	if err := installUpdate(context.Background(), r, "/tmp/u.dmg", appPath); err == nil {
		t.Fatal("expected failure when the install location is not writable")
	}
	_ = os.Chmod(parent, 0o755)
	if got := marker(t, appPath); got != "old" {
		t.Fatalf("bundle marker = %q, want old (rolled back)", got)
	}
}
