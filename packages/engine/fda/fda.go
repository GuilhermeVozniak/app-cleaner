// Package fda detects Full Disk Access by probing a TCC-protected
// directory. Note: in the GUI app, FDA must be granted to App Cleaner
// itself (not the user's terminal).
package fda

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// SettingsURL deep-links to System Settings → Privacy & Security → Full
// Disk Access.
const SettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

// Check probes <home>/Library/Safari (TCC-protected — macOS returns EPERM
// on reads without FDA, even for the owning user).
// Tri-state result: true = granted, false = denied (EPERM/EACCES),
// nil = unknown (any other error, e.g. ENOENT when Safari never ran).
func Check(home string) *bool {
	_, err := os.ReadDir(filepath.Join(home, "Library", "Safari"))
	return classifyErr(err)
}

// classifyErr maps a probe error to the tri-state FDA result. Extracted
// from Check so tests can exercise the EPERM/EACCES/other classification
// directly, since macOS's real denial errno (EPERM) can't be reproduced
// via chmod on most filesystems (chmod 0o000 yields EACCES instead).
func classifyErr(err error) *bool {
	if err == nil {
		return boolPtr(true)
	}
	// errors.Is unwraps the *fs.PathError to its syscall.Errno.
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return boolPtr(false)
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }
