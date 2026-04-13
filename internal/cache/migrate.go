package cache

import (
	"context"
	"fmt"
	"os"
)

// EnsureReady checks the cache state and performs any necessary migration
// or enrichment. It should be called before any resolve operation.
//
// States:
//   - No meta + no legacy → fresh install, needs full warm (returns needsWarm=true)
//   - No meta + legacy exists → v1 migration: split files, then enrich
//   - Meta version 1 → enrichment pass only (channels already split)
//   - Meta version 2 → current, check staleness
func EnsureReady(ctx context.Context, fetcher SlackFetcher) (needsWarm bool, err error) {
	version, err := MetaVersion()
	if err != nil {
		return false, fmt.Errorf("check version: %w", err)
	}

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
		// Current version. Check staleness.
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
}

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

// enrichAndReturn runs the enrichment pass. If enrichment fails (e.g.,
// network error), it returns needsWarm=false so the caller can still
// resolve from the flat data. The enrichment will retry on next access.
func enrichAndReturn(ctx context.Context, fetcher SlackFetcher) (bool, error) {
	if fetcher == nil {
		// No client available for enrichment — resolve from flat data.
		return false, nil
	}
	if err := EnrichOnly(ctx, fetcher); err != nil {
		// Enrichment failed. Don't block resolve — flat data still works
		// for ID lookups. Enrichment retries on next access.
		return false, nil
	}
	return false, nil
}

// migrateV1 reads the legacy cache.json, splits it into three files,
// and writes version 1 metadata. The legacy file is deleted on success.
func migrateV1() error {
	lock, err := AcquireExclusive()
	if err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer lock.Close()

	legacy, err := LoadEntity[LegacyData](LegacyFileName)
	if err != nil {
		return fmt.Errorf("read legacy: %w", err)
	}

	// Write channels as-is (flat name→ID).
	channels := ChannelCache(legacy.Channels)
	if err := SaveEntity(ChannelsFileName, channels); err != nil {
		return err
	}

	// Convert flat users to UserEntry (ID only, enrichment happens next).
	people := make(PeopleCache, len(legacy.Users))
	for name, id := range legacy.Users {
		people[name] = UserEntry{ID: id}
	}
	if err := SaveEntity(PeopleFileName, people); err != nil {
		return err
	}

	// Convert flat usergroups to Usergroup (ID only).
	groups := make(UsergroupCache, len(legacy.Usergroups))
	for handle, id := range legacy.Usergroups {
		groups[handle] = UsergroupEntry{ID: id}
	}
	if err := SaveEntity(UsergroupsFileName, groups); err != nil {
		return err
	}

	// Write version 1 meta (split but not enriched).
	if err := SaveMeta(CacheMeta{Version: 1}); err != nil {
		return err
	}

	// Delete legacy file.
	p, err := FilePath(LegacyFileName)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy: %w", err)
	}

	return nil
}
