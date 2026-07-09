package scanners

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const dupMiB = 1 << 20

// dupWrite creates a file (and parents) with the given content and mtime.
func dupWrite(t *testing.T, path string, content []byte, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicatesScanner(t *testing.T) {
	home := t.TempDir()
	now := time.Now()

	// Trio of identical 2 MiB files across all three roots, distinct mtimes:
	// the NEWEST (newest.bin) is kept, the two older copies are listed.
	trio := bytes.Repeat([]byte{0xAB}, 2*dupMiB)
	dupWrite(t, filepath.Join(home, "Downloads", "old.bin"), trio, now.Add(-48*time.Hour))
	dupWrite(t, filepath.Join(home, "Documents", "middle.bin"), trio, now.Add(-24*time.Hour))
	dupWrite(t, filepath.Join(home, "Desktop", "newest.bin"), trio, now)

	// Same size, different content from byte 0 → NOT duplicates.
	dupWrite(t, filepath.Join(home, "Downloads", "diff-a.bin"), bytes.Repeat([]byte{0x01}, dupMiB+1), now)
	dupWrite(t, filepath.Join(home, "Downloads", "diff-b.bin"), bytes.Repeat([]byte{0x02}, dupMiB+1), now)

	// Partial-hash collision: same size, same first MiB, different tail
	// → survives the partial pre-filter but the full hash differs → NOT duplicates.
	head := bytes.Repeat([]byte{0x0F}, dupMiB)
	collA := append(append([]byte{}, head...), []byte("tail-one")...)
	collB := append(append([]byte{}, head...), []byte("tail-two")...)
	dupWrite(t, filepath.Join(home, "Documents", "coll-a.bin"), collA, now)
	dupWrite(t, filepath.Join(home, "Documents", "coll-b.bin"), collB, now)

	// Identical but below the 1 MiB floor → ignored.
	small := bytes.Repeat([]byte{0x33}, 1024)
	dupWrite(t, filepath.Join(home, "Downloads", "small-a.bin"), small, now)
	dupWrite(t, filepath.Join(home, "Downloads", "small-b.bin"), small, now)

	// Identical pair inside a dot-directory → never walked (would otherwise
	// join the trio's 2 MiB size group and break the expected count).
	dupWrite(t, filepath.Join(home, "Downloads", ".cache", "h-a.bin"), trio, now)
	dupWrite(t, filepath.Join(home, "Downloads", ".cache", "h-b.bin"), trio, now)

	res := newDuplicatesScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %q", res.Error)
	}

	got := map[string]bool{}
	for _, it := range res.Items {
		got[it.Name] = true
		if it.IsDirectory {
			t.Errorf("%s: IsDirectory = true, want false", it.Name)
		}
		if filepath.Base(it.Path) == "newest.bin" {
			t.Errorf("newest copy was listed for deletion: %q", it.Path)
		}
		if it.Size != int64(2*dupMiB) {
			t.Errorf("%s: Size = %d, want %d", it.Name, it.Size, 2*dupMiB)
		}
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items (%v), want exactly the 2 older trio copies", len(res.Items), got)
	}
	for _, w := range []string{"old.bin (dup of newest.bin)", "middle.bin (dup of newest.bin)"} {
		if !got[w] {
			t.Errorf("missing expected item %q (got %v)", w, got)
		}
	}
	if res.TotalSize != int64(4*dupMiB) {
		t.Errorf("TotalSize = %d, want %d", res.TotalSize, 4*dupMiB)
	}

	// Sorted by size descending.
	for i := 1; i < len(res.Items); i++ {
		if res.Items[i-1].Size < res.Items[i].Size {
			t.Errorf("items not sorted by size desc at index %d", i)
		}
	}
}
