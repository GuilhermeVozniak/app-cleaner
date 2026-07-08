package scanners

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/guhcostan/app-cleaner/internal/core"
	"github.com/guhcostan/app-cleaner/internal/fsx"
)

type languageFilesScanner struct {
	// preferred returns the user's system preferred-language tags
	// (e.g. ["en-US", "pt-BR"]). Injectable in tests; the default reads
	// AppleLanguages from the user's GlobalPreferences plist.
	preferred func(home string) []string
}

func newLanguageFilesScanner() *languageFilesScanner {
	return &languageFilesScanner{preferred: readAppleLanguages}
}

func init() { register(newLanguageFilesScanner()) }

// readAppleLanguages parses <home>/Library/Preferences/.GlobalPreferences.plist
// (binary or XML) and returns the AppleLanguages array; nil on any error.
// No subprocess is spawned (engine rule: no shells, minimal exec surface).
func readAppleLanguages(home string) []string {
	data, err := os.ReadFile(filepath.Join(home, "Library", "Preferences", ".GlobalPreferences.plist"))
	if err != nil {
		return nil
	}
	var prefs struct {
		AppleLanguages []string `plist:"AppleLanguages"`
	}
	if _, err := plist.Unmarshal(data, &prefs); err != nil {
		return nil
	}
	return prefs.AppleLanguages
}

// keepLanguageSet expands each preferred tag into {verbatim, '-'→'_' variant,
// base language} and unions {"en", "Base"} plus the config keep-list (verbatim).
// Matching is case-sensitive (spec §5 / §10.6).
func keepLanguageSet(tags, cfgKeep []string) map[string]bool {
	keep := map[string]bool{"en": true, "Base": true}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		keep[tag] = true
		keep[strings.ReplaceAll(tag, "-", "_")] = true
		base := tag
		if i := strings.IndexAny(tag, "-_"); i > 0 {
			base = tag[:i]
		}
		keep[base] = true
	}
	for _, k := range cfgKeep {
		if k != "" {
			keep[k] = true
		}
	}
	return keep
}

func (s *languageFilesScanner) Category() core.Category {
	return core.Categories["language-files"]
}

func (s *languageFilesScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	var tags []string
	if s.preferred != nil {
		tags = s.preferred(opts.Roots.Home)
	}
	keep := keepLanguageSet(tags, opts.Cfg.KeepLanguages)

	apps, err := os.ReadDir(opts.Roots.Applications)
	if err != nil {
		return core.ScanResult{Category: s.Category()} // unreadable → empty, silent (CLI parity)
	}

	var items []core.CleanableItem
	for _, app := range apps {
		if ctx.Err() != nil {
			break
		}
		if !strings.HasSuffix(app.Name(), ".app") {
			continue // top-level *.app bundles only
		}
		resources := filepath.Join(opts.Roots.Applications, app.Name(), "Contents", "Resources")
		entries, err := os.ReadDir(resources)
		if err != nil {
			continue // per-app errors silently skipped
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".lproj") {
				continue
			}
			lang := strings.TrimSuffix(e.Name(), ".lproj")
			if keep[lang] {
				continue
			}
			full := filepath.Join(resources, e.Name())
			info, err := os.Stat(full)
			if err != nil {
				continue // per-lproj errors silently skipped
			}
			mod := info.ModTime()
			items = append(items, core.CleanableItem{
				Path:        full,
				Size:        fsx.GetSize(full),
				Name:        app.Name() + ": " + e.Name(), // e.g. "Slack.app: fr.lproj"
				IsDirectory: true,
				ModifiedAt:  &mod,
			})
		}
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	return core.ScanResult{Category: s.Category(), Items: items, TotalSize: total}
}

func (s *languageFilesScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(s.Category(), ctx, items, dryRun, progress)
}
