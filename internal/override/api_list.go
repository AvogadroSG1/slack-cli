package override

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/spf13/cobra"
)

// RegisterBuiltins adds built-in commands to root that are not generated from
// the method registry. Currently this adds the "api list" subcommand for
// discovering available API methods.
func RegisterBuiltins(root *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "Discover available Slack API methods",
	}

	apiCmd.AddCommand(newListCmd())
	root.AddCommand(apiCmd)
}

// methodJSON is the JSON-serialisable representation of a registry method
// used by the --json output mode.
type methodJSON struct {
	APIMethod   string `json:"api_method"`
	Category    string `json:"category"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// newListCmd builds the "api list" subcommand that enumerates all methods
// in the global registry.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available Slack API methods",
		RunE:  runList,
	}

	cmd.Flags().String("category", "", "Filter methods by category")
	cmd.Flags().Bool("pretty", false, "Display output as a formatted table")
	cmd.Flags().Bool("json", false, "Display output as a JSON array")

	return cmd
}

// runList is the RunE handler for the list subcommand.
func runList(cmd *cobra.Command, _ []string) error {
	category, _ := cmd.Flags().GetString("category")
	pretty, _ := cmd.Flags().GetBool("pretty")
	asJSON, _ := cmd.Flags().GetBool("json")

	methods := filteredMethods(registry.Registry, category)

	w := cmd.OutOrStdout()

	switch {
	case asJSON:
		return writeJSON(w, methods)
	case pretty:
		return writePretty(w, methods)
	default:
		return writePlain(w, methods)
	}
}

// filteredMethods returns a sorted copy of defs, optionally filtered by
// category. The result is sorted alphabetically by APIMethod.
func filteredMethods(defs []registry.MethodDef, category string) []registry.MethodDef {
	out := make([]registry.MethodDef, 0, len(defs))
	for _, d := range defs {
		if category != "" && d.Category != category {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].APIMethod < out[j].APIMethod
	})
	return out
}

// writePlain writes one API method name per line.
func writePlain(w io.Writer, methods []registry.MethodDef) error {
	for _, m := range methods {
		if _, err := fmt.Fprintln(w, m.APIMethod); err != nil {
			return err
		}
	}
	return nil
}

// writePretty writes a tabwriter table with COMMAND, API METHOD, and
// DESCRIPTION columns.
func writePretty(w io.Writer, methods []registry.MethodDef) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "COMMAND\tAPI METHOD\tDESCRIPTION")
	for _, m := range methods {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.Command, m.APIMethod, m.Description)
	}
	return tw.Flush()
}

// writeJSON writes the methods as a JSON array of objects.
func writeJSON(w io.Writer, methods []registry.MethodDef) error {
	items := make([]methodJSON, len(methods))
	for i, m := range methods {
		items[i] = methodJSON{
			APIMethod:   m.APIMethod,
			Category:    m.Category,
			Command:     m.Command,
			Description: m.Description,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}
