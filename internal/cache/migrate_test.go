package cache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMigrateV1(t *testing.T) {
	dir := withTempCacheDir(t)

	// Write a v1 legacy cache.
	legacy := LegacyData{
		Channels:   map[string]string{"general": "C01", "random": "C02"},
		Users:      map[string]string{"poconnor": "U01", "jsmith": "U02"},
		Usergroups: map[string]string{"platform-team": "S01"},
	}
	raw, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, LegacyFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	// Run migration (no fetcher needed for split-only).
	if err := migrateV1(); err != nil {
		t.Fatalf("migrateV1: %v", err)
	}

	// Legacy file deleted.
	if _, err := os.Stat(filepath.Join(dir, LegacyFileName)); !os.IsNotExist(err) {
		t.Error("legacy file not deleted")
	}

	// Channels split correctly.
	channels, err := LoadEntity[ChannelCache](ChannelsFileName)
	if err != nil {
		t.Fatalf("load channels: %v", err)
	}
	wantChannels := ChannelCache{"general": "C01", "random": "C02"}
	if diff := cmp.Diff(wantChannels, channels); diff != "" {
		t.Errorf("channels (-want +got):\n%s", diff)
	}

	// People split (flat, pre-enrichment).
	people, err := LoadEntity[PeopleCache](PeopleFileName)
	if err != nil {
		t.Fatalf("load people: %v", err)
	}
	if people["poconnor"].ID != "U01" {
		t.Errorf("people poconnor.ID = %q, want U01", people["poconnor"].ID)
	}
	if people["poconnor"].Email != "" {
		t.Errorf("people poconnor.Email = %q, want empty (pre-enrichment)", people["poconnor"].Email)
	}

	// Usergroups split (flat, pre-enrichment).
	groups, err := LoadEntity[UsergroupCache](UsergroupsFileName)
	if err != nil {
		t.Fatalf("load usergroups: %v", err)
	}
	if groups["platform-team"].ID != "S01" {
		t.Errorf("usergroup ID = %q, want S01", groups["platform-team"].ID)
	}

	// Meta version is 1 (split but not enriched).
	version, _ := MetaVersion()
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
}

func TestEnsureReadyFreshInstall(t *testing.T) {
	withTempCacheDir(t)

	needsWarm, err := EnsureReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if !needsWarm {
		t.Error("expected needsWarm=true for fresh install")
	}
}

