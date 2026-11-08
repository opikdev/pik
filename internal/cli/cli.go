// Package cli wires pik's commands.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type app struct {
	version string
	stdout  io.Writer
	stderr  io.Writer
	stdin   *os.File
	json    bool
	catalog string
}

// Main runs pik with the given arguments and returns the exit code.
func Main(version string, args []string) int {
	return run(&app{version: version, stdout: os.Stdout, stderr: os.Stderr, stdin: os.Stdin}, args)
}

func run(a *app, args []string) int {
	root := a.rootCommand()
	root.SetArgs(args)
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)
	err := root.Execute()
	if err == nil {
		return 0
	}
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Fprintln(a.stderr, "Cancelled. Nothing was created.")
		return 1
	}
	if a.json {
		a.printJSON(map[string]any{"ok": false, "error": err.Error()})
	} else {
		fmt.Fprintf(a.stderr, "Error: %v\n", err)
	}
	return 1
}

func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "pik",
		Short:         "Create projects from agent-ready starter templates",
		Version:       a.version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&a.json, "json", false, "print machine-readable JSON")
	root.PersistentFlags().StringVar(&a.catalog, "catalog", "", "catalog URL or file (default: the opikdev/starters catalog)")
	root.AddCommand(a.listCommand(), a.doctorCommand(), a.newCommand())
	return root
}

func (a *app) interactive() bool {
	return !a.json && term.IsTerminal(int(a.stdin.Fd()))
}

func (a *app) printJSON(value any) {
	encoder := json.NewEncoder(a.stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}
