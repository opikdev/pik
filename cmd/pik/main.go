package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/opikdev/starters/packages/cli/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Main(resolveVersion(version, debug.ReadBuildInfo), os.Args[1:]))
}

// resolveVersion prefers the release version set at build time, then the module version that go install records.
func resolveVersion(linked string, buildInfo func() (*debug.BuildInfo, bool)) string {
	if linked != "dev" {
		return linked
	}
	if info, ok := buildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return linked
}
