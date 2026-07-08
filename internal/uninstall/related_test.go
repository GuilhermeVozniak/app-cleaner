package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func write(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func xmlInfoPlist(bundleID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
</dict>
</plist>
`, bundleID)
}

func makeApp(t *testing.T, dir, name string, infoPlist []byte) string {
	t.Helper()
	appPath := filepath.Join(dir, name+".app")
	mkdir(t, appPath, "Contents")
	if infoPlist != nil {
		write(t, filepath.Join(appPath, "Contents", "Info.plist"), string(infoPlist))
	}
	return appPath
}

func TestBundleIDFromXMLPlist(t *testing.T) {
	dir := t.TempDir()
	app := makeApp(t, dir, "XmlApp", []byte(xmlInfoPlist("com.example.xmlapp")))
	if got := BundleID(app); got != "com.example.xmlapp" {
		t.Fatalf("BundleID = %q, want com.example.xmlapp", got)
	}
}

func TestBundleIDFromBinaryPlist(t *testing.T) {
	dir := t.TempDir()
	data, err := plist.Marshal(map[string]string{"CFBundleIdentifier": "com.example.binapp"}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	app := makeApp(t, dir, "BinApp", data)
	if got := BundleID(app); got != "com.example.binapp" {
		t.Fatalf("BundleID = %q, want com.example.binapp", got)
	}
}

func TestBundleIDFallback(t *testing.T) {
	dir := t.TempDir()
	// Identifier failing ^[a-zA-Z][a-zA-Z0-9.-]*$ (leading digit + space) → fallback.
	app := makeApp(t, dir, "My Cool App", []byte(xmlInfoPlist("9bad id")))
	if got := BundleID(app); got != "my.cool.app" {
		t.Fatalf("BundleID = %q, want my.cool.app", got)
	}
	// Missing Info.plist → fallback too.
	app2 := makeApp(t, dir, "No Plist", nil)
	if got := BundleID(app2); got != "no.plist" {
		t.Fatalf("BundleID = %q, want no.plist", got)
	}
}

func TestFindRelatedPathsTemplatesVariationsAndGlob(t *testing.T) {
	home := t.TempDir()
	// verbatim {APP}
	appSupport := mkdir(t, home, "Library", "Application Support", "My App")
	write(t, filepath.Join(appSupport, "data.bin"), "1234") // 4 bytes → size assertion
	// whitespace-stripped {APP}
	mkdir(t, home, "Library", "Caches", "MyApp")
	// lowercased {APP}
	mkdir(t, home, "Library", "Logs", "my app")
	// {BID} template
	write(t, filepath.Join(home, "Library", "Preferences", "com.example.myapp.plist"), "x")
	// glob template Group Containers/*.{APP} (whitespace-stripped variation)
	mkdir(t, home, "Library", "Group Containers", "ABC123.MyApp")
	// noise that must NOT match
	mkdir(t, home, "Library", "Group Containers", "ABC123.OtherApp")

	got := FindRelatedPaths("My App", "com.example.myapp", home)

	// Compare case-insensitively: on macOS's case-insensitive default volumes
	// a differently-cased variation resolves to the same directory.
	byLower := map[string]RelatedPath{}
	for _, rp := range got {
		byLower[strings.ToLower(rp.Path)] = rp
	}
	want := []string{
		strings.ToLower(filepath.Join(home, "Library", "Application Support", "My App")),
		strings.ToLower(filepath.Join(home, "Library", "Caches", "MyApp")),
		strings.ToLower(filepath.Join(home, "Library", "Logs", "my app")),
		strings.ToLower(filepath.Join(home, "Library", "Preferences", "com.example.myapp.plist")),
		strings.ToLower(filepath.Join(home, "Library", "Group Containers", "ABC123.MyApp")),
	}
	for _, w := range want {
		if _, ok := byLower[w]; !ok {
			t.Errorf("missing related path %q (got %+v)", w, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %+v", len(got), len(want), got)
	}
	if rp := byLower[want[0]]; rp.Size != 4 {
		t.Fatalf("Application Support size = %d, want 4 (sized at scan time)", rp.Size)
	}
	noise := strings.ToLower(filepath.Join(home, "Library", "Group Containers", "ABC123.OtherApp"))
	if _, ok := byLower[noise]; ok {
		t.Fatal("glob must not match ABC123.OtherApp")
	}
}

func TestIncludableRejectsProtectedAndOutsideHome(t *testing.T) {
	home := t.TempDir()
	if !includable(filepath.Join(home, "Library", "Caches", "X"), home) {
		t.Fatal("in-home unprotected candidate must be includable")
	}
	// Real protected paths can't be created in a temp fixture, so the
	// predicate is exercised directly: /System/… is under home=/System but
	// fsx.IsProtectedPath must veto it.
	if includable("/System/Library/Caches/Foo", "/System") {
		t.Fatal("protected candidate must be rejected even when under home")
	}
	if includable(filepath.Join(home, "..", "escape"), home) {
		t.Fatal("candidate escaping home must be rejected")
	}
}

func TestGlobToRegexpEscapesMetachars(t *testing.T) {
	re, err := globToRegexp("*.My+App(1)")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("group.My+App(1)") {
		t.Fatal("literal metachars must match themselves")
	}
	if re.MatchString("group.MyXApp(1)") {
		t.Fatal("+ must not act as a regex quantifier")
	}
	if !re.MatchString("GROUP.my+app(1)") {
		t.Fatal("match must be case-insensitive")
	}
}
