package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/maintenance"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Self-update: download the release .dmg, verify its signature, and swap the
// running bundle in place. External binaries (hdiutil, codesign, ditto) run
// behind the shared maintenance.Runner seam so tests use fakes.

const downloadURLBase = "https://github.com/GuilhermeVozniak/app-cleaner/releases/download"

// updateAssetURL builds the release-asset URL for a version. The version is
// validated as strict semver and the URL is assembled from constants — the
// frontend can never steer the download anywhere else.
func updateAssetURL(version string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if _, ok := parseSemver(v); !ok || strings.ContainsAny(v, "/\\") {
		return "", fmt.Errorf("invalid version %q", version)
	}
	// parseSemver tolerates a pre-release suffix; asset names don't carry one.
	if strings.ContainsAny(v, "-+") || strings.Count(v, ".") != 2 {
		return "", fmt.Errorf("invalid version %q", version)
	}
	return fmt.Sprintf("%s/v%s/app-cleaner_%s_darwin_universal.dmg", downloadURLBase, v, v), nil
}

// downloadFile streams url into dest, calling progress(done, total) as bytes
// arrive (total is -1 when Content-Length is unknown). A failed download
// never leaves a partial dest file behind.
func downloadFile(ctx context.Context, client *http.Client, url, dest string, progress func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 128*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Close()
				_ = os.Remove(dest)
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = f.Close()
			_ = os.Remove(dest)
			return rerr
		}
	}
	return f.Close()
}

// parseHdiutilMount extracts the mount point from `hdiutil attach` output:
// the last tab-separated field that is an absolute path and not a /dev node
// (real mounts land under /Volumes; tests use temp dirs).
func parseHdiutilMount(out string) (string, error) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\t")
		last := strings.TrimSpace(fields[len(fields)-1])
		if len(fields) > 1 && strings.HasPrefix(last, "/") && !strings.HasPrefix(last, "/dev/") {
			return last, nil
		}
	}
	return "", errors.New("no mount point in hdiutil output")
}

// findAppBundle returns the single .app bundle at the top of mount.
func findAppBundle(mount string) (string, error) {
	entries, err := os.ReadDir(mount)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
			return filepath.Join(mount, e.Name()), nil
		}
	}
	return "", errors.New("no .app bundle found in the disk image")
}

// bundlePathFrom maps an executable path to its enclosing .app bundle.
// Refuses non-bundle executables (dev builds) and translocated bundles
// (an in-place swap under AppTranslocation would not stick).
func bundlePathFrom(execPath string) (string, error) {
	const marker = ".app/Contents/MacOS/"
	i := strings.Index(execPath, marker)
	if i < 0 {
		return "", errors.New("not running from an app bundle (dev build?)")
	}
	bundle := execPath[:i+len(".app")]
	if strings.Contains(bundle, "/AppTranslocation/") {
		return "", errors.New("app is running translocated — move it to Applications first")
	}
	return bundle, nil
}

// installUpdate mounts dmgPath, verifies the new bundle's code signature,
// stages a copy next to appPath, and swaps it in via two renames (macOS
// allows renaming a running app). Any failure leaves appPath untouched;
// the dmg is always detached.
func installUpdate(ctx context.Context, r maintenance.Runner, dmgPath, appPath string) error {
	out, err := r.Run(ctx, 60*time.Second, "/usr/bin/hdiutil", "attach", "-nobrowse", "-readonly", dmgPath)
	if err != nil {
		return fmt.Errorf("could not mount the update image: %w", err)
	}
	mount, err := parseHdiutilMount(out)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = r.Run(ctx, 60*time.Second, "/usr/bin/hdiutil", "detach", mount)
	}()

	newApp, err := findAppBundle(mount)
	if err != nil {
		return err
	}
	if _, err := r.Run(ctx, 120*time.Second, "/usr/bin/codesign", "--verify", "--deep", "--strict", newApp); err != nil {
		return fmt.Errorf("the downloaded update failed signature verification: %w", err)
	}

	// Stage on the same volume as appPath so the swap renames never cross
	// filesystems. ditto preserves signatures/xattrs where cp does not.
	parent := filepath.Dir(appPath)
	staging := filepath.Join(parent, "."+filepath.Base(appPath)+".update")
	previous := filepath.Join(parent, "."+filepath.Base(appPath)+".previous")
	_ = os.RemoveAll(staging)
	if _, err := r.Run(ctx, 300*time.Second, "/usr/bin/ditto", newApp, staging); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("could not stage the update: %w", err)
	}
	if st, err := os.Stat(staging); err != nil || !st.IsDir() {
		_ = os.RemoveAll(staging)
		return errors.New("could not stage the update: staging copy missing")
	}

	_ = os.RemoveAll(previous)
	if err := os.Rename(appPath, previous); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("could not replace the app: %w", err)
	}
	if err := os.Rename(staging, appPath); err != nil {
		// Roll the old bundle back so the user still has a working app.
		_ = os.Rename(previous, appPath)
		_ = os.RemoveAll(staging)
		return fmt.Errorf("could not install the update: %w", err)
	}
	_ = os.RemoveAll(previous)
	return nil
}

// ---------------------------------------------------------------------------
// Wails bindings
// ---------------------------------------------------------------------------

// updateDmgPath is where DownloadUpdate stores the dmg for InstallUpdate /
// OpenDownloadedUpdate. Guarded by a.mu.

// DownloadUpdate downloads the release dmg for version, emitting
// update:progress events {done, total}. Returns the downloaded path.
func (a *App) DownloadUpdate(version string) (string, error) {
	url, err := updateAssetURL(version)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(os.TempDir(), fmt.Sprintf("app-cleaner-update-%s.dmg", strings.TrimPrefix(version, "v")))
	client := &http.Client{Timeout: 15 * time.Minute}
	var lastEmit time.Time
	err = downloadFile(a.ctx, client, url, dest, func(done, total int64) {
		// Throttle events: dmg downloads are ~10MB+, one event per chunk is noise.
		if time.Since(lastEmit) < 100*time.Millisecond && done != total {
			return
		}
		lastEmit = time.Now()
		wruntime.EventsEmit(a.ctx, "update:progress", map[string]any{"done": done, "total": total})
	})
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.updateDmgPath = dest
	a.mu.Unlock()
	return dest, nil
}

// InstallUpdate swaps the running bundle for the one in the downloaded dmg
// and relaunches. Only returns on failure (success quits the app).
func (a *App) InstallUpdate() error {
	a.mu.Lock()
	dmg := a.updateDmgPath
	a.mu.Unlock()
	if dmg == "" {
		return errors.New("no update downloaded")
	}
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	appPath, err := bundlePathFrom(execPath)
	if err != nil {
		return err
	}
	if err := installUpdate(a.ctx, a.runner, dmg, appPath); err != nil {
		return err
	}
	_ = os.Remove(dmg)
	// Relaunch detached: `open` re-resolves the bundle we just swapped in.
	_ = exec.Command("/usr/bin/open", appPath).Start()
	wruntime.Quit(a.ctx)
	return nil
}

// OpenDownloadedUpdate reveals the downloaded dmg (manual-install fallback
// when the in-place swap is refused, e.g. translocated or dev builds).
func (a *App) OpenDownloadedUpdate() error {
	a.mu.Lock()
	dmg := a.updateDmgPath
	a.mu.Unlock()
	if dmg == "" {
		return errors.New("no update downloaded")
	}
	return exec.CommandContext(a.ctx, "/usr/bin/open", dmg).Run()
}
