package scanners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/guhcostan/app-cleaner/internal/core"
)

// laHome creates a temp home with an existing Library/LaunchAgents dir.
func laHome(t *testing.T) (home, agents string) {
	t.Helper()
	home = t.TempDir()
	agents = filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, agents
}

// laWriteXML writes an XML plist with a Program key.
func laWriteXML(t *testing.T, path, program string) {
	t.Helper()
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>test.label</string>
	<key>Program</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, program)
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// laWriteBinary writes a BINARY plist (bplist00) encoded with the same library
// the scanner uses — proves binary-plist coverage the CLI's regex never had.
func laWriteBinary(t *testing.T, path string, dict map[string]interface{}) {
	t.Helper()
	data, err := plist.Marshal(dict, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchAgentsScanner(t *testing.T) {
	home, agents := laHome(t)
	targets := t.TempDir()
	missing := filepath.Join(targets, "gone-binary")
	existing := filepath.Join(targets, "real-binary")
	if err := os.WriteFile(existing, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. XML plist, Program → missing path: ORPHANED.
	laWriteXML(t, filepath.Join(agents, "com.test.xml-orphan.plist"), missing)
	// 2. Binary plist, ProgramArguments[0] → missing path: ORPHANED.
	laWriteBinary(t, filepath.Join(agents, "com.test.bin-orphan.plist"), map[string]interface{}{
		"Label":            "com.test.bin-orphan",
		"ProgramArguments": []string{missing, "--flag"},
	})
	// 3. Existing target → skipped.
	laWriteXML(t, filepath.Join(agents, "com.test.alive.plist"), existing)
	// 4. System-binary prefix → skipped even though the path doesn't exist.
	laWriteXML(t, filepath.Join(agents, "com.test.system.plist"), "/usr/bin/no-such-tool-xyz")
	// 5. Program takes precedence over ProgramArguments[0]:
	//    Program exists → skipped, even though ProgramArguments[0] is missing.
	laWriteBinary(t, filepath.Join(agents, "com.test.precedence.plist"), map[string]interface{}{
		"Program":          existing,
		"ProgramArguments": []string{missing},
	})
	// 6. Relative program path → skipped.
	laWriteXML(t, filepath.Join(agents, "com.test.relative.plist"), "relative/bin/tool")
	// 7. Non-.plist files → ignored.
	if err := os.WriteFile(filepath.Join(agents, "README.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" {
		t.Fatalf("unexpected scan error: %q", res.Error)
	}

	got := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		got[it.Name] = it
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items (%v), want exactly the two orphans", len(res.Items), got)
	}

	wantXML := fmt.Sprintf("com.test.xml-orphan.plist → %s (missing)", missing)
	xmlItem, ok := got[wantXML]
	if !ok {
		t.Fatalf("missing XML orphan item %q in %v", wantXML, got)
	}
	if xmlItem.Path != filepath.Join(agents, "com.test.xml-orphan.plist") {
		t.Errorf("XML orphan Path = %q, want the plist file path", xmlItem.Path)
	}
	fi, err := os.Stat(xmlItem.Path)
	if err != nil {
		t.Fatal(err)
	}
	if xmlItem.Size != fi.Size() {
		t.Errorf("Size = %d, want plist file size %d", xmlItem.Size, fi.Size())
	}
	if xmlItem.IsDirectory {
		t.Error("IsDirectory = true, want false")
	}
	if xmlItem.ModifiedAt == nil {
		t.Error("ModifiedAt is nil, want plist mtime")
	}

	wantBin := fmt.Sprintf("com.test.bin-orphan.plist → %s (missing)", missing)
	if _, ok := got[wantBin]; !ok {
		t.Errorf("missing binary-plist orphan item %q (got %v)", wantBin, got)
	}
}

func TestLaunchAgentsScannerNoProgramKeys(t *testing.T) {
	home, agents := laHome(t)
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.no.program</string>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(agents, "com.no.program.plist"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}

	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("plist with neither Program nor ProgramArguments: got %+v, want zero items no error", res)
	}
}

func TestLaunchAgentsScannerSkipsHomebrewPrefixBinary(t *testing.T) {
	home, agents := laHome(t)
	laWriteXML(t, filepath.Join(agents, "homebrew.mxcl.service.plist"), "/opt/homebrew/bin/service")

	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("Program under /opt/homebrew/bin/ must be skipped (systemBinaryPrefixes), got %+v", res)
	}
}

func TestLaunchAgentsScannerTotalSizeSumOfOrphans(t *testing.T) {
	home, agents := laHome(t)
	missing1 := filepath.Join(t.TempDir(), "gone1")
	missing2 := filepath.Join(t.TempDir(), "gone2")
	laWriteXML(t, filepath.Join(agents, "com.deleted.app1.plist"), missing1)
	laWriteXML(t, filepath.Join(agents, "com.deleted.app2.plist"), missing2)

	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if res.Error != "" || len(res.Items) != 2 {
		t.Fatalf("want 2 orphaned items, got %+v", res)
	}
	var want int64
	for _, it := range res.Items {
		want += it.Size
	}
	if res.TotalSize != want {
		t.Fatalf("TotalSize = %d, want sum of item sizes %d", res.TotalSize, want)
	}
}

func TestLaunchAgentsScannerMissingAndUnreadableDir(t *testing.T) {
	// Missing ~/Library/LaunchAgents → empty result, NO error.
	res := newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir()}})
	if res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing dir: got error=%q items=%d, want clean empty result", res.Error, len(res.Items))
	}

	// Path exists but is not a directory → readdir failure → ScanResult.Error.
	home := t.TempDir()
	lib := filepath.Join(home, "Library")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "LaunchAgents"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = newLaunchAgentsScanner().Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if !strings.HasPrefix(res.Error, "Failed to read LaunchAgents directory: ") {
		t.Fatalf("Error = %q, want prefix 'Failed to read LaunchAgents directory: '", res.Error)
	}
}

func TestLaunchAgentsClean(t *testing.T) {
	home, agents := laHome(t)
	missing := filepath.Join(t.TempDir(), "gone")
	laWriteXML(t, filepath.Join(agents, "com.test.rm.plist"), missing)

	s := newLaunchAgentsScanner()
	res := s.Scan(context.Background(), Options{Roots: Roots{Home: home}})
	if len(res.Items) != 1 {
		t.Fatalf("scan found %d items, want 1", len(res.Items))
	}

	noop := func(current, total int, item core.CleanableItem) {}
	cr := s.Clean(context.Background(), res.Items, false, noop)
	if cr.CleanedItems != 1 || len(cr.Errors) != 0 {
		t.Fatalf("CleanResult = %+v, want 1 cleaned / no errors", cr)
	}
	if cr.FreedSpace != res.Items[0].Size {
		t.Errorf("FreedSpace = %d, want %d", cr.FreedSpace, res.Items[0].Size)
	}
	if _, err := os.Stat(res.Items[0].Path); !os.IsNotExist(err) {
		t.Errorf("plist still exists after clean (stat err = %v)", err)
	}
}
