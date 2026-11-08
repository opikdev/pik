package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/opikdev/starters/packages/cli/internal/catalog"
	"github.com/opikdev/starters/packages/cli/internal/manifest"
	"github.com/opikdev/starters/packages/cli/internal/project"
)

type newFlags struct {
	set     []string
	yes     bool
	noSetup bool
	noGit   bool
}

func (a *app) newCommand() *cobra.Command {
	var flags newFlags
	command := &cobra.Command{
		Use:   "new [starter] [dir]",
		Short: "Create a project from a starter",
		Example: `  pik new
  pik new api-hono-starter my-api
  pik new api-hono-starter my-api --set owner=acme --yes`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runNew(cmd, args, flags)
		},
	}
	command.Flags().StringArrayVar(&flags.set, "set", nil, "answer a prompt, as name=value (repeatable)")
	command.Flags().BoolVar(&flags.yes, "yes", false, "accept defaults and run setup commands without asking")
	command.Flags().BoolVar(&flags.noSetup, "no-setup", false, "skip the starter's setup commands")
	command.Flags().BoolVar(&flags.noGit, "no-git", false, "skip creating a Git repo")
	return command
}

func (a *app) runNew(cmd *cobra.Command, args []string, flags newFlags) error {
	answers, err := parseSet(flags.set)
	if err != nil {
		return err
	}
	parsed, src, err := a.loadCatalog(cmd)
	if err != nil {
		return err
	}
	interactive := a.interactive() && !flags.yes

	name, dir, err := a.target(parsed, args, interactive)
	if err != nil {
		return err
	}
	entry, err := parsed.Starter(name)
	if err != nil {
		return err
	}

	var ask project.Ask
	if interactive {
		ask = askPrompt
	}
	a.progress("Downloading %s %s", entry.Name, entry.Version)
	result, err := project.Generate(cmd.Context(), src, project.Options{
		Entry:      entry,
		Dir:        dir,
		Answers:    answers,
		Ask:        ask,
		PikVersion: a.version,
	})
	if err != nil {
		return err
	}
	a.progress("Created %s", result.Dir)

	gitReady := false
	if !flags.noGit {
		gitReady, err = initGit(result.Dir)
		if err != nil {
			return err
		}
	}

	setupRan, err := a.setup(result, flags, interactive)
	if err != nil {
		return err
	}

	if a.json {
		a.printJSON(map[string]any{
			"ok":    true,
			"dir":   result.Dir,
			"pik":   result.Record,
			"git":   gitReady,
			"setup": map[string]any{"commands": result.Manifest.Setup, "ran": setupRan},
			"next":  result.Manifest.Next,
		})
		return nil
	}
	a.printNext(result, setupRan)
	return nil
}

func parseSet(pairs []string) (map[string]string, error) {
	answers := map[string]string{}
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("--set %q must be name=value", pair)
		}
		answers[name] = value
	}
	return answers, nil
}

func (a *app) target(parsed *catalog.Catalog, args []string, interactive bool) (string, string, error) {
	name, dir := "", ""
	if len(args) > 0 {
		name = args[0]
	}
	if len(args) > 1 {
		dir = args[1]
	}
	if name == "" {
		if !interactive {
			return "", "", errors.New("name a starter, such as pik new api-hono-starter my-api. Run pik list to see them")
		}
		var options []huh.Option[string]
		for _, entry := range parsed.Starters {
			if entry.Type == "starter" && entry.Version != "" {
				options = append(options, huh.NewOption(fmt.Sprintf("%s  %s", entry.Name, entry.Description), entry.Name))
			}
		}
		if err := huh.NewSelect[string]().Title("Which starter?").Options(options...).Value(&name).Run(); err != nil {
			return "", "", err
		}
	}
	if dir == "" {
		if !interactive {
			return "", "", errors.New("name a folder for the project, such as pik new api-hono-starter my-api")
		}
		if err := huh.NewInput().Title("Project folder").Placeholder("my-project").Value(&dir).
			Validate(func(value string) error {
				if strings.TrimSpace(value) == "" {
					return errors.New("enter a folder name")
				}
				return nil
			}).Run(); err != nil {
			return "", "", err
		}
	}
	return name, dir, nil
}

func askPrompt(prompt manifest.Prompt, initial string) (string, error) {
	value := initial
	title := prompt.Message
	if prompt.Optional {
		title += " (optional)"
	}
	err := huh.NewInput().Title(title).Value(&value).Validate(prompt.Validate).Run()
	return value, err
}

func initGit(dir string) (bool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return false, nil
	}
	if output, err := exec.Command("git", "-C", dir, "init", "--quiet", "--initial-branch=main").CombinedOutput(); err != nil {
		return false, fmt.Errorf("git init: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(dir, ".githooks")); err == nil {
		if output, err := exec.Command("git", "-C", dir, "config", "core.hooksPath", ".githooks").CombinedOutput(); err != nil {
			return false, fmt.Errorf("enable Git hooks: %v: %s", err, output)
		}
	}
	return true, nil
}

func (a *app) setup(result *project.Result, flags newFlags, interactive bool) (bool, error) {
	commands := result.Manifest.Setup
	if len(commands) == 0 || flags.noSetup {
		return false, nil
	}
	consent := flags.yes
	if !consent && interactive {
		description := "These run in " + result.Dir + ":\n\n  " + strings.Join(commands, "\n  ")
		if err := huh.NewConfirm().Title("Run the starter's setup commands?").
			Description(description).Affirmative("Run them").Negative("Skip").Value(&consent).Run(); err != nil {
			return false, err
		}
	}
	if !consent {
		return false, nil
	}
	for _, line := range commands {
		a.progress("Running %s", line)
		command := exec.Command("sh", "-c", line)
		command.Dir = result.Dir
		command.Stdout = a.stderr
		if !a.json {
			command.Stdout = a.stdout
		}
		command.Stderr = a.stderr
		if err := command.Run(); err != nil {
			return false, fmt.Errorf("setup command %q failed: %w. The project is in %s", line, err, result.Dir)
		}
	}
	return true, nil
}

func (a *app) progress(format string, args ...any) {
	if !a.json {
		fmt.Fprintf(a.stdout, format+"\n", args...)
	}
}

func (a *app) printNext(result *project.Result, setupRan bool) {
	fmt.Fprintln(a.stdout, "\nNext:")
	fmt.Fprintf(a.stdout, "  cd %s\n", result.Dir)
	if !setupRan {
		for _, line := range result.Manifest.Setup {
			fmt.Fprintf(a.stdout, "  %s\n", line)
		}
	}
	for _, line := range result.Manifest.Next {
		fmt.Fprintf(a.stdout, "  %s\n", line)
	}
}
