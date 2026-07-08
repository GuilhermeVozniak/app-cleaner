// Package cmd implements the apps/cli cobra command tree: the bare root
// command (interactive TUI), plus scan/clean/uninstall/maintenance/backups
// subcommands added by later tasks in this package.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "app-cleaner",
	Short: "Reclaim disk space on your Mac",
	Long: "App Cleaner scans, cleans, and maintains your Mac.\n" +
		"Run with no arguments for the interactive picker, or use a\n" +
		"subcommand (scan, clean, uninstall, maintenance, backups) for\n" +
		"scriptable, non-interactive use.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunInteractive(cmd, args)
	},
}

// RunInteractive launches the full-screen interactive TUI: category picker
// -> file picker -> confirm -> clean -> results (design spec section 8.2/8.3).
// Task 10 reassigns this var to the real Bubble Tea program. The signature
// mirrors cobra.Command.RunE so rootCmd can delegate to it directly.
// Interactive-mode flags (e.g. --risky, --file-picker) are added to rootCmd
// by Task 10 and read from cmd.Flags() inside the real implementation — this
// stub takes none and does nothing.
var RunInteractive = func(cmd *cobra.Command, args []string) error {
	return nil
}

// Execute runs the root command and its subcommand tree. version is baked in
// by main.go — "dev" locally, or the release tag via
// -ldflags "-X main.version=...". On error it prints to stderr and exits 1.
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
