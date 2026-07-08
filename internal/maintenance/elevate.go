package maintenance

import (
	"context"
	"strings"
	"time"
)

// elevatedTimeout is generous because the user must type a password in the
// native macOS auth dialog before the shell script even starts.
const elevatedTimeout = 120 * time.Second

// OsaElevator runs a shell script with admin rights via
//
//	osascript -e 'do shell script "<script>" with administrator privileges'
//
// The script is embedded in an AppleScript string literal, so backslashes
// and double quotes are escaped. Callers only ever pass fixed literals plus
// regex-validated snapshot dates — never user-controlled text.
type OsaElevator struct{ Runner Runner }

func (e OsaElevator) RunElevated(ctx context.Context, shellScript string) (string, error) {
	escaped := strings.ReplaceAll(shellScript, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	stmt := `do shell script "` + escaped + `" with administrator privileges`
	return e.Runner.Run(ctx, elevatedTimeout, "/usr/bin/osascript", "-e", stmt)
}
