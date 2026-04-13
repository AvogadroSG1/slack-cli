// Package dispatch formats CLI output as JSON or human-readable text and
// writes it to the supplied io.Writer.
package dispatch

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

// FormatOutput writes data to w. When pretty is false the output is
// indented JSON (2-space indent). When pretty is true the format depends
// on the underlying type:
//   - Maps produce aligned key-value pairs via text/tabwriter.
//   - Everything else (including slices) falls back to indented JSON.
func FormatOutput(w io.Writer, data any, pretty bool) error {
	if !pretty {
		return writeJSON(w, data)
	}

	// Attempt map formatting when pretty is requested.
	if m, ok := toStringMap(data); ok {
		return writeMap(w, m)
	}

	// Slices and all other types fall back to indented JSON.
	return writeJSON(w, data)
}

// FormatError writes a JSON error envelope to w with the shape
// {"ok": false, "error": "<errMsg>", "exit_code": <code>}.
func FormatError(w io.Writer, errMsg string, code int) error {
	envelope := struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}{
		OK:       false,
		Error:    errMsg,
		ExitCode: code,
	}
	return writeJSON(w, envelope)
}

// writeJSON encodes v as 2-space-indented JSON followed by a newline.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// toStringMap attempts to interpret data as a map[string]any. It handles
// both map[string]any directly and the generic map[string]T via a JSON
// round-trip.
func toStringMap(data any) (map[string]any, bool) {
	// Fast path: already the right type.
	if m, ok := data.(map[string]any); ok {
		return m, true
	}

	// Slow path: round-trip through JSON to normalise typed maps
	// (e.g. map[string]string) into map[string]any.
	b, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}

// writeMap writes key-value pairs from m in sorted key order using
// text/tabwriter for aligned columns.
func writeMap(w io.Writer, m map[string]any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if _, err := fmt.Fprintf(tw, "%s\t%v\n", k, m[k]); err != nil {
			return err
		}
	}
	return tw.Flush()
}
