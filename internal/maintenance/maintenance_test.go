package maintenance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type runnerCall struct {
	bin     string
	args    []string
	timeout time.Duration
}

type fakeRunner struct {
	out   string
	err   error
	calls []runnerCall
}

func (f *fakeRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	f.calls = append(f.calls, runnerCall{bin: bin, args: args, timeout: timeout})
	return f.out, f.err
}

type fakeElevator struct {
	out     string
	err     error
	scripts []string
}

func (f *fakeElevator) RunElevated(ctx context.Context, script string) (string, error) {
	f.scripts = append(f.scripts, script)
	return f.out, f.err
}

func TestOsaElevatorBuildsEscapedOsascriptCall(t *testing.T) {
	r := &fakeRunner{out: "done"}
	e := OsaElevator{Runner: r}
	out, err := e.RunElevated(context.Background(), `echo "OK $d" || echo "ERR $d"`)
	if err != nil || out != "done" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(r.calls))
	}
	call := r.calls[0]
	if call.bin != "/usr/bin/osascript" {
		t.Fatalf("bin = %q", call.bin)
	}
	if call.timeout != elevatedTimeout {
		t.Fatalf("timeout = %v, want %v", call.timeout, elevatedTimeout)
	}
	wantStmt := `do shell script "echo \"OK $d\" || echo \"ERR $d\"" with administrator privileges`
	if len(call.args) != 2 || call.args[0] != "-e" || call.args[1] != wantStmt {
		t.Fatalf("args = %q, want [-e %q]", call.args, wantStmt)
	}
}

func TestUserCanceledDetection(t *testing.T) {
	if !userCanceled(errTest("execution error: User canceled. (-128)")) {
		t.Fatal("osascript cancel message must be detected")
	}
	if userCanceled(errTest("some other failure")) {
		t.Fatal("unrelated errors are not a cancel")
	}
	if userCanceled(nil) {
		t.Fatal("nil error is not a cancel")
	}
}

// errTest is a tiny helper so tests read cleanly.
type errTest string

func (e errTest) Error() string { return string(e) }

func TestFlushDNSSuccess(t *testing.T) {
	e := &fakeElevator{}
	res := FlushDNS(context.Background(), e)
	if !res.Success || res.Message != "DNS cache flushed successfully" || res.Error != "" {
		t.Fatalf("result = %+v", res)
	}
	wantScript := "/usr/bin/dscacheutil -flushcache && /usr/bin/killall -HUP mDNSResponder"
	if len(e.scripts) != 1 || e.scripts[0] != wantScript {
		t.Fatalf("scripts = %q, want [%q] (both commands, ONE prompt)", e.scripts, wantScript)
	}
}

func TestFlushDNSUserCanceled(t *testing.T) {
	e := &fakeElevator{err: errors.New("execution error: User canceled. (-128)")}
	res := FlushDNS(context.Background(), e)
	if res.Success || res.Message != "Failed to flush DNS cache" {
		t.Fatalf("result = %+v", res)
	}
	if !res.RequiresAdmin || !strings.Contains(res.Error, "User canceled") {
		t.Fatalf("cancel must set RequiresAdmin and keep the error: %+v", res)
	}
}

func TestFreePurgeablePlainFirst(t *testing.T) {
	r := &fakeRunner{}
	e := &fakeElevator{}
	res := FreePurgeable(context.Background(), r, e)
	if !res.Success || res.Message != "Purgeable space freed successfully" {
		t.Fatalf("result = %+v", res)
	}
	if len(e.scripts) != 0 {
		t.Fatal("must NOT elevate when the plain run succeeds")
	}
	if len(r.calls) != 1 || r.calls[0].bin != "/usr/sbin/purge" || len(r.calls[0].args) != 0 {
		t.Fatalf("plain call = %+v, want /usr/sbin/purge with no args", r.calls)
	}
	if r.calls[0].timeout != 60*time.Second {
		t.Fatalf("timeout = %v, want 60s", r.calls[0].timeout)
	}
}

func TestFreePurgeableElevatesOnPermissionFailure(t *testing.T) {
	r := &fakeRunner{err: errors.New("purge: Operation not permitted")}
	e := &fakeElevator{}
	res := FreePurgeable(context.Background(), r, e)
	if !res.Success || res.Message != "Purgeable space freed successfully" {
		t.Fatalf("result = %+v", res)
	}
	if len(e.scripts) != 1 || e.scripts[0] != "/usr/sbin/purge" {
		t.Fatalf("elevated scripts = %q, want [/usr/sbin/purge]", e.scripts)
	}
}

func TestFreePurgeableNonPermissionFailureDoesNotElevate(t *testing.T) {
	r := &fakeRunner{err: errors.New("some other failure")}
	e := &fakeElevator{}
	res := FreePurgeable(context.Background(), r, e)
	if res.Success || res.Message != "Failed to free purgeable space" || res.Error != "some other failure" {
		t.Fatalf("result = %+v", res)
	}
	if res.RequiresAdmin {
		t.Fatal("non-permission failure must not claim admin is required")
	}
	if len(e.scripts) != 0 {
		t.Fatal("must not elevate on a non-permission failure")
	}
}

