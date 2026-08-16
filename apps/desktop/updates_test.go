package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.4.0", "v1.4.1", true},
		{"1.4.0", "v1.5.0", true},
		{"1.4.0", "v2.0.0", true},
		{"1.4.0", "v1.4.0", false},
		{"1.4.1", "v1.4.0", false},
		{"2.0.0", "v1.9.9", false},
		{"dev", "v1.4.0", false},    // dev builds never nag
		{"1.4.0", "nightly", false}, // malformed remote tag
		{"1.4.0", "v1.4.1-rc1", true},
		{"1.9.0", "v1.10.0", true}, // numeric, not lexicographic
	}
	for _, c := range cases {
		if got := newerVersion(c.current, c.latest); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCheckForUpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q", got)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://example.com/rel"}`))
	}))
	defer srv.Close()

	info := checkForUpdate(srv.Client(), srv.URL, "1.4.0")
	if info.Error != "" {
		t.Fatalf("unexpected error: %s", info.Error)
	}
	if !info.Available || info.Latest != "9.9.9" || info.Current != "1.4.0" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.URL != "https://example.com/rel" {
		t.Fatalf("URL should come from the release payload, got %q", info.URL)
	}
}

func TestCheckForUpdateUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.4.0"}`))
	}))
	defer srv.Close()

	info := checkForUpdate(srv.Client(), srv.URL, "1.4.0")
	if info.Available || info.Error != "" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.URL != releasesPageURL {
		t.Fatalf("URL should fall back to the releases page, got %q", info.URL)
	}
}

func TestCheckForUpdateAPIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // rate-limited
	}))
	defer srv.Close()

	info := checkForUpdate(srv.Client(), srv.URL, "1.4.0")
	if info.Available {
		t.Fatal("failure must not report an update")
	}
	if info.Error == "" || info.Latest != "" {
		t.Fatalf("expected error + empty latest, got %+v", info)
	}
}

func TestCheckForUpdateNetworkError(t *testing.T) {
	client := &http.Client{Timeout: 50 * time.Millisecond}
	info := checkForUpdate(client, "http://127.0.0.1:1/nope", "1.4.0")
	if info.Available || info.Error == "" {
		t.Fatalf("expected quiet failure, got %+v", info)
	}
}

func TestCheckForUpdateBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{broken`))
	}))
	defer srv.Close()

	info := checkForUpdate(srv.Client(), srv.URL, "1.4.0")
	if info.Available || info.Error == "" {
		t.Fatalf("expected decode error, got %+v", info)
	}
}