func TestEnsureReadyV1Migration(t *testing.T) {
	dir := withTempCacheDir(t)

	// Write v1 legacy cache.
	legacy := LegacyData{
		Channels:   map[string]string{"general": "C01"},
		Users:      map[string]string{"poconnor": "U01"},
		Usergroups: map[string]string{"team": "S01"},
	}
	raw, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, LegacyFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	fetcher := enrichedMockFetcher()
	needsWarm, err := EnsureReady(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if needsWarm {
		t.Error("expected needsWarm=false after migration+enrichment")
	}

	// Should be fully migrated to v2.
	version, _ := MetaVersion()
	if version != CurrentVersion {
		t.Errorf("version = %d, want %d", version, CurrentVersion)
	}

	// Legacy deleted.
	if _, err := os.Stat(filepath.Join(dir, LegacyFileName)); !os.IsNotExist(err) {
		t.Error("legacy file not deleted")
	}
}

func TestEnsureReadyVersion1NeedsEnrichment(t *testing.T) {
	withTempCacheDir(t)

	// Set up version 1 state (split but not enriched).
	for _, err := range []error{
		SaveEntity(ChannelsFileName, ChannelCache{"general": "C01"}),
		SaveEntity(PeopleFileName, PeopleCache{"poconnor": {ID: "U01"}}),
		SaveEntity(UsergroupsFileName, UsergroupCache{"team": {ID: "S01"}}),
		SaveMeta(CacheMeta{Version: 1}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	fetcher := enrichedMockFetcher()
	needsWarm, err := EnsureReady(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if needsWarm {
		t.Error("expected needsWarm=false after enrichment")
	}

	version, _ := MetaVersion()
	if version != CurrentVersion {
		t.Errorf("version = %d, want %d", version, CurrentVersion)
	}

	// People should be enriched.
	people, _ := LoadEntity[PeopleCache](PeopleFileName)
	if people["poconnor"].Email == "" {
		t.Error("people not enriched - email still empty")
	}
}

func TestEnsureReadyVersion2Fresh(t *testing.T) {
	withTempCacheDir(t)

	if err := SaveEntity(ChannelsFileName, ChannelCache{"general": "C01"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMeta(CacheMeta{Version: CurrentVersion}); err != nil {
		t.Fatal(err)
	}

	needsWarm, err := EnsureReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if needsWarm {
		t.Error("expected needsWarm=false for fresh v2 cache")
	}
}

func TestEnsureReadyEnrichmentFailsGracefully(t *testing.T) {
	withTempCacheDir(t)

	// Version 1 state, but no fetcher (nil client).
	for _, err := range []error{
		SaveEntity(ChannelsFileName, ChannelCache{"general": "C01"}),
		SaveEntity(PeopleFileName, PeopleCache{"poconnor": {ID: "U01"}}),
		SaveMeta(CacheMeta{Version: 1}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	// nil fetcher - enrichment can't happen.
	needsWarm, err := EnsureReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if needsWarm {
		t.Error("expected needsWarm=false (resolve from flat data)")
	}

	// Version should still be 1 (enrichment didn't happen).
	version, _ := MetaVersion()
	if version != 1 {
		t.Errorf("version = %d, want 1 (enrichment skipped)", version)
	}
}

func TestMigrateV2ToV3(t *testing.T) {
	withTempCacheDir(t)

	// Set up a v2 cache: people.json exists, meta is version 2.
	people := PeopleCache{
		"poconnor": {ID: "U01POCONNOR", DisplayName: "Peter O'Connor"},
		"jsmith":   {ID: "U02JSMITH", DisplayName: ""},  // empty display name → falls back to username
	}
	if err := SaveEntity(PeopleFileName, people); err != nil {
		t.Fatal(err)
	}
	if err := SaveMeta(CacheMeta{Version: 2}); err != nil {
		t.Fatal(err)
	}

	if err := migrateV2toV3(); err != nil {
		t.Fatalf("migrateV2toV3: %v", err)
	}

	// id-to-name.json written before meta bumped.
	idToName, err := LoadIDToNameMap()
	if err != nil {
		t.Fatalf("LoadIDToNameMap: %v", err)
	}
	if idToName["U01POCONNOR"] != "Peter O'Connor" {
		t.Errorf("id-to-name[U01POCONNOR] = %q, want Peter O'Connor", idToName["U01POCONNOR"])
	}
	if idToName["U02JSMITH"] != "jsmith" {
		t.Errorf("id-to-name[U02JSMITH] = %q, want jsmith (username fallback)", idToName["U02JSMITH"])
	}

	// Meta bumped to 3.
	version, _ := MetaVersion()
	if version != 3 {
		t.Errorf("version = %d, want 3", version)
	}
}

func TestEnsureReadyVersion2Migration(t *testing.T) {
	withTempCacheDir(t)

	// Set up a v2 cache.
	people := PeopleCache{
		"poconnor": {ID: "U01", DisplayName: "Peter O'Connor"},
	}
	if err := SaveEntity(ChannelsFileName, ChannelCache{"general": "C01"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveEntity(PeopleFileName, people); err != nil {
		t.Fatal(err)
	}
	if err := SaveMeta(CacheMeta{Version: 2}); err != nil {
		t.Fatal(err)
	}

	needsWarm, err := EnsureReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if needsWarm {
		t.Error("expected needsWarm=false after v2→v3 migration")
	}

	// Should now be v3.
	version, _ := MetaVersion()
	if version != 3 {
		t.Errorf("version = %d, want 3", version)
	}

	// id-to-name.json should exist.
	idToName, err := LoadIDToNameMap()
	if err != nil {
		t.Fatalf("LoadIDToNameMap after migration: %v", err)
	}
	if idToName["U01"] != "Peter O'Connor" {
		t.Errorf("id-to-name[U01] = %q, want Peter O'Connor", idToName["U01"])
	}
}
