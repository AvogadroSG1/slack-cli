// Command introspect generates registry and dispatch source files by
// introspecting the slack-go/slack SDK's *Client method set.
//
// Usage:
//
//	go run ./cmd/introspect
//
// The generator loads the slack-go/slack package with full type information,
// iterates every method on *Client that ends with "Context", and -- for those
// present in the hand-maintained mapping table -- emits:
//
//   - internal/registry/generated.go   (MethodDef entries)
//   - internal/dispatch/generated_dispatch.go (stub DispatchFunc + init)
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// methodInfo holds the extracted metadata for a single SDK method.
type methodInfo struct {
	GoName    string // e.g. "PostMessageContext"
	BaseName  string // e.g. "PostMessage"
	APIMethod string // e.g. "chat.postMessage"
	Category  string // e.g. "chat"
	Command   string // e.g. "post-message" (kebab-case)
	Params    []paramInfo
	Paginated bool
	CallStyle string // "positional", "struct", "msgoption"
}

// paramInfo holds per-parameter metadata.
type paramInfo struct {
	Name    string // kebab-case CLI flag name
	SDKName string // Go identifier name
	Type    string // "string", "int", "bool", "string-slice", "json"
}

// paramNameOverrides maps SDK parameter/field names to their canonical CLI
// flag names. The generator applies these before the default camelToKebab
// conversion so that generated flags match the hand-written dispatch impls
// and the Slack API docs.
var paramNameOverrides = map[string]string{
	"ChannelID":        "channel",
	"channelID":        "channel",
	"ChannelName":      "name",
	"channelName":      "name",
	"Timestamp":        "ts",
	"timestamp":        "ts",
	"messageTimestamp":  "ts",
	"userID":           "user",
}

func main() {
	log.SetPrefix("introspect: ")
	log.SetFlags(0)

	methods := loadMethods()
	if len(methods) == 0 {
		log.Fatal("no methods found -- is the slack-go/slack package available?")
	}

	log.Printf("found %d mapped methods", len(methods))

	projectRoot := findProjectRoot()

	registryPath := filepath.Join(projectRoot, "internal", "registry", "generated.go")
	dispatchPath := filepath.Join(projectRoot, "internal", "dispatch", "generated_dispatch.go")

	writeFormatted(registryPath, generateRegistry(methods))
	writeFormatted(dispatchPath, generateDispatch(methods))

	log.Printf("wrote %s", registryPath)
	log.Printf("wrote %s", dispatchPath)
}

// loadMethods uses golang.org/x/tools/go/packages to load the slack-go/slack
// package with type information, then extracts methods from *Client.
func loadMethods() []methodInfo {
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedName | packages.NeedImports | packages.NeedDeps,
	}

	pkgs, err := packages.Load(cfg, "github.com/slack-go/slack")
	if err != nil {
		log.Fatalf("packages.Load: %v", err)
	}
	if len(pkgs) == 0 {
		log.Fatal("no packages loaded")
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		for _, e := range pkg.Errors {
			log.Printf("package error: %v", e)
		}
		log.Fatal("slack-go/slack had load errors")
	}

	// Find the Client type.
	obj := pkg.Types.Scope().Lookup("Client")
	if obj == nil {
		log.Fatal("type Client not found in slack-go/slack")
	}

	named, ok := obj.Type().(*types.Named)
	if !ok {
		log.Fatal("Client is not a named type")
	}

	// Build the method set for *Client (pointer receiver).
	ptrType := types.NewPointer(named)
	mset := types.NewMethodSet(ptrType)

	var results []methodInfo
	for i := 0; i < mset.Len(); i++ {
		sel := mset.At(i)
		methodName := sel.Obj().Name()

		// Only process methods ending with "Context".
		if !strings.HasSuffix(methodName, "Context") {
			continue
		}

		baseName := strings.TrimSuffix(methodName, "Context")

		apiMethod, mapped := methodMapping[baseName]
		if !mapped {
			continue
		}

		// Skip admin.* methods.
		if strings.HasPrefix(apiMethod, "admin.") {
			continue
		}

		sig, sigOK := sel.Obj().Type().(*types.Signature)
		if !sigOK {
			continue
		}

		mi := methodInfo{
			GoName:    methodName,
			BaseName:  baseName,
			APIMethod: apiMethod,
			Category:  categoryFromAPI(apiMethod),
			Command:   commandFromAPI(apiMethod),
		}

		mi.Params, mi.CallStyle, mi.Paginated = extractParams(sig, pkg.Types)

		results = append(results, mi)
	}

	// Sort for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].APIMethod < results[j].APIMethod
	})

	return results
}

