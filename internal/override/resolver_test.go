package override

import (
	"strings"
	"testing"

	"github.com/poconnor/slack-cli/internal/cache"
)

// seedResolverCache points the cache at a temp dir and seeds channel, people,
// and usergroup fixtures.
func seedResolverCache(t *testing.T) {
	t.Helper()
	t.Setenv("SLACK_CLI_CACHE_DIR", t.TempDir())

	if err := cache.SaveEntity(cache.ChannelsFileName, cache.ChannelCache{
		"general": "C0AAAAAAAAA",
	}); err != nil {
		t.Fatal(err)
	}
	if err := cache.SaveEntity(cache.PeopleFileName, cache.PeopleCache{
		"alice": {ID: "U0AAAAAAAAA", Email: "alice@example.com"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := cache.SaveEntity(cache.UsergroupsFileName, cache.UsergroupCache{
		"oncall": {ID: "S0AAAAAAAAA"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestResolveChannelArg(t *testing.T) {
	seedResolverCache(t)

	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"hash prefix", "#general", "C0AAAAAAAAA"},
		{"bare name", "general", "C0AAAAAAAAA"},
		{"raw ID bypasses cache", "C0ZZZZZZZZZ", "C0ZZZZZZZZZ"},
		{"dm ID bypasses cache", "D0ZZZZZZZZZ", "D0ZZZZZZZZZ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveChannelArg(tt.arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveChannelArg(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestResolveChannelArgMissHint(t *testing.T) {
	seedResolverCache(t)
	_, err := resolveChannelArg("#nope")
	if err == nil || !strings.Contains(err.Error(), "cache warm") {
		t.Errorf("expected cache warm hint, got %v", err)
	}
}

func TestResolveUserArg(t *testing.T) {
	seedResolverCache(t)

	tests := []struct {
		arg  string
		want string
	}{
		{"@alice", "U0AAAAAAAAA"},
		{"alice", "U0AAAAAAAAA"},
		{"U0ZZZZZZZZZ", "U0ZZZZZZZZZ"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			got, err := resolveUserArg(tt.arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveUserArg(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestResolveTarget(t *testing.T) {
	seedResolverCache(t)

	tests := []struct {
		name     string
		arg      string
		wantID   string
		wantKind targetKind
	}{
		{"hash forces channel", "#general", "C0AAAAAAAAA", targetChannel},
		{"at forces user", "@alice", "U0AAAAAAAAA", targetUser},
		{"channel ID", "C0ZZZZZZZZZ", "C0ZZZZZZZZZ", targetChannel},
		{"user ID", "U0ZZZZZZZZZ", "U0ZZZZZZZZZ", targetUser},
		{"usergroup ID", "S0ZZZZZZZZZ", "S0ZZZZZZZZZ", targetUsergroup},
		{"bare channel name", "general", "C0AAAAAAAAA", targetChannel},
		{"bare user name", "alice", "U0AAAAAAAAA", targetUser},
		{"bare usergroup handle", "oncall", "S0AAAAAAAAA", targetUsergroup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, kind, err := resolveTarget(tt.arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID || kind != tt.wantKind {
				t.Errorf("resolveTarget(%q) = (%q, %d), want (%q, %d)", tt.arg, id, kind, tt.wantID, tt.wantKind)
			}
		})
	}
}

func TestResolveTargetMiss(t *testing.T) {
	seedResolverCache(t)
	_, _, err := resolveTarget("nobody")
	if err == nil || !strings.Contains(err.Error(), "cache warm") {
		t.Errorf("expected cache warm hint, got %v", err)
	}
}
