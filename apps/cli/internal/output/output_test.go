package output

import (
	"strings"
	"testing"
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
