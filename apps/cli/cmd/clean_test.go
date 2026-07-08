package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// newCleanTestCmd builds a throwaway *cobra.Command carrying only the
// backup/no-backup bool flags resolveBackupFlag inspects. Tests use this
// instead of the package-singleton cleanCmd because pflag's
// Flags().Changed() is sticky (never resets to false once Set), which
// would leak --backup/--no-backup state across test cases sharing one
// FlagSet.
func newCleanTestCmd() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().Bool("backup", false, "")
	c.Flags().Bool("no-backup", false, "")
	return c
}

func TestResolveBackupFlagPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		backup     bool
		noBackup   bool
		cfgDefault bool
		want       bool
	}{
		{"neither set falls back to config default true", false, false, true, true},
		{"neither set falls back to config default false", false, false, false, false},
		{"explicit --backup wins over config default false", true, false, false, true},
		{"explicit --no-backup wins over config default true", false, true, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := newCleanTestCmd()
			if c.backup {
				if err := cmd.Flags().Set("backup", "true"); err != nil {
					t.Fatalf("Set(backup) error = %v", err)
				}
			}
			if c.noBackup {
				if err := cmd.Flags().Set("no-backup", "true"); err != nil {
					t.Fatalf("Set(no-backup) error = %v", err)
				}
			}
			got, err := resolveBackupFlag(cmd, c.cfgDefault)
			if err != nil {
				t.Fatalf("resolveBackupFlag() error = %v", err)
			}
			if got != c.want {
				t.Fatalf("resolveBackupFlag() = %v, want %v", got, c.want)
			}
		})
	}

	t.Run("both set is an error", func(t *testing.T) {
		cmd := newCleanTestCmd()
		_ = cmd.Flags().Set("backup", "true")
		_ = cmd.Flags().Set("no-backup", "true")
		if _, err := resolveBackupFlag(cmd, true); err == nil {
			t.Fatal("resolveBackupFlag() error = nil, want a mutually-exclusive error")
		}
	})
}
