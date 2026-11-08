package main

import (
	"os"

	"github.com/opikdev/starters/packages/cli/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Main(version, os.Args[1:]))
}
