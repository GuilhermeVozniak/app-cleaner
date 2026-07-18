package tui

import (
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestSafetyBadgeRendersADotForEveryKnownLevel(t *testing.T) {
	cases := []struct {
		name  string
		level core.SafetyLevel
	}{
		{"safe", core.SafetySafe},
		{"moderate", core.SafetyModerate},
		{"risky", core.SafetyRisky},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := safetyBadge(c.level)
			if !strings.Contains(got, "●") {
				t.Fatalf("safetyBadge(%v) = %q, want it to contain the dot glyph", c.level, got)
			}
		})
	}
}

func TestSafetyBadgeUnknownLevelRendersBlank(t *testing.T) {
	got := safetyBadge(core.SafetyLevel("bogus"))
	if strings.Contains(got, "●") {
		t.Fatalf("safetyBadge(unknown) = %q, want no dot glyph for an unrecognized level", got)
	}
	if got != " " {
		t.Fatalf("safetyBadge(unknown) = %q, want a single blank space", got)
	}
}

func TestPadStartPadsShorterStringsAndLeavesLongerOnesUnchanged(t *testing.T) {
	cases := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"pads to width", "42", 5, "   42"},
		{"already at width", "12345", 5, "12345"},
		{"already longer than width, left unchanged", "123456789", 5, "123456789"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := padStart(c.s, c.n); got != c.want {
				t.Fatalf("padStart(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
			}
		})
	}
}
