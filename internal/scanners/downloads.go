package scanners

import (
	"context"
	"path/filepath"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

// downloadsScanner lists immediate children of ~/Downloads whose age
// (now − lstat mtime) is >= Cfg.DownloadsDaysOld days. Non-recursive; child
// directories are sized recursively by fsx. No dot-file exclusion (CLI parity).
type downloadsScanner struct{}

func init() { register(downloadsScanner{}) }

func (downloadsScanner) Category() core.Category { return core.Categories["downloads"] }

func (s downloadsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Downloads")
	items := fsx.GetItems(dir, fsx.ItemFilter{MinAgeDays: opts.Cfg.DownloadsDaysOld})
	return newScanResult(s.Category(), items)
}

func (s downloadsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
