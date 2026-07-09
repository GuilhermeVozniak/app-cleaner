package scanners

import (
	"context"
	"os"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// browserCandidates are the four fixed browser cache locations, relative to
// Roots.Home, checked in this exact order. Each existing directory becomes
// ONE CleanableItem (even at 0 bytes — CLI parity: no size gate).
var browserCandidates = []struct {
	name string // display name; item Name = name + " Cache"
	rel  string // path relative to home
}{
	{"Google Chrome", "Library/Caches/Google/Chrome"},
	{"Safari", "Library/Caches/com.apple.Safari"},
	{"Firefox", "Library/Caches/Firefox/Profiles"},
	{"Arc", "Library/Caches/company.thebrowser.Browser"},
}

type browserCacheScanner struct{}

func init() { register(browserCacheScanner{}) }

func (browserCacheScanner) Category() core.Category { return core.Categories["browser-cache"] }

func (s browserCacheScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	var items []core.CleanableItem
	for _, b := range browserCandidates {
		p := filepath.Join(opts.Roots.Home, b.rel)
		fi, err := os.Stat(p)
		if err != nil || !fi.IsDir() {
			continue // missing/unreadable browser silently omitted
		}
		mt := fi.ModTime()
		items = append(items, core.CleanableItem{
			Path:        p,
			Size:        fsx.GetSize(p),
			Name:        b.name + " Cache",
			IsDirectory: true,
			ModifiedAt:  &mt,
		})
	}
	return newScanResult(s.Category(), items)
}

func (s browserCacheScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
