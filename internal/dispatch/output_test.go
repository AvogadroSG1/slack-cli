package dispatch_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/poconnor/slack-cli/internal/dispatch"
)

func TestFormatOutputJSON(t *testing.T) {
	tests := []struct {
		name string
		data any
		want string
	}{
		{
			name: "map produces indented JSON",
			data: map[string]any{"ok": true, "channel": "C123"},
			want: `{
  "channel": "C123",
  "ok": true
}
`,
		},
		{
			name: "slice produces indented JSON",
			data: []string{"alpha", "bravo"},
			want: `[
  "alpha",
  "bravo"
]
`,
		},
		{
			name: "string produces indented JSON",
			data: "hello",
			want: "\"hello\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := dispatch.FormatOutput(&buf, tt.data, false); err != nil {
				t.Fatalf("FormatOutput() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, buf.String()); diff != "" {
				t.Errorf("FormatOutput() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatOutputPrettyMap(t *testing.T) {
	data := map[string]any{
		"channel": "general",
		"ok":      true,
	}

	var buf bytes.Buffer
	if err := dispatch.FormatOutput(&buf, data, true); err != nil {
		t.Fatalf("FormatOutput(pretty=true) error: %v", err)
	}

	got := buf.String()
	if len(got) == 0 {
		t.Fatal("FormatOutput(pretty=true) produced empty output for map")
	}

	// Verify the output is NOT valid JSON (it should be tabwriter output).
	if json.Valid([]byte(got)) {
		t.Error("FormatOutput(pretty=true) for map should produce tabwriter output, not JSON")
	}
}

func TestFormatOutputPrettySlice(t *testing.T) {
	data := []string{"alpha", "bravo"}

	var buf bytes.Buffer
	if err := dispatch.FormatOutput(&buf, data, true); err != nil {
		t.Fatalf("FormatOutput(pretty=true, slice) error: %v", err)
	}

	// Pretty output for slices falls back to indented JSON.
	want := `[
  "alpha",
  "bravo"
]
`
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("FormatOutput(pretty=true, slice) mismatch (-want +got):\n%s", diff)
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		code     int
		wantOK   bool
		wantErr  string
		wantCode int
	}{
		{
			name:     "auth error",
			errMsg:   "invalid_auth",
			code:     2,
			wantOK:   false,
			wantErr:  "invalid_auth",
			wantCode: 2,
		},
		{
			name:     "input error",
			errMsg:   "missing required flag --channel",
			code:     3,
			wantOK:   false,
			wantErr:  "missing required flag --channel",
			wantCode: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := dispatch.FormatError(&buf, tt.errMsg, tt.code); err != nil {
				t.Fatalf("FormatError() error: %v", err)
			}

			var got struct {
				OK       bool   `json:"ok"`
				Error    string `json:"error"`
				ExitCode int    `json:"exit_code"`
			}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("FormatError() produced invalid JSON: %v\nraw: %s", err, buf.String())
			}

			if diff := cmp.Diff(tt.wantOK, got.OK); diff != "" {
				t.Errorf("ok mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantErr, got.Error); diff != "" {
				t.Errorf("error mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantCode, got.ExitCode); diff != "" {
				t.Errorf("exit_code mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
