package fda

import (
	"os"
	"path/filepath"
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

func TestSettingsURL(t *testing.T) {
	const want = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"
	if SettingsURL != want {
		t.Fatalf("SettingsURL = %q, want %q", SettingsURL, want)
	}
}
