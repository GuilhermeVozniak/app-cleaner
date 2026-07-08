package cmd

import (
	"strings"
	"testing"
)

func TestPromptConfirmDefaults(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"bare enter takes default no", "\n", false, false},
		{"bare enter takes default yes", "\n", true, true},
		{"y answers yes", "y\n", false, true},
		{"yes answers yes", "yes\n", false, true},
		{"n answers no", "n\n", true, false},
		{"empty reader takes default", "", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf strings.Builder
			got := promptConfirm(strings.NewReader(c.input), &buf, "Proceed?", c.defaultYes)
			if got != c.want {
				t.Fatalf("promptConfirm(%q, default=%v) = %v, want %v", c.input, c.defaultYes, got, c.want)
			}
			if !strings.Contains(buf.String(), "Proceed?") {
				t.Fatalf("prompt not written to w: %q", buf.String())
			}
		})
	}
}
