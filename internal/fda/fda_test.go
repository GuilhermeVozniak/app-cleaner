package fda

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCheckReadableIsTrue(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Library", "Safari"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Check(home)
	if got == nil || !*got {
		t.Fatalf("Check = %v, want true", got)
	}
}

func TestCheckMissingDirIsUnknown(t *testing.T) {
	home := t.TempDir() // no Library/Safari → ENOENT → unknown
	if got := Check(home); got != nil {
		t.Fatalf("Check = %v, want nil (unknown)", *got)
	}
}

func TestCheckPermissionDeniedIsFalse(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: mode 000 does not block reads")
	}
	home := t.TempDir()
	safari := filepath.Join(home, "Library", "Safari")
	if err := os.MkdirAll(safari, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(safari, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(safari, 0o755) }) // let t.TempDir clean up
	got := Check(home)
	if got == nil || *got {
		t.Fatalf("Check = %v, want false (EACCES)", got)
	}
}

// TestClassifyErr mirrors the CLI's hasFullDiskAccess unit tests
// (readdir mocked to reject with a given errno), which the port can't
// reproduce via chmod alone: macOS TCC denials raise EPERM, but chmod
// 0o000 on most local filesystems yields EACCES instead. classifyErr is
// the extracted seam that lets us construct each errno directly.
func TestClassifyErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want *bool
	}{
		{
			name: "nil is true",
			err:  nil,
			want: boolPtr(true),
		},
		{
			name: "EPERM is false",
			err:  &os.PathError{Op: "open", Path: "x", Err: syscall.EPERM},
			want: boolPtr(false),
		},
		{
			name: "EACCES is false",
			err:  &os.PathError{Op: "open", Path: "x", Err: syscall.EACCES},
			want: boolPtr(false),
		},
		{
			name: "ENOENT is unknown",
			err:  &os.PathError{Op: "open", Path: "x", Err: syscall.ENOENT},
			want: nil,
		},
		{
			name: "other error is unknown",
			err:  &os.PathError{Op: "open", Path: "x", Err: syscall.EIO},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyErr(tt.err)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("classifyErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestSettingsURL(t *testing.T) {
	const want = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"
	if SettingsURL != want {
		t.Fatalf("SettingsURL = %q, want %q", SettingsURL, want)
	}
}
