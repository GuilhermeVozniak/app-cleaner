package uninstall

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// Runner matches maintenance.Runner's shape but is defined locally so this
// package never imports maintenance (no engine-package coupling/cycles).
type Runner interface {
	Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error)
}

// runner is the seam for the pgrep call; tests replace it.
var runner Runner = execRunner{}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, timeout time.Duration, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	return string(out), err
}

// AppInfo describes an installed app bundle and its leftovers.
type AppInfo struct {
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	BundleID     string        `json:"bundleId"`
	AppSize      int64         `json:"appSize"`
	RelatedPaths []RelatedPath `json:"relatedPaths"`
	TotalSize    int64         `json:"totalSize"`
	Running      bool          `json:"running"`
}

// Summary reports an Uninstall run.
type Summary struct {
	Uninstalled int      `json:"uninstalled"`
	FreedSpace  int64    `json:"freedSpace"`
	Errors      []string `json:"errors"`
}

// IsAppRunning reports whether any process matches the bundle path
// (pgrep -f <appPath>). pgrep exits non-zero when nothing matches.
func IsAppRunning(appPath string) bool {
	return isRunning(context.Background(), appPath)
}

func isRunning(ctx context.Context, appPath string) bool {
	out, err := runner.Run(ctx, 5*time.Second, "/usr/bin/pgrep", "-f", appPath)
	return err == nil && strings.TrimSpace(out) != ""
}

// ListApps enumerates top-level .app bundles in each existing appDir,
// resolves bundle ids, related paths, and sizes (all at scan time), and
// returns the apps sorted by TotalSize descending. Missing/unreadable dirs
// and non-directory .app entries are skipped silently, like the CLI.
func ListApps(ctx context.Context, appDirs []string, home string) []AppInfo {
	var apps []AppInfo
	for _, dir := range appDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			appPath := filepath.Join(dir, e.Name())
			st, err := os.Stat(appPath)
			if err != nil || !st.IsDir() {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".app")
			bid := BundleID(appPath)
			related := FindRelatedPaths(name, bid, home)
			appSize := fsx.GetSize(appPath)
			total := appSize
			for _, rp := range related {
				total += rp.Size
			}
			apps = append(apps, AppInfo{
				Name:         name,
				Path:         appPath,
				BundleID:     bid,
				AppSize:      appSize,
				RelatedPaths: related,
				TotalSize:    total,
				Running:      isRunning(ctx, appPath),
			})
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].TotalSize > apps[j].TotalSize })
	return apps
}

// removeOne deletes a single path through fsx.RemoveItems, which performs
// ValidatePathSafety plus the symlink-aware TOCTOU re-Lstat remove — the
// fsx-style safeRemove mandated by the spec.
func removeOne(ctx context.Context, path string, size int64) bool {
	item := core.CleanableItem{Path: path, Size: size, Name: filepath.Base(path)}
	out := fsx.RemoveItems(ctx, []core.CleanableItem{item}, false, nil)
	return len(out.Failures) == 0
}

// Uninstall removes each app bundle and then its related paths,
// sequentially. progress fires BEFORE each app, 1-based. Freed space uses
// the sizes recorded at scan time (never re-measured). A bundle failure
// records one error, SKIPS that app's related paths, and continues with the
// next app. Dry-run counts every app as uninstalled and credits TotalSize
// without touching disk.
func Uninstall(ctx context.Context, apps []AppInfo, dryRun bool, progress func(current, total int, appName string)) Summary {
	s := Summary{}
	total := len(apps)
	for i, app := range apps {
		if progress != nil {
			progress(i+1, total, app.Name)
		}
		if ctx.Err() != nil {
			break
		}
		if dryRun {
			s.Uninstalled++
			s.FreedSpace += app.TotalSize
			continue
		}
		if !removeOne(ctx, app.Path, app.AppSize) {
			s.Errors = append(s.Errors, app.Name+": Failed to remove (security check failed or permission denied)")
			continue
		}
		s.FreedSpace += app.AppSize
		for _, rp := range app.RelatedPaths {
			if removeOne(ctx, rp.Path, rp.Size) {
				s.FreedSpace += rp.Size
			}
		}
		s.Uninstalled++
	}
	return s
}
