// Package uninstall implements app discovery, bundle-id resolution,
// related-path (leftover) search, the running-app check, and safe removal
// of app bundles plus their leftovers.
package uninstall

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"howett.net/plist"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/fsx"
)

// RelatedPath is a leftover file/dir associated with an app bundle.
// Size is recorded at scan time and reused for freed-space accounting.
type RelatedPath struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

var (
	bundleIDRe   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9.-]*$`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// BundleID parses <app>/Contents/Info.plist (XML or binary) and returns
// CFBundleIdentifier when it matches ^[a-zA-Z][a-zA-Z0-9.-]*$; otherwise it
// falls back to the lowercased app name with whitespace runs replaced by ".".
func BundleID(appPath string) string {
	name := strings.TrimSuffix(filepath.Base(appPath), ".app")
	fallback := whitespaceRe.ReplaceAllString(strings.ToLower(name), ".")
	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist"))
	if err != nil {
		return fallback
	}
	var info struct {
		CFBundleIdentifier string `plist:"CFBundleIdentifier"`
	}
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return fallback
	}
	id := strings.TrimSpace(info.CFBundleIdentifier)
	if bundleIDRe.MatchString(id) {
		return id
	}
	return fallback
}

// relatedTemplates are the CLI's 11 templates, relative to $HOME.
// {APP} expands with 3 variations; {BID} is always the resolved bundle id.
var relatedTemplates = []string{
	"Library/Application Support/{APP}",
	"Library/Preferences/{BID}.plist",
	"Library/Preferences/{APP}.plist",
	"Library/Caches/{APP}",
	"Library/Caches/{BID}",
	"Library/Logs/{APP}",
	"Library/Saved Application State/{BID}.savedState",
	"Library/WebKit/{APP}",
	"Library/HTTPStorages/{BID}",
	"Library/Containers/{BID}",
	"Library/Group Containers/*.{APP}",
}

// includable reports whether a candidate may be offered for deletion: it
// must resolve strictly under home and must not be a protected path.
// Existence is checked separately by the caller.
func includable(path, home string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	home = filepath.Clean(home)
	if !strings.HasPrefix(abs, home+string(filepath.Separator)) {
		return false
	}
	return !fsx.IsProtectedPath(abs)
}

// globToRegexp converts a template basename containing '*' into a
// case-insensitive anchored regexp: every regexp metachar is escaped, then
// the escaped \* becomes ".*" (exactly the CLI's matchPattern).
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	quoted := regexp.QuoteMeta(pattern)
	return regexp.Compile(`(?i)^` + strings.ReplaceAll(quoted, `\*`, `.*`) + `$`)
}

// FindRelatedPaths expands the 11 templates × 3 app-name variations
// (verbatim, lowercased, whitespace-stripped), resolves globs by readdir of
// the template's parent dir, and returns existing, home-contained,
// non-protected candidates. Candidates are deduplicated case-insensitively
// because macOS default volumes are case-insensitive (two casings of one
// path are the same file — listing both would double-count freed space).
// Sizes are computed here, at scan time, via fsx.GetSize.
func FindRelatedPaths(appName, bundleID, home string) []RelatedPath {
	home = filepath.Clean(home)
	variations := []string{
		appName,
		strings.ToLower(appName),
		whitespaceRe.ReplaceAllString(appName, ""),
	}
	seen := map[string]bool{}
	// Non-nil so the JSON bridge serializes "relatedPaths":[] (not null) for
	// apps with no leftovers — a null crashes the GUI's .length/.map/.reduce.
	out := []RelatedPath{}
	add := func(candidate string) {
		abs, err := filepath.Abs(candidate)
		if err != nil || !includable(abs, home) {
			return
		}
		key := strings.ToLower(abs)
		if seen[key] {
			return
		}
		if _, err := os.Lstat(abs); err != nil {
			return
		}
		seen[key] = true
		out = append(out, RelatedPath{Path: abs, Size: fsx.GetSize(abs)})
	}
	for _, tpl := range relatedTemplates {
		for _, v := range variations {
			rel := strings.ReplaceAll(tpl, "{APP}", v)
			rel = strings.ReplaceAll(rel, "{BID}", bundleID)
			candidate := filepath.Join(home, rel)
			if !strings.Contains(candidate, "*") {
				add(candidate)
				continue
			}
			dir := filepath.Dir(candidate)
			re, err := globToRegexp(filepath.Base(candidate))
			if err != nil {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if re.MatchString(e.Name()) {
					add(filepath.Join(dir, e.Name()))
				}
			}
		}
	}
	return out
}
