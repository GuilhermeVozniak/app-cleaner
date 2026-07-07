package uninstall

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

// AppIcon returns the app's icon as a base64-encoded PNG, or "" on ANY
// failure — icons are cosmetic chrome, fetched lazily by the bridge's
// GetAppIcon, so errors never propagate. The icon name comes from
// CFBundleIconFile in Contents/Info.plist (default "AppIcon" when the
// plist or key is missing; ".icns" appended when the name has no
// extension). The .icns is converted to PNG with /usr/bin/sips into
// cacheDir, keyed by the sha1 of the bundle path; an existing cached PNG
// skips the conversion entirely.
func AppIcon(ctx context.Context, r Runner, appPath, cacheDir string) string {
	name := "AppIcon"
	if data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist")); err == nil {
		var info struct {
			CFBundleIconFile string `plist:"CFBundleIconFile"`
		}
		if _, err := plist.Unmarshal(data, &info); err == nil {
			if v := strings.TrimSpace(info.CFBundleIconFile); v != "" {
				name = v
			}
		}
	}
	if filepath.Ext(name) == "" {
		name += ".icns"
	}
	icnsPath := filepath.Join(appPath, "Contents", "Resources", name)
	if _, err := os.Stat(icnsPath); err != nil {
		return ""
	}
	sum := sha1.Sum([]byte(appPath))
	pngPath := filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".png")
	if _, err := os.Stat(pngPath); err != nil {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return ""
		}
		if _, err := r.Run(ctx, 10*time.Second, "/usr/bin/sips", "-s", "format", "png", icnsPath, "--out", pngPath); err != nil {
			return ""
		}
	}
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}
