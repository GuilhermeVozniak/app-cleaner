package scanners

import (
	"context"
	"os"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// devCachePaths are the fixed single-item developer caches (part 1), relative
// to Roots.Home, in this exact order. Each is included only if it exists AND
// its recursive size is > 0 (CLI parity).
var devCachePaths = []struct {
	name string // item Name, verbatim
	rel  string // path relative to home
}{
	{"npm cache", ".npm/_cacache"},
	{"Yarn cache", "Library/Caches/Yarn"},
	{"pnpm store", "Library/pnpm/store"},
	{"pip cache", ".cache/pip"},
	{"CocoaPods cache", "Library/Caches/CocoaPods"},
	{"Gradle cache", ".gradle/caches"},
	{"Cargo cache", ".cargo/registry"},
}

type devCacheScanner struct{}

func init() { register(devCacheScanner{}) }

func (devCacheScanner) Category() core.Category { return core.Categories["dev-cache"] }

func (s devCacheScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	home := opts.Roots.Home
	var items []core.CleanableItem

	// Part 1: fixed paths, exists AND size > 0.
	for _, c := range devCachePaths {
		p := filepath.Join(home, c.rel)
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		size := fsx.GetSize(p)
		if size <= 0 {
			continue
		}
		mt := fi.ModTime()
		items = append(items, core.CleanableItem{
			Path:        p,
			Size:        size,
			Name:        c.name,
			IsDirectory: true,
			ModifiedAt:  &mt,
		})
	}

	// Part 2: Xcode DerivedData — one item per child (per-project folder),
	// name rewritten to "Xcode: <child>". Deliberately NO size>0 gate.
	dd := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	for _, it := range fsx.GetDirectoryItems(dd) {
		it.Name = "Xcode: " + it.Name
		items = append(items, it)
	}

	// Part 3: Xcode Archives as ONE item, if it exists and size > 0.
	ar := filepath.Join(home, "Library", "Developer", "Xcode", "Archives")
	if fi, err := os.Stat(ar); err == nil {
		if size := fsx.GetSize(ar); size > 0 {
			mt := fi.ModTime()
			items = append(items, core.CleanableItem{
				Path:        ar,
				Size:        size,
				Name:        "Xcode Archives",
				IsDirectory: true,
				ModifiedAt:  &mt,
			})
		}
	}

	return newScanResult(s.Category(), items)
}

func (s devCacheScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
