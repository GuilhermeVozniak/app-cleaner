package scanners

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CmdRunner abstracts external binaries (brew, docker, ...) so scanner and
// maintenance-style code can be tested with fakes. Implementations must never
// spawn a shell: bin must be an absolute path, args a plain argv slice.
type CmdRunner interface {
	Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (stdout string, err error)
}

// ExecRunner is the real CmdRunner: exec.CommandContext with a per-call
// timeout. Non-zero exit returns an error carrying the trimmed stderr text.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	if !filepath.IsAbs(bin) {
		return "", fmt.Errorf("binary path must be absolute, got %q", bin)
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			return stdout.String(), fmt.Errorf("%s timed out after %s", filepath.Base(bin), timeout)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s failed: %s", filepath.Base(bin), detail)
	}
	return stdout.String(), nil
}
