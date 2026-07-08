package maintenance

import (
	"context"
	"strings"
	"time"
)

// FreePurgeable invokes /usr/sbin/purge. It tries unprivileged FIRST (it
// usually works on modern macOS — spec change #4); only a permission-style
// failure triggers the elevated fallback.
func FreePurgeable(ctx context.Context, r Runner, e Elevator) Result {
	_, err := r.Run(ctx, 60*time.Second, "/usr/sbin/purge")
	if err == nil {
		return Result{Success: true, Message: "Purgeable space freed successfully"}
	}
	msg := err.Error()
	if !strings.Contains(msg, "Operation not permitted") && !strings.Contains(msg, "Permission denied") {
		return Result{Success: false, Message: "Failed to free purgeable space", Error: msg}
	}
	if _, eerr := e.RunElevated(ctx, "/usr/sbin/purge"); eerr != nil {
		return Result{
			Success:       false,
			Message:       "Failed to free purgeable space",
			Error:         eerr.Error(),
			RequiresAdmin: userCanceled(eerr),
		}
	}
	return Result{Success: true, Message: "Purgeable space freed successfully"}
}
