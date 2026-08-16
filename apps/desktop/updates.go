package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// appVersion is stamped by the release workflow via
// -ldflags "-X main.appVersion=1.4.0"; "dev" for local builds.
var appVersion = "dev"

const (
	releasesAPIURL  = "https://api.github.com/repos/GuilhermeVozniak/app-cleaner/releases/latest"
	releasesPageURL = "https://github.com/GuilhermeVozniak/app-cleaner/releases/latest"
)

// UpdateInfo is the CheckForUpdate payload. Latest is "" (with Error set)
// when the releases API was unreachable — the frontend treats that as
// "unknown", never as "up to date".
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
}

// parseSemver reads "v1.2.3" or "1.2.3" into comparable parts. Returns
// ok=false for anything else (e.g. "dev"), which disables comparison.
func parseSemver(s string) (parts [3]int, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	fields := strings.SplitN(s, ".", 3)
	if len(fields) != 3 {
		return parts, false
	}
	for i, f := range fields {
		// Tolerate a pre-release/build suffix on the last field ("3-rc1").
		if i == 2 {
			if dash := strings.IndexAny(f, "-+"); dash >= 0 {
				f = f[:dash]
			}
		}
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// newerVersion reports whether latest is strictly newer than current.
// Unparseable versions (dev builds, malformed tags) never report an update.
func newerVersion(current, latest string) bool {
	c, okC := parseSemver(current)
	l, okL := parseSemver(latest)
	if !okC || !okL {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// checkForUpdate fetches the latest release tag from apiURL. Split from the
// bound method so tests can point it at an httptest server.
func checkForUpdate(client *http.Client, apiURL, current string) UpdateInfo {
	info := UpdateInfo{Current: current, URL: releasesPageURL}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("releases API returned %d", resp.StatusCode)
		return info
	}
	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		info.Error = err.Error()
		return info
	}
	info.Latest = strings.TrimPrefix(payload.TagName, "v")
	if payload.HTMLURL != "" {
		info.URL = payload.HTMLURL
	}
	info.Available = newerVersion(current, payload.TagName)
	return info
}

// GetVersion returns the running app version ("dev" for unstamped builds).
func (a *App) GetVersion() string {
	return appVersion
}

// CheckForUpdate queries GitHub for the latest release. Network errors land
// in UpdateInfo.Error rather than a rejected promise so the frontend can
// degrade quietly.
func (a *App) CheckForUpdate() UpdateInfo {
	client := &http.Client{Timeout: 10 * time.Second}
	return checkForUpdate(client, releasesAPIURL, appVersion)
}

// OpenReleasePage opens the GitHub releases page in the default browser.
// Fixed URL — never derived from frontend input.
func (a *App) OpenReleasePage() {
	_ = exec.CommandContext(a.ctx, "/usr/bin/open", releasesPageURL).Run()
}
