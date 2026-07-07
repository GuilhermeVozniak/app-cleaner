// Package config loads, validates, and saves App Cleaner's JSON
// configuration. Load never fails: unusable files fall back to Default()
// and individually invalid fields are dropped to their defaults with a
// warning on the standard log package. It never imports Wails.
package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxConfigBytes = 100 * 1024
	maxExtraPaths  = 50
)

// homeDir is the current user's home directory, resolved once at package
// initialization. Package variable (not a call site) so same-package tests
// can point it at a fake home via setTestHome; production never mutates it.
var homeDir = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}()

type ExtraPaths struct {
	NodeModules []string `json:"nodeModules"`
	Projects    []string `json:"projects"`
}

type Config struct {
	DownloadsDaysOld    int        `json:"downloadsDaysOld"`    // default 30, valid 1..365
	LargeFilesMinSize   int64      `json:"largeFilesMinSize"`   // default 524288000, valid 1024..100GiB
	BackupByDefault     bool       `json:"backupByDefault"`     // default true
	BackupRetentionDays int        `json:"backupRetentionDays"` // default 7, valid 1..365
	Concurrency         int        `json:"concurrency"`         // default 4, valid 1..16
	ShowRisky           bool       `json:"showRisky"`           // default false
	KeepLanguages       []string   `json:"keepLanguages"`       // extra .lproj basenames to keep
	ExtraPaths          ExtraPaths `json:"extraPaths"`          // ≤50 each; resolved under $HOME, /Users, or /Volumes
}

// Default returns the hard defaults.
func Default() Config {
	return Config{
		DownloadsDaysOld:    30,
		LargeFilesMinSize:   524288000,
		BackupByDefault:     true,
		BackupRetentionDays: 7,
		Concurrency:         4,
		ShowRisky:           false,
		KeepLanguages:       []string{},
		ExtraPaths:          ExtraPaths{NodeModules: []string{}, Projects: []string{}},
	}
}

// DefaultPath returns the canonical config location for a home directory.
func DefaultPath(home string) string {
	return filepath.Join(home, "Library", "Application Support", "AppCleaner", "config.json")
}

// Load reads path and overlays its valid fields onto Default(). It never
// returns an error: a missing/unreadable file, a file over 100KB, or
// syntactically invalid JSON all yield Default(); an individually invalid
// field keeps its default and logs a warning.
func Load(path string) Config {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if len(data) > maxConfigBytes {
		log.Printf("config: %s is larger than 100KB, using defaults", path)
		return cfg
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("config: invalid JSON in %s, using defaults: %v", path, err)
		return cfg
	}
	loadInt(raw, "downloadsDaysOld", 1, 365, &cfg.DownloadsDaysOld)
	loadInt64(raw, "largeFilesMinSize", 1024, 100*1024*1024*1024, &cfg.LargeFilesMinSize)
	loadBool(raw, "backupByDefault", &cfg.BackupByDefault)
	loadInt(raw, "backupRetentionDays", 1, 365, &cfg.BackupRetentionDays)
	loadInt(raw, "concurrency", 1, 16, &cfg.Concurrency)
	loadBool(raw, "showRisky", &cfg.ShowRisky)
	if v, ok := raw["keepLanguages"]; ok {
		var langs []string
		if err := json.Unmarshal(v, &langs); err != nil {
			log.Printf("config: invalid keepLanguages, keeping default: %v", err)
		} else if langs != nil { // JSON null decodes to nil: treat as absent
			cfg.KeepLanguages = langs
		}
	}
	if v, ok := raw["extraPaths"]; ok {
		var ep struct {
			NodeModules []any `json:"nodeModules"`
			Projects    []any `json:"projects"`
		}
		if err := json.Unmarshal(v, &ep); err != nil {
			log.Printf("config: invalid extraPaths, keeping default: %v", err)
		} else {
			cfg.ExtraPaths.NodeModules = sanitizeExtraPaths(ep.NodeModules)
			cfg.ExtraPaths.Projects = sanitizeExtraPaths(ep.Projects)
		}
	}
	return cfg
}

func loadInt(raw map[string]json.RawMessage, key string, min, max int, dst *int) {
	v, ok := raw[key]
	if !ok {
		return
	}
	var n int
	if err := json.Unmarshal(v, &n); err != nil || n < min || n > max {
		log.Printf("config: invalid %s %s (valid %d..%d), keeping default %d", key, string(v), min, max, *dst)
		return
	}
	*dst = n
}

func loadInt64(raw map[string]json.RawMessage, key string, min, max int64, dst *int64) {
	v, ok := raw[key]
	if !ok {
		return
	}
	var n int64
	if err := json.Unmarshal(v, &n); err != nil || n < min || n > max {
		log.Printf("config: invalid %s %s (valid %d..%d), keeping default %d", key, string(v), min, max, *dst)
		return
	}
	*dst = n
}

func loadBool(raw map[string]json.RawMessage, key string, dst *bool) {
	v, ok := raw[key]
	if !ok {
		return
	}
	var b bool
	if err := json.Unmarshal(v, &b); err != nil {
		log.Printf("config: invalid %s %s (want true/false), keeping default %v", key, string(v), *dst)
		return
	}
	*dst = b
}

// sanitizeExtraPaths applies the CLI's extraPaths rules: keep only strings,
// truncate to the first 50 BEFORE validation (CLI .slice(0,50) order),
// expand "~"/"~/" against homeDir, resolve to an absolute clean path, and
// require the result under homeDir, /Users, or /Volumes. Returns resolved
// absolute paths; never nil.
func sanitizeExtraPaths(vals []any) []string {
	strs := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok {
			strs = append(strs, s)
		}
	}
	if len(strs) > maxExtraPaths {
		log.Printf("config: extraPaths list truncated to %d entries", maxExtraPaths)
		strs = strs[:maxExtraPaths]
	}
	out := []string{}
	for _, s := range strs {
		p := s
		if p == "~" {
			p = homeDir
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(homeDir, p[2:])
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			log.Printf("config: skipping unresolvable path: %s", s)
			continue
		}
		if !underAllowedRoot(abs) {
			log.Printf("config: skipping path outside allowed directories: %s", s)
			continue
		}
		out = append(out, abs)
	}
	return out
}

func underAllowedRoot(abs string) bool {
	for _, root := range []string{homeDir, "/Users", "/Volumes"} {
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if abs == root || strings.HasPrefix(abs, root+"/") {
			return true
		}
	}
	return false
}

// Save writes c to path as indented JSON, creating parent directories.
func Save(c Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
