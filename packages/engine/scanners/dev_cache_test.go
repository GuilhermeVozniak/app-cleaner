package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func devCacheItemsByName(t *testing.T, s Scanner, opts Options) map[string]core.CleanableItem {
	t.Helper()
	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if len(byName) != len(res.Items) {
		t.Fatalf("duplicate item names in %+v", res.Items)
	}
	return byName
}

func TestDevCacheScannerFixedPathsSizeGate(t *testing.T) {
	s, ok := Get("dev-cache")
	if !ok {
		t.Fatal("dev-cache scanner not registered")
	}
	if s.Category() != core.Categories["dev-cache"] {
		t.Fatalf("Category() = %+v, want core.Categories[dev-cache]", s.Category())
	}

	opts := testOptions(t)
	// Nothing exists => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("empty home must yield empty result without error, got %+v", res)
	}

	home := opts.Roots.Home
	mkFile(t, filepath.Join(home, ".npm", "_cacache", "content-v2", "blob"), 2048) // exists, size>0 => in
	mkDir(t, filepath.Join(home, "Library", "Caches", "Yarn"))                     // exists, size 0  => OUT (gate)
	mkFile(t, filepath.Join(home, ".cargo", "registry", "cache", "pkg.crate"), 512)
	// pnpm, pip, CocoaPods, Gradle: absent => out.

	byName := devCacheItemsByName(t, s, opts)
	if len(byName) != 2 {
		t.Fatalf("got %d items (%v), want npm + cargo only", len(byName), byName)
	}
	npm := byName["npm cache"]
	if npm.Size != 2048 || !npm.IsDirectory || npm.Path != filepath.Join(home, ".npm", "_cacache") {
		t.Fatalf("npm cache = %+v; want dir item of 2048 at ~/.npm/_cacache", npm)
	}
	if _, hasYarn := byName["Yarn cache"]; hasYarn {
		t.Fatal("0-byte Yarn cache must be excluded by the size>0 gate")
	}
	if cargo := byName["Cargo cache"]; cargo.Size != 512 {
		t.Fatalf("Cargo cache = %+v; want size 512", cargo)
	}
}

func TestDevCacheScannerXcodeDerivedDataPerChild(t *testing.T) {
	s, _ := Get("dev-cache")
	opts := testOptions(t)
	home := opts.Roots.Home
	dd := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	mkFile(t, filepath.Join(dd, "MyApp-abcdefgh", "Build", "x.o"), 700)
	mkDir(t, filepath.Join(dd, "Empty-00000000"))                              // 0-byte child: NO size gate for DerivedData
	mkDir(t, filepath.Join(home, "Library", "Developer", "Xcode", "Archives")) // empty Archives => out

	byName := devCacheItemsByName(t, s, opts)
	if len(byName) != 2 {
		t.Fatalf("got %d items (%v), want the two DerivedData children", len(byName), byName)
	}
	my := byName["Xcode: MyApp-abcdefgh"]
	if my.Size != 700 || !my.IsDirectory || my.Path != filepath.Join(dd, "MyApp-abcdefgh") {
		t.Fatalf("DerivedData child = %+v; want 'Xcode: MyApp-abcdefgh', size 700", my)
	}
	if empty, ok := byName["Xcode: Empty-00000000"]; !ok || empty.Size != 0 {
		t.Fatalf("0-byte DerivedData child = %+v ok=%v; must still be listed", empty, ok)
	}
	if _, hasArchives := byName["Xcode Archives"]; hasArchives {
		t.Fatal("empty Archives dir must be excluded by the size>0 gate")
	}
}

func TestDevCacheScannerXcodeArchivesSingleItem(t *testing.T) {
	s, _ := Get("dev-cache")
	opts := testOptions(t)
	home := opts.Roots.Home
	ar := filepath.Join(home, "Library", "Developer", "Xcode", "Archives")
	mkFile(t, filepath.Join(ar, "2026-01-01", "App.xcarchive", "Info.plist"), 900)

	byName := devCacheItemsByName(t, s, opts)
	if len(byName) != 1 {
		t.Fatalf("got %d items (%v), want exactly one 'Xcode Archives'", len(byName), byName)
	}
	arch := byName["Xcode Archives"]
	if arch.Size != 900 || !arch.IsDirectory || arch.Path != ar {
		t.Fatalf("Archives = %+v; want ONE dir item of 900 at %s", arch, ar)
	}
}
