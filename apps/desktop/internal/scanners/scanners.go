// Package scanners implements the 16 cleaning-category scanners and the
// parallel scan runner. Engine-only: never imports Wails.
package scanners

import (
	"context"
	"os"

	"github.com/guhcostan/app-cleaner/internal/config"
	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// Roots is the filesystem test seam: every scanner derives its paths from
// these roots instead of hardcoding absolute paths.
type Roots struct {
	Home         string // default os.UserHomeDir()
	Tmp          string // "/tmp"
	VarFolders   string // "/private/var/folders"
	Applications string // "/Applications"
}

// DefaultRoots returns the real macOS roots.
func DefaultRoots() Roots {
	home, _ := os.UserHomeDir()
	return Roots{
		Home:         home,
		Tmp:          "/tmp",
		VarFolders:   "/private/var/folders",
		Applications: "/Applications",
	}
}

// Options carries everything a scanner may need. The same value is passed to
// every scanner in a batch run.
type Options struct {
	Roots  Roots
	Cfg    config.Config
	Runner CmdRunner
}

// Scanner is implemented once per category.
type Scanner interface {
	Category() core.Category
	Scan(ctx context.Context, opts Options) core.ScanResult
	Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult
}

// registry maps category id -> singleton scanner. Populated by register()
// from each scanner file's init(); grows as scanner tasks land. The final
// scanner task asserts len(All()) == 16.
var registry = map[core.CategoryID]Scanner{}

func register(s Scanner) { registry[s.Category().ID] = s }

// Get returns the scanner registered for id.
func Get(id core.CategoryID) (Scanner, bool) {
	s, ok := registry[id]
	return s, ok
}

// All returns every registered scanner in stable display order
// (core.CategoriesInOrder()).
func All() []Scanner {
	var out []Scanner
	for _, c := range core.CategoriesInOrder() {
		if s, ok := registry[c.ID]; ok {
			out = append(out, s)
		}
	}
	return out
}

// newScanResult assembles a ScanResult with TotalSize = sum of item sizes.
// Items is never nil so the frontend always receives a JSON array.
func newScanResult(cat core.Category, items []core.CleanableItem) core.ScanResult {
	if items == nil {
		items = []core.CleanableItem{}
	}
	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: cat, Items: items, TotalSize: total}
}

// cleanWithFsx is the shared Clean implementation for every filesystem-backed
// scanner: sequential fsx.RemoveItems + one aggregated error string.
func cleanWithFsx(cat core.Category, ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	out := fsx.RemoveItems(ctx, items, dryRun, progress)
	errs := fsx.AggregateFailures(out.Failures)
	if errs == nil {
		errs = []string{}
	}
	return core.CleanResult{
		Category:     cat,
		CleanedItems: out.Cleaned,
		FreedSpace:   out.Freed,
		Errors:       errs,
	}
}
