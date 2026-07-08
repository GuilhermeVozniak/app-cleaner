package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandUse(t *testing.T) {
	if rootCmd.Use != "app-cleaner" {
		t.Fatalf("rootCmd.Use = %q, want %q", rootCmd.Use, "app-cleaner")
	}
}

func TestRootCommandVersionFlag(t *testing.T) {
	rootCmd.Version = "9.9.9"
	rootCmd.SetArgs([]string{"--version"})
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if _, err := rootCmd.ExecuteC(); err != nil {
		t.Fatalf("ExecuteC() error = %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "9.9.9") {
		t.Fatalf("version output = %q, want it to contain %q", got, "9.9.9")
	}
}

func TestRunInteractiveStubIsNoop(t *testing.T) {
	if err := RunInteractive(rootCmd, nil); err != nil {
		t.Fatalf("RunInteractive stub returned error: %v", err)
	}
}
