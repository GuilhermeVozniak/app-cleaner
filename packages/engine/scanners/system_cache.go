package scanners

import (
	"context"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

type systemCacheScanner struct{}

func init() { register(systemCacheScanner{}) }

func (systemCacheScanner) Category() core.Category { return core.Categories["system-cache"] }

func (s systemCacheScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Caches")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s systemCacheScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
