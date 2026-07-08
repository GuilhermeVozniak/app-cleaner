package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

func TestRule(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{5, "─────"},
		{1, "─"},
		{0, ""},
		{-1, ""},
	}
	for _, c := range cases {
		if got := Rule(c.n); got != c.want {
			t.Errorf("Rule(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestDryRunPrefix(t *testing.T) {
	if DryRunPrefix != "[DRY RUN]" {
		t.Errorf("DryRunPrefix = %q, want %q", DryRunPrefix, "[DRY RUN]")
	}
}

func TestContractHome(t *testing.T) {
	cases := []struct {
		path, home, want string
	}{
		{"/Users/mac/Documents", "/Users/mac", "~/Documents"},
		{"/Users/mac", "/Users/mac", "~"},
		{"/etc/hosts", "/Users/mac", "/etc/hosts"},
		{"relative/path", "", "relative/path"},
	}
	for _, c := range cases {
		if got := ContractHome(c.path, c.home); got != c.want {
			t.Errorf("ContractHome(%q, %q) = %q, want %q", c.path, c.home, got, c.want)
		}
	}
}

func TestTruncateNameExact(t *testing.T) {
	cases := []struct {
		name string
		max  int
		want string
	}{
		{"file.txt", 20, "file.txt"},
		{"short.js", 10, "short.js"},
		{"exact.txt", 9, "exact.txt"},
		{"test.txt", 8, "test.txt"},
		{"a.txt", 10, "a.txt"},
		{".txt", 10, ".txt"},
		{"a-very-long-file-name.txt", 15, "a-ve...name.txt"},
		{"verylongfilenamewithoutextension", 15, "verylo...ension"},
		{"abcdefghij.txt", 10, "ab...j.txt"},
		{"verylongfilename", 10, "very...ame"},
		{"verylongfilename.txt", 5, "ve..."},
	}
	for _, c := range cases {
		if got := TruncateName(c.name, c.max); got != c.want {
			t.Errorf("TruncateName(%q, %d) = %q, want %q", c.name, c.max, got, c.want)
		}
	}
}

func TestTruncateNameProperties(t *testing.T) {
	cases := []struct {
		name       string
		max        int
		wantLen    int
		wantSuffix string
		wantPrefix string
	}{
		{"very-long-filename-that-needs-truncation.txt", 20, 20, ".txt", ""},
		{"my.file.name.tar.gz", 12, 12, ".gz", ""},
		{".gitignore-very-long-name", 15, 15, "", ".git"},
	}
	for _, c := range cases {
		got := TruncateName(c.name, c.max)
		if len(got) != c.wantLen {
			t.Errorf("TruncateName(%q, %d) length = %d, want %d (got %q)", c.name, c.max, len(got), c.wantLen, got)
		}
		if !strings.Contains(got, "...") {
			t.Errorf("TruncateName(%q, %d) = %q, want it to contain %q", c.name, c.max, got, "...")
		}
		if c.wantSuffix != "" && !strings.HasSuffix(got, c.wantSuffix) {
			t.Errorf("TruncateName(%q, %d) = %q, want suffix %q", c.name, c.max, got, c.wantSuffix)
		}
		if c.wantPrefix != "" && !strings.HasPrefix(got, c.wantPrefix) {
			t.Errorf("TruncateName(%q, %d) = %q, want prefix %q", c.name, c.max, got, c.wantPrefix)
		}
	}
}

func TestTruncateNameHardTruncateEdgeCase(t *testing.T) {
	// Extension + ellipsis longer than maxLength -> hard truncation path.
	got := TruncateName("file.verylongextension", 10)
	if len(got) != 10 {
		t.Errorf("len = %d, want 10 (got %q)", len(got), got)
	}
	if !strings.Contains(got, "...") {
		t.Errorf("got %q, want it to contain %q", got, "...")
	}
}

func TestErrnoBreakdownDelegatesToFsx(t *testing.T) {
	failures := []fsx.RemoveFailure{
		{Path: "/a", Code: "EPERM"},
		{Path: "/b", Code: "EPERM"},
		{Path: "/c", Code: "EACCES"},
	}

	got := ErrnoBreakdown(failures)
	want := fsx.AggregateFailures(failures)

	if len(got) != len(want) {
		t.Fatalf("ErrnoBreakdown returned %d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}

	wantLine := "Failed to remove 3 items (2 EPERM, 1 EACCES)"
	if len(got) != 1 || got[0] != wantLine {
		t.Errorf("ErrnoBreakdown = %v, want [%q]", got, wantLine)
	}
}

func TestErrnoBreakdownEmpty(t *testing.T) {
	if got := ErrnoBreakdown(nil); got != nil {
		t.Errorf("ErrnoBreakdown(nil) = %v, want nil", got)
	}
}

func scanSummaryFixture() core.ScanSummary {
	return core.ScanSummary{
		TotalSize:  1000,
		TotalItems: 1,
		Results: []core.ScanResult{
			{
				Category: core.Category{
					ID:          "trash",
					Name:        "Trash",
					Group:       "Storage",
					SafetyLevel: "safe",
				},
				Items: []core.CleanableItem{
					{Path: "/Users/mac/.Trash/foo.txt", Size: 1000, Name: "foo.txt"},
				},
				TotalSize: 1000,
			},
			{
				Category: core.Category{
					ID:          "docker",
					Name:        "Docker",
					Group:       "Development",
					SafetyLevel: "moderate",
				},
				Items:     nil,
				TotalSize: 0,
			},
		},
	}
}

const wantScanJSONNonVerbose = `{
  "totalSize": 1000,
  "totalItems": 1,
  "categories": [
    {
      "id": "trash",
      "name": "Trash",
      "group": "Storage",
      "safetyLevel": "safe",
      "totalSize": 1000,
      "itemCount": 1
    }
  ]
}`

const wantScanJSONVerbose = `{
  "totalSize": 1000,
  "totalItems": 1,
  "categories": [
    {
      "id": "trash",
      "name": "Trash",
      "group": "Storage",
      "safetyLevel": "safe",
      "totalSize": 1000,
      "itemCount": 1,
      "items": [
        {
          "path": "/Users/mac/.Trash/foo.txt",
          "size": 1000
        }
      ]
    }
  ]
}`

func TestMarshalScanJSONNonVerboseOmitsCategoryWithNoItemsAndOmitsItemsKey(t *testing.T) {
	got, err := MarshalScanJSON(scanSummaryFixture(), false)
	if err != nil {
		t.Fatalf("MarshalScanJSON error = %v", err)
	}
	if string(got) != wantScanJSONNonVerbose {
		t.Errorf("MarshalScanJSON(verbose=false) =\n%s\nwant:\n%s", got, wantScanJSONNonVerbose)
	}
}

func TestMarshalScanJSONVerboseIncludesItems(t *testing.T) {
	got, err := MarshalScanJSON(scanSummaryFixture(), true)
	if err != nil {
		t.Fatalf("MarshalScanJSON error = %v", err)
	}
	if string(got) != wantScanJSONVerbose {
		t.Errorf("MarshalScanJSON(verbose=true) =\n%s\nwant:\n%s", got, wantScanJSONVerbose)
	}
}

func TestEncodeScanJSONEmptyCategoriesSliceNotNil(t *testing.T) {
	empty := core.ScanSummary{}
	got := EncodeScanJSON(empty, false)
	if got.Categories == nil {
		t.Error("EncodeScanJSON(empty).Categories = nil, want non-nil empty slice (so JSON encodes [] not null)")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if !strings.Contains(string(b), `"categories":[]`) {
		t.Errorf("marshaled empty summary = %s, want it to contain \"categories\":[]", b)
	}
}
