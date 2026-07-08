// Package fsx is App Cleaner's safe filesystem layer: path-safety
// validation, logical sizing, directory listing, and the deletion engine.
// It never imports Wails, never spawns processes, and its walk/size/list
// functions never return errors (unreadable = skip / size 0).
package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowedPaths are always deletable, even though some sit under otherwise
// protected prefixes (e.g. /var/folders). Checked BEFORE protectedPaths.
var allowedPaths = []string{
	"/tmp",
	"/private/tmp",
	"/var/tmp",
	"/private/var/tmp",
	"/var/folders",
	"/private/var/folders",
}

// protectedPaths are never deletable (exact match or prefix + "/").
var protectedPaths = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/etc",
	"/var/log",
	"/var/db",
	"/var/root",
	"/private/var/db",
	"/private/var/root",
	"/private/var/log",
	"/Library/Apple",
	"/Applications/Utilities",
}

// homeDir is the current user's home directory, resolved once at package
// initialization. It is deliberately a package variable rather than an
// os.UserHomeDir() call at check time: same-package tests reassign it to a
// fake home (see setTestHome in safety_test.go). Production code never
// mutates it. Empty string (home unknown) disables only the exact-home check.
var homeDir = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}()

// normalizePath mirrors Node's path.resolve(): absolutize against the
// current working directory and collapse "."/".." segments. It must NOT
// follow symlinks (never use filepath.EvalSymlinks here).
func normalizePath(path string) string {
	abs, err := filepath.Abs(path) // Abs also Cleans
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

// matchesRoot reports whether p equals root or lives under root (root+"/"
// prefix — so "/usr2" never matches root "/usr").
func matchesRoot(p, root string) bool {
	return p == root || strings.HasPrefix(p, root+"/")
}

// IsProtectedPath reports whether path falls under a protected system root.
// Allowed-path overrides are checked first: anything under /tmp, /var/tmp,
// or /var/folders (and their /private twins) is never protected.
func IsProtectedPath(path string) bool {
	p := normalizePath(path)
	for _, a := range allowedPaths {
		if matchesRoot(p, a) {
			return false
		}
	}
	for _, pr := range protectedPaths {
		if matchesRoot(p, pr) {
			return true
		}
	}
	return false
}

// ValidatePathSafety returns "" when path is safe to delete, or a
// human-readable refusal reason. Checks run on the cleaned absolute path;
// messages echo the caller's original path string (CLI parity).
func ValidatePathSafety(path string) string {
	p := normalizePath(path)
	if IsProtectedPath(p) {
		return fmt.Sprintf("Refusing to delete protected system path: %s", path)
	}
	if p == "/" {
		return "Refusing to delete root directory"
	}
	if homeDir != "" && p == filepath.Clean(homeDir) {
		return "Refusing to delete home directory"
	}
	return ""
}
