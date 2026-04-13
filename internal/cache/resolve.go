package cache

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EntityType identifies which cached entity to query.
type EntityType string

const (
	Channel   EntityType = "channel"
	User      EntityType = "user"
	Usergroup EntityType = "usergroup"
)

// ValidPeopleFields lists the valid --field values for people.
var ValidPeopleFields = []string{"id", "email", "display_name", "title", "all"}

// ValidUsergroupFields lists the valid --field values for usergroups.
var ValidUsergroupFields = []string{"id", "description", "members", "all"}

// ValidChannelFields lists the valid --field values for channels.
var ValidChannelFields = []string{"id", "all"}

// ResolveChannel looks up a channel name and returns its ID.
func ResolveChannel(name string) (string, error) {
	channels, err := LoadEntity[ChannelCache](ChannelsFileName)
	if err != nil {
		return "", err
	}
	id, ok := channels[name]
	if !ok {
		return "", fmt.Errorf("no channel named %q", name)
	}
	return id, nil
}

// ResolveUser looks up a user name and returns the requested field value.
// An empty field returns the ID (default behavior).
func ResolveUser(name, field string) (string, error) {
	people, err := LoadEntity[PeopleCache](PeopleFileName)
	if err != nil {
		return "", err
	}
	entry, ok := people[name]
	if !ok {
		return "", fmt.Errorf("no user named %q", name)
	}
	return userField(entry, field)
}

// ResolveUsergroup looks up a usergroup handle and returns the requested
// field value. An empty field returns the ID (default behavior).
func ResolveUsergroup(handle, field string) (string, error) {
	groups, err := LoadEntity[UsergroupCache](UsergroupsFileName)
	if err != nil {
		return "", err
	}
	entry, ok := groups[handle]
	if !ok {
		return "", fmt.Errorf("no usergroup named %q", handle)
	}
	return usergroupField(entry, field)
}

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

func userField(e UserEntry, field string) (string, error) {
	switch field {
	case "", "id":
		return e.ID, nil
	case "email":
		return e.Email, nil
	case "display_name":
		return e.DisplayName, nil
	case "title":
		return e.Title, nil
	case "all":
		return marshalJSON(e)
	default:
		return "", fmt.Errorf("unknown field %q for user (valid: %s)", field, strings.Join(ValidPeopleFields, ", "))
	}
}

func usergroupField(ug UsergroupEntry, field string) (string, error) {
	switch field {
	case "", "id":
		return ug.ID, nil
	case "description":
		return ug.Description, nil
	case "members":
		return marshalJSON(ug.Members)
	case "all":
		return marshalJSON(ug)
	default:
		return "", fmt.Errorf("unknown field %q for usergroup (valid: %s)", field, strings.Join(ValidUsergroupFields, ", "))
	}
}

func marshalJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
