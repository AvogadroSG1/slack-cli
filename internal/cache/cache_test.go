package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// withTempCacheDir sets SLACK_CLI_CACHE_DIR to a temp directory for the
// duration of the test and restores the original value on cleanup.
func withTempCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SLACK_CLI_CACHE_DIR", dir)
	return dir
}

func testChannels() ChannelCache {
	return ChannelCache{
		"general":              "C01GENERAL",
		"platform-engineering": "C02PLATFORM",
	}
}

func testPeople() PeopleCache {
	return PeopleCache{
		"poconnor": {
			ID:          "U01POCONNOR",
			Email:       "poconnor@stackoverflow.com",
			DisplayName: "Peter O'Connor",
			Title:       "Sr. Director",
		},
		"jsmith": {
			ID:          "U02JSMITH",
			Email:       "jsmith@stackoverflow.com",
			DisplayName: "Jane Smith",
			Title:       "Staff Engineer",
		},
	}
}

func testUsergroups() UsergroupCache {
	return UsergroupCache{
		"platform-team": {
			ID:          "S01PLATTEAM",
			Description: "Platform Engineering",
			Members:     []string{"U01POCONNOR", "U02JSMITH"},
		},
	}
}

func TestSaveAndLoadChannels(t *testing.T) {
	withTempCacheDir(t)
	want := testChannels()
	if err := SaveEntity(ChannelsFileName, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadEntity[ChannelCache](ChannelsFileName)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("channels mismatch (-want +got):\n%s", diff)
	}
}

func TestSaveAndLoadPeople(t *testing.T) {
	withTempCacheDir(t)
	want := testPeople()
	if err := SaveEntity(PeopleFileName, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadEntity[PeopleCache](PeopleFileName)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("people mismatch (-want +got):\n%s", diff)
	}
}

func TestSaveAndLoadUsergroups(t *testing.T) {
	withTempCacheDir(t)
	want := testUsergroups()
	if err := SaveEntity(UsergroupsFileName, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadEntity[UsergroupCache](UsergroupsFileName)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("usergroups mismatch (-want +got):\n%s", diff)
	}
}

func TestUsergroupMemberCount(t *testing.T) {
	ug := UsergroupEntry{Members: []string{"U01", "U02", "U03"}}
	if got := ug.MemberCount(); got != 3 {
		t.Errorf("MemberCount() = %d, want 3", got)
	}
}

func TestUsergroupMemberCountEmpty(t *testing.T) {
	ug := UsergroupEntry{}
	if got := ug.MemberCount(); got != 0 {
		t.Errorf("MemberCount() = %d, want 0", got)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested", "deep")
	t.Setenv("SLACK_CLI_CACHE_DIR", nested)

	if err := SaveEntity(ChannelsFileName, testChannels()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p := filepath.Join(nested, ChannelsFileName)
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file not created at %s: %v", p, err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	withTempCacheDir(t)
	_, err := LoadEntity[ChannelCache](ChannelsFileName)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := withTempCacheDir(t)
	p := filepath.Join(dir, ChannelsFileName)
	if err := os.WriteFile(p, []byte("not json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadEntity[ChannelCache](ChannelsFileName)
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestIsStale(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		wantBool bool
	}{
		{
			name:     "missing meta is stale",
			setup:    func(t *testing.T, dir string) {},
			wantBool: true,
		},
		{
			name: "fresh meta is not stale",
			setup: func(t *testing.T, dir string) {
				if err := SaveMeta(CacheMeta{Version: CurrentVersion}); err != nil {
					t.Fatal(err)
				}
			},
			wantBool: false,
		},
		{
			name: "old meta is stale",
			setup: func(t *testing.T, dir string) {
				if err := SaveMeta(CacheMeta{Version: CurrentVersion}); err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(dir, MetaFileName)
				old := time.Now().Add(-25 * time.Hour)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			},
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := withTempCacheDir(t)
			tt.setup(t, dir)
			got, err := IsStale()
			if err != nil {
				t.Fatalf("IsStale: %v", err)
			}
			if got != tt.wantBool {
				t.Errorf("IsStale = %v, want %v", got, tt.wantBool)
			}
		})
	}
}

func TestMetaVersion(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		want    int
	}{
		{
			name:  "no meta returns 0",
			setup: func(t *testing.T) {},
			want:  0,
		},
		{
			name: "version 1",
			setup: func(t *testing.T) {
				t.Helper()
				if err := SaveMeta(CacheMeta{Version: 1}); err != nil {
					t.Fatal(err)
				}
			},
			want: 1,
		},
		{
			name: "version 2",
			setup: func(t *testing.T) {
				t.Helper()
				if err := SaveMeta(CacheMeta{Version: 2}); err != nil {
					t.Fatal(err)
				}
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTempCacheDir(t)
			tt.setup(t)
			got, err := MetaVersion()
			if err != nil {
				t.Fatalf("MetaVersion: %v", err)
			}
			if got != tt.want {
				t.Errorf("MetaVersion = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestClear(t *testing.T) {
	dir := withTempCacheDir(t)

	// Write all files.
	for _, err := range []error{
		SaveEntity(ChannelsFileName, testChannels()),
		SaveEntity(PeopleFileName, testPeople()),
		SaveEntity(UsergroupsFileName, testUsergroups()),
		SaveMeta(CacheMeta{Version: 2}),
		SaveEntity(IDToNameFileName, map[string]string{"U01": "Peter"}),
		os.WriteFile(filepath.Join(dir, LockFileName), nil, 0o644),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	for _, f := range []string{ChannelsFileName, PeopleFileName, UsergroupsFileName, MetaFileName, LockFileName} {
		p := filepath.Join(dir, f)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after Clear", f)
		}
	}
}

func TestClearEmpty(t *testing.T) {
	withTempCacheDir(t)
	if err := Clear(); err != nil {
		t.Fatalf("Clear on empty dir: %v", err)
	}
}

func TestDirEnvOverride(t *testing.T) {
	t.Setenv("SLACK_CLI_CACHE_DIR", "/tmp/custom")
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/custom" {
		t.Errorf("Dir = %q, want /tmp/custom", dir)
	}
}

func TestDirDefault(t *testing.T) {
	t.Setenv("SLACK_CLI_CACHE_DIR", "")
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, DefaultDir)
	if dir != want {
		t.Errorf("Dir = %q, want %q", dir, want)
	}
}

func TestFileLocking(t *testing.T) {
	withTempCacheDir(t)

	lock1, err := AcquireExclusive()
	if err != nil {
		t.Fatalf("AcquireExclusive: %v", err)
	}
	lock1.Close()

	lock2, err := AcquireShared()
	if err != nil {
		t.Fatalf("AcquireShared: %v", err)
	}
	lock3, err := AcquireShared()
	if err != nil {
		t.Fatalf("second AcquireShared: %v", err)
	}
	lock2.Close()
	lock3.Close()
}
