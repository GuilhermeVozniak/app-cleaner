// Package spacelens builds a size map of a directory tree for the Space
// Lens view: every node carries its fully-computed size; children are
// sorted largest-first and truncated to a per-directory cap so huge
// directories cannot flood the JSON bridge.
package spacelens

import (
	"context"
	"os"
	"path/filepath"
	"sort"
)

// Node is one file or directory in the size map. Children is non-nil for
// directories (never nil — a Go nil slice reaches the frontend as JSON
// null) and always nil for files.
type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"isDir"`
	Children []Node `json:"children,omitempty"`
	// Truncated counts children dropped by the per-directory cap (their
	// sizes still count toward Size).
	Truncated int `json:"truncated,omitempty"`
}

// MaxChildren caps how many children each directory node keeps (largest
// first); the rest are summed into the parent but not materialized.
const MaxChildren = 25

// Build walks root to maxDepth levels of children (maxDepth <= 0 means
// just the root's own total) and returns the size map. Unreadable
// directories yield a zero-child node rather than an error; a cancelled
// context stops descending and returns what was accumulated so far.
func Build(ctx context.Context, root string, maxDepth int) Node {
	return build(ctx, root, maxDepth)
}

func build(ctx context.Context, path string, depth int) Node {
	n := Node{Name: filepath.Base(path), Path: path}
	info, err := os.Lstat(path)
	if err != nil {
		return n
	}
	if info.Mode()&os.ModeSymlink != 0 {
		n.Size = info.Size() // never follow symlinks
		return n
	}
	if !info.IsDir() {
		n.Size = info.Size()
		return n
	}
	n.IsDir = true
	n.Children = []Node{}
	entries, err := os.ReadDir(path)
	if err != nil {
		return n
	}
	for _, e := range entries {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		child := build(ctx, filepath.Join(path, e.Name()), depth-1)
		n.Size += child.Size
		n.Children = append(n.Children, child)
	}
	sort.SliceStable(n.Children, func(i, j int) bool { return n.Children[i].Size > n.Children[j].Size })
	if len(n.Children) > MaxChildren {
		n.Truncated = len(n.Children) - MaxChildren
		n.Children = n.Children[:MaxChildren]
	}
	if depth <= 0 {
		// Below the requested depth only totals matter, not the listing.
		n.Truncated += len(n.Children)
		n.Children = []Node{}
	}
	return n
}
