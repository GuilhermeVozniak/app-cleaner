// Package maintenance implements the three system maintenance tasks
// (DNS flush, purgeable space, Time Machine snapshots). External binaries
// run behind the Runner/Elevator interfaces so unit tests use fakes.
package maintenance

import (
	"context"
	"strings"
	"time"
)

// Result is the single shared result contract for all maintenance tasks.
type Result struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	Error         string `json:"error,omitempty"`
	RequiresAdmin bool   `json:"requiresAdmin"`
}

// Runner executes an external binary (absolute path, arg slice, no shell).
// Same shape as scanners.CmdRunner but defined locally so engine packages
// stay decoupled.
type Runner interface {
	Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error)
}

// Elevator runs a shell script with administrator privileges.
type Elevator interface {
	RunElevated(ctx context.Context, shellScript string) (string, error)
}

// userCanceled reports whether the user dismissed the macOS admin prompt
// (osascript reports: "execution error: User canceled. (-128)").
func userCanceled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "User canceled")
}
