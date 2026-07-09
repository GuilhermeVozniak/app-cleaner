package scanners

import (
	"context"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// systemLogsScanner scans ~/Library/Logs ONLY. The CLI also listed /var/log,
// but its own safety layer made every /var/log item undeletable (always
// PROTECTED), so the port drops /var/log from scanning (spec §10.1).
type systemLogsScanner struct{}

func init() { register(systemLogsScanner{}) }

func (systemLogsScanner) Category() core.Category { return core.Categories["system-logs"] }

func (s systemLogsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Logs")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s systemLogsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
