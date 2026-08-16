package spacelens

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildSizesAndSorting(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "big", "a.bin"), 300)
	write(t, filepath.Join(root, "small", "b.bin"), 100)
	write(t, filepath.Join(root, "c.txt"), 50)

	n := Build(context.Background(), root, 2)
	if !n.IsDir {
		t.Fatal("root should be a directory")
	}
	if n.Size != 450 {
		t.Fatalf("root size = %d, want 450", n.Size)
	}
	if len(n.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(n.Children))
	}
	// Sorted largest-first.
	if n.Children[0].Name != "big" || n.Children[1].Name != "small" || n.Children[2].Name != "c.txt" {
		t.Fatalf("unexpected order: %s, %s, %s", n.Children[0].Name, n.Children[1].Name, n.Children[2].Name)
	}
	// Depth 2: grandchildren are materialized.
	if len(n.Children[0].Children) != 1 || n.Children[0].Children[0].Name != "a.bin" {
		t.Fatalf("expected big/a.bin child, got %+v", n.Children[0].Children)
	}
}

func TestBuildDepthLimitKeepsTotals(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "d1", "d2", "deep.bin"), 200)

	n := Build(context.Background(), root, 1)
	if n.Size != 200 {
		t.Fatalf("root size = %d, want 200 (depth limit must not drop bytes)", n.Size)
	}
	d1 := n.Children[0]
	if d1.Size != 200 {
		t.Fatalf("d1 size = %d, want 200", d1.Size)
	}
	if len(d1.Children) != 0 {
		t.Fatalf("d1 children should be pruned at depth limit, got %d", len(d1.Children))
	}
	if d1.Truncated != 1 {
		t.Fatalf("d1 truncated = %d, want 1", d1.Truncated)
	}
}

func TestBuildFileRoot(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "only.bin")
	write(t, p, 42)
	n := Build(context.Background(), p, 3)
	if n.IsDir || n.Size != 42 || n.Children != nil {
		t.Fatalf("file node wrong: %+v", n)
	}
}

func TestBuildMissingPath(t *testing.T) {
	n := Build(context.Background(), filepath.Join(t.TempDir(), "nope"), 2)
	if n.Size != 0 {
		t.Fatalf("missing path size = %d, want 0", n.Size)
	}
}

func TestBuildChildCap(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < MaxChildren+5; i++ {
		write(t, filepath.Join(root, string(rune('a'+i%26))+string(rune('0'+i/26))+".bin"), 10+i)
	}
	n := Build(context.Background(), root, 1)
	if len(n.Children) != MaxChildren {
		t.Fatalf("children = %d, want cap %d", len(n.Children), MaxChildren)
	}
	if n.Truncated != 5 {
		t.Fatalf("truncated = %d, want 5", n.Truncated)
	}
}

func TestBuildCancelledContext(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.bin"), 100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := Build(ctx, root, 2)
	if len(n.Children) != 0 {
		t.Fatalf("cancelled walk should not descend, got %d children", len(n.Children))
	}
}

func TestBuildSymlinkNotFollowed(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "real", "big.bin"), 500)
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable")
	}
	n := Build(context.Background(), root, 2)
	for _, c := range n.Children {
		if c.Name == "link" && (c.IsDir || c.Size >= 500) {
			t.Fatalf("symlink was followed: %+v", c)
		}
	}
}
