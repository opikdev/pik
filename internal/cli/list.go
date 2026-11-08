package cli

import (
	"cmp"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/opikdev/pik/internal/catalog"
	"github.com/opikdev/pik/internal/source"
)

func (a *app) loadCatalog(cmd *cobra.Command) (*catalog.Catalog, *source.Source, error) {
	src := source.FromEnv(a.catalog)
	data, err := src.Catalog(cmd.Context())
	if err != nil {
		return nil, nil, err
	}
	parsed, err := catalog.Parse(data)
	if err != nil {
		return nil, nil, err
	}
	return parsed, src, nil
}

func (a *app) listCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the starters and add-ons in the catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsed, _, err := a.loadCatalog(cmd)
			if err != nil {
				return err
			}
			if a.json {
				a.printJSON(map[string]any{"ok": true, "starters": parsed.Starters})
				return nil
			}
			table := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "NAME\tTYPE\tVERSION\tDESCRIPTION")
			for _, entry := range parsed.Starters {
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", entry.Name, entry.Type, cmp.Or(entry.Version, "unreleased"), entry.Description)
			}
			return table.Flush()
		},
	}
}
