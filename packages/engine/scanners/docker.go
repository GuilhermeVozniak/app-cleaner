package scanners

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// Docker binary allowlist, priority order. $PATH is never consulted.
var defaultDockerCandidates = []string{
	"/usr/local/bin/docker",
	"/opt/homebrew/bin/docker",
	"/Applications/Docker.app/Contents/Resources/bin/docker",
}

// Row types accepted from `docker system df` (output allowlist / injection
// guard). "local volumes" is deliberately EXCLUDED: `docker system prune -af`
// without --volumes never frees it (spec §10.14).
var validDockerTypes = map[string]bool{
	"images":      true,
	"containers":  true,
	"build cache": true,
}

var dockerSizeRe = regexp.MustCompile(`([\d.]+)\s*([kKMGT]?B)`)

// parseDockerSize converts docker's human-readable sizes ("1.5GB",
// "346.2MB (100%)") to bytes using SI/decimal multipliers — docker prints
// decimal units (kB=1e3, MB=1e6, GB=1e9, TB=1e12); spec §10.5 fixes the
// CLI's 1024-based parsing bug. Unparseable input → 0.
func parseDockerSize(s string) int64 {
	m := dockerSizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	var mult float64
	switch strings.ToUpper(m[2]) {
	case "B":
		mult = 1
	case "KB":
		mult = 1e3
	case "MB":
		mult = 1e6
	case "GB":
		mult = 1e9
	case "TB":
		mult = 1e12
	default:
		mult = 1
	}
	return int64(value * mult)
}

type dockerScanner struct {
	candidates []string  // injectable in tests
	runner     CmdRunner // cached from the last Scan, reused by Clean
	dockerPath string    // cached from the last Scan
}

func newDockerScanner() *dockerScanner {
	return &dockerScanner{candidates: defaultDockerCandidates}
}

func init() { register(newDockerScanner()) }

func (s *dockerScanner) Category() core.Category {
	return core.Categories["docker"]
}

func (s *dockerScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	s.runner = opts.Runner
	s.dockerPath = findExecutable(s.candidates)
	if s.dockerPath == "" {
		return core.ScanResult{Category: s.Category()} // Docker not installed
	}

	out, err := opts.Runner.Run(ctx, 30*time.Second, s.dockerPath,
		"system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}")
	if err != nil {
		return core.ScanResult{Category: s.Category()} // daemon down etc. → silently empty
	}

	var items []core.CleanableItem
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(fields[0]))
		if !validDockerTypes[typ] {
			continue // drops "local volumes" and any unexpected output
		}
		size := parseDockerSize(fields[2])
		if size <= 0 {
			continue
		}
		items = append(items, core.CleanableItem{
			Path:        "docker:" + strings.ReplaceAll(typ, " ", "-"), // virtual path, e.g. docker:build-cache
			Size:        size,
			Name:        "Docker " + strings.TrimSpace(fields[0]), // original type text, e.g. "Docker Build Cache"
			IsDirectory: false,
		})
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func (s *dockerScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	if dryRun {
		return externalDryRun(s.Category(), items, progress)
	}
	if s.dockerPath == "" {
		s.dockerPath = findExecutable(s.candidates)
	}
	if s.dockerPath == "" || s.runner == nil {
		// Items have virtual docker:* paths — the filesystem cleaner must NEVER
		// receive them, so a missing binary is a hard per-category error.
		return core.CleanResult{
			Category: s.Category(),
			Errors:   []string{"Docker binary not found in safe locations"},
		}
	}

	var freed int64
	for i, it := range items {
		if progress != nil {
			progress(i+1, len(items), it)
		}
		freed += it.Size // pre-computed scan sizes
	}
	// prune -af is all-or-nothing (it cannot prune a single df row) and is
	// INTENTIONALLY invoked without --volumes to prevent data loss.
	if _, err := s.runner.Run(ctx, 60*time.Second, s.dockerPath, "system", "prune", "-af"); err != nil {
		return core.CleanResult{
			Category: s.Category(),
			Errors:   []string{"Docker cleanup failed: " + err.Error()},
		}
	}
	return core.CleanResult{Category: s.Category(), CleanedItems: len(items), FreedSpace: freed}
}
