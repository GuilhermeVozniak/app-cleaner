// Package loginitems lists what launches automatically at login: launchd
// agents and daemons parsed from the standard plist directories. Listing is
// read-only; the desktop app only surfaces the information (removal of
// orphaned agents is already covered by the launch-agents scan category).
package loginitems

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"howett.net/plist"
)

// Item is one launchd agent/daemon registration.
type Item struct {
	Label     string `json:"label"`
	Path      string `json:"path"`    // the .plist file
	Program   string `json:"program"` // resolved Program or ProgramArguments[0] ("" if absent)
	Kind      string `json:"kind"`    // "user-agent" | "global-agent" | "daemon"
	RunAtLoad bool   `json:"runAtLoad"`
	// ProgramMissing is true when Program is set but no longer exists on disk.
	ProgramMissing bool `json:"programMissing"`
}

type plistPayload struct {
	Label            string   `plist:"Label"`
	Program          string   `plist:"Program"`
	ProgramArguments []string `plist:"ProgramArguments"`
	RunAtLoad        bool     `plist:"RunAtLoad"`
}

// Dir pairs a plist directory with the Kind its entries get.
type Dir struct {
	Path string
	Kind string
}

// StandardDirs returns the three launchd plist locations for home.
func StandardDirs(home string) []Dir {
	return []Dir{
		{Path: filepath.Join(home, "Library", "LaunchAgents"), Kind: "user-agent"},
		{Path: "/Library/LaunchAgents", Kind: "global-agent"},
		{Path: "/Library/LaunchDaemons", Kind: "daemon"},
	}
}

// List parses every .plist in dirs. Unreadable directories or files are
// skipped silently (a login-items listing must never hard-fail). The result
// is sorted by Label and never nil.
func List(dirs []Dir) []Item {
	items := []Item{}
	for _, d := range dirs {
		entries, err := os.ReadDir(d.Path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}
			full := filepath.Join(d.Path, e.Name())
			it, ok := parse(full, d.Kind)
			if ok {
				items = append(items, it)
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

func parse(path, kind string) (Item, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Item{}, false
	}
	var p plistPayload
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return Item{}, false
	}
	program := p.Program
	if program == "" && len(p.ProgramArguments) > 0 {
		program = p.ProgramArguments[0]
	}
	it := Item{
		Label:     p.Label,
		Path:      path,
		Program:   program,
		Kind:      kind,
		RunAtLoad: p.RunAtLoad,
	}
	if it.Label == "" { // a plist without a Label is not a launchd job
		it.Label = strings.TrimSuffix(filepath.Base(path), ".plist")
	}
	if program != "" {
		if _, err := os.Stat(program); err != nil {
			it.ProgramMissing = true
		}
	}
	return it, true
}
