// Package dispatch builds the Cobra command tree from the method registry.
package dispatch

import (
	"fmt"
	"sort"

	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/spf13/cobra"
)

// BuildCommands populates root with sub-commands generated from reg.
// Methods are grouped by category; each category becomes a parent command
// and each method becomes a child of that parent.  If overrides contains
// an entry for a method's APIMethod, the override command is used as-is;
// otherwise a generic command is created from the MethodDef metadata.
func BuildCommands(root *cobra.Command, reg []registry.MethodDef, overrides map[string]*cobra.Command) {
	groups := registry.GroupByCategory(reg)

	// Sort categories for deterministic command order.
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		methods := groups[cat]

		parent := &cobra.Command{
			Use:   cat,
			Short: fmt.Sprintf("Slack %s API methods", cat),
		}

		for _, m := range methods {
			if cmd, ok := overrides[m.APIMethod]; ok {
				parent.AddCommand(cmd)
				continue
			}
			parent.AddCommand(genericCommand(m))
		}

		root.AddCommand(parent)
	}
}

// genericCommand builds a Cobra command from a MethodDef, wiring up flags
// that match the method's parameter definitions.  The RunE returns an error
// because the actual dispatch layer is not yet wired.
func genericCommand(m registry.MethodDef) *cobra.Command {
	cmd := &cobra.Command{
		Use:     m.Command,
		Short:   m.Description,
		Aliases: m.Aliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("dispatch not yet implemented for %s", m.APIMethod)
		},
	}

	for _, p := range m.Params {
		switch p.Type {
		case "string", "json":
			cmd.Flags().String(p.Name, p.Default, p.Description)
		case "int":
			cmd.Flags().Int(p.Name, 0, p.Description)
		case "bool":
			cmd.Flags().Bool(p.Name, false, p.Description)
		case "string-slice":
			cmd.Flags().StringSlice(p.Name, nil, p.Description)
		}

		if p.Required {
			_ = cmd.MarkFlagRequired(p.Name)
		}
	}

	return cmd
}
