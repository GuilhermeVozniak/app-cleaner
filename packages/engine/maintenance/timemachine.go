package maintenance

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// snapshotDateRe is a strict injection guard: only lines shaped like
// 2024-01-15-123456 are accepted as snapshot dates and interpolated into
// the elevated script.
var snapshotDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{6}$`)

// ClearTMSnapshots lists local Time Machine snapshots unprivileged, then
// deletes them all in ONE elevated invocation (one password prompt): the
// elevated shell script loops over the pre-validated dates and prints one
// "OK <date>" or "ERR <date>" line per date, parsed here for per-date
// progress and errors. A date with no status line counts as failed.
func ClearTMSnapshots(ctx context.Context, r Runner, e Elevator, progress func(done, total int, date, errMsg string)) Result {
	out, err := r.Run(ctx, 30*time.Second, "/usr/bin/tmutil", "listlocalsnapshotdates")
	if err != nil {
		return Result{
			Success: false,
			Message: "Failed to list Time Machine snapshots",
			Error:   "tmutil not available or Time Machine is not configured on this Mac",
		}
	}
	var dates []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if snapshotDateRe.MatchString(line) {
			dates = append(dates, line)
		}
	}
	if len(dates) == 0 {
		return Result{Success: true, Message: "No Time Machine local snapshots found"}
	}

	script := "for d in " + strings.Join(dates, " ") +
		`; do /usr/bin/tmutil deletelocalsnapshots $d && echo "OK $d" || echo "ERR $d"; done`
	stdout, err := e.RunElevated(ctx, script)
	if err != nil {
		return Result{
			Success:       false,
			Message:       "Failed to delete Time Machine snapshots",
			Error:         err.Error(),
			RequiresAdmin: userCanceled(err),
		}
	}

	status := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if date, ok := strings.CutPrefix(line, "OK "); ok && snapshotDateRe.MatchString(date) {
			status[date] = "OK"
			continue
		}
		if date, ok := strings.CutPrefix(line, "ERR "); ok && snapshotDateRe.MatchString(date) {
			status[date] = "ERR"
		}
	}

	total := len(dates)
	var errs []string
	for i, d := range dates {
		errMsg := ""
		if status[d] != "OK" { // "ERR" or no status line at all
			errMsg = d + ": failed to delete snapshot"
			errs = append(errs, errMsg)
		}
		if progress != nil {
			progress(i+1, total, d, errMsg)
		}
	}
	deleted := total - len(errs)
	switch {
	case deleted == 0:
		return Result{Success: false, Message: "Failed to delete Time Machine snapshots", Error: errs[0]}
	case len(errs) > 0:
		return Result{Success: true, Message: fmt.Sprintf("Deleted %d/%d Time Machine snapshots (%d error(s))", deleted, total, len(errs))}
	default:
		plural := "s"
		if deleted == 1 {
			plural = ""
		}
		return Result{Success: true, Message: fmt.Sprintf("Deleted %d Time Machine snapshot%s", deleted, plural)}
	}
}
