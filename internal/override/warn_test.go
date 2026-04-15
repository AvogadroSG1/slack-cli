package override

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/spf13/cobra"
)

func TestWarnIfCacheNotReady(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string)
		wantStderr string
		wantEmpty  bool
	}{
		{
			name:       "stale_no_meta_file",
			setup:      func(t *testing.T, dir string) {},
			wantStderr: `cache not warmed`,
		},
		{
			name: "fresh_cache",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeMeta(t, dir, cache.CacheMeta{Version: cache.CurrentVersion})
			},
			wantEmpty: true,
		},
		{
			name: "stale_old_mtime",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeMeta(t, dir, cache.CacheMeta{Version: cache.CurrentVersion})
				p := filepath.Join(dir, cache.MetaFileName)
				old := time.Now().Add(-25 * time.Hour)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			},
			wantStderr: `cache not warmed`,
		},
		{
			name: "corrupted_meta",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				p := filepath.Join(dir, cache.MetaFileName)
				if err := os.WriteFile(p, []byte(`{corrupt`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantStderr: `cache migration failed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("SLACK_CLI_CACHE_DIR", dir)
			tt.setup(t, dir)

			cmd := &cobra.Command{Use: "test"}
			var errBuf bytes.Buffer
			cmd.SetErr(&errBuf)

			warnIfCacheNotReady(cmd)

			stderr := errBuf.String()
			if tt.wantEmpty {
				if stderr != "" {
					t.Errorf("want empty stderr, got %q", stderr)
				}
				return
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantStderr)
			}
		})
	}
}

// writeMeta writes a CacheMeta JSON file to dir.
func writeMeta(t *testing.T, dir string, m cache.CacheMeta) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, cache.MetaFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
