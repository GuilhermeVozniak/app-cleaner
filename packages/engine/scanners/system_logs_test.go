package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestSystemLogsScanner(t *testing.T) {
	s, ok := Get("system-logs")
	if !ok {
		t.Fatal("system-logs scanner not registered")
	}
	if s.Category() != core.Categories["system-logs"] {
		t.Fatalf("Category() = %+v, want core.Categories[system-logs]", s.Category())
	}

	opts := testOptions(t)
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing dir must yield empty result without error, got %+v", res)
	}

	// Only ~/Library/Logs is scanned — /var/log was deliberately dropped
	// (spec §10.1), so a Roots-relative fixture fully specifies the scanner.
	logs := filepath.Join(opts.Roots.Home, "Library", "Logs")
	mkFile(t, filepath.Join(logs, "app.log"), 150)
	mkFile(t, filepath.Join(logs, "CrashReporter", "dump.crash"), 350)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 2 || res.TotalSize != 500 {
		t.Fatalf("got %+v; want 2 items totalling 500", res)
	}
}
