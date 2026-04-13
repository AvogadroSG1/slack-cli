# thread-read and message-read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `thread-read` and `message-read` commands that return name-resolved, human-readable Slack content in a single call.

**Architecture:** Two top-level override commands using the existing `cache`/`resolve`/`api` pattern. User ID→name resolution backed by a new `id-to-name.json` cache file added via a zero-API-call v2→v3 migration. URL parsing via string manipulation from Slack's `p`-prefixed timestamp format. Shared formatter handles both plain text and JSON output.

**Tech Stack:** Go, Cobra v1.10.2, slack-go/slack, standard `testing` + `go-cmp/cmp`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/cache/cache.go` | Modify | Add `IDToNameFileName` constant, bump `CurrentVersion` to 3, add `id-to-name.json` to `Clear()` |
| `internal/cache/resolve.go` | Modify | Add `LoadIDToNameMap()`, `ResolveUserByID()` |
| `internal/cache/warm.go` | Modify | Add `buildIDToNameMap()` helper; `Warm` and `EnrichOnly` write `id-to-name.json` |
| `internal/cache/migrate.go` | Modify | Add `migrateV2toV3()`; add `case 2:` to `EnsureReady` switch |
| `internal/override/cache_cmd.go` | Modify | Add `ID mappings: N` line to `newCacheInfoCmd` |
| `internal/override/slack_url.go` | Create | `parseSlackURL()` — URL → (channel, ts) |
| `internal/override/read_format.go` | Create | `readMessage` struct, `formatMessages()`, `resolveUser()`, `parseSlackTimestamp()`, `resolveChannelTS()` |
| `internal/override/thread_read_cmd.go` | Create | `newThreadReadCmd()` and `runThreadRead()` |
| `internal/override/message_read_cmd.go` | Create | `newMessageReadCmd()` and `runMessageRead()` |
| `internal/override/api_list.go` | Modify | Register `thread-read` and `message-read` in `RegisterBuiltins` |
| `internal/cache/cache_test.go` | Modify | Add `IDToNameFileName` assertion to `TestClear` |
| `internal/cache/resolve_test.go` | Modify | Add `TestLoadIDToNameMap`, `TestResolveUserByID` |
| `internal/cache/warm_test.go` | Modify | Add `id-to-name.json` assertions to `TestWarm`, `TestEnrichOnly` |
| `internal/cache/migrate_test.go` | Modify | Add `TestMigrateV2ToV3`, `TestEnsureReadyVersion2Migration` |
| `internal/override/slack_url_test.go` | Create | Table-driven tests for `parseSlackURL` |
| `internal/override/read_format_test.go` | Create | Tests for formatter, helpers |
| `internal/override/thread_read_cmd_test.go` | Create | nil client, bad URL, flag contract tests |
| `internal/override/message_read_cmd_test.go` | Create | nil client, bad URL, no-message tests |

---

## Task 1: IDToNameFileName constant + Clear update

**Files:**
- Modify: `internal/cache/cache.go`
- Modify: `internal/cache/cache_test.go`

- [ ] **Step 1: Write failing test — Clear must delete id-to-name.json**

Add to `TestClear` in `internal/cache/cache_test.go`, inside the "Write all files" setup block:

```go
// Add after the existing SaveMeta line in TestClear setup:
if err := SaveEntity(IDToNameFileName, map[string]string{"U01": "Peter"}); err != nil {
    t.Fatal(err)
}
```

Add to the assertion loop at the bottom of `TestClear`:

```go
// Change existing loop to include IDToNameFileName:
for _, f := range []string{ChannelsFileName, PeopleFileName, UsergroupsFileName, MetaFileName, LockFileName, IDToNameFileName} {
```

- [ ] **Step 2: Run test — confirm it fails**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run TestClear -v
```

Expected: FAIL — `IDToNameFileName` undefined.

- [ ] **Step 3: Add constant and update Clear in cache.go**

In `internal/cache/cache.go`, add `IDToNameFileName` to the constants block (after `LegacyFileName`):

```go
const (
	MetaFileName       = "cache-meta.json"
	ChannelsFileName   = "channels.json"
	PeopleFileName     = "people.json"
	UsergroupsFileName = "usergroups.json"
	IDToNameFileName   = "id-to-name.json"
	LegacyFileName     = "cache.json" // v1 single-file format
	LockFileName       = "cache.lock"
)
```

In `Clear()`, add `IDToNameFileName` to the files slice:

```go
files := []string{
    MetaFileName, ChannelsFileName, PeopleFileName,
    UsergroupsFileName, IDToNameFileName, LockFileName, LegacyFileName,
}
```

- [ ] **Step 4: Run test — confirm it passes**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run TestClear -v
```

Expected: PASS.

- [ ] **Step 5: Run full cache test suite — no regressions**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/cache/cache.go internal/cache/cache_test.go && git commit -m "$(cat <<'EOF'
feat(cache): add IDToNameFileName constant and include in Clear

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: LoadIDToNameMap and ResolveUserByID

**Files:**
- Modify: `internal/cache/resolve.go`
- Modify: `internal/cache/resolve_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/cache/resolve_test.go`:

```go
func TestLoadIDToNameMap(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		withTempCacheDir(t)
		want := map[string]string{
			"U01POCONNOR": "Peter O'Connor",
			"U02JSMITH":   "Jane Smith",
		}
		if err := SaveEntity(IDToNameFileName, want); err != nil {
			t.Fatal(err)
		}
		got, err := LoadIDToNameMap()
		if err != nil {
			t.Fatalf("LoadIDToNameMap: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("(-want +got):\n%s", diff)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		withTempCacheDir(t)
		_, err := LoadIDToNameMap()
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestResolveUserByID(t *testing.T) {
	withTempCacheDir(t)
	idToName := map[string]string{
		"U01POCONNOR": "Peter O'Connor",
		"U02JSMITH":   "Jane Smith",
	}
	if err := SaveEntity(IDToNameFileName, idToName); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		id        string
		wantName  string
		wantFound bool
	}{
		{name: "hit", id: "U01POCONNOR", wantName: "Peter O'Connor", wantFound: true},
		{name: "hit second", id: "U02JSMITH", wantName: "Jane Smith", wantFound: true},
		{name: "miss", id: "U99NOBODY", wantName: "", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, found, err := ResolveUserByID(tt.id)
			if err != nil {
				t.Fatalf("ResolveUserByID(%q): %v", tt.id, err)
			}
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestResolveUserByIDMissingFile(t *testing.T) {
	withTempCacheDir(t)
	_, _, err := ResolveUserByID("U01")
	if err == nil {
		t.Fatal("expected error when id-to-name.json missing")
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run "TestLoadIDToNameMap|TestResolveUserByID" -v
```

Expected: FAIL — `LoadIDToNameMap` and `ResolveUserByID` undefined.

- [ ] **Step 3: Implement in resolve.go**

Add to `internal/cache/resolve.go` (after the existing `ResolveUsergroup` function):

```go
// LoadIDToNameMap reads id-to-name.json and returns the full userID→displayName
// map. Used by commands that need to resolve multiple IDs in one pass.
func LoadIDToNameMap() (map[string]string, error) {
	return LoadEntity[map[string]string](IDToNameFileName)
}

// ResolveUserByID looks up a Slack user ID and returns the display name.
// Returns ("", false, nil) when the ID is not in the cache — the caller
// decides the fallback (typically showing the raw ID).
// Returns ("", false, err) only on file I/O or parse failure.
func ResolveUserByID(id string) (name string, found bool, err error) {
	m, err := LoadIDToNameMap()
	if err != nil {
		return "", false, err
	}
	name, found = m[id]
	return name, found, nil
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run "TestLoadIDToNameMap|TestResolveUserByID" -v
```

Expected: PASS.

- [ ] **Step 5: Run full cache suite**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/cache/resolve.go internal/cache/resolve_test.go && git commit -m "$(cat <<'EOF'
feat(cache): add LoadIDToNameMap and ResolveUserByID

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: buildIDToNameMap + Warm/EnrichOnly write id-to-name.json

**Files:**
- Modify: `internal/cache/warm.go`
- Modify: `internal/cache/warm_test.go`

- [ ] **Step 1: Write failing tests**

In `TestWarm` in `internal/cache/warm_test.go`, add after the existing "Verify meta version" block:

```go
// Verify id-to-name.json written with correct reverse mappings.
idToName, err := LoadIDToNameMap()
if err != nil {
    t.Fatalf("LoadIDToNameMap after Warm: %v", err)
}
if idToName["U01"] != "Peter O'Connor" {
    t.Errorf("id-to-name[U01] = %q, want Peter O'Connor", idToName["U01"])
}
if idToName["U02"] != "Jane Smith" {
    t.Errorf("id-to-name[U02] = %q, want Jane Smith", idToName["U02"])
}
if _, ok := idToName["U03"]; ok {
    t.Error("id-to-name should not contain deleted user U03")
}
```

Add new test for `EnrichOnly` (after `TestEnrichOnly`):

```go
func TestEnrichOnlyWritesIDToNameMap(t *testing.T) {
	withTempCacheDir(t)

	if err := SaveEntity(ChannelsFileName, ChannelCache{"general": "C01"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMeta(CacheMeta{Version: 1}); err != nil {
		t.Fatal(err)
	}

	fetcher := enrichedMockFetcher()
	if err := EnrichOnly(context.Background(), fetcher); err != nil {
		t.Fatalf("EnrichOnly: %v", err)
	}

	idToName, err := LoadIDToNameMap()
	if err != nil {
		t.Fatalf("LoadIDToNameMap after EnrichOnly: %v", err)
	}
	if idToName["U01"] != "Peter O'Connor" {
		t.Errorf("id-to-name[U01] = %q, want Peter O'Connor", idToName["U01"])
	}
	if idToName["U02"] != "Jane Smith" {
		t.Errorf("id-to-name[U02] = %q, want Jane Smith", idToName["U02"])
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run "TestWarm$|TestEnrichOnlyWritesIDToNameMap" -v
```

Expected: FAIL — `LoadIDToNameMap` call in `TestWarm` fails (file not written), `TestEnrichOnlyWritesIDToNameMap` fails.

- [ ] **Step 3: Add buildIDToNameMap helper to warm.go**

Add to `internal/cache/warm.go` (after `EnrichOnly`):

```go
// buildIDToNameMap derives a userID→displayName reverse index from a
// PeopleCache. Prefers DisplayName; falls back to the Slack username key.
// The map is preallocated to the size of people to avoid rehashing.
func buildIDToNameMap(people PeopleCache) map[string]string {
	m := make(map[string]string, len(people))
	for username, entry := range people {
		if entry.ID == "" {
			continue
		}
		name := entry.DisplayName
		if name == "" {
			name = username
		}
		m[entry.ID] = name
	}
	return m
}
```

- [ ] **Step 4: Update Warm to write id-to-name.json**

In `internal/cache/warm.go`, update `Warm` to build and save the map. Add `idToName := buildIDToNameMap(people)` after the `fetchAllUsergroups` call, and add `SaveEntity(IDToNameFileName, idToName)` inside the lock block:

```go
func Warm(ctx context.Context, fetcher SlackFetcher) (*WarmResult, error) {
	channels, err := fetchAllChannels(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("warm channels: %w", err)
	}

	people, err := fetchAllPeople(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("warm people: %w", err)
	}

	usergroups, err := fetchAllUsergroups(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("warm usergroups: %w", err)
	}

	idToName := buildIDToNameMap(people)

	lock, err := AcquireExclusive()
	if err != nil {
		return nil, fmt.Errorf("warm lock: %w", err)
	}
	defer lock.Close()

	if err := SaveEntity(ChannelsFileName, channels); err != nil {
		return nil, err
	}
	if err := SaveEntity(PeopleFileName, people); err != nil {
		return nil, err
	}
	if err := SaveEntity(UsergroupsFileName, usergroups); err != nil {
		return nil, err
	}
	if err := SaveEntity(IDToNameFileName, idToName); err != nil {
		return nil, err
	}
	if err := SaveMeta(CacheMeta{Version: CurrentVersion}); err != nil {
		return nil, err
	}

	return &WarmResult{
		Channels:   len(channels),
		Users:      len(people),
		Usergroups: len(usergroups),
	}, nil
}
```

- [ ] **Step 5: Update EnrichOnly to write id-to-name.json**

In `internal/cache/warm.go`, update `EnrichOnly`:

```go
func EnrichOnly(ctx context.Context, fetcher SlackFetcher) error {
	people, err := fetchAllPeople(ctx, fetcher)
	if err != nil {
		return fmt.Errorf("enrich people: %w", err)
	}

	usergroups, err := fetchAllUsergroups(ctx, fetcher)
	if err != nil {
		return fmt.Errorf("enrich usergroups: %w", err)
	}

	idToName := buildIDToNameMap(people)

	lock, err := AcquireExclusive()
	if err != nil {
		return fmt.Errorf("enrich lock: %w", err)
	}
	defer lock.Close()

	if err := SaveEntity(PeopleFileName, people); err != nil {
		return err
	}
	if err := SaveEntity(UsergroupsFileName, usergroups); err != nil {
		return err
	}
	if err := SaveEntity(IDToNameFileName, idToName); err != nil {
		return err
	}
	return SaveMeta(CacheMeta{Version: CurrentVersion})
}
```

- [ ] **Step 6: Run tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run "TestWarm$|TestEnrichOnlyWritesIDToNameMap|TestEnrichOnly$" -v
```

Expected: PASS.

- [ ] **Step 7: Run full cache suite**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -v
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/cache/warm.go internal/cache/warm_test.go && git commit -m "$(cat <<'EOF'
feat(cache): Warm and EnrichOnly write id-to-name.json reverse index

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: migrateV2toV3 + EnsureReady case 2 + CurrentVersion=3

**Files:**
- Modify: `internal/cache/cache.go`
- Modify: `internal/cache/migrate.go`
- Modify: `internal/cache/migrate_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/cache/migrate_test.go`:

```go
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
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run "TestMigrateV2ToV3|TestEnsureReadyVersion2Migration" -v
```

Expected: FAIL — `migrateV2toV3` undefined, `case 2:` not handled.

- [ ] **Step 3: Bump CurrentVersion to 3 in cache.go**

In `internal/cache/cache.go`, change:

```go
const CurrentVersion = 3
```

- [ ] **Step 4: Add migrateV2toV3 to migrate.go**

Add after `enrichAndReturn` in `internal/cache/migrate.go`:

```go
// migrateV2toV3 derives id-to-name.json from the existing people.json,
// then bumps the cache version to 3. Zero API calls required.
// id-to-name.json is written before SaveMeta so that a crash between the
// two leaves the cache at v2 (safe to re-migrate).
func migrateV2toV3() error {
	lock, err := AcquireExclusive()
	if err != nil {
		return fmt.Errorf("v3 migration lock: %w", err)
	}
	defer lock.Close()

	people, err := LoadEntity[PeopleCache](PeopleFileName)
	if err != nil {
		return fmt.Errorf("v3 migration read people: %w", err)
	}

	idToName := buildIDToNameMap(people)

	if err := SaveEntity(IDToNameFileName, idToName); err != nil {
		return fmt.Errorf("v3 migration write id-to-name: %w", err)
	}

	return SaveMeta(CacheMeta{Version: 3})
}
```

- [ ] **Step 5: Update EnsureReady switch in migrate.go**

Replace the switch body in `EnsureReady` with the new case structure. The updated switch (leave case 0 and case 1 unchanged):

```go
switch version {
case 0:
    // No metadata file. Check for legacy cache.
    hasLegacy, err := HasLegacyCache()
    if err != nil {
        return false, err
    }
    if !hasLegacy {
        // Fresh install — caller should do a full warm.
        return true, nil
    }
    // Migrate v1 → split files.
    if err := migrateV1(); err != nil {
        return false, fmt.Errorf("v1 migration: %w", err)
    }
    // Fall through to enrichment.
    return enrichAndReturn(ctx, fetcher)

case 1:
    // Split files exist but not enriched.
    return enrichAndReturn(ctx, fetcher)

case 2:
    // v2→v3: derive id-to-name.json from existing people.json, no API calls.
    if err := migrateV2toV3(); err != nil {
        return false, fmt.Errorf("v3 migration: %w", err)
    }
    stale, err := IsStale()
    if err != nil {
        return false, err
    }
    return stale, nil

case CurrentVersion: // 3
    stale, err := IsStale()
    if err != nil {
        return false, err
    }
    return stale, nil

default:
    // Unknown future version — treat as current.
    stale, err := IsStale()
    if err != nil {
        return false, err
    }
    return stale, nil
}
```

- [ ] **Step 6: Run new tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -run "TestMigrateV2ToV3|TestEnsureReadyVersion2Migration" -v
```

Expected: PASS.

- [ ] **Step 7: Run full cache suite — no regressions**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/cache/ -v
```

Expected: all PASS. (Existing tests using `CurrentVersion` constant automatically use 3.)

- [ ] **Step 8: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/cache/cache.go internal/cache/migrate.go internal/cache/migrate_test.go && git commit -m "$(cat <<'EOF'
feat(cache): v2->v3 migration derives id-to-name.json without API calls

CurrentVersion bumped to 3. EnsureReady case 2 runs migrateV2toV3.

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: cache info shows ID mappings count

**Files:**
- Modify: `internal/override/cache_cmd.go`

No automated tests for this command handler — verified manually via build.

- [ ] **Step 1: Add ID mappings line to newCacheInfoCmd**

In `internal/override/cache_cmd.go`, inside `newCacheInfoCmd`'s `RunE`, add after the usergroups block (still inside the shared lock):

```go
idToName, err := cache.LoadIDToNameMap()
if err == nil {
    fmt.Fprintf(w, "ID mappings: %d\n", len(idToName))
}
```

The full lock section in `newCacheInfoCmd` should now read:

```go
lock, err := cache.AcquireShared()
if err != nil {
    return nil
}
defer lock.Close()

channels, err := cache.LoadEntity[cache.ChannelCache](cache.ChannelsFileName)
if err == nil {
    fmt.Fprintf(w, "Channels: %d\n", len(channels))
}

people, err := cache.LoadEntity[cache.PeopleCache](cache.PeopleFileName)
if err == nil {
    fmt.Fprintf(w, "People: %d\n", len(people))
}

groups, err := cache.LoadEntity[cache.UsergroupCache](cache.UsergroupsFileName)
if err == nil {
    fmt.Fprintf(w, "Usergroups: %d\n", len(groups))
}

idToName, err := cache.LoadIDToNameMap()
if err == nil {
    fmt.Fprintf(w, "ID mappings: %d\n", len(idToName))
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/override/cache_cmd.go && git commit -m "$(cat <<'EOF'
feat(cache): show ID mappings count in cache info

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: URL parsing

**Files:**
- Create: `internal/override/slack_url.go`
- Create: `internal/override/slack_url_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/override/slack_url_test.go`:

```go
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
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run TestParseSlackURL -v
```

Expected: FAIL — `parseSlackURL` undefined.

- [ ] **Step 3: Implement parseSlackURL**

Create `internal/override/slack_url.go`:

```go
package override

import (
	"fmt"
	"net/url"
	"strings"
)

// parseSlackURL extracts the channel ID and message timestamp from a Slack
// message URL of the form:
//
//	https://<workspace>.slack.com/archives/<channelID>/p<ts_no_dot>
//
// Timestamp reconstruction uses string manipulation: strip the 'p' prefix and
// insert '.' before the last 6 digits. No float parsing to avoid precision loss.
func parseSlackURL(rawURL string) (channel, ts string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid slack url: %w", err)
	}

	// Find the /archives/ segment.
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	archivesIdx := -1
	for i, p := range parts {
		if p == "archives" {
			archivesIdx = i
			break
		}
	}
	if archivesIdx < 0 || len(parts) < archivesIdx+2 {
		return "", "", fmt.Errorf("invalid slack url: missing /archives/ path")
	}

	channelSeg := parts[archivesIdx+1]
	if len(channelSeg) == 0 || (channelSeg[0] != 'C' && channelSeg[0] != 'D') {
		return "", "", fmt.Errorf("invalid slack url: channel must start with C or D")
	}

	if len(parts) < archivesIdx+3 || parts[archivesIdx+2] == "" {
		return "", "", fmt.Errorf("invalid slack url: missing timestamp segment")
	}

	tsSeg := parts[archivesIdx+2]
	// Must start with 'p' and have at least 7 chars after stripping 'p' (1 sec digit + 6 frac digits).
	if len(tsSeg) <= 7 {
		return "", "", fmt.Errorf("invalid slack url: timestamp segment too short")
	}

	// Strip 'p' and insert '.' before the last 6 digits.
	digits := tsSeg[1:]
	ts = digits[:len(digits)-6] + "." + digits[len(digits)-6:]

	return channelSeg, ts, nil
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run TestParseSlackURL -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/override/slack_url.go internal/override/slack_url_test.go && git commit -m "$(cat <<'EOF'
feat(override): add parseSlackURL for channel+ts extraction from URLs

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Shared formatter and helpers

**Files:**
- Create: `internal/override/read_format.go`
- Create: `internal/override/read_format_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/override/read_format_test.go`:

```go
package override

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// errWriter always fails writes — used to test error propagation.
type errWriter struct{}

func (e *errWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func TestFormatMessagesText(t *testing.T) {
	msgs := []readMessage{
		{User: "Peter O'Connor", Time: time.Unix(1712746695, 0), Text: "Hello world"},
		{User: "[bot]", Time: time.Unix(1712746800, 0), Text: "Bot response"},
		{User: "U01UNKNOWN", Time: time.Unix(1712746900, 0), Text: "Unresolved user"},
	}

	var buf bytes.Buffer
	if err := formatMessages(msgs, false, &buf); err != nil {
		t.Fatalf("formatMessages: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), out)
	}

	for _, want := range []string{"Peter O'Connor", "[bot]", "U01UNKNOWN", "Hello world", "Bot response", "Unresolved user"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}

	// Verify format: "Name [YYYY-MM-DD HH:MM]: text\n"
	if !strings.Contains(lines[0], "[") || !strings.Contains(lines[0], "]: Hello world") {
		t.Errorf("unexpected line format: %q", lines[0])
	}
}

func TestFormatMessagesJSON(t *testing.T) {
	msgs := []readMessage{
		{User: "Peter O'Connor", Time: time.Unix(1712746695, 0), Text: "Hello world"},
		{User: "U99", Time: time.Unix(1712746800, 0), Text: "Raw ID user"},
	}

	var buf bytes.Buffer
	if err := formatMessages(msgs, true, &buf); err != nil {
		t.Fatalf("formatMessages JSON: %v", err)
	}

	var items []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First message.
	if items[0]["user"] != "Peter O'Connor" {
		t.Errorf("user = %q, want Peter O'Connor", items[0]["user"])
	}
	if items[0]["text"] != "Hello world" {
		t.Errorf("text = %q, want Hello world", items[0]["text"])
	}
	if _, err := time.Parse(time.RFC3339, items[0]["ts"]); err != nil {
		t.Errorf("ts not RFC3339: %q, err: %v", items[0]["ts"], err)
	}

	// Second message: raw user ID in "user" field.
	if items[1]["user"] != "U99" {
		t.Errorf("user = %q, want U99 (raw ID fallback)", items[1]["user"])
	}
}

func TestFormatMessagesWriteError(t *testing.T) {
	msgs := []readMessage{
		{User: "test", Time: time.Now(), Text: "hello"},
	}
	if err := formatMessages(msgs, false, &errWriter{}); err == nil {
		t.Fatal("expected write error to propagate")
	}
}

func TestResolveUser(t *testing.T) {
	idMap := map[string]string{
		"U01": "Peter O'Connor",
		"U02": "Jane Smith",
	}

	tests := []struct {
		name   string
		userID string
		botID  string
		want   string
	}{
		{name: "resolved user", userID: "U01", want: "Peter O'Connor"},
		{name: "resolved second", userID: "U02", want: "Jane Smith"},
		{name: "unresolved falls back to ID", userID: "U99", want: "U99"},
		{name: "bot by BotID", userID: "U01", botID: "B01", want: "[bot]"},
		{name: "bot by empty userID", userID: "", want: "[bot]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveUser(tt.userID, tt.botID, idMap)
			if got != tt.want {
				t.Errorf("resolveUser(%q, %q) = %q, want %q", tt.userID, tt.botID, got, tt.want)
			}
		})
	}
}

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		ts      string
		wantSec int64
	}{
		{ts: "1775827095.264229", wantSec: 1775827095},
		{ts: "1712746695.000000", wantSec: 1712746695},
		{ts: "1712746695", wantSec: 1712746695}, // no fractional part
	}

	for _, tt := range tests {
		t.Run(tt.ts, func(t *testing.T) {
			got := parseSlackTimestamp(tt.ts)
			if got.Unix() != tt.wantSec {
				t.Errorf("parseSlackTimestamp(%q).Unix() = %d, want %d", tt.ts, got.Unix(), tt.wantSec)
			}
		})
	}
}

func TestResolveChannelTS(t *testing.T) {
	tests := []struct {
		name        string
		urlFlag     string
		channelFlag string
		tsFlag      string
		wantChannel string
		wantTS      string
		wantErrStr  string
	}{
		{
			name:        "via url",
			urlFlag:     "https://x.slack.com/archives/C0AFM69/p1234567890123456",
			wantChannel: "C0AFM69",
			wantTS:      "1234567890.123456",
		},
		{
			name:        "via channel and ts flags",
			channelFlag: "C0AFM69",
			tsFlag:      "1234567890.123456",
			wantChannel: "C0AFM69",
			wantTS:      "1234567890.123456",
		},
		{
			name:       "neither url nor flags",
			wantErrStr: "provide either --url or both --channel and --ts",
		},
		{
			name:    "bad url",
			urlFlag: "https://x.slack.com/nope",
			wantErrStr: "invalid slack url: missing /archives/ path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, ts, err := resolveChannelTSFromValues(tt.urlFlag, tt.channelFlag, tt.tsFlag)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErrStr)
				}
				if err.Error() != tt.wantErrStr {
					t.Errorf("error = %q, want %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run "TestFormatMessages|TestResolveUser|TestParseSlackTimestamp|TestResolveChannelTS" -v
```

Expected: FAIL — types and functions undefined.

- [ ] **Step 3: Implement read_format.go**

Create `internal/override/read_format.go`:

```go
package override

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// readMessage is the normalised representation of a single Slack message
// used by both thread-read and message-read formatters.
type readMessage struct {
	User string    // display name, "[bot]", or raw Slack user ID
	Time time.Time // from time.Unix (UTC); formatter applies local timezone
	Text string
}

// readMessageJSON is the JSON wire format for --json output.
type readMessageJSON struct {
	User string `json:"user"`
	Ts   string `json:"ts"`
	Text string `json:"text"`
}

// formatMessages writes msgs to w. When asJSON is false the output is
// "Name [YYYY-MM-DD HH:MM]: text\n" per message in local time.
// When asJSON is true the output is a JSON array of objects with
// "user", "ts" (RFC3339), and "text" fields.
// All write errors are propagated to the caller.
func formatMessages(msgs []readMessage, asJSON bool, w io.Writer) error {
	if asJSON {
		return formatMessagesJSON(msgs, w)
	}
	return formatMessagesText(msgs, w)
}

func formatMessagesText(msgs []readMessage, w io.Writer) error {
	for _, m := range msgs {
		localTime := m.Time.Local().Format("2006-01-02 15:04")
		if _, err := fmt.Fprintf(w, "%s [%s]: %s\n", m.User, localTime, m.Text); err != nil {
			return err
		}
	}
	return nil
}

func formatMessagesJSON(msgs []readMessage, w io.Writer) error {
	out := make([]readMessageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = readMessageJSON{
			User: m.User,
			Ts:   m.Time.Format(time.RFC3339),
			Text: m.Text,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// resolveUser returns the display name for a Slack message sender.
// Bot detection: if botID is non-empty or userID is empty, returns "[bot]".
// If userID is in idMap, returns the mapped name; otherwise returns userID.
func resolveUser(userID, botID string, idMap map[string]string) string {
	if botID != "" || userID == "" {
		return "[bot]"
	}
	if name, ok := idMap[userID]; ok {
		return name
	}
	return userID
}

// parseSlackTimestamp converts a Slack float timestamp string (e.g.
// "1775827095.264229") to time.Time. Returns the zero value for empty input.
func parseSlackTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	parts := strings.SplitN(ts, ".", 2)
	sec, _ := strconv.ParseInt(parts[0], 10, 64)
	var nsec int64
	if len(parts) == 2 {
		frac := parts[1]
		// Pad to 9 digits (nanoseconds).
		for len(frac) < 9 {
			frac += "0"
		}
		nsec, _ = strconv.ParseInt(frac[:9], 10, 64)
	}
	return time.Unix(sec, nsec)
}

// resolveChannelTSFromValues extracts channel and ts from pre-read flag values.
// If rawURL is non-empty, it is parsed via parseSlackURL.
// Otherwise, channelFlag and tsFlag must both be non-empty.
func resolveChannelTSFromValues(rawURL, channelFlag, tsFlag string) (channel, ts string, err error) {
	if rawURL != "" {
		return parseSlackURL(rawURL)
	}
	if channelFlag == "" || tsFlag == "" {
		return "", "", fmt.Errorf("provide either --url or both --channel and --ts")
	}
	return channelFlag, tsFlag, nil
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run "TestFormatMessages|TestResolveUser|TestParseSlackTimestamp|TestResolveChannelTS" -v
```

Expected: PASS.

- [ ] **Step 5: Run full override suite**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/override/read_format.go internal/override/read_format_test.go && git commit -m "$(cat <<'EOF'
feat(override): add shared readMessage formatter and helpers

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: thread-read command

**Files:**
- Create: `internal/override/thread_read_cmd.go`
- Create: `internal/override/thread_read_cmd_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/override/thread_read_cmd_test.go`:

```go
package override

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeThreadRead runs "thread-read" with the given args using a nil client
// and returns stdout, stderr, and the error.
func executeThreadRead(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newThreadReadCmd(nil))

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"thread-read"}, args...))

	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestThreadReadNilClientReturnsAuthError(t *testing.T) {
	_, stderr, err := executeThreadRead(t, "--url", "https://x.slack.com/archives/C0AFM69EB1B/p1775827095264229")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if !strings.Contains(stderr, "SLACK_TOKEN") {
		t.Errorf("stderr missing SLACK_TOKEN: %s", stderr)
	}
}

func TestThreadReadBadURLReturnsInputError(t *testing.T) {
	// A nil client check happens first, so we can't reach URL parsing with nil.
	// Test URL parsing independently via parseSlackURL (white-box).
	_, err := parseSlackURL("https://x.slack.com/nope")
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
	if !strings.Contains(err.Error(), "invalid slack url") {
		t.Errorf("error = %q, want 'invalid slack url'", err.Error())
	}
}

func TestThreadReadFlagContract(t *testing.T) {
	// --channel without --ts should fail at Cobra flag validation.
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newThreadReadCmd(nil))
	root.SetArgs([]string{"thread-read", "--channel", "C01"})

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error: --channel without --ts")
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run TestThreadRead -v
```

Expected: FAIL — `newThreadReadCmd` undefined.

- [ ] **Step 3: Implement thread-read command**

Create `internal/override/thread_read_cmd.go`:

```go
package override

import (
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newThreadReadCmd builds the "thread-read" command that fetches a full Slack
// thread and outputs it as name-resolved plain text or JSON.
func newThreadReadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread-read",
		Short: "Read a Slack thread as name-resolved plain text or JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThreadRead(cmd, client)
		},
	}

	cmd.Flags().String("url", "", "Slack thread URL (e.g. https://…/archives/CXXX/pYYY)")
	cmd.Flags().String("channel", "", "Channel ID (alternative to --url)")
	cmd.Flags().String("ts", "", "Thread timestamp, e.g. 1775827095.264229 (alternative to --url)")
	cmd.Flags().Bool("json", false, "Output as JSON array")

	cmd.MarkFlagsMutuallyExclusive("url", "channel")
	cmd.MarkFlagsMutuallyExclusive("url", "ts")
	cmd.MarkFlagsRequiredTogether("channel", "ts")

	return cmd
}

func runThreadRead(cmd *cobra.Command, client *slack.Client) error {
	if client == nil {
		return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
	}

	rawURL, _ := cmd.Flags().GetString("url")
	channelFlag, _ := cmd.Flags().GetString("channel")
	tsFlag, _ := cmd.Flags().GetString("ts")
	asJSON, _ := cmd.Flags().GetBool("json")

	channel, ts, err := resolveChannelTSFromValues(rawURL, channelFlag, tsFlag)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	if err := ensureCacheReady(cmd, client); err != nil {
		return err
	}

	// Load the full id→name map once; fall back to raw IDs on error.
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	params := &slack.GetConversationRepliesParameters{
		ChannelID: channel,
		Timestamp: ts,
	}
	msgs, _, _, err := client.GetConversationRepliesContext(cmd.Context(), params)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	readMsgs := make([]readMessage, 0, len(msgs))
	for _, msg := range msgs {
		readMsgs = append(readMsgs, readMessage{
			User: resolveUser(msg.User, msg.BotID, idMap),
			Time: parseSlackTimestamp(msg.Timestamp),
			Text: msg.Text,
		})
	}

	if err := formatMessages(readMsgs, asJSON, cmd.OutOrStdout()); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run TestThreadRead -v
```

Expected: PASS.

- [ ] **Step 5: Run full override suite**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/override/thread_read_cmd.go internal/override/thread_read_cmd_test.go && git commit -m "$(cat <<'EOF'
feat(override): add thread-read command

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: message-read command

**Files:**
- Create: `internal/override/message_read_cmd.go`
- Create: `internal/override/message_read_cmd_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/override/message_read_cmd_test.go`:

```go
package override

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeMessageRead runs "message-read" with the given args using a nil client.
func executeMessageRead(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newMessageReadCmd(nil))

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"message-read"}, args...))

	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestMessageReadNilClientReturnsAuthError(t *testing.T) {
	_, stderr, err := executeMessageRead(t, "--url", "https://x.slack.com/archives/C0AFM69EB1B/p1775827095264229")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if !strings.Contains(stderr, "SLACK_TOKEN") {
		t.Errorf("stderr missing SLACK_TOKEN: %s", stderr)
	}
}

func TestMessageReadFlagContract(t *testing.T) {
	// --ts without --channel should fail at Cobra flag validation.
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newMessageReadCmd(nil))
	root.SetArgs([]string{"message-read", "--ts", "1775827095.264229"})

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error: --ts without --channel")
	}
}

func TestMessageReadMutualExclusivity(t *testing.T) {
	// --url with --channel should fail at Cobra flag validation.
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newMessageReadCmd(nil))
	root.SetArgs([]string{"message-read", "--url", "https://x.slack.com/archives/C01/p1234567890123456", "--channel", "C01"})

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error: --url and --channel are mutually exclusive")
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run TestMessageRead -v
```

Expected: FAIL — `newMessageReadCmd` undefined.

- [ ] **Step 3: Implement message-read command**

Create `internal/override/message_read_cmd.go`:

```go
package override

import (
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newMessageReadCmd builds the "message-read" command that fetches a single
// Slack message and outputs it as name-resolved plain text or JSON.
func newMessageReadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message-read",
		Short: "Read a single Slack message as name-resolved plain text or JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessageRead(cmd, client)
		},
	}

	cmd.Flags().String("url", "", "Slack message URL (e.g. https://…/archives/CXXX/pYYY)")
	cmd.Flags().String("channel", "", "Channel ID (alternative to --url)")
	cmd.Flags().String("ts", "", "Message timestamp, e.g. 1776101206.614149 (alternative to --url)")
	cmd.Flags().Bool("json", false, "Output as JSON array")

	cmd.MarkFlagsMutuallyExclusive("url", "channel")
	cmd.MarkFlagsMutuallyExclusive("url", "ts")
	cmd.MarkFlagsRequiredTogether("channel", "ts")

	return cmd
}

func runMessageRead(cmd *cobra.Command, client *slack.Client) error {
	if client == nil {
		return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
	}

	rawURL, _ := cmd.Flags().GetString("url")
	channelFlag, _ := cmd.Flags().GetString("channel")
	tsFlag, _ := cmd.Flags().GetString("ts")
	asJSON, _ := cmd.Flags().GetBool("json")

	channel, ts, err := resolveChannelTSFromValues(rawURL, channelFlag, tsFlag)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	if err := ensureCacheReady(cmd, client); err != nil {
		return err
	}

	// Load the full id→name map once; fall back to raw IDs on error.
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	params := &slack.GetConversationHistoryParameters{
		ChannelID: channel,
		Latest:    ts,
		Oldest:    ts,
		Inclusive: true,
		Limit:     1,
	}
	resp, err := client.GetConversationHistoryContext(cmd.Context(), params)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	if len(resp.Messages) == 0 {
		return formatAndExit(cmd,
			fmt.Errorf("no message found in %s at %s", channel, ts),
			exitcode.InputError)
	}

	msg := resp.Messages[0]
	readMsgs := []readMessage{
		{
			User: resolveUser(msg.User, msg.BotID, idMap),
			Time: parseSlackTimestamp(msg.Timestamp),
			Text: msg.Text,
		},
	}

	if err := formatMessages(readMsgs, asJSON, cmd.OutOrStdout()); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -run TestMessageRead -v
```

Expected: PASS.

- [ ] **Step 5: Run full override suite**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test ./internal/override/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/override/message_read_cmd.go internal/override/message_read_cmd_test.go && git commit -m "$(cat <<'EOF'
feat(override): add message-read command

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Register commands + full build verification

**Files:**
- Modify: `internal/override/api_list.go`

- [ ] **Step 1: Register thread-read and message-read in RegisterBuiltins**

In `internal/override/api_list.go`, update `RegisterBuiltins`:

```go
func RegisterBuiltins(root *cobra.Command, client *slack.Client) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "Discover available Slack API methods",
	}

	apiCmd.AddCommand(newListCmd())
	root.AddCommand(apiCmd)

	root.AddCommand(newCacheCmd(client))
	root.AddCommand(newResolveCmd(client))
	root.AddCommand(newThreadReadCmd(client))
	root.AddCommand(newMessageReadCmd(client))
}
```

- [ ] **Step 2: Build the full project**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run the full test suite**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go test -race -count=1 ./...
```

Expected: all PASS.

- [ ] **Step 4: Smoke test — verify commands appear in help**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go run ./cmd/slack-cli --help
```

Expected output includes `thread-read` and `message-read` in the available commands list.

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go run ./cmd/slack-cli thread-read --help
```

Expected: shows `--url`, `--channel`, `--ts`, `--json` flags.

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && go run ./cmd/slack-cli message-read --help
```

Expected: shows `--url`, `--channel`, `--ts`, `--json` flags.

- [ ] **Step 5: Commit**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI && git add internal/override/api_list.go && git commit -m "$(cat <<'EOF'
feat: register thread-read and message-read commands

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Covered by |
|-----------------|------------|
| `id-to-name.json` constant + `Clear()` update | Task 1 |
| `LoadIDToNameMap()` | Task 2 |
| `ResolveUserByID()` | Task 2 |
| `buildIDToNameMap` + `Warm` writes `id-to-name.json` | Task 3 |
| `EnrichOnly` writes `id-to-name.json` | Task 3 |
| `migrateV2toV3` | Task 4 |
| `EnsureReady` explicit `case 2:` | Task 4 |
| `CurrentVersion = 3` | Task 4 |
| `cache info` ID mappings line | Task 5 |
| `parseSlackURL` | Task 6 |
| `readMessage` struct | Task 7 |
| `formatMessages` | Task 7 |
| `resolveUser` + `parseSlackTimestamp` + `resolveChannelTSFromValues` | Task 7 |
| `thread-read` command with all flag wiring | Task 8 |
| `message-read` command with all flag wiring | Task 9 |
| Register in `RegisterBuiltins` | Task 10 |
| `time.RFC3339` for JSON ts | Task 7 (formatMessagesJSON) |
| Bot detection (`BotID != "" \|\| User == ""`) | Task 7 (resolveUser) |
| `idMap` loaded once per invocation | Tasks 8 + 9 |
| Write-order: `id-to-name.json` before meta bump | Task 4 (migrateV2toV3) |
| `MarkFlagsMutuallyExclusive` + `MarkFlagsRequiredTogether` | Tasks 8 + 9 |

No gaps found.
