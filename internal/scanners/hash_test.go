package scanners

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileMD5(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	full, err := fileMD5(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if full != "5eb63bbbe01eeed093cb22bb8f5acdc3" { // md5("hello world")
		t.Errorf("full hash = %q, want 5eb63bbbe01eeed093cb22bb8f5acdc3", full)
	}

	partial, err := fileMD5(p, 5)
	if err != nil {
		t.Fatal(err)
	}
	if partial != "5d41402abc4b2a76b9719d911017c592" { // md5("hello")
		t.Errorf("partial hash = %q, want 5d41402abc4b2a76b9719d911017c592", partial)
	}

	// limit beyond EOF hashes the whole file — identical to the full hash.
	beyond, err := fileMD5(p, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if beyond != full {
		t.Errorf("limit-beyond-EOF hash = %q, want %q", beyond, full)
	}

	if _, err := fileMD5(filepath.Join(dir, "missing"), 0); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
