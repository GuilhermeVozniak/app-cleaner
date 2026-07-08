package main

import "github.com/GuilhermeVozniak/app-cleaner/apps/cli/cmd"

// version is overridden at release-build time via
// -ldflags "-X main.version=<semver>" (see Taskfile build:cli and
// .github/workflows/release.yml); local/dev builds report "dev".
var version = "dev"

func main() {
	cmd.Execute(version)
}
