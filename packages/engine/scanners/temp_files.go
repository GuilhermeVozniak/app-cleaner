package scanners

import (
	"context"
	"os"
	"path/filepath"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// tempFilesScanner lists children of Roots.Tmp plus, for every
// Roots.VarFolders/<d1>/<d2>/T directory that exists, the children of that T
// dir. Read/permission errors at any level are silently skipped (on the real
// system many /var/folders entries belong to other users — expected).
type tempFilesScanner struct{}

func init() { register(tempFilesScanner{}) }

func (tempFilesScanner) Category() core.Category { return core.Categories["temp-files"] }

func (s tempFilesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	items := fsx.GetDirectoryItems(opts.Roots.Tmp)
	if level1, err := os.ReadDir(opts.Roots.VarFolders); err == nil {
		for _, d1 := range level1 {
			l2 := filepath.Join(opts.Roots.VarFolders, d1.Name())
			level2, err := os.ReadDir(l2)
			if err != nil {
				continue // non-dir or unreadable entry: silently skipped
			}
			for _, d2 := range level2 {
				tDir := filepath.Join(l2, d2.Name(), "T")
				if fi, err := os.Stat(tDir); err == nil && fi.IsDir() {
					items = append(items, fsx.GetDirectoryItems(tDir)...)
				}
			}
		}
	}
	return newScanResult(s.Category(), items)
}

func (s tempFilesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
