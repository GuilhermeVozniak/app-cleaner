package scanners

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecRunnerCapturesStdout(t *testing.T) {
	var r ExecRunner
	out, err := r.Run(context.Background(), 5*time.Second, "/bin/echo", "hello", "world")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(out) != "hello world" {
		t.Fatalf("stdout = %q, want %q", out, "hello world")
	}
}

func TestExecRunnerNonZeroExitCarriesStderr(t *testing.T) {
	var r ExecRunner
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")
	_, err := r.Run(context.Background(), 5*time.Second, "/bin/cat", missing)
	if err == nil {
		t.Fatal("want error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "No such file or directory") {
		t.Fatalf("error %q does not carry stderr text", err)
	}
}

func TestExecRunnerRejectsRelativeBinaryPath(t *testing.T) {
	var r ExecRunner
	_, err := r.Run(context.Background(), 5*time.Second, "echo", "hi")
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("err = %v, want absolute-path refusal", err)
	}
}

func TestExecRunnerEnforcesTimeout(t *testing.T) {
	var r ExecRunner
	start := time.Now()
	_, err := r.Run(context.Background(), 150*time.Millisecond, "/bin/sleep", "5")
	if err == nil {
		t.Fatal("want timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout not enforced, took %s", elapsed)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout message", err)
	}
}
