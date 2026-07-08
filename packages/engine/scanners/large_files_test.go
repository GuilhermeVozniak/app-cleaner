package scanners

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestLargeFilesScanner(t *testing.T) {
	s, ok := Get("large-files")
	if !ok {
		t.Fatal("large-files scanner not registered")
	}
	if s.Category() != core.Categories["large-files"] {
		t.Fatalf("Category() = %+v, want core.Categories[large-files]", s.Category())
	}

	opts := testOptions(t)
	// Missing ~/Downloads and ~/Documents => empty result, no error.
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing roots must yield empty result without error, got %+v", res)
	}

	opts.Cfg.LargeFilesMinSize = 1000 // keep fixtures tiny
	dl := filepath.Join(opts.Roots.Home, "Downloads")
	docs := filepath.Join(opts.Roots.Home, "Documents")

	mkFile(t, filepath.Join(dl, "big.bin"), 1500)                             // in
	mkFile(t, filepath.Join(dl, "exact.bin"), 1000)                           // in: size >= min is inclusive
	mkFile(t, filepath.Join(dl, "small.bin"), 999)                            // out: below threshold
	mkFile(t, filepath.Join(dl, ".hidden.bin"), 5000)                         // out: dot file skipped
	mkFile(t, filepath.Join(dl, ".hiddendir", "inside.bin"), 5000)            // out: dot dir never descended
	mkFile(t, filepath.Join(dl, "d1", "d2", "d3", "deep.bin"), 1200)          // in: file at depth 4 (dir d3 at depth 3 is read)
	mkFile(t, filepath.Join(dl, "d1", "d2", "d3", "d4", "toodeep.bin"), 9000) // out: dir d4 at depth 4 not descended
	mkFile(t, filepath.Join(docs, "doc.bin"), 3000)                           // in: second root
	if err := os.Symlink(filepath.Join(dl, "big.bin"), filepath.Join(dl, "link.bin")); err != nil {
		t.Fatal(err) // out: symlinks are not regular files, never followed
	}

	res := s.Scan(context.Background(), opts)
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %s", res.Error)
	}
	var names []string
	for _, it := range res.Items {
		names = append(names, it.Name)
	}
	want := []string{"doc.bin", "big.bin", "deep.bin", "exact.bin"} // size desc: 3000, 1500, 1200, 1000
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v (sorted size desc)", names, want)
	}
	for _, it := range res.Items {
		if it.IsDirectory {
			t.Fatalf("%s flagged as directory; large-files returns regular files only", it.Name)
		}
		if it.ModifiedAt == nil {
			t.Fatalf("%s has nil ModifiedAt", it.Name)
		}
		if filepath.Base(it.Path) != it.Name {
			t.Fatalf("Name %q must be the basename of Path %q", it.Name, it.Path)
		}
	}
	if res.TotalSize != 3000+1500+1200+1000 {
		t.Fatalf("TotalSize = %d, want %d", res.TotalSize, 3000+1500+1200+1000)
	}
}
