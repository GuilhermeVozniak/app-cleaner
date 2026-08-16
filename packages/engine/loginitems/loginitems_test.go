package loginitems

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%LABEL%</string>
%BODY%
</dict></plist>`

func writePlist(t *testing.T, dir, name, label, body string) string {
	t.Helper()
	content := strings.ReplaceAll(plistTemplate, "%LABEL%", label)
	content = strings.ReplaceAll(content, "%BODY%", body)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListParsesAgents(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "tool")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePlist(t, dir, "b.plist", "com.example.b",
		"<key>Program</key><string>"+program+"</string><key>RunAtLoad</key><true/>")
	writePlist(t, dir, "a.plist", "com.example.a",
		"<key>ProgramArguments</key><array><string>/does/not/exist</string><string>--flag</string></array>")

	items := List([]Dir{{Path: dir, Kind: "user-agent"}})
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	// Sorted by label.
	if items[0].Label != "com.example.a" || items[1].Label != "com.example.b" {
		t.Fatalf("unexpected order: %s, %s", items[0].Label, items[1].Label)
	}
	if items[0].Program != "/does/not/exist" || !items[0].ProgramMissing {
		t.Fatalf("ProgramArguments fallback / missing detection failed: %+v", items[0])
	}
	if items[1].Program != program || items[1].ProgramMissing || !items[1].RunAtLoad {
		t.Fatalf("program item wrong: %+v", items[1])
	}
	if items[0].Kind != "user-agent" {
		t.Fatalf("kind = %s, want user-agent", items[0].Kind)
	}
}

func TestListSkipsGarbageAndMissingDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.plist"), []byte("not a plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	items := List([]Dir{
		{Path: dir, Kind: "user-agent"},
		{Path: filepath.Join(dir, "missing"), Kind: "daemon"},
	})
	if len(items) != 0 {
		t.Fatalf("items = %d, want 0 (garbage skipped, missing dir skipped)", len(items))
	}
	if items == nil {
		t.Fatal("List must never return nil (JSON null trap)")
	}
}

func TestListLabelFallsBackToFilename(t *testing.T) {
	dir := t.TempDir()
	writePlist(t, dir, "com.fallback.name.plist", "", "<key>RunAtLoad</key><false/>")
	items := List([]Dir{{Path: dir, Kind: "user-agent"}})
	if len(items) != 1 || items[0].Label != "com.fallback.name" {
		t.Fatalf("fallback label wrong: %+v", items)
	}
}

func TestStandardDirs(t *testing.T) {
	dirs := StandardDirs("/Users/x")
	if len(dirs) != 3 || dirs[0].Path != "/Users/x/Library/LaunchAgents" || dirs[0].Kind != "user-agent" {
		t.Fatalf("unexpected dirs: %+v", dirs)
	}
	if dirs[2].Kind != "daemon" {
		t.Fatalf("third dir kind = %s, want daemon", dirs[2].Kind)
	}
}
