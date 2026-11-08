package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opikdev/starters/packages/cli/internal/manifest"
	"github.com/opikdev/starters/packages/cli/internal/tools"
)

func (a *app) doctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [starter]",
		Short: "Check that the tools pik and a starter need are installed",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results := []tools.Result{tools.Check(cmd.Context(), manifest.Requirement{Name: "git"})}
			results[0].Required = false

			if len(args) == 1 {
				requirements, err := a.requirements(cmd, args[0])
				if err != nil {
					return err
				}
				for _, requirement := range requirements {
					results = append(results, tools.Check(cmd.Context(), requirement))
				}
			}

			healthy := true
			for _, result := range results {
				if result.Required && !result.OK {
					healthy = false
				}
			}

			if a.json {
				a.printJSON(map[string]any{"ok": healthy, "tools": results})
			} else {
				for _, result := range results {
					fmt.Fprintln(a.stdout, describe(result))
				}
			}
			if !healthy {
				return errMissingTools
			}
			return nil
		},
	}
}

func (a *app) requirements(cmd *cobra.Command, name string) ([]manifest.Requirement, error) {
	parsed, src, err := a.loadCatalog(cmd)
	if err != nil {
		return nil, err
	}
	entry, err := parsed.Starter(name)
	if err != nil {
		return nil, err
	}
	owner, repo, err := entry.GitHubRepo()
	if err != nil {
		return nil, err
	}
	data, err := src.Manifest(cmd.Context(), owner, repo, entry.Version)
	if err != nil {
		return nil, err
	}
	spec, err := manifest.Parse(data)
	if err != nil {
		return nil, err
	}
	return spec.Requires, nil
}

func describe(result tools.Result) string {
	mark := "✓"
	if !result.OK {
		mark = "✗"
		if !result.Required {
			mark = "!"
		}
	}
	line := fmt.Sprintf("%s %s", mark, result.Name)
	if result.Found != "" {
		line += " " + result.Found
	}
	if result.Needs != "" {
		line += fmt.Sprintf(" (needs %s or later)", result.Needs)
	}
	if result.Problem != "" {
		line += ": " + result.Problem
	}
	if !result.Required && !result.OK {
		line += ", which pik needs to set up a Git repo"
	}
	return line
}

var errMissingTools = errors.New("some required tools are missing or too old")
