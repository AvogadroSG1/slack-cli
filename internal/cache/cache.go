// Package cache provides a file-based name-to-ID mapping for Slack channels,
// users, and usergroups. The cache is split across three entity-specific JSON
// files protected by a single file lock for concurrent access safety.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// DefaultDir is the default directory for the cache files when
// SLACK_CLI_CACHE_DIR is not set.
const DefaultDir = ".slack-cli"

// File names for the cache.
const (
	MetaFileName       = "cache-meta.json"
	ChannelsFileName   = "channels.json"
	PeopleFileName     = "people.json"
	UsergroupsFileName = "usergroups.json"
	IDToNameFileName   = "id-to-name.json"
	LegacyFileName     = "cache.json" // v1 single-file format
	// LockFileName protects ALL cache data files atomically. Any operation
	// that reads or writes channels.json, people.json, usergroups.json, or
	// cache-meta.json must hold this lock.
	LockFileName = "cache.lock"
)

// StaleDuration is how long before the cache is considered stale and
// triggers an automatic warm on the next resolve.
const StaleDuration = 24 * time.Hour

// Current cache format version.
const CurrentVersion = 3

// CacheMeta tracks the cache format version. Staleness is determined by
// this file's mtime, not the data files.
type CacheMeta struct {
	Version int `json:"version"`
}

// ChannelCache is a flat name-to-ID mapping for Slack channels.
type ChannelCache map[string]string

// UserEntry holds enriched profile data for a Slack user.
type UserEntry struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Title       string `json:"title"`
}

// PeopleCache maps Slack username to enriched user data.
type PeopleCache map[string]UserEntry

// UsergroupEntry holds enriched data for a Slack user group.
type UsergroupEntry struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// MemberCount returns the number of members in the usergroup.
func (ug UsergroupEntry) MemberCount() int {
	return len(ug.Members)
}

// UsergroupCache maps usergroup handle to enriched usergroup data.
type UsergroupCache map[string]UsergroupEntry

// LegacyData represents the v1 single-file cache format.
type LegacyData struct {
	Channels   map[string]string `json:"channels"`
	Users      map[string]string `json:"users"`
	Usergroups map[string]string `json:"usergroups"`
}

// Dir returns the cache directory path. It checks SLACK_CLI_CACHE_DIR
// first, falling back to ~/.slack-cli.
func Dir() (string, error) {
	if dir := os.Getenv("SLACK_CLI_CACHE_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cache dir: %w", err)
	}
	return filepath.Join(home, DefaultDir), nil
}

// FilePath returns the full path to a cache file by name.
func FilePath(filename string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// EnsureDir creates the cache directory if it does not exist.
func EnsureDir() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// LoadEntity reads and deserializes a JSON cache file into the given type.
func LoadEntity[T any](filename string) (T, error) {
	var zero T
	p, err := FilePath(filename)
	if err != nil {
		return zero, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return zero, fmt.Errorf("read %s: %w", filename, err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return zero, fmt.Errorf("parse %s: %w", filename, err)
	}
	return v, nil
}

// SaveEntity serializes data and writes it to the named cache file
// atomically (write to temp, rename).
func SaveEntity[T any](filename string, data T) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	p, err := FilePath(filename)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filename, err)
	}
	raw = append(raw, '\n')

	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write temp %s: %w", filename, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", filename, err)
	}
	return nil
}

// LoadMeta reads the cache metadata file.
func LoadMeta() (CacheMeta, error) {
	return LoadEntity[CacheMeta](MetaFileName)
}

// SaveMeta writes the cache metadata file.
func SaveMeta(m CacheMeta) error {
	return SaveEntity(MetaFileName, m)
}

// IsStale returns true if the cache needs warming. It checks the mtime
// of cache-meta.json; if the file is missing or older than StaleDuration,
// the cache is stale.
func IsStale() (bool, error) {
	p, err := FilePath(MetaFileName)
	if err != nil {
		return true, err
	}
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("stat meta: %w", err)
	}
	return time.Since(info.ModTime()) > StaleDuration, nil
}

// HasLegacyCache returns true if a v1 cache.json file exists.
func HasLegacyCache() (bool, error) {
	p, err := FilePath(LegacyFileName)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// MetaVersion returns the current cache version, or 0 if no metadata
// file exists.
func MetaVersion() (int, error) {
	m, err := LoadMeta()
	if err != nil {
		// LoadEntity wraps the error, so check with errors.Is.
		if pathErr, ok := unwrapPathError(err); ok && os.IsNotExist(pathErr) {
			return 0, nil
		}
		return 0, err
	}
	return m.Version, nil
}

// unwrapPathError digs through wrapped errors to find an *os.PathError.
func unwrapPathError(err error) (error, bool) {
	for err != nil {
		if _, ok := err.(*os.PathError); ok {
			return err, true
		}
		err = errors.Unwrap(err)
	}
	return nil, false
}

// Clear deletes all cache files and the lock file.
func Clear() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	files := []string{
		MetaFileName, ChannelsFileName, PeopleFileName,
		UsergroupsFileName, IDToNameFileName, LockFileName, LegacyFileName,
	}
	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", f, err)
		}
	}
	return nil
}

// lockTimeout is how long to wait for a file lock before giving up.
const lockTimeout = 10 * time.Second

// AcquireShared opens the lock file and acquires a shared (read) lock.
// The caller must close the returned file to release the lock.
func AcquireShared() (*os.File, error) {
	return acquireLock(syscall.LOCK_SH)
}

// AcquireExclusive opens the lock file and acquires an exclusive (write)
// lock. The caller must close the returned file to release the lock.
func AcquireExclusive() (*os.File, error) {
	return acquireLock(syscall.LOCK_EX)
}

func acquireLock(how int) (*os.File, error) {
	if err := EnsureDir(); err != nil {
		return nil, err
	}
	lp, err := FilePath(LockFileName)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lp, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// Try non-blocking first, then poll with a timeout.
	err = syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
	if err == nil {
		return f, nil
	}

	deadline := time.Now().Add(lockTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		err = syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("lock timeout after %s", lockTimeout)
		}
	}

	_ = f.Close()
	return nil, fmt.Errorf("lock timeout after %s", lockTimeout)
}
