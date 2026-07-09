package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/uninstall"
	"github.com/spf13/cobra"
)

// listAppsFn, isAppRunningFn, and uninstallFn are the engine seams tests
// replace with fakes (see uninstall_test.go); production wires the real
// uninstall package functions, mirroring app.go's bridge methods.
var (
	listAppsFn     = uninstall.ListApps
	isAppRunningFn = uninstall.IsAppRunning
	uninstallFn    = uninstall.Uninstall
)

// uninstallHome resolves $HOME ("/" on failure, matching app.go's startup
// fallback).
var uninstallHome = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return h
}

func newUninstallCmd() *cobra.Command {
	var appName string
	var yes bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall an application and its leftover files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), appName, yes)
		},
	}
	c.Flags().StringVar(&appName, "app", "", `Exact name of the app to uninstall (e.g. "Google Chrome")`)
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return c
}

func init() {
	rootCmd.AddCommand(newUninstallCmd())
}

func runUninstall(ctx context.Context, in io.Reader, w io.Writer, appName string, yes bool) error {
	if appName == "" {
		_, _ = fmt.Fprintln(w, "No --app given.")
		_, _ = fmt.Fprintln(w, "Interactive app picking arrives with the checkbox TUI in a later release;")
		_, _ = fmt.Fprintln(w, `for now pass --app "<exact name>" to choose an app non-interactively.`)
		return fmt.Errorf("uninstall: --app is required (interactive picker not yet available)")
	}

	home := uninstallHome()
	appDirs := []string{"/Applications", filepath.Join(home, "Applications")}
	apps := listAppsFn(ctx, appDirs, home)

	var matches []uninstall.AppInfo
	for _, a := range apps {
		if a.Name == appName {
			matches = append(matches, a)
		}
	}

	if len(matches) == 0 {
		if near := nearMisses(apps, appName); len(near) > 0 {
			return fmt.Errorf("no installed app named %q found; did you mean: %s?", appName, strings.Join(near, ", "))
		}
		return fmt.Errorf("no installed app named %q found", appName)
	}
	if len(matches) > 1 {
		var paths []string
		for _, m := range matches {
			paths = append(paths, m.Path)
		}
		return fmt.Errorf("%q matches more than one installed app: %s", appName, strings.Join(paths, ", "))
	}

	app := matches[0]
	if isAppRunningFn(app.Path) {
		_, _ = fmt.Fprintf(w, "%s is currently running. Quit the app first.\n", app.Name)
		return fmt.Errorf("%s is running: quit it before uninstalling", app.Name)
	}

	printUninstallPreview(w, home, app)

	if !yes && !promptConfirm(in, w, "Proceed with uninstallation?", false) {
		_, _ = fmt.Fprintln(w, "Uninstall cancelled.")
		return nil
	}

	summary := uninstallFn(ctx, []uninstall.AppInfo{app}, false, func(current, total int, name string) {
		_, _ = fmt.Fprintf(w, "Uninstalling %s...\n", name)
	})

	if len(summary.Errors) > 0 {
		_, _ = fmt.Fprintln(w, "Errors:")
		for _, e := range summary.Errors {
			_, _ = fmt.Fprintf(w, "  ✗ %s\n", e)
		}
		return fmt.Errorf("uninstall completed with %d error(s)", len(summary.Errors))
	}

	_, _ = fmt.Fprintf(w, "✓ Uninstalled %s\n", app.Name)
	_, _ = fmt.Fprintf(w, "Space freed: %s\n", core.FormatSize(summary.FreedSpace))
	return nil
}

func printUninstallPreview(w io.Writer, home string, app uninstall.AppInfo) {
	_, _ = fmt.Fprintf(w, "Application to uninstall: %s (%s)\n", app.Name, core.FormatSize(app.TotalSize))
	for _, rp := range app.RelatedPaths {
		_, _ = fmt.Fprintf(w, "  └─ %s\n", uninstallContractHome(rp.Path, home))
	}
	_, _ = fmt.Fprintf(w, "Total: %s will be freed (%d item(s))\n", core.FormatSize(app.TotalSize), 1+len(app.RelatedPaths))
}

// uninstallContractHome replaces a leading $HOME with "~", mirroring the
// GUI's path display (frontend/src/lib/uninstallMath.ts's contractHome).
func uninstallContractHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

// nearMisses returns installed app names that case-insensitively contain
// (or are contained by) the requested name, for the "did you mean" error
// hint shown when --app has no exact match.
func nearMisses(apps []uninstall.AppInfo, name string) []string {
	needle := strings.ToLower(name)
	var out []string
	for _, a := range apps {
		hay := strings.ToLower(a.Name)
		if strings.Contains(hay, needle) || strings.Contains(needle, hay) {
			out = append(out, a.Name)
		}
	}
	sort.Strings(out)
	return out
}
