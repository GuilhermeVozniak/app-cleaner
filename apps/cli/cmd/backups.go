package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/backup"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/config"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/spf13/cobra"
)

// backupManagerFor and backupHome are the seams production wires to
// backup.NewManager(home) / os.UserHomeDir(); tests replace both to point
// at a temp backup root instead of the real $HOME.
var backupManagerFor = func(home string) *backup.Manager {
	return backup.NewManager(home)
}

var backupHome = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return h
}

func newBackupsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "backups",
		Short: "List, restore, or prune cleanup backups",
	}
	c.AddCommand(newBackupsListCmd(), newBackupsRestoreCmd(), newBackupsCleanOldCmd())
	return c
}

func init() {
	rootCmd.AddCommand(newBackupsCmd())
}

func newBackupsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List backup sessions, newest first",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupsList(cmd.OutOrStdout())
		},
	}
}

func runBackupsList(w io.Writer) error {
	infos := backupManagerFor(backupHome()).List()
	if len(infos) == 0 {
		_, _ = fmt.Fprintln(w, "No backups found.")
		return nil
	}
	_, _ = fmt.Fprintf(w, "%-20s %-10s %s\n", "DATE", "SIZE", "PATH")
	for _, info := range infos {
		_, _ = fmt.Fprintf(w, "%-20s %-10s %s\n", info.Date.Format("2006-01-02 15:04:05"), core.FormatSize(info.Size), info.Path)
	}
	return nil
}

func newBackupsRestoreCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "restore <path>",
		Short: "Restore a backup session back to its original location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupsRestore(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], yes)
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return c
}

func runBackupsRestore(in io.Reader, w io.Writer, sessionPath string, yes bool) error {
	if !yes && !promptConfirm(in, w, fmt.Sprintf("Restore backup at %s?", sessionPath), false) {
		_, _ = fmt.Fprintln(w, "Restore cancelled.")
		return nil
	}
	home := backupHome()
	result := backupManagerFor(home).Restore(sessionPath, home)
	_, _ = fmt.Fprintf(w, "Restored: %d\n", result.Restored)
	_, _ = fmt.Fprintf(w, "Failed: %d\n", result.Failed)
	for _, e := range result.Errors {
		_, _ = fmt.Fprintf(w, "  %s\n", e)
	}
	if result.Failed > 0 {
		return fmt.Errorf("restore completed with %d failure(s)", result.Failed)
	}
	return nil
}

func newBackupsCleanOldCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean-old",
		Short: "Remove backup sessions past the configured retention window",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupsCleanOld(cmd.OutOrStdout())
		},
	}
}

func runBackupsCleanOld(w io.Writer) error {
	home := backupHome()
	cfg := config.Load(config.DefaultPath(home))
	removed := backupManagerFor(home).CleanOld(cfg.BackupRetentionDays)
	_, _ = fmt.Fprintf(w, "Removed %d old backup session(s).\n", removed)
	return nil
}
