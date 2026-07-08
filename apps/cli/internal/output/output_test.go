package output

import "testing"

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