func TestClearTMSnapshotsFiltersGarbageAndRunsOneElevatedCall(t *testing.T) {
	r := &fakeRunner{out: "Snapshot dates for all disks:\n2024-01-15-123456\nnot-a-date\n2024-01-16-654321\n\ncom.apple.TimeMachine.2024-01-17\n"}
	e := &fakeElevator{out: "OK 2024-01-15-123456\nOK 2024-01-16-654321\n"}
	var prog []string
	res := ClearTMSnapshots(context.Background(), r, e, func(done, total int, date, errMsg string) {
		prog = append(prog, fmt.Sprintf("%d/%d %s %q", done, total, date, errMsg))
	})
	if !res.Success || res.Message != "Deleted 2 Time Machine snapshots" {
		t.Fatalf("result = %+v", res)
	}
	if len(r.calls) != 1 || r.calls[0].bin != "/usr/bin/tmutil" || r.calls[0].args[0] != "listlocalsnapshotdates" {
		t.Fatalf("list call = %+v", r.calls)
	}
	if r.calls[0].timeout != 30*time.Second {
		t.Fatalf("list timeout = %v, want 30s", r.calls[0].timeout)
	}
	if len(e.scripts) != 1 {
		t.Fatalf("elevated invocations = %d, want exactly 1 (one password prompt)", len(e.scripts))
	}
	script := e.scripts[0]
	if !strings.HasPrefix(script, "for d in 2024-01-15-123456 2024-01-16-654321; do ") {
		t.Fatalf("script = %q", script)
	}
	if !strings.Contains(script, `/usr/bin/tmutil deletelocalsnapshots $d && echo "OK $d" || echo "ERR $d"; done`) {
		t.Fatalf("script = %q", script)
	}
	if strings.Contains(script, "not-a-date") || strings.Contains(script, "com.apple.TimeMachine") {
		t.Fatal("garbage lines must never reach the elevated script")
	}
	wantProg := []string{`1/2 2024-01-15-123456 ""`, `2/2 2024-01-16-654321 ""`}
	if len(prog) != 2 || prog[0] != wantProg[0] || prog[1] != wantProg[1] {
		t.Fatalf("progress = %v, want %v", prog, wantProg)
	}
}

func TestClearTMSnapshotsNoSnapshots(t *testing.T) {
	r := &fakeRunner{out: "Snapshot dates for all disks:\nnothing here\n"}
	e := &fakeElevator{}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if !res.Success || res.Message != "No Time Machine local snapshots found" {
		t.Fatalf("result = %+v", res)
	}
	if len(e.scripts) != 0 {
		t.Fatal("must not elevate when there is nothing to delete")
	}
}

func TestClearTMSnapshotsPartialSuccess(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n2024-01-16-654321\n"}
	e := &fakeElevator{out: "OK 2024-01-15-123456\nERR 2024-01-16-654321\n"}
	var errMsgs []string
	res := ClearTMSnapshots(context.Background(), r, e, func(done, total int, date, errMsg string) {
		errMsgs = append(errMsgs, errMsg)
	})
	if !res.Success || res.Message != "Deleted 1/2 Time Machine snapshots (1 error(s))" {
		t.Fatalf("result = %+v", res)
	}
	if len(errMsgs) != 2 || errMsgs[0] != "" || errMsgs[1] == "" {
		t.Fatalf("per-date errMsgs = %q", errMsgs)
	}
}

func TestClearTMSnapshotsTotalFailure(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n2024-01-16-654321\n"}
	e := &fakeElevator{out: "ERR 2024-01-15-123456\nERR 2024-01-16-654321\n"}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if res.Success || res.Message != "Failed to delete Time Machine snapshots" {
		t.Fatalf("result = %+v", res)
	}
	if res.Error != "2024-01-15-123456: failed to delete snapshot" {
		t.Fatalf("Error = %q, want the FIRST per-date error", res.Error)
	}
}

func TestClearTMSnapshotsSingularMessage(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n"}
	e := &fakeElevator{out: "OK 2024-01-15-123456\n"}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if !res.Success || res.Message != "Deleted 1 Time Machine snapshot" {
		t.Fatalf("result = %+v (singular, no trailing 's')", res)
	}
}

func TestClearTMSnapshotsListFailure(t *testing.T) {
	r := &fakeRunner{err: errors.New("boom")}
	e := &fakeElevator{}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if res.Success || res.Message != "Failed to list Time Machine snapshots" {
		t.Fatalf("result = %+v", res)
	}
	if res.Error != "tmutil not available or Time Machine is not configured on this Mac" {
		t.Fatalf("Error = %q", res.Error)
	}
}

func TestClearTMSnapshotsElevationCanceled(t *testing.T) {
	r := &fakeRunner{out: "2024-01-15-123456\n"}
	e := &fakeElevator{err: errors.New("execution error: User canceled. (-128)")}
	res := ClearTMSnapshots(context.Background(), r, e, nil)
	if res.Success || res.Message != "Failed to delete Time Machine snapshots" || !res.RequiresAdmin {
		t.Fatalf("result = %+v", res)
	}
}
