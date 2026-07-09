package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/uninstall"
)

// swapUninstallSeams replaces the engine-call seams for one test and
// restores the originals on cleanup. uninstallCalled (if non-nil) records
// whether uninstallFn actually ran.
func swapUninstallSeams(
	t *testing.T,
	apps []uninstall.AppInfo,
	running map[string]bool,
	summary uninstall.Summary,
	uninstallCalled *bool,
) {
	t.Helper()
	origList, origRunning, origUninstall := listAppsFn, isAppRunningFn, uninstallFn
	listAppsFn = func(ctx context.Context, dirs []string, home string) []uninstall.AppInfo { return apps }
	isAppRunningFn = func(path string) bool { return running[path] }
	uninstallFn = func(ctx context.Context, apps []uninstall.AppInfo, dryRun bool, progress func(int, int, string)) uninstall.Summary {
		if uninstallCalled != nil {
			*uninstallCalled = true
		}
		for i, a := range apps {
			if progress != nil {
				progress(i+1, len(apps), a.Name)
			}
		}
		return summary
	}
	t.Cleanup(func() {
		listAppsFn, isAppRunningFn, uninstallFn = origList, origRunning, origUninstall
	})
}

func TestUninstallRequiresAppFlag(t *testing.T) {
	var buf bytes.Buffer
	err := runUninstall(context.Background(), strings.NewReader(""), &buf, "", false)
	if err == nil {
		t.Fatal("want error when --app is not given")
	}
	if !strings.Contains(buf.String(), "No --app given.") {
		t.Fatalf("out = %q", buf.String())
	}
}

func TestUninstallNoMatchListsNearMisses(t *testing.T) {
	apps := []uninstall.AppInfo{
		{Name: "Google Chrome", Path: "/Applications/Google Chrome.app"},
		{Name: "Google Chrome Canary", Path: "/Applications/Google Chrome Canary.app"},
	}
	var called bool
	swapUninstallSeams(t, apps, nil, uninstall.Summary{}, &called)

	var buf bytes.Buffer
	err := runUninstall(context.Background(), strings.NewReader(""), &buf, "Chrome", false)
	if err == nil {
		t.Fatal("want error for a name with no exact match")
	}
	if !strings.Contains(err.Error(), "Google Chrome") || !strings.Contains(err.Error(), "Google Chrome Canary") {
		t.Fatalf("err = %v, want both near-misses listed", err)
	}
	if called {
		t.Fatal("uninstallFn must not run when no app is selected")
	}
}

func TestUninstallNoMatchNoNearMiss(t *testing.T) {
	apps := []uninstall.AppInfo{{Name: "Slack", Path: "/Applications/Slack.app"}}
	swapUninstallSeams(t, apps, nil, uninstall.Summary{}, nil)

	err := runUninstall(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "Nonexistent", false)
	if err == nil || strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("err = %v, want a plain not-found error", err)
	}
}

func TestUninstallAmbiguousMatchListsPaths(t *testing.T) {
	apps := []uninstall.AppInfo{
		{Name: "Foo", Path: "/Applications/Foo.app"},
		{Name: "Foo", Path: "/Users/tester/Applications/Foo.app"},
	}
	swapUninstallSeams(t, apps, nil, uninstall.Summary{}, nil)

	err := runUninstall(context.Background(), strings.NewReader(""), &bytes.Buffer{}, "Foo", false)
	if err == nil {
		t.Fatal("want error for an ambiguous exact match")
	}
	if !strings.Contains(err.Error(), "/Applications/Foo.app") || !strings.Contains(err.Error(), "/Users/tester/Applications/Foo.app") {
		t.Fatalf("err = %v, want both paths listed", err)
	}
}

func TestUninstallRefusesRunningApp(t *testing.T) {
	apps := []uninstall.AppInfo{{Name: "Busy", Path: "/Applications/Busy.app"}}
	var called bool
	swapUninstallSeams(t, apps, map[string]bool{"/Applications/Busy.app": true}, uninstall.Summary{}, &called)

	var buf bytes.Buffer
	err := runUninstall(context.Background(), strings.NewReader(""), &buf, "Busy", false)
	if err == nil {
		t.Fatal("want error when the app is running")
	}
	if !strings.Contains(buf.String(), "Busy is currently running. Quit the app first.") {
		t.Fatalf("out = %q", buf.String())
	}
	if called {
		t.Fatal("uninstallFn must not run for a running app")
	}
}

func TestUninstallDefaultNoDeclines(t *testing.T) {
	apps := []uninstall.AppInfo{{Name: "Foo", Path: "/Applications/Foo.app", TotalSize: 1024}}
	var called bool
	swapUninstallSeams(t, apps, nil, uninstall.Summary{}, &called)

	var buf bytes.Buffer
	err := runUninstall(context.Background(), strings.NewReader("\n"), &buf, "Foo", false)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "Uninstall cancelled.") {
		t.Fatalf("out = %q, want cancellation on bare Enter (default no)", buf.String())
	}
	if called {
		t.Fatal("uninstallFn must not run when the confirm is declined")
	}
}

func TestUninstallYesFlagReportsFreedSpace(t *testing.T) {
	home := "/Users/tester"
	origHome := uninstallHome
	uninstallHome = func() string { return home }
	t.Cleanup(func() { uninstallHome = origHome })

	apps := []uninstall.AppInfo{{
		Name:         "Foo",
		Path:         "/Applications/Foo.app",
		TotalSize:    182784, // 178.5 KB
		RelatedPaths: []uninstall.RelatedPath{{Path: "/Users/tester/Library/Caches/Foo", Size: 2048}},
	}}
	summary := uninstall.Summary{Uninstalled: 1, FreedSpace: 182784}
	var called bool
	swapUninstallSeams(t, apps, nil, summary, &called)

	var buf bytes.Buffer
	err := runUninstall(context.Background(), strings.NewReader(""), &buf, "Foo", true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Fatal("uninstallFn must run when --yes is set")
	}
	out := buf.String()
	if !strings.Contains(out, "└─ ~/Library/Caches/Foo") {
		t.Fatalf("out = %q, want the related path contracted to ~", out)
	}
	if !strings.Contains(out, "Uninstalling Foo...") {
		t.Fatalf("out = %q, want a per-app progress line", out)
	}
	want := "✓ Uninstalled Foo\nSpace freed: " + core.FormatSize(182784) + "\n"
	if !strings.HasSuffix(out, want) {
		t.Fatalf("out = %q, want suffix %q", out, want)
	}
}

func TestUninstallReportsSummaryErrors(t *testing.T) {
	apps := []uninstall.AppInfo{{Name: "Foo", Path: "/Applications/Foo.app"}}
	summary := uninstall.Summary{Errors: []string{"Foo: Failed to remove (security check failed or permission denied)"}}
	swapUninstallSeams(t, apps, nil, summary, nil)

	var buf bytes.Buffer
	err := runUninstall(context.Background(), strings.NewReader(""), &buf, "Foo", true)
	if err == nil || !strings.Contains(err.Error(), "1 error(s)") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "✗ Foo: Failed to remove (security check failed or permission denied)") {
		t.Fatalf("out = %q", buf.String())
	}
}
