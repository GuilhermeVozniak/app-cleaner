package uninstall

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pngFixture stands in for sips output; AppIcon never inspects the bytes,
// it only round-trips them to base64.
var pngFixture = []byte("\x89PNG\r\n\x1a\nfake-png-bytes")

// sipsRunner fakes /usr/bin/sips: it writes pngFixture to the --out path
// and records every call.
type sipsRunner struct {
	t     *testing.T
	calls [][]string
}

func (f *sipsRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.t.Helper()
	f.calls = append(f.calls, append([]string{bin}, args...))
	if timeout != 10*time.Second {
		f.t.Fatalf("timeout = %v, want 10s", timeout)
	}
	outPath := ""
	for i, a := range args {
		if a == "--out" && i+1 < len(args) {
			outPath = args[i+1]
		}
	}
	if outPath == "" {
		f.t.Fatalf("no --out argument in %q", args)
	}
	if err := os.WriteFile(outPath, pngFixture, 0o644); err != nil {
		f.t.Fatal(err)
	}
	return "", nil
}

func xmlIconPlist(iconFile string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.iconapp</string>
	<key>CFBundleIconFile</key>
	<string>%s</string>
</dict>
</plist>
`, iconFile)
}

func TestAppIconConvertsCachesAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "icons") // does not exist yet → AppIcon must MkdirAll it
	// CFBundleIconFile has no extension → ".icns" must be appended.
	app := makeApp(t, dir, "IconApp", []byte(xmlIconPlist("MyIcon")))
	icns := filepath.Join(app, "Contents", "Resources", "MyIcon.icns")
	write(t, icns, "icns-bytes")

	f := &sipsRunner{t: t}
	got := AppIcon(context.Background(), f, app, cacheDir)
	if got == "" {
		t.Fatal("AppIcon = \"\", want base64 PNG for a bundle with a valid .icns")
	}
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil || string(decoded) != string(pngFixture) {
		t.Fatalf("base64 round-trip failed: err = %v, decoded = %q", err, decoded)
	}
	sum := sha1.Sum([]byte(app))
	wantPng := filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".png")
	want := strings.Join([]string{"/usr/bin/sips", "-s", "format", "png", icns, "--out", wantPng}, " ")
	if len(f.calls) != 1 || strings.Join(f.calls[0], " ") != want {
		t.Fatalf("sips calls = %q, want [%q]", f.calls, want)
	}

	// Second call: the cached PNG (sha1-of-bundle-path key) short-circuits sips.
	again := AppIcon(context.Background(), f, app, cacheDir)
	if again != got {
		t.Fatalf("cache hit returned different data: %q vs %q", again, got)
	}
	if len(f.calls) != 1 {
		t.Fatalf("sips calls across two AppIcon calls = %d, want exactly 1 (cache hit)", len(f.calls))
	}
}

func TestAppIconDefaultsToAppIconName(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()
	// No CFBundleIconFile key at all → default name "AppIcon" (+ ".icns").
	app := makeApp(t, dir, "PlainApp", []byte(xmlInfoPlist("com.example.plainapp")))
	write(t, filepath.Join(app, "Contents", "Resources", "AppIcon.icns"), "icns-bytes")

	f := &sipsRunner{t: t}
	if got := AppIcon(context.Background(), f, app, cacheDir); got == "" {
		t.Fatal("missing CFBundleIconFile must fall back to AppIcon.icns")
	}
	if len(f.calls) != 1 || f.calls[0][4] != filepath.Join(app, "Contents", "Resources", "AppIcon.icns") {
		t.Fatalf("sips calls = %q, want the AppIcon.icns source path", f.calls)
	}
}

func TestAppIconMissingIcnsReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()
	app := makeApp(t, dir, "NoIcon", []byte(xmlInfoPlist("com.example.noicon"))) // no Resources/*.icns

	f := &sipsRunner{t: t}
	if got := AppIcon(context.Background(), f, app, cacheDir); got != "" {
		t.Fatalf("AppIcon = %q, want \"\" when the .icns is missing", got)
	}
	if len(f.calls) != 0 {
		t.Fatalf("sips must not run when the .icns is missing: %q", f.calls)
	}
}
