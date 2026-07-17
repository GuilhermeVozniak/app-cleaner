package scanners

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// Brew binary allowlist, priority order. $PATH is never consulted (hijack
// prevention). macOS-only port: the CLI's linuxbrew path is dropped.
var defaultBrewCandidates = []string{
	"/opt/homebrew/bin/brew", // Apple Silicon
	"/usr/local/bin/brew",    // Intel
}

// findExecutable returns the first candidate that passes an X_OK access
// check, or "" when none does. Shared by the homebrew and docker scanners.
func findExecutable(candidates []string) string {
	for _, p := range candidates {
		if syscall.Access(p, 0x1) == nil { // 0x1 = X_OK
			return p
		}
	}
	return ""
}

// externalDryRun is the dry-run short-circuit shared by the command-backed
// scanners (homebrew, docker): every item counts as cleaned, full size
// credited, no commands executed, no disk IO.
func externalDryRun(cat core.Category, items []core.CleanableItem, progress core.ProgressFunc) core.CleanResult {
	var freed int64
	for i, it := range items {
		if progress != nil {
			progress(i+1, len(items), it)
		}
		freed += it.Size
	}
	// Errors non-nil so the Wails bridge serialises "errors":[] (not null).
	return core.CleanResult{Category: cat, CleanedItems: len(items), FreedSpace: freed, Errors: []string{}}
}

type homebrewScanner struct {
	candidates []string  // injectable in tests
	runner     CmdRunner // cached from the last Scan, reused by Clean
	brewPath   string    // cached from the last Scan
}

func newHomebrewScanner() *homebrewScanner {
	return &homebrewScanner{candidates: defaultBrewCandidates}
}

func init() { register(newHomebrewScanner()) }

func (s *homebrewScanner) Category() core.Category {
	return core.Categories["homebrew"]
}

// brewCacheAllowedRoots lists where `brew --cache` output may point; anything
// else is rejected (command-output allowlist validation, CLI parity).
func brewCacheAllowedRoots(home string) []string {
	return []string{
		filepath.Join(home, "Library", "Caches", "Homebrew"),
		"/opt/homebrew/Caches",
		"/usr/local/Caches",
	}
}

func validBrewCachePath(p, home string) bool {
	resolved := filepath.Clean(p)
	if !filepath.IsAbs(resolved) {
		return false
	}
	for _, root := range brewCacheAllowedRoots(home) {
		if resolved == root || strings.HasPrefix(resolved, root+"/") {
			return true
		}
	}
	return false
}

func (s *homebrewScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	s.runner = opts.Runner
	s.brewPath = findExecutable(s.candidates)
	if s.brewPath == "" {
		return core.ScanResult{Category: s.Category()} // Homebrew not installed
	}

	out, err := opts.Runner.Run(ctx, 30*time.Second, s.brewPath, "--cache")
	if err != nil {
		return core.ScanResult{Category: s.Category()} // scan errors are swallowed
	}
	cachePath := strings.TrimSpace(out)
	if !validBrewCachePath(cachePath, opts.Roots.Home) {
		log.Printf("Unexpected Homebrew cache location: %s", cachePath)
		return core.ScanResult{Category: s.Category()}
	}

	info, err := os.Stat(cachePath)
	if err != nil {
		return core.ScanResult{Category: s.Category()} // cache dir does not exist
	}
	size := fsx.GetSize(cachePath)
	if size <= 0 {
		return core.ScanResult{Category: s.Category()}
	}
	mod := info.ModTime()
	item := core.CleanableItem{
		Path:        cachePath,
		Size:        size,
		Name:        "Homebrew Download Cache",
		IsDirectory: true,
		ModifiedAt:  &mod,
	}
	return core.ScanResult{Category: s.Category(), Items: []core.CleanableItem{item}, TotalSize: size}
}

func (s *homebrewScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	if dryRun {
		return externalDryRun(s.Category(), items, progress)
	}
	if s.brewPath == "" {
		s.brewPath = findExecutable(s.candidates)
	}
	if s.brewPath == "" || s.runner == nil {
		// No brew (or Clean before Scan) → plain filesystem deletion.
		return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
	}

	// Re-resolve the CURRENT cache root, exactly like the CLI's clean() does.
	cacheRoot := ""
	if out, err := s.runner.Run(ctx, 30*time.Second, s.brewPath, "--cache"); err == nil {
		cacheRoot = strings.TrimSpace(out)
	}
	selected := false
	if cacheRoot != "" {
		for _, it := range items {
			if it.Path == cacheRoot {
				selected = true
				break
			}
		}
	}
	if !selected {
		// Cache root not among the selection → direct deletion of what was picked.
		return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
	}

	var freed int64
	for i, it := range items {
		if progress != nil {
			progress(i+1, len(items), it)
		}
		freed += it.Size // freed space = PRE-computed scan sizes, never re-measured
	}
	if _, err := s.runner.Run(ctx, 60*time.Second, s.brewPath, "cleanup", "--prune=all"); err != nil {
		return core.CleanResult{
			Category: s.Category(),
			Errors:   []string{"Homebrew cleanup failed: " + err.Error()},
		}
	}
	return core.CleanResult{Category: s.Category(), CleanedItems: len(items), FreedSpace: freed, Errors: []string{}}
}
