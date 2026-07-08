package fsx

import (
	"os"
	"path/filepath"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// GetSize returns the logical size in bytes of the file, directory tree, or
// symlink at path. Symlinks report the size of the link itself and are never
// followed. Unreadable or missing paths report 0 — sizing never fails.
func GetSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	return sizeFromInfo(path, info)
}

// sizeFromInfo sizes an already-lstat'ed path. The symlink check must come
// before IsDir: a symlink to a directory is a link, not a directory.
func sizeFromInfo(path string, info os.FileInfo) int64 {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return info.Size()
	case info.IsDir():
		return dirSize(path)
	default:
		return info.Size()
	}
}

// dirSize recursively sums logical sizes under dir. Unreadable directories
// contribute 0; unreadable entries are skipped.
func dirSize(dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		total += sizeFromInfo(full, info)
	}
	return total
}

// ItemFilter filters GetItems results. Zero values disable each filter.
type ItemFilter struct {
	MinAgeDays int   // age = now − lstat mtime, compared in fractional days
	MinSize    int64 // minimum fully-computed size in bytes
}

// GetDirectoryItems lists dir's immediate children, each fully sized
// (directories recursively). Unreadable dir = empty slice, never an error.
func GetDirectoryItems(dir string) []core.CleanableItem {
	return listChildren(dir, ItemFilter{})
}

// GetItems lists dir's immediate children matching f (non-recursive).
func GetItems(dir string, f ItemFilter) []core.CleanableItem {
	return listChildren(dir, f)
}

func listChildren(dir string, f ItemFilter) []core.CleanableItem {
	items := []core.CleanableItem{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}
	now := time.Now()
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		if f.MinAgeDays > 0 {
			ageDays := now.Sub(info.ModTime()).Hours() / 24
			if ageDays < float64(f.MinAgeDays) {
				continue
			}
		}
		size := sizeFromInfo(full, info)
		if f.MinSize > 0 && size < f.MinSize {
			continue
		}
		mt := info.ModTime()
		items = append(items, core.CleanableItem{
			Path:        full,
			Size:        size,
			Name:        e.Name(),
			IsDirectory: info.IsDir(),
			ModifiedAt:  &mt,
		})
	}
	return items
}
