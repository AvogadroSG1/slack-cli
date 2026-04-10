// Package registry defines the method registry types for the Slack CLI.
// Each MethodDef describes a single Slack API method, including its
// parameters, pagination behaviour, and SDK calling convention.
package registry

// ParamDef describes a single parameter accepted by a Slack API method.
type ParamDef struct {
	Name        string // CLI flag name (kebab-case)
	SDKName     string // Go struct field or param name
	Type        string // "string", "int", "bool", "string-slice", "json"
	Required    bool
	Description string
	Default     string
}

// MethodDef describes a single Slack API method and the metadata the CLI
// needs to expose it as a subcommand.
type MethodDef struct {
	APIMethod   string   // e.g., "chat.postMessage"
	Category    string   // e.g., "chat"
	Command     string   // e.g., "post-message"
	SDKMethod   string   // e.g., "PostMessageContext" (documentation only)
	Description string
	DocsURL     string
	Aliases     []string
	Params      []ParamDef
	Paginated   bool
	CursorParam string
	CursorField string // JSON field name for next_cursor
	CallStyle   string // "positional", "struct", "msgoption"
	ResponseKey string // JSON key for primary data
}

// Registry is the global list of all known Slack API method definitions.
var Registry []MethodDef

// GroupByCategory partitions a slice of MethodDef by their Category field
// and returns the result as a map keyed by category name.
func GroupByCategory(defs []MethodDef) map[string][]MethodDef {
	out := make(map[string][]MethodDef)
	for _, d := range defs {
		out[d.Category] = append(out[d.Category], d)
	}
	return out
}