// categoryFromAPI extracts the category from a Slack API method name.
// "chat.postMessage" -> "chat", "usergroups.users.list" -> "usergroups"
func categoryFromAPI(apiMethod string) string {
	parts := strings.SplitN(apiMethod, ".", 2)
	return parts[0]
}

// commandFromAPI derives a kebab-case CLI command name from the API method.
// It uses the last segment of the API method path: "conversations.setPurpose"
// -> "set-purpose", "usergroups.users.list" -> "list".
// For methods whose category has sub-namespaces (like usergroups.users.*),
// we include the sub-namespace: "usergroups.users.list" -> "users-list".
func commandFromAPI(apiMethod string) string {
	parts := strings.Split(apiMethod, ".")
	if len(parts) <= 2 {
		return camelToKebab(parts[len(parts)-1])
	}
	// For nested namespaces like "usergroups.users.list", combine sub-parts.
	sub := strings.Join(parts[1:], "-")
	return camelToKebab(sub)
}

// camelToKebab converts a camelCase or MixedCaps string to kebab-case.
// "postMessage" -> "post-message"
// "setPurpose" -> "set-purpose"
// "endDnd" -> "end-dnd"
func camelToKebab(s string) string {
	var buf strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			prev := rune(s[i-1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				buf.WriteRune('-')
			} else if i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				buf.WriteRune('-')
			}
		}
		buf.WriteRune(unicode.ToLower(r))
	}
	return buf.String()
}

// extractParams inspects the method signature and returns parameter info,
// the call style, and whether the method supports pagination.
func extractParams(sig *types.Signature, slackPkg *types.Package) ([]paramInfo, string, bool) {
	params := sig.Params()
	var result []paramInfo
	seen := make(map[string]bool)
	callStyle := "positional"
	paginated := false

	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		pType := p.Type()
		pName := p.Name()

		// Skip context.Context parameters.
		if isContextType(pType) {
			continue
		}

		// Detect variadic ...MsgOption -> msgoption call style.
		if sig.Variadic() && i == params.Len()-1 {
			if slice, isSlice := pType.(*types.Slice); isSlice {
				if namedElem, isNamed := slice.Elem().(*types.Named); isNamed {
					if namedElem.Obj().Name() == "MsgOption" {
						callStyle = "msgoption"
						if !seen["options"] {
							result = append(result, paramInfo{
								Name:    "options",
								SDKName: "options",
								Type:    "json",
							})
							seen["options"] = true
						}
						continue
					}
				}
			}
		}

		// Detect struct/pointer-to-struct parameter -> struct call style.
		if structType := extractStructType(pType); structType != nil {
			callStyle = "struct"
			fields := extractStructFields(structType)
			for _, f := range fields {
				if seen[f.Name] {
					continue
				}
				seen[f.Name] = true
				result = append(result, f)
				if f.SDKName == "Cursor" {
					paginated = true
				}
			}
			continue
		}

		// Plain positional parameter.
		flagName := pName
		if override, ok := paramNameOverrides[pName]; ok {
			flagName = override
		} else {
			flagName = camelToKebab(pName)
		}
		if !seen[flagName] {
			seen[flagName] = true
			pi := paramInfo{
				Name:    flagName,
				SDKName: pName,
				Type:    goTypeToParamType(pType),
			}
			result = append(result, pi)
		}
	}

	return result, callStyle, paginated
}

// isContextType returns true if t is context.Context.
func isContextType(t types.Type) bool {
	// context.Context is an interface, so check both Named and the path.
	switch v := t.(type) {
	case *types.Named:
		obj := v.Obj()
		return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
	default:
		return false
	}
}

// extractStructType returns the underlying *types.Struct if t is a struct or
// *struct, otherwise nil.
func extractStructType(t types.Type) *types.Struct {
	// Dereference pointer.
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if namedT, ok := t.(*types.Named); ok {
		if s, isStruct := namedT.Underlying().(*types.Struct); isStruct {
			return s
		}
	}
	if s, ok := t.(*types.Struct); ok {
		return s
	}
	return nil
}

// extractStructFields extracts paramInfo entries from a struct type's exported fields.
func extractStructFields(s *types.Struct) []paramInfo {
	var result []paramInfo
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		name := f.Name()
		if override, ok := paramNameOverrides[name]; ok {
			name = override
		} else {
			name = camelToKebab(name)
		}
		pi := paramInfo{
			Name:    name,
			SDKName: f.Name(),
			Type:    goTypeToParamType(f.Type()),
		}
		result = append(result, pi)
	}
	return result
}

