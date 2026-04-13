package override

import (
	"testing"
)

func TestParseSlackURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantChannel string
		wantTS      string
		wantErr     string
	}{
		{
			name:        "public channel URL",
			url:         "https://stackexchange.slack.com/archives/C0AFM69EB1B/p1775827095264229",
			wantChannel: "C0AFM69EB1B",
			wantTS:      "1775827095.264229",
		},
		{
			name:        "DM channel URL",
			url:         "https://stackexchange.slack.com/archives/D09C0KHRF9B/p1776101206614149",
			wantChannel: "D09C0KHRF9B",
			wantTS:      "1776101206.614149",
		},
		{
			name:        "short timestamp",
			url:         "https://stackexchange.slack.com/archives/C01ABC/p1234567890123456",
			wantChannel: "C01ABC",
			wantTS:      "1234567890.123456",
		},
		{
			name:    "missing archives segment",
			url:     "https://stackexchange.slack.com/channels/C0AFM69EB1B/p1775827095264229",
			wantErr: "invalid slack url: missing /archives/ path",
		},
		{
			name:    "channel does not start with C or D",
			url:     "https://stackexchange.slack.com/archives/E0AFM69EB1B/p1775827095264229",
			wantErr: "invalid slack url: channel must start with C or D",
		},
		{
			name:    "missing timestamp segment",
			url:     "https://stackexchange.slack.com/archives/C0AFM69EB1B",
			wantErr: "invalid slack url: missing timestamp segment",
		},
		{
			name:    "timestamp segment too short",
			url:     "https://stackexchange.slack.com/archives/C0AFM69EB1B/p12345",
			wantErr: "invalid slack url: timestamp segment too short",
		},
		{
			name:    "timestamp missing p prefix",
			url:     "https://stackexchange.slack.com/archives/C0AFM69EB1B/1775827095264229",
			wantErr: "invalid slack url: timestamp segment too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, ts, err := parseSlackURL(tt.url)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseSlackURL(%q): expected error %q, got nil", tt.url, tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSlackURL(%q): unexpected error: %v", tt.url, err)
			}
			if channel != tt.wantChannel {
				t.Errorf("channel = %q, want %q", channel, tt.wantChannel)
			}
			if ts != tt.wantTS {
				t.Errorf("ts = %q, want %q", ts, tt.wantTS)
			}
		})
	}
}
