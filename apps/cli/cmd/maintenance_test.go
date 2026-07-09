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