// goTypeToParamType maps a Go type to a CLI parameter type string.
func goTypeToParamType(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			return "string"
		case types.Int, types.Int64, types.Int32:
			return "int"
		case types.Bool:
			return "bool"
		case types.Float64, types.Float32:
			return "string" // Represent floats as strings for CLI.
		default:
			return "string"
		}
	case *types.Slice:
		if basic, isBasic := u.Elem().Underlying().(*types.Basic); isBasic && basic.Kind() == types.String {
			return "string-slice"
		}
		return "json"
	case *types.Pointer:
		return goTypeToParamType(u.Elem())
	case *types.Struct:
		return "json"
	case *types.Interface:
		return "json"
	default:
		return "string"
	}
}

// generateRegistry produces the source code for internal/registry/generated.go.
func generateRegistry(methods []methodInfo) []byte {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by cmd/introspect; DO NOT EDIT.\n\n")
	buf.WriteString("package registry\n\n")
	buf.WriteString("func init() {\n")
	buf.WriteString("\tRegistry = []MethodDef{\n")

	for _, m := range methods {
		docsURL := fmt.Sprintf("https://api.slack.com/methods/%s", m.APIMethod)

		buf.WriteString("\t\t{\n")
		fmt.Fprintf(&buf, "\t\t\tAPIMethod:   %q,\n", m.APIMethod)
		fmt.Fprintf(&buf, "\t\t\tCategory:    %q,\n", m.Category)
		fmt.Fprintf(&buf, "\t\t\tCommand:     %q,\n", m.Command)
		fmt.Fprintf(&buf, "\t\t\tSDKMethod:   %q,\n", m.GoName)
		fmt.Fprintf(&buf, "\t\t\tDescription: %q,\n", m.APIMethod)
		fmt.Fprintf(&buf, "\t\t\tDocsURL:     %q,\n", docsURL)
		fmt.Fprintf(&buf, "\t\t\tCallStyle:   %q,\n", m.CallStyle)

		if m.Paginated {
			buf.WriteString("\t\t\tPaginated:   true,\n")
			buf.WriteString("\t\t\tCursorParam: \"cursor\",\n")
			buf.WriteString("\t\t\tCursorField: \"next_cursor\",\n")
		}

		if len(m.Params) > 0 {
			buf.WriteString("\t\t\tParams: []ParamDef{\n")
			for _, p := range m.Params {
				buf.WriteString("\t\t\t\t{")
				fmt.Fprintf(&buf, "Name: %q, SDKName: %q, Type: %q", p.Name, p.SDKName, p.Type)
				buf.WriteString("},\n")
			}
			buf.WriteString("\t\t\t},\n")
		}

		buf.WriteString("\t\t},\n")
	}

	buf.WriteString("\t}\n")
	buf.WriteString("}\n")

	return buf.Bytes()
}

// generateDispatch produces the source code for internal/dispatch/generated_dispatch.go.
func generateDispatch(methods []methodInfo) []byte {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by cmd/introspect; DO NOT EDIT.\n\n")
	buf.WriteString("package dispatch\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"fmt\"\n\n")
	buf.WriteString("\t\"github.com/slack-go/slack\"\n")
	buf.WriteString(")\n\n")

	// Generate a stub dispatch function per method.
	for _, m := range methods {
		funcName := "dispatch" + m.BaseName
		fmt.Fprintf(&buf, "func %s(_ context.Context, _ *slack.Client, _ map[string]any) (any, error) {\n", funcName)
		fmt.Fprintf(&buf, "\treturn nil, fmt.Errorf(\"not yet implemented for %s\")\n", m.APIMethod)
		buf.WriteString("}\n\n")
	}

	// Generate init() that registers all dispatch functions.
	buf.WriteString("func init() {\n")
	for _, m := range methods {
		funcName := "dispatch" + m.BaseName
		fmt.Fprintf(&buf, "\tRegisterDispatch(%q, %s)\n", m.APIMethod, funcName)
	}
	buf.WriteString("}\n")

	return buf.Bytes()
}

// writeFormatted runs gofmt on src and writes the result to path.
func writeFormatted(path string, src []byte) {
	formatted, err := format.Source(src)
	if err != nil {
		// Write raw source for debugging.
		_ = os.WriteFile(path+".raw", src, 0o644)
		log.Fatalf("format.Source for %s: %v", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

// findProjectRoot walks up from the current directory looking for go.mod.
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}
