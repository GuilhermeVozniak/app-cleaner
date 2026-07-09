package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/scanners"
)

// fakeScanFunc matches scanners.RunScans's signature; every test that
// fakes scanRunner uses this alias so the type stays in one place.
type fakeScanFunc = func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(completed, total int, r core.ScanResult)) core.ScanSummary

func withFakeScan(t *testing.T, fn fakeScanFunc) {
	t.Helper()
	orig := scanRunner
	scanRunner = fn
	t.Cleanup(func() { scanRunner = orig })
}

// withTempHome points homeDirFn/configLoadFn at a throwaway t.TempDir()
// instead of the real $HOME, per the "t.TempDir() only" contract rule. It
// returns the temp home so callers can build real file fixtures under it.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origHome, origCfg := homeDirFn, configLoadFn
	homeDirFn = func() (string, error) { return dir, nil }
	configLoadFn = func(home string) config.Config {
		cfg := config.Default()
		cfg.Concurrency = 2
		return cfg
	}
	t.Cleanup(func() {
		homeDirFn, configLoadFn = origHome, origCfg
	})
	return dir
}

func resetScanFlags(t *testing.T) {
	t.Helper()
	origCats, origJSON := scanCategories, scanJSON
	scanCategories, scanJSON = nil, false
	t.Cleanup(func() { scanCategories, scanJSON = origCats, origJSON })
}

func fakeScanSummary() core.ScanSummary {
	trash := core.Categories["trash"]
	downloads := core.Categories["downloads"]
	results := []core.ScanResult{
		{Category: trash, Items: []core.CleanableItem{{Path: "/x/a", Size: 1024, Name: "a"}}, TotalSize: 1024},
		{Category: downloads, Items: []core.CleanableItem{
			{Path: "/x/b", Size: 2048, Name: "b"},
			{Path: "/x/c", Size: 1024, Name: "c"},
		}, TotalSize: 3072},
	}
	return core.ScanSummary{Results: results, TotalSize: 4096, TotalItems: 3}
}

func TestScanAndCleanCommandsRegisterOnRoot(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["scan"] {
		t.Error(`rootCmd is missing the "scan" subcommand`)
	}
	if !names["clean"] {
		t.Error(`rootCmd is missing the "clean" subcommand`)
	}
}

func TestResolveCategoriesDefaultsToAllWhenEmpty(t *testing.T) {
	ids, unknown := resolveCategories(nil)
	if len(unknown) != 0 {
		t.Fatalf("unknown = %v, want none", unknown)
	}
	want := core.CategoriesInOrder()
	if len(ids) != len(want) {
		t.Fatalf("len(ids) = %d, want %d (all categories)", len(ids), len(want))
	}
	for i, cat := range want {
		if ids[i] != cat.ID {
			t.Fatalf("ids[%d] = %q, want %q (stable order)", i, ids[i], cat.ID)
		}
	}
}

func TestResolveCategoriesWarnsAndSkipsUnknown(t *testing.T) {
	ids, unknown := resolveCategories([]string{"trash", "no-such-category", "downloads"})
	if len(unknown) != 1 || unknown[0] != "no-such-category" {
		t.Fatalf("unknown = %v, want [no-such-category]", unknown)
	}
	want := []core.CategoryID{"trash", "downloads"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestScanCmdRejectsAllUnknownCategories(t *testing.T) {
	resetScanFlags(t)
	withTempHome(t)
	scanCategories = []string{"no-such-category"}

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	scanCmd.SetOut(buf)
	scanCmd.SetErr(errBuf)

	err := runScan(scanCmd, nil)
	if err == nil {
		t.Fatal("runScan() error = nil, want an error when every category is unknown")
	}
	if !strings.Contains(errBuf.String(), `unknown category "no-such-category"`) {
		t.Fatalf("stderr = %q, want a warning about the unknown category", errBuf.String())
	}
}

func TestScanCmdJSONShape(t *testing.T) {
	resetScanFlags(t)
	withTempHome(t)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		return fakeScanSummary()
	})
	scanJSON = true

	buf := &bytes.Buffer{}
	scanCmd.SetOut(buf)
	scanCmd.SetErr(&bytes.Buffer{})

	if err := runScan(scanCmd, nil); err != nil {
		t.Fatalf("runScan() error = %v", err)
	}

	var got struct {
		TotalSize  int64 `json:"totalSize"`
		TotalItems int   `json:"totalItems"`
		Categories []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Group       string `json:"group"`
			SafetyLevel string `json:"safetyLevel"`
			TotalSize   int64  `json:"totalSize"`
			ItemCount   int    `json:"itemCount"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", buf.String(), err)
	}
	if got.TotalSize != 4096 || got.TotalItems != 3 {
		t.Fatalf("got totals (%d, %d), want (4096, 3)", got.TotalSize, got.TotalItems)
	}
	if len(got.Categories) != 2 {
		t.Fatalf("len(categories) = %d, want 2", len(got.Categories))
	}
	if got.Categories[0].ID != "trash" || got.Categories[0].SafetyLevel != "safe" || got.Categories[0].ItemCount != 1 {
		t.Fatalf("categories[0] = %+v, want trash/safe/1 item", got.Categories[0])
	}
	if got.Categories[1].ID != "downloads" || got.Categories[1].ItemCount != 2 {
		t.Fatalf("categories[1] = %+v, want downloads/2 items", got.Categories[1])
	}
}

func TestScanCmdHumanOutputShowsSizesAndTotal(t *testing.T) {
	resetScanFlags(t)
	withTempHome(t)
	withFakeScan(t, func(ctx context.Context, ids []core.CategoryID, opts scanners.Options, concurrency int, onResult func(int, int, core.ScanResult)) core.ScanSummary {
		return fakeScanSummary()
	})

	buf := &bytes.Buffer{}
	scanCmd.SetOut(buf)
	scanCmd.SetErr(&bytes.Buffer{})

	if err := runScan(scanCmd, nil); err != nil {
		t.Fatalf("runScan() error = %v", err)
	}
	text := buf.String()
	if !strings.Contains(text, "Trash") || !strings.Contains(text, core.FormatSize(1024)) {
		t.Fatalf("output = %q, want a Trash line with its formatted size", text)
	}
	if !strings.Contains(text, "Old Downloads") || !strings.Contains(text, core.FormatSize(3072)) {
		t.Fatalf("output = %q, want an Old Downloads line with its formatted size", text)
	}
	if !strings.Contains(text, core.FormatSize(4096)) {
		t.Fatalf("output = %q, want the total formatted size", text)
	}
}
