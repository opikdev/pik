package cli

import (
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/opikdev/pik/internal/addon"
	"github.com/opikdev/pik/internal/catalog"
)

type addFlags struct {
	yes     bool
	noSetup bool
	dryRun  bool
}

func (a *app) addCommand() *cobra.Command {
	var flags addFlags
	command := &cobra.Command{
		Use:   "add [add-on] [dir]",
		Short: "Add an add-on to an existing project",
		Long: "Copies an add-on's files into a project, keeping every file that already exists, " +
			"and records the add-on in .pik.yaml. dir defaults to the current folder.",
		Example: `  pik add
  pik add agent-harness
  pik add agent-harness ./my-repo --dry-run`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAdd(cmd, args, flags)
		},
	}
	command.Flags().BoolVar(&flags.yes, "yes", false, "run setup commands without asking")
	command.Flags().BoolVar(&flags.noSetup, "no-setup", false, "skip the add-on's setup commands")
	command.Flags().BoolVar(&flags.dryRun, "dry-run", false, "list what would change, without writing anything")
	return command
}

func (a *app) runAdd(cmd *cobra.Command, args []string, flags addFlags) error {
	parsed, src, err := a.loadCatalog(cmd)
	if err != nil {
		return err
	}
	interactive := a.interactive() && !flags.yes

	name, dir := "", "."
	if len(args) > 0 {
		name = args[0]
	}
	if len(args) > 1 {
		dir = args[1]
	}
	if name == "" {
		if name, err = pickAddon(parsed, interactive); err != nil {
			return err
		}
	}
	entry, err := parsed.Addon(name)
	if err != nil {
		return err
	}

	a.progress("Downloading %s %s", entry.Name, entry.Version)
	result, err := addon.Add(cmd.Context(), src, addon.Options{Entry: entry, Dir: dir, DryRun: flags.dryRun})
	if err != nil {
		return err
	}

	hooks, setupRan := "", false
	if !flags.dryRun {
		if hooks, err = enableHooks(result.Dir); err != nil {
			return err
		}
		setup := newFlags{yes: flags.yes, noSetup: flags.noSetup}
		if setupRan, err = a.setup(result.Dir, result.Manifest.Setup, setup, interactive); err != nil {
			return err
		}
	}

	if a.json {
		a.printJSON(map[string]any{
			"ok":     true,
			"dryRun": flags.dryRun,
			"dir":    result.Dir,
			"addon":  result.Record,
			"added":  emptyIfNil(result.Added),
			"kept":   emptyIfNil(result.Kept),
			"hooks":  hooks,
			"setup":  map[string]any{"commands": emptyIfNil(result.Manifest.Setup), "ran": setupRan},
			"next":   emptyIfNil(result.Manifest.Next),
		})
		return nil
	}
	a.printAdd(result, flags.dryRun, hooks, setupRan)
	return nil
}

func pickAddon(parsed *catalog.Catalog, interactive bool) (string, error) {
	if !interactive {
		return "", errors.New("name an add-on, such as pik add agent-harness. Run pik list to see them")
	}
	var options []huh.Option[string]
	for _, entry := range parsed.Starters {
		if entry.Type == "addon" && entry.Version != "" {
			options = append(options, huh.NewOption(fmt.Sprintf("%s  %s", entry.Name, entry.Description), entry.Name))
		}
	}
	if len(options) == 0 {
		return "", errors.New("the catalog has no released add-ons yet")
	}
	var name string
	err := huh.NewSelect[string]().Title("Which add-on?").Options(options...).Value(&name).Run()
	return name, err
}

func emptyIfNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (a *app) printAdd(result *addon.Result, dryRun bool, hooks string, setupRan bool) {
	verb, keptVerb := "added", "kept "
	if dryRun {
		verb, keptVerb = "would add", "would keep"
	}
	for _, path := range result.Added {
		fmt.Fprintf(a.stdout, "  %s  %s\n", verb, path)
	}
	for _, path := range result.Kept {
		fmt.Fprintf(a.stdout, "  %s  %s (already exists)\n", keptVerb, path)
	}
	if dryRun {
		fmt.Fprintf(a.stdout, "\nDry run: nothing was written to %s\n", result.Dir)
		return
	}
	fmt.Fprintf(a.stdout, "\nAdded %s %s to %s, and recorded it in %s\n",
		result.Record.Name, result.Record.Version, result.Dir, addon.RecordFile)
	switch hooks {
	case hooksEnabled:
		fmt.Fprintln(a.stdout, "Git hooks in .githooks are on")
	case hooksKept:
		fmt.Fprintln(a.stdout, "core.hooksPath is already set, so call .githooks/pre-commit from your own hook")
	case hooksNotAGit:
		fmt.Fprintln(a.stdout, "This isn't a Git repo root, so after git init, run: git config core.hooksPath .githooks")
	}
	var next []string
	if !setupRan {
		next = append(next, result.Manifest.Setup...)
	}
	next = append(next, result.Manifest.Next...)
	if len(next) > 0 {
		fmt.Fprintln(a.stdout, "\nNext:")
		for _, line := range next {
			fmt.Fprintf(a.stdout, "  %s\n", line)
		}
	}
}
