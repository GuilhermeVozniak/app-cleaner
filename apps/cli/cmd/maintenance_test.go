package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/maintenance"
)

type fakeMaintRunner struct {
	err error
}

func (f *fakeMaintRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	return "", f.err
}

type fakeMaintElevator struct {
	err error
}

func (f *fakeMaintElevator) RunElevated(ctx context.Context, script string) (string, error) {
	return "", f.err
}

// swapMaintenanceSeams replaces the package-level Runner/Elevator for one
// test and restores the originals on cleanup.
func swapMaintenanceSeams(t *testing.T, r maintenance.Runner, e maintenance.Elevator) {
	t.Helper()
	origRunner, origElevator := maintenanceRunner, maintenanceElevator
	maintenanceRunner, maintenanceElevator = r, e
	t.Cleanup(func() { maintenanceRunner, maintenanceElevator = origRunner, origElevator })
}

func TestRunMaintenanceNoFlagsPrintsHint(t *testing.T) {
	var buf bytes.Buffer
	if err := runMaintenance(context.Background(), &buf, false, false, false, false); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "No maintenance tasks specified.\nUse --dns, --purgeable, or --timemachine for maintenance tasks.\n"
	if buf.String() != want {
		t.Fatalf("out = %q, want %q", buf.String(), want)
	}
}

func TestRunMaintenanceAllTasksSucceedInOrder(t *testing.T) {
	swapMaintenanceSeams(t, &fakeMaintRunner{}, &fakeMaintElevator{})
	var buf bytes.Buffer
	if err := runMaintenance(context.Background(), &buf, false, true, true, true); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "Running Maintenance Tasks\n" + strings.Repeat("─", 50) + "\n" +
		"Running: Flush DNS Cache...\n" +
		"✓ DNS cache flushed successfully\n" +
		"Running: Free Purgeable Space...\n" +
		"✓ Purgeable space freed successfully\n" +
		"Running: Clear Time Machine Snapshots...\n" +
		"✓ No Time Machine local snapshots found\n"
	if buf.String() != want {
		t.Fatalf("out = %q, want %q", buf.String(), want)
	}
}

func TestRunMaintenanceFailureRendersErrorLine(t *testing.T) {
	swapMaintenanceSeams(t, &fakeMaintRunner{}, &fakeMaintElevator{err: errors.New("execution error: User canceled. (-128)")})
	var buf bytes.Buffer
	if err := runMaintenance(context.Background(), &buf, false, true, false, false); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "✗ Failed to flush DNS cache: execution error: User canceled. (-128)\n") {
		t.Fatalf("out = %q", buf.String())
	}
}

func TestPrintMaintenanceResultRendersAllThreeShapes(t *testing.T) {
	cases := []struct {
		name   string
		result maintenance.Result
		want   string
	}{
		{"success", maintenance.Result{Success: true, Message: "DNS cache flushed"}, "✓ DNS cache flushed\n"},
		{"failure with error detail", maintenance.Result{Success: false, Message: "Flush failed", Error: "boom"}, "✗ Flush failed: boom\n"},
		{"failure without error detail", maintenance.Result{Success: false, Message: "Flush failed"}, "✗ Flush failed\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			printMaintenanceResult(&buf, c.result)
			if buf.String() != c.want {
				t.Fatalf("printMaintenanceResult() = %q, want %q", buf.String(), c.want)
			}
		})
	}
}

// TestRunMaintenanceTaskTTYRendersSpinnerFramesBeforeTheResult drives the
// isTTY=true branch (runMaintenanceTask's default non-TTY path is already
// exercised by every other runMaintenance test): a task that outlasts one
// 80ms ticker interval must produce at least one "\r<frame> <name>" spinner
// redraw before the "\r\033[K"-cleared final result line.
func TestRunMaintenanceTaskTTYRendersSpinnerFramesBeforeTheResult(t *testing.T) {
	task := maintenanceTask{
		Name: "Slow Task",
		Run: func(ctx context.Context) maintenance.Result {
			time.Sleep(180 * time.Millisecond) // > one 80ms ticker interval
			return maintenance.Result{Success: true, Message: "done"}
		},
	}
	var buf bytes.Buffer
	runMaintenanceTask(context.Background(), &buf, true, task)
	out := buf.String()

	if !strings.Contains(out, "Slow Task") {
		t.Fatalf("out = %q, want at least one spinner frame naming the task", out)
	}
	if !strings.Contains(out, "✓ done") {
		t.Fatalf("out = %q, want the final success line", out)
	}
	if strings.Count(out, "\r") < 2 {
		t.Fatalf("out = %q, want >= 2 carriage returns (a spinner redraw plus the final clear)", out)
	}
}
