package scanners

import (
	"context"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

type mailAttachmentsScanner struct{}

func init() { register(mailAttachmentsScanner{}) }

func (mailAttachmentsScanner) Category() core.Category {
	return core.Categories["mail-attachments"]
}

func (s mailAttachmentsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "Containers", "com.apple.mail",
		"Data", "Library", "Mail Downloads")
	return newScanResult(s.Category(), fsx.GetDirectoryItems(dir))
}

func (s mailAttachmentsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
