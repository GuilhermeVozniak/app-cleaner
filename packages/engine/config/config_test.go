package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// setTestHome points the package's home-directory seam at a fake home for
// the duration of one test. Restored automatically via t.Cleanup.
func setTestHome(t *testing.T, home string) {
	t.Helper()
	old := homeDir
	homeDir = home
	t.Cleanup(func() { homeDir = old })
}

func loadFrom(t *testing.T, content string) Config {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(p)
}

func TestDefault(t *testing.T) {
	want := Config{
		DownloadsDaysOld:    30,
		LargeFilesMinSize:   524288000,
		BackupByDefault:     true,
		BackupRetentionDays: 7,
		Concurrency:         4,
		ShowRisky:           false,
		KeepLanguages:       []string{},
		ExtraPaths:          ExtraPaths{NodeModules: []string{}, Projects: []string{}},
	}
	if got := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Default() = %+v, want %+v", got, want)
	}
}

func TestDefaultPath(t *testing.T) {
	got := DefaultPath("/Users/x")
	want := "/Users/x/Library/Application Support/AppCleaner/config.json"
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestLoadValidationBounds(t *testing.T) {
	cases := []struct {
		name string
		json string
		mut  func(c *Config) // applies the expected surviving change to Default()
	}{
		{"downloadsDaysOld below min dropped", `{"downloadsDaysOld":0}`, func(c *Config) {}},
		{"downloadsDaysOld above max dropped", `{"downloadsDaysOld":366}`, func(c *Config) {}},
		{"downloadsDaysOld min kept", `{"downloadsDaysOld":1}`, func(c *Config) { c.DownloadsDaysOld = 1 }},
		{"downloadsDaysOld max kept", `{"downloadsDaysOld":365}`, func(c *Config) { c.DownloadsDaysOld = 365 }},
		{"downloadsDaysOld non-integer dropped", `{"downloadsDaysOld":30.5}`, func(c *Config) {}},
		{"downloadsDaysOld wrong type dropped", `{"downloadsDaysOld":"abc"}`, func(c *Config) {}},
		{"largeFilesMinSize below 1KB dropped", `{"largeFilesMinSize":1023}`, func(c *Config) {}},
		{"largeFilesMinSize 1KB kept", `{"largeFilesMinSize":1024}`, func(c *Config) { c.LargeFilesMinSize = 1024 }},
		{"largeFilesMinSize 100GiB kept", `{"largeFilesMinSize":107374182400}`, func(c *Config) { c.LargeFilesMinSize = 107374182400 }},
		{"largeFilesMinSize above 100GiB dropped", `{"largeFilesMinSize":107374182401}`, func(c *Config) {}},
		{"backupRetentionDays zero dropped", `{"backupRetentionDays":0}`, func(c *Config) {}},
		{"backupRetentionDays above max dropped", `{"backupRetentionDays":366}`, func(c *Config) {}},
		{"backupRetentionDays kept", `{"backupRetentionDays":14}`, func(c *Config) { c.BackupRetentionDays = 14 }},
		{"concurrency zero dropped", `{"concurrency":0}`, func(c *Config) {}},
		{"concurrency 17 dropped", `{"concurrency":17}`, func(c *Config) {}},
		{"concurrency 1 kept", `{"concurrency":1}`, func(c *Config) { c.Concurrency = 1 }},
		{"concurrency 16 kept", `{"concurrency":16}`, func(c *Config) { c.Concurrency = 16 }},
		{"backupByDefault false kept", `{"backupByDefault":false}`, func(c *Config) { c.BackupByDefault = false }},
		{"backupByDefault wrong type dropped", `{"backupByDefault":"yes"}`, func(c *Config) {}},
		{"showRisky true kept", `{"showRisky":true}`, func(c *Config) { c.ShowRisky = true }},
		{"showRisky wrong type dropped", `{"showRisky":1}`, func(c *Config) {}},
		{"keepLanguages kept", `{"keepLanguages":["pt-BR","fr"]}`, func(c *Config) { c.KeepLanguages = []string{"pt-BR", "fr"} }},
		{"keepLanguages wrong type dropped", `{"keepLanguages":"pt"}`, func(c *Config) {}},
		{"keepLanguages null treated as absent", `{"keepLanguages":null}`, func(c *Config) {}},
		{"unknown fields ignored", `{"someFutureField":123}`, func(c *Config) {}},
		{"valid fields survive alongside invalid ones", `{"downloadsDaysOld":60,"concurrency":99}`, func(c *Config) { c.DownloadsDaysOld = 60 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := Default()
			c.mut(&want)
			if got := loadFrom(t, c.json); !reflect.DeepEqual(got, want) {
				t.Errorf("Load(%s) = %+v, want %+v", c.json, got, want)
			}
		})
	}
}

func TestLoadDegenerateInputs(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		got := Load(filepath.Join(t.TempDir(), "nope.json"))
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("missing file: got %+v, want defaults", got)
		}
	})
	t.Run("bad JSON", func(t *testing.T) {
		got := loadFrom(t, `{not json at all`)
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("bad JSON: got %+v, want defaults", got)
		}
	})
	t.Run("over 100KB uses defaults even when valid", func(t *testing.T) {
		pad := strings.Repeat("x", 100*1024)
		got := loadFrom(t, fmt.Sprintf(`{"downloadsDaysOld":90,"pad":"%s"}`, pad))
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("oversized config must be ignored entirely, got %+v", got)
		}
	})
}

