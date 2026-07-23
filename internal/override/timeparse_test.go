package override

import (
	"strings"
	"testing"
	"time"
)

func TestParseTimeSpec(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		spec string
		want string
	}{
		{"raw slack ts passes through", "1775827095.264229", "1775827095.264229"},
		{"hours ago", "2h", slackTS(now.Add(-2 * time.Hour))},
		{"minutes ago", "30m", slackTS(now.Add(-30 * time.Minute))},
		{"seconds ago", "90s", slackTS(now.Add(-90 * time.Second))},
		{"days ago", "2d", slackTS(now.Add(-48 * time.Hour))},
		{"weeks ago", "1w", slackTS(now.Add(-7 * 24 * time.Hour))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeSpec(tt.spec, now)
			if err != nil {
				t.Fatalf("parseTimeSpec(%q) error: %v", tt.spec, err)
			}
			if got != tt.want {
				t.Errorf("parseTimeSpec(%q) = %q, want %q", tt.spec, got, tt.want)
			}
		})
	}
}

func TestParseTimeSpecDate(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	got, err := parseTimeSpec("2026-07-01", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := slackTS(time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local))
	if got != want {
		t.Errorf("parseTimeSpec(date) = %q, want %q", got, want)
	}
}

func TestParseTimeSpecErrors(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	for _, spec := range []string{"", "garbage", "-2h", "07/01/2026"} {
		t.Run(spec, func(t *testing.T) {
			if _, err := parseTimeSpec(spec, now); err == nil {
				t.Errorf("parseTimeSpec(%q) expected error", spec)
			}
		})
	}
}

func TestParseTimeSpecAsDate(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		spec string
		want string
	}{
		{"2026-07-01", "2026-07-01"},
		{"1w", now.Add(-7 * 24 * time.Hour).Local().Format("2006-01-02")},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got, err := parseTimeSpecAsDate(tt.spec, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseTimeSpecAsDate(%q) = %q, want %q", tt.spec, got, tt.want)
			}
		})
	}

	if _, err := parseTimeSpecAsDate("nope", now); err == nil {
		t.Error("expected error for invalid spec")
	}
}

func TestParseTimeSpecErrorMentionsExpectedForms(t *testing.T) {
	_, err := parseTimeSpec("bogus", time.Now())
	if err == nil || !strings.Contains(err.Error(), "2h") {
		t.Errorf("error should hint at accepted forms, got %v", err)
	}
}
