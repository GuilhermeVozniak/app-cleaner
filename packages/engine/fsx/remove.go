package fsx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// RemoveFailure records one item that could not be deleted.
// Code is one of "PROTECTED", "EPERM", "EACCES", "ENOENT", "UNKNOWN" —
// callers key on these strings (e.g. the UI's Full Disk Access hint).
type RemoveFailure struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// RemoveOutcome summarizes one RemoveItems batch.
type RemoveOutcome struct {
	Cleaned  int             `json:"cleaned"`
	Freed    int64           `json:"freed"`
	Failures []RemoveFailure `json:"failures"`
}

// RemoveItems permanently deletes items sequentially. progress (may be nil)
// fires BEFORE each item with a 1-based index. dryRun counts every item as
// cleaned with its full scan-time size and performs zero disk IO. Freed
// space always credits the scan-time item.Size — never re-measured.
// Context cancellation stops between items: the in-flight item completes,
// remaining items are left untouched.
func RemoveItems(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) RemoveOutcome {
	var out RemoveOutcome
	total := len(items)
	for i, item := range items {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress(i+1, total, item)
		}
		if dryRun {
			out.Cleaned++
			out.Freed += item.Size
			continue
		}
		if code := removeOne(item.Path); code != "" {
			out.Failures = append(out.Failures, RemoveFailure{Path: item.Path, Code: code})
			continue
		}
		out.Cleaned++
		out.Freed += item.Size
	}
	return out
}

// removeOne deletes a single path. Returns "" on success or a failure code.
func removeOne(path string) string {
	if reason := ValidatePathSafety(path); reason != "" {
		return "PROTECTED"
	}
	// TOCTOU guard: re-Lstat immediately before deleting — the path could
	// have vanished or been swapped for a symlink since scan time.
	info, err := os.Lstat(path)
	if err != nil {
		return errnoCode(err) // ENOENT when the item is already gone
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// Delete the link itself; never follow it.
		if err := os.Remove(path); err != nil {
			return errnoCode(err)
		}
		return ""
	}
	if err := os.RemoveAll(path); err != nil {
		return errnoCode(err)
	}
	return ""
}

// errnoCode maps an error to the CLI's errno string contract.
func errnoCode(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EPERM:
			return "EPERM"
		case syscall.EACCES:
			return "EACCES"
		case syscall.ENOENT:
			return "ENOENT"
		}
	}
	return "UNKNOWN"
}

// AggregateFailures collapses failures into the CLI's single summary line:
// "Failed to remove N items (32 EPERM, 8 EACCES)" — codes ordered by count
// desc, ties broken alphabetically. Returns nil for no failures.
func AggregateFailures(fs []RemoveFailure) []string {
	if len(fs) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, f := range fs {
		counts[f.Code]++
	}
	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool {
		if counts[codes[i]] != counts[codes[j]] {
			return counts[codes[i]] > counts[codes[j]]
		}
		return codes[i] < codes[j]
	})
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = fmt.Sprintf("%d %s", counts[c], c)
	}
	return []string{fmt.Sprintf("Failed to remove %d items (%s)", len(fs), strings.Join(parts, ", "))}
}