func TestLoadWarnsOnInvalidField(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	loadFrom(t, `{"downloadsDaysOld":9999}`)
	if !strings.Contains(buf.String(), "downloadsDaysOld") {
		t.Errorf("expected a log warning naming downloadsDaysOld, got: %q", buf.String())
	}
}

func TestLoadExtraPaths(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	cases := []struct {
		name string
		json string
		want []string // expected NodeModules after load (resolved absolute paths)
	}{
		{"tilde expansion", `{"extraPaths":{"nodeModules":["~/Projects"],"projects":[]}}`, []string{"/Users/testuser/Projects"}},
		{"bare tilde resolves to home and is kept", `{"extraPaths":{"nodeModules":["~"],"projects":[]}}`, []string{"/Users/testuser"}},
		{"absolute under /Users kept", `{"extraPaths":{"nodeModules":["/Users/other/code"],"projects":[]}}`, []string{"/Users/other/code"}},
		{"absolute under /Volumes kept", `{"extraPaths":{"nodeModules":["/Volumes/Ext/repos"],"projects":[]}}`, []string{"/Volumes/Ext/repos"}},
		{"outside allowed roots dropped", `{"extraPaths":{"nodeModules":["/etc/nginx","/tmp/work","/opt/stuff"],"projects":[]}}`, []string{}},
		{"traversal out of home dropped", `{"extraPaths":{"nodeModules":["~/../../etc"],"projects":[]}}`, []string{}},
		{"non-strings filtered", `{"extraPaths":{"nodeModules":[42,true,null,"~/ok"],"projects":[]}}`, []string{"/Users/testuser/ok"}},
		{"dot segments cleaned", `{"extraPaths":{"nodeModules":["/Users/other/../other2/code"],"projects":[]}}`, []string{"/Users/other2/code"}},
		{"wrong container type keeps defaults", `{"extraPaths":"nope"}`, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := loadFrom(t, c.json)
			if !reflect.DeepEqual(got.ExtraPaths.NodeModules, c.want) {
				t.Errorf("NodeModules = %v, want %v", got.ExtraPaths.NodeModules, c.want)
			}
			if len(got.ExtraPaths.Projects) != 0 {
				t.Errorf("Projects = %v, want empty", got.ExtraPaths.Projects)
			}
		})
	}
}

func TestLoadExtraPathsProjectsValidatedIndependently(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	got := loadFrom(t, `{"extraPaths":{"nodeModules":[],"projects":["~/work","/etc/bad"]}}`)
	want := []string{"/Users/testuser/work"}
	if !reflect.DeepEqual(got.ExtraPaths.Projects, want) {
		t.Errorf("Projects = %v, want %v", got.ExtraPaths.Projects, want)
	}
	if len(got.ExtraPaths.NodeModules) != 0 {
		t.Errorf("NodeModules = %v, want empty", got.ExtraPaths.NodeModules)
	}
}

func TestLoadExtraPathsCap50(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	entries := make([]string, 60)
	for i := range entries {
		entries[i] = fmt.Sprintf("%q", fmt.Sprintf("~/p%02d", i))
	}
	got := loadFrom(t, fmt.Sprintf(`{"extraPaths":{"nodeModules":[%s],"projects":[]}}`, strings.Join(entries, ",")))
	if len(got.ExtraPaths.NodeModules) != 50 {
		t.Fatalf("got %d paths, want capped at 50", len(got.ExtraPaths.NodeModules))
	}
	if got.ExtraPaths.NodeModules[0] != "/Users/testuser/p00" || got.ExtraPaths.NodeModules[49] != "/Users/testuser/p49" {
		t.Errorf("cap kept wrong entries: first=%s last=%s", got.ExtraPaths.NodeModules[0], got.ExtraPaths.NodeModules[49])
	}
}

func TestSaveCreatesDirsAndRoundTrips(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Library", "Application Support", "AppCleaner", "config.json")
	c := Default()
	c.DownloadsDaysOld = 60
	c.ShowRisky = true
	if err := Save(c, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "  \"downloadsDaysOld\": 60") {
		t.Errorf("expected 2-space-indented JSON, got:\n%s", data)
	}
	if got := Load(p); !reflect.DeepEqual(got, c) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, c)
	}
}
