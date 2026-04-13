// Package dispatch builds the Cobra command tree from the method registry.
package dispatch

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/slack-go/slack"
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

// BuildCommandsWithClient populates root with sub-commands that are fully
// wired to the dispatch/execute/output pipeline. Each generated command's
// RunE extracts flags, calls Execute (or Paginate when --all is set), and
// writes JSON output to stdout. The client parameter may be nil; commands
// will fail at invocation time with an auth error when no token is available.
func BuildCommandsWithClient(
	root *cobra.Command,
	reg []registry.MethodDef,
	overrides map[string]*cobra.Command,
	client *slack.Client,
	stdout io.Writer,
) {
	groups := registry.GroupByCategory(reg)

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
			parent.AddCommand(wiredCommand(m, client, stdout))
		}

		root.AddCommand(parent)
	}
}

// wiredCommand builds a Cobra command from a MethodDef with its RunE fully
// connected to the dispatch pipeline: flag extraction, execution (or
// pagination), output formatting, and error handling.
func wiredCommand(m registry.MethodDef, client *slack.Client, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     m.Command,
		Short:   m.Description,
		Aliases: m.Aliases,
	}

	addFlags(cmd, m.Params)

	// Capture m by value so the closure is safe for concurrent use.
	method := m
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if client == nil {
			code := exitcode.AuthError
			_ = FormatError(cmd.ErrOrStderr(), "SLACK_TOKEN is not set", code)
			return &exitError{code: code}
		}

		flags := extractFlags(cmd, method.Params)

		// Read global flags.
		pretty, _ := cmd.Flags().GetBool("pretty")
		fetchAll, _ := cmd.Flags().GetBool("all")
		limit, _ := cmd.Flags().GetInt("limit")
		maxResults, _ := cmd.Flags().GetInt("max-results")

		ctx := cmd.Context()

		var result any
		var err error
		if method.Paginated && fetchAll {
			result, err = Paginate(ctx, client, method, flags, true, limit, maxResults)
		} else {
			result, err = Execute(ctx, client, method.APIMethod, flags)
		}

		if err != nil {
			code := exitcode.Classify(err)
			_ = FormatError(cmd.ErrOrStderr(), err.Error(), code)
			return &exitError{code: code}
		}

		return FormatOutput(stdout, result, pretty)
	}

	return cmd
}

// addFlags registers cobra flags on cmd based on the given parameter
// definitions. This logic is shared between genericCommand and wiredCommand.
func addFlags(cmd *cobra.Command, params []registry.ParamDef) {
	for _, p := range params {
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
}

// extractFlags reads the cobra flags defined by params into a map[string]any.
// Only flags that were explicitly set by the user are included, so callers can
// distinguish "not provided" from a zero value.
func extractFlags(cmd *cobra.Command, params []registry.ParamDef) map[string]any {
	flags := make(map[string]any)

	for _, p := range params {
		f := cmd.Flags().Lookup(p.Name)
		if f == nil || !f.Changed {
			continue
		}

		switch p.Type {
		case "string", "json":
			v, _ := cmd.Flags().GetString(p.Name)
			flags[p.Name] = v
		case "int":
			v, _ := cmd.Flags().GetInt(p.Name)
			flags[p.Name] = v
		case "bool":
			v, _ := cmd.Flags().GetBool(p.Name)
			flags[p.Name] = v
		case "string-slice":
			v, _ := cmd.Flags().GetStringSlice(p.Name)
			flags[p.Name] = v
		default:
			// Fall back to raw string value.
			flags[p.Name] = f.Value.String()
		}
	}

	return flags
}

// ExitCoder is implemented by errors that carry a numeric exit code.
type ExitCoder interface {
	ExitCode() int
}

// exitError wraps an exit code so the root command can extract it.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return "exit code " + strconv.Itoa(e.code)
}

func (e *exitError) ExitCode() int {
	return e.code
}

// NewExitError creates an error that carries the given exit code. Use this
// from command handlers so the root command can extract the code.
func NewExitError(code int) error {
	return &exitError{code: code}
}

// ExitCode returns the numeric exit code from an error that implements
// ExitCoder, or 1 if the error does not.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ec, ok := err.(ExitCoder); ok {
		return ec.ExitCode()
	}
	return 1
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

	addFlags(cmd, m.Params)

	return cmd
}
