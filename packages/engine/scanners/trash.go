package scanners

import (
	"context"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

type trashScanner struct{}

func init() { register(trashScanner{}) }

func (trashScanner) Category() core.Category { return core.Categories["trash"] }

func (s trashScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, ".Trash")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s trashScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
