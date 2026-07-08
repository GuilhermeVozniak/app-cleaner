package scanners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/config"
)

func lfMkFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKeepLanguageSet(t *testing.T) {
	keep := keepLanguageSet([]string{"pt-BR", "zh_Hans"}, []string{"ja"})
	for _, want := range []string{"en", "Base", "pt-BR", "pt_BR", "pt", "zh_Hans", "zh", "ja"} {
		if !keep[want] {
			t.Errorf("keep-set missing %q (got %v)", want, keep)
		}
	}
	if keep["fr"] || keep["it"] {
		t.Errorf("keep-set unexpectedly contains fr/it: %v", keep)
	}
	// Matching is case-sensitive (spec §5 / §10.6): the lowercase built-in
	// "en" must not also imply "EN", and expanding "pt-BR" must not imply
	// the all-caps variant "PT_BR".
	if keep["EN"] {
		t.Errorf("keep-set unexpectedly contains EN (case-insensitive match): %v", keep)
	}
	if keep["PT_BR"] {
		t.Errorf("keep-set unexpectedly contains PT_BR (case-insensitive match): %v", keep)
	}
}

func TestLanguageFilesScanner(t *testing.T) {
	apps := t.TempDir()
	res := filepath.Join(apps, "Slack.app", "Contents", "Resources")
	// Create various language .lproj directories
	for _, lang := range []string{"en", "Base", "fr", "de", "pt", "pt_BR", "pt-BR", "it"} {
		lfMkFile(t, filepath.Join(res, lang+".lproj", "Localizable.strings"), "strings for "+lang)
	}
	// Non-.lproj resource entries are ignored.
	lfMkFile(t, filepath.Join(res, "AppIcon.icns"), "icon")
	// App bundle without Contents/Resources → silently skipped.
	if err := os.MkdirAll(filepath.Join(apps, "Bare.app", "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Entries not ending in .app are ignored entirely.
	lfMkFile(t, filepath.Join(apps, "NotAnApp", "Contents", "Resources", "es.lproj", "x.strings"), "x")

	s := newLanguageFilesScanner()
	// Test seam: inject the system preferred-language tags.
	s.preferred = func(home string) []string { return []string{"pt-BR"} }

	cfg := config.Default()
	cfg.KeepLanguages = []string{"de"}

	result := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir(), Applications: apps}, Cfg: cfg})
	if result.Error != "" {
		t.Fatalf("unexpected scan error: %q", result.Error)
	}

	// keep-set = {en, Base} ∪ expand(pt-BR)={pt-BR, pt_BR, pt} ∪ config {de}.
	// Expected to exclude: fr, it (not in keep-set)
	got := map[string]string{} // name → path
	for _, it := range result.Items {
		got[it.Name] = it.Path
		if !it.IsDirectory {
			t.Errorf("%s: IsDirectory = false, want true", it.Name)
		}
		if it.Size <= 0 {
			t.Errorf("%s: Size = %d, want > 0", it.Name, it.Size)
		}
		if it.ModifiedAt == nil {
			t.Errorf("%s: ModifiedAt is nil", it.Name)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d items %v, want exactly 2 (fr.lproj and it.lproj)", len(got), got)
	}
	if p := got["Slack.app: fr.lproj"]; p != filepath.Join(res, "fr.lproj") {
		t.Errorf("fr item path = %q, want %q", p, filepath.Join(res, "fr.lproj"))
	}
	if _, ok := got["Slack.app: it.lproj"]; !ok {
		t.Errorf("it.lproj should be listed as not in keep-set (got %v)", got)
	}

	var sum int64
	for _, it := range result.Items {
		sum += it.Size
	}
	if result.TotalSize != sum {
		t.Errorf("TotalSize = %d, want %d", result.TotalSize, sum)
	}
}

func TestLanguageFilesScannerNonexistentApplicationsRoot(t *testing.T) {
	s := newLanguageFilesScanner()
	s.preferred = func(home string) []string { return nil }

	opts := Options{
		Roots: Roots{Home: t.TempDir(), Applications: filepath.Join(t.TempDir(), "nonexistent-applications")},
		Cfg:   config.Default(),
	}
	result := s.Scan(context.Background(), opts)

	if result.Error != "" || len(result.Items) != 0 || result.TotalSize != 0 {
		t.Fatalf("nonexistent Applications root: got %+v, want empty ScanResult, no error", result)
	}
}

// TestLanguageFilesScannerCaseSensitiveKeepLanguages proves KeepLanguages
// matching is case-sensitive: an uppercase config entry must NOT keep a
// lowercase on-disk .lproj directory of the "same" language (spec §5 /
// §10.6). Uses its own temp dir/fixture (rather than reusing
// TestLanguageFilesScanner's) to avoid two same-parent dirs differing only
// by case, which macOS's case-insensitive filesystem can't represent.
func TestLanguageFilesScannerCaseSensitiveKeepLanguages(t *testing.T) {
	apps := t.TempDir()
	res := filepath.Join(apps, "Slack.app", "Contents", "Resources")
	lfMkFile(t, filepath.Join(res, "de.lproj", "Localizable.strings"), "strings for de")

	s := newLanguageFilesScanner()
	s.preferred = func(home string) []string { return nil }

	cfg := config.Default()
	cfg.KeepLanguages = []string{"DE"} // uppercase config vs. lowercase on-disk de.lproj

	result := s.Scan(context.Background(), Options{Roots: Roots{Home: t.TempDir(), Applications: apps}, Cfg: cfg})
	if result.Error != "" {
		t.Fatalf("unexpected scan error: %q", result.Error)
	}

	if len(result.Items) != 1 || result.Items[0].Name != "Slack.app: de.lproj" {
		t.Fatalf("got %v, want de.lproj swept (case mismatch means it is not in the keep-set)", result.Items)
	}
}
