package scanners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
)

// mkFile creates a file at path (creating parent dirs) filled with size bytes.
func mkFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkDir creates a directory (with parents).
func mkDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// testRoots returns Roots pointing at fresh temp dirs. Only Home exists on
// disk; Tmp/VarFolders/Applications are created lazily by tests that need them.
func testRoots(t *testing.T) Roots {
	t.Helper()
	base := t.TempDir()
	r := Roots{
		Home:         filepath.Join(base, "home"),
		Tmp:          filepath.Join(base, "tmp"),
		VarFolders:   filepath.Join(base, "varfolders"),
		Applications: filepath.Join(base, "apps"),
	}
	mkDir(t, r.Home)
	return r
}

// testOptions returns Options wired to temp roots and the default config.
func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{Roots: testRoots(t), Cfg: config.Default()}
}

// registerFake registers a scanner for the duration of one test. Fake
// category ids must not collide with the 16 real ones (use "fake-*").
func registerFake(t *testing.T, s Scanner) {
	t.Helper()
	id := s.Category().ID
	if _, exists := registry[id]; exists {
		t.Fatalf("scanner %s already registered", id)
	}
	registry[id] = s
	t.Cleanup(func() { delete(registry, id) })
}
