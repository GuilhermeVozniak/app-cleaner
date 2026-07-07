package maintenance

import "context"

// FlushDNS flushes the macOS DNS cache. It always requires admin rights, so
// both commands run in ONE elevated shell script (one password prompt).
func FlushDNS(ctx context.Context, e Elevator) Result {
	_, err := e.RunElevated(ctx, "/usr/bin/dscacheutil -flushcache && /usr/bin/killall -HUP mDNSResponder")
	if err != nil {
		return Result{
			Success:       false,
			Message:       "Failed to flush DNS cache",
			Error:         err.Error(),
			RequiresAdmin: userCanceled(err),
		}
	}
	return Result{Success: true, Message: "DNS cache flushed successfully"}
}
