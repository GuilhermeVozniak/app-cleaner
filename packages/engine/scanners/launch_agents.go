package scanners

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// Program paths under these prefixes are assumed managed by the system or a
// package manager — never reported as orphaned (CLI SYSTEM_BINARY_PREFIXES).
var systemBinaryPrefixes = []string{
	"/usr/bin/", "/bin/", "/sbin/", "/usr/sbin/", "/usr/local/bin/", "/opt/homebrew/bin/",
}

type launchAgentsScanner struct{}

func newLaunchAgentsScanner() *launchAgentsScanner { return &launchAgentsScanner{} }

func init() { register(newLaunchAgentsScanner()) }

func (s *launchAgentsScanner) Category() core.Category {
	return core.Categories["launch-agents"]
}

// launchAgentProgram extracts the target program path from a LaunchAgent
// plist (XML or binary — spec §10.8 replaces the CLI's regex-on-text):
// Program (string) wins; otherwise ProgramArguments[0]. "" = none found
// or unparseable (caller skips silently, CLI parity).
func launchAgentProgram(plistPath string) string {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return ""
	}
	var payload struct {
		Program          string   `plist:"Program"`
		ProgramArguments []string `plist:"ProgramArguments"`
	}
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if p := strings.TrimSpace(payload.Program); p != "" {
		return p
	}
	if len(payload.ProgramArguments) > 0 {
		return strings.TrimSpace(payload.ProgramArguments[0])
	}
	return ""
}

func hasSystemBinaryPrefix(p string) bool {
	for _, prefix := range systemBinaryPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func (s *launchAgentsScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	dir := filepath.Join(opts.Roots.Home, "Library", "LaunchAgents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return core.ScanResult{Category: s.Category()} // no LaunchAgents dir → nothing to report
		}
		return core.ScanResult{
			Category: s.Category(),
			Error:    fmt.Sprintf("Failed to read LaunchAgents directory: %v", err),
		}
	}

	var items []core.CleanableItem
	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		plistPath := filepath.Join(dir, e.Name())

		program := launchAgentProgram(plistPath)
		if program == "" || !filepath.IsAbs(program) || hasSystemBinaryPrefix(program) {
			continue
		}
		if _, err := os.Stat(program); err == nil {
			continue // target exists → not orphaned
		}

		info, err := e.Info()
		if err != nil {
			continue // per-plist errors skip silently
		}
		mod := info.ModTime()
		items = append(items, core.CleanableItem{
			Path:        plistPath,
			Size:        info.Size(),
			Name:        fmt.Sprintf("%s → %s (missing)", e.Name(), program),
			IsDirectory: false,
			ModifiedAt:  &mod,
		})
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func (s *launchAgentsScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	// Deletes the .plist only — deliberately no `launchctl bootout` (CLI parity);
	// the agent stays loaded until logout/reboot.
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
