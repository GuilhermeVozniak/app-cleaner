package scanners

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// largeFilesScanner walks ~/Downloads and ~/Documents looking for regular
// files >= Cfg.LargeFilesMinSize. CLI-parity depth semantics: the root call
// is depth 0 and the guard is depth > 3, so directories at depth 0..3 are
// read and files up to 4 path components below a root are found. Entries
// whose name starts with '.' are skipped (files and dirs). Symlinks are
// neither regular files nor directories under lstat semantics, so they are
// skipped entirely and never followed.
type largeFilesScanner struct{}

func init() { register(largeFilesScanner{}) }

func (largeFilesScanner) Category() core.Category { return core.Categories["large-files"] }

func (s largeFilesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	var items []core.CleanableItem
	for _, root := range []string{
		filepath.Join(opts.Roots.Home, "Downloads"),
		filepath.Join(opts.Roots.Home, "Documents"),
	} {
		items = append(items, findLargeFiles(root, 0, opts.Cfg.LargeFilesMinSize)...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	return newScanResult(s.Category(), items)
}

// findLargeFiles reads dir (at the given depth relative to the walk root) and
// returns matching regular files. Unreadable dirs and per-entry stat errors
// are silently skipped.
func findLargeFiles(dir string, depth int, minSize int64) []core.CleanableItem {
	if depth > 3 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []core.CleanableItem
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(dir, name)
		if e.IsDir() { // DirEntry.Type() is lstat-based: a symlink to a dir is NOT IsDir
			out = append(out, findLargeFiles(p, depth+1, minSize)...)
			continue
		}
		if !e.Type().IsRegular() { // symlinks, sockets, pipes: skipped
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() < minSize {
			continue
		}
		mt := info.ModTime()
		out = append(out, core.CleanableItem{
			Path:        p,
			Size:        info.Size(),
			Name:        name,
			IsDirectory: false,
			ModifiedAt:  &mt,
		})
	}
	return out
}

func (s largeFilesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
