package scanners

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

const (
	// Root call is depth 0, guard is `depth > max`: dirs at depth 0..5 are read.
	duplicatesMaxDepth = 5
	// Fixed candidate floor (CLI: MIN_FILE_SIZE = 1 MiB). Deliberately NOT tied
	// to Cfg.LargeFilesMinSize — that knob belongs to the large-files scanner.
	duplicatesMinSize = 1 << 20
	// Partial-hash prefix length (spec §10.7 pre-filter).
	duplicatesHashLimit = 1 << 20
)

type dupFileInfo struct {
	path       string
	size       int64
	modifiedAt time.Time
}

type duplicatesScanner struct{}

func newDuplicatesScanner() *duplicatesScanner { return &duplicatesScanner{} }

func init() { register(newDuplicatesScanner()) }

func (s *duplicatesScanner) Category() core.Category {
	return core.Categories["duplicates"]
}

func (s *duplicatesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	roots := []string{
		filepath.Join(opts.Roots.Home, "Downloads"),
		filepath.Join(opts.Roots.Home, "Documents"),
		filepath.Join(opts.Roots.Home, "Desktop"),
	}

	// Phase 1: group candidate files by exact byte size.
	bySize := map[int64][]dupFileInfo{}
	for _, root := range roots {
		collectDupCandidates(ctx, root, 0, bySize)
	}

	var items []core.CleanableItem
	for _, group := range bySize {
		if len(group) < 2 {
			continue
		}
		// Phase 2a: cheap pre-filter — regroup by hash of the first 1 MiB.
		byPartial := map[string][]dupFileInfo{}
		for _, f := range group {
			h, err := fileMD5(f.path, duplicatesHashLimit)
			if err != nil {
				continue // unreadable → drop this file, keep going
			}
			byPartial[h] = append(byPartial[h], f)
		}
		for _, pg := range byPartial {
			if len(pg) < 2 {
				continue
			}
			// Phase 2b: confirm with the full-content hash. (Files exactly
			// limit-sized hash identically in both passes — handled naturally.)
			byFull := map[string][]dupFileInfo{}
			for _, f := range pg {
				h, err := fileMD5(f.path, 0)
				if err != nil {
					continue
				}
				byFull[h] = append(byFull[h], f)
			}
			// Phase 3: newest kept, older copies become items.
			for _, dg := range byFull {
				if len(dg) < 2 {
					continue
				}
				sort.SliceStable(dg, func(i, j int) bool {
					return dg[i].modifiedAt.After(dg[j].modifiedAt)
				})
				newest := filepath.Base(dg[0].path)
				for _, f := range dg[1:] {
					mod := f.modifiedAt
					items = append(items, core.CleanableItem{
						Path:        f.path,
						Size:        f.size,
						Name:        filepath.Base(f.path) + " (dup of " + newest + ")",
						IsDirectory: false,
						ModifiedAt:  &mod,
					})
				}
			}
		}
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].Size > items[j].Size })

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func collectDupCandidates(ctx context.Context, dir string, depth int, bySize map[int64][]dupFileInfo) {
	if depth > duplicatesMaxDepth || ctx.Err() != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // missing root / unreadable dir → silent skip
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
			collectDupCandidates(ctx, full, depth+1, bySize)
		case e.Type().IsRegular(): // lstat-based: symlinks are skipped
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.Size() < duplicatesMinSize {
				continue
			}
			bySize[info.Size()] = append(bySize[info.Size()], dupFileInfo{
				path:       full,
				size:       info.Size(),
				modifiedAt: info.ModTime(),
			})
		}
	}
}

func (s *duplicatesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
