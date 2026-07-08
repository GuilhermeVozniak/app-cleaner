// Package cmd implements the app-cleaner CLI's cobra command tree.
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/maintenance"
	"github.com/spf13/cobra"
)

// maintenanceRunner and maintenanceElevator back every maintenance task.
// Production wires the real engine implementations (mirroring the desktop
// app's app.go NewApp); tests swap both for fakes via swapMaintenanceSeams.
var (
	maintenanceRunner   maintenance.Runner   = maintenance.ExecRunner{}
	maintenanceElevator maintenance.Elevator = maintenance.OsaElevator{Runner: maintenanceRunner}
)

// maintenanceTask is one sequential step: Name labels the spinner/plain
// line, Run performs the engine call.
type maintenanceTask struct {
	Name string
	Run  func(ctx context.Context) maintenance.Result
}

func newMaintenanceCmd() *cobra.Command {
	var dns, purgeable, timemachine bool
	c := &cobra.Command{
		Use:   "maintenance",
		Short: "Run system maintenance tasks (DNS flush, purgeable space, Time Machine snapshots)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMaintenance(cmd.Context(), cmd.OutOrStdout(), isTerminalStdout(), dns, purgeable, timemachine)
		},
	}
	c.Flags().BoolVar(&dns, "dns", false, "Flush the DNS cache")
	c.Flags().BoolVar(&purgeable, "purgeable", false, "Free purgeable disk space")
	c.Flags().BoolVar(&timemachine, "timemachine", false, "Clear local Time Machine snapshots")
	return c
}

func init() {
	rootCmd.AddCommand(newMaintenanceCmd())
}

// isTerminalStdout reports whether stdout is an interactive terminal
// (stdlib-only isatty check; no new dependency).
func isTerminalStdout() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// runMaintenance is the testable core. isTTY is passed in (rather than
// detected here) so tests can force the deterministic plain-line path
// instead of depending on the test runner's own stdout.
func runMaintenance(ctx context.Context, w io.Writer, isTTY, dns, purgeable, timemachine bool) error {
	if !dns && !purgeable && !timemachine {
		_, _ = fmt.Fprintln(w, "No maintenance tasks specified.")
		_, _ = fmt.Fprintln(w, "Use --dns, --purgeable, or --timemachine for maintenance tasks.")
		return nil
	}

	var tasks []maintenanceTask
	if dns {
		tasks = append(tasks, maintenanceTask{
			Name: "Flush DNS Cache",
			Run: func(ctx context.Context) maintenance.Result {
				return maintenance.FlushDNS(ctx, maintenanceElevator)
			},
		})
	}
	if purgeable {
		tasks = append(tasks, maintenanceTask{
			Name: "Free Purgeable Space",
			Run: func(ctx context.Context) maintenance.Result {
				return maintenance.FreePurgeable(ctx, maintenanceRunner, maintenanceElevator)
			},
		})
	}
	if timemachine {
		tasks = append(tasks, maintenanceTask{
			Name: "Clear Time Machine Snapshots",
			Run: func(ctx context.Context) maintenance.Result {
				return maintenance.ClearTMSnapshots(ctx, maintenanceRunner, maintenanceElevator, nil)
			},
		})
	}

	_, _ = fmt.Fprintln(w, "Running Maintenance Tasks")
	_, _ = fmt.Fprintln(w, strings.Repeat("─", 50))

	for _, task := range tasks {
		runMaintenanceTask(ctx, w, isTTY, task)
	}
	return nil
}

func runMaintenanceTask(ctx context.Context, w io.Writer, isTTY bool, task maintenanceTask) {
	done := make(chan maintenance.Result, 1)
	go func() { done <- task.Run(ctx) }()

	if !isTTY {
		_, _ = fmt.Fprintf(w, "Running: %s...\n", task.Name)
		printMaintenanceResult(w, <-done)
		return
	}

	frames := [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case result := <-done:
			_, _ = fmt.Fprint(w, "\r\033[K")
			printMaintenanceResult(w, result)
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, "\r%s %s", frames[i%len(frames)], task.Name)
			i++
		}
	}
}

func printMaintenanceResult(w io.Writer, result maintenance.Result) {
	if result.Success {
		_, _ = fmt.Fprintf(w, "✓ %s\n", result.Message)
		return
	}
	if result.Error != "" {
		_, _ = fmt.Fprintf(w, "✗ %s: %s\n", result.Message, result.Error)
		return
	}
	_, _ = fmt.Fprintf(w, "✗ %s\n", result.Message)
}
