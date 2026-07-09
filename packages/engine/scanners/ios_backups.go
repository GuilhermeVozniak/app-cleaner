package scanners

import (
	"context"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

type iosBackupsScanner struct{}

func init() { register(iosBackupsScanner{}) }

func (iosBackupsScanner) Category() core.Category { return core.Categories["ios-backups"] }

func (s iosBackupsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Application Support", "MobileSync", "Backup")
	items := fsx.GetDirectoryItems(dir)
	for i := range items {
		// CLI parity: name = "iOS Backup: " + dirName.substring(0,8) + "..."
		// (whole name when shorter than 8 chars; UDIDs are ASCII hex).
		udid := items[i].Name
		if len(udid) > 8 {
			udid = udid[:8]
		}
		items[i].Name = "iOS Backup: " + udid + "..."
	}
	return newScanResult(s.Category(), items)
}

func (s iosBackupsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
