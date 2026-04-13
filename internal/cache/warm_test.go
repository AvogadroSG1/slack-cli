package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

// mockFetcher implements SlackFetcher for testing.
type mockFetcher struct {
	channels   []slack.Channel
	users      []slack.User
	usergroups []slack.UserGroup

	channelPages [][]slack.Channel
	callCount    int

	channelsErr   error
	usersErr      error
	usergroupsErr error
}

func (m *mockFetcher) GetConversationsContext(_ context.Context, params *slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	if m.channelsErr != nil {
		return nil, "", m.channelsErr
	}
	if len(m.channelPages) > 0 {
		page := m.callCount
		m.callCount++
		if page >= len(m.channelPages) {
			return nil, "", nil
		}
		nextCursor := ""
		if page < len(m.channelPages)-1 {
			nextCursor = fmt.Sprintf("cursor_%d", page+1)
		}
		return m.channelPages[page], nextCursor, nil
	}
	return m.channels, "", nil
}

func (m *mockFetcher) GetUsersContext(_ context.Context, _ ...slack.GetUsersOption) ([]slack.User, error) {
	if m.usersErr != nil {
		return nil, m.usersErr
	}
	return m.users, nil
}

func (m *mockFetcher) GetUserGroupsContext(_ context.Context, _ ...slack.GetUserGroupsOption) ([]slack.UserGroup, error) {
	if m.usergroupsErr != nil {
		return nil, m.usergroupsErr
	}
	return m.usergroups, nil
}

func enrichedMockFetcher() *mockFetcher {
	return &mockFetcher{
		channels: []slack.Channel{
			{GroupConversation: slack.GroupConversation{Name: "general", Conversation: slack.Conversation{ID: "C01"}}},
			{GroupConversation: slack.GroupConversation{Name: "random", Conversation: slack.Conversation{ID: "C02"}}},
		},
		users: []slack.User{
			{
				ID:   "U01",
				Name: "poconnor",
				Profile: slack.UserProfile{
					Email:       "poconnor@stackoverflow.com",
					DisplayName: "Peter O'Connor",
					Title:       "Sr. Director",
				},
			},
			{
				ID:   "U02",
				Name: "jsmith",
				Profile: slack.UserProfile{
					Email:       "jsmith@stackoverflow.com",
					DisplayName: "Jane Smith",
					Title:       "Staff Engineer",
				},
			},
			{
				ID:      "U03",
				Name:    "deleted-user",
				Deleted: true,
			},
		},
		usergroups: []slack.UserGroup{
			{
				ID:          "S01",
				Handle:      "platform-team",
				Description: "Platform Engineering",
				Users:       []string{"U01", "U02"},
			},
		},
	}
}

func TestWarm(t *testing.T) {
	withTempCacheDir(t)
	fetcher := enrichedMockFetcher()

	result, err := Warm(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("Warm: %v", err)
	}

	if result.Channels != 2 {
		t.Errorf("Channels = %d, want 2", result.Channels)
	}
	if result.Users != 2 {
		t.Errorf("Users = %d, want 2 (deleted excluded)", result.Users)
	}
	if result.Usergroups != 1 {
		t.Errorf("Usergroups = %d, want 1", result.Usergroups)
	}

	// Verify channels file.
	channels, err := LoadEntity[ChannelCache](ChannelsFileName)
	if err != nil {
		t.Fatalf("LoadChannels: %v", err)
	}
	wantChannels := ChannelCache{"general": "C01", "random": "C02"}
	if diff := cmp.Diff(wantChannels, channels); diff != "" {
		t.Errorf("channels (-want +got):\n%s", diff)
	}

	// Verify people file with enriched data.
	people, err := LoadEntity[PeopleCache](PeopleFileName)
	if err != nil {
		t.Fatalf("LoadPeople: %v", err)
	}
	if people["poconnor"].Email != "poconnor@stackoverflow.com" {
		t.Errorf("email = %q, want poconnor@stackoverflow.com", people["poconnor"].Email)
	}
	if people["poconnor"].DisplayName != "Peter O'Connor" {
		t.Errorf("display_name = %q, want Peter O'Connor", people["poconnor"].DisplayName)
	}
	if people["poconnor"].Title != "Sr. Director" {
		t.Errorf("title = %q, want Sr. Director", people["poconnor"].Title)
	}

	// Verify usergroups file with members.
	groups, err := LoadEntity[UsergroupCache](UsergroupsFileName)
	if err != nil {
		t.Fatalf("LoadUsergroups: %v", err)
	}
	ug := groups["platform-team"]
	if ug.Description != "Platform Engineering" {
		t.Errorf("description = %q", ug.Description)
	}
	wantMembers := []string{"U01", "U02"}
	if diff := cmp.Diff(wantMembers, ug.Members); diff != "" {
		t.Errorf("members (-want +got):\n%s", diff)
	}

	// Verify meta version.
	version, _ := MetaVersion()
	if version != CurrentVersion {
		t.Errorf("version = %d, want %d", version, CurrentVersion)
	}

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
}

func TestWarmPagination(t *testing.T) {
	withTempCacheDir(t)
	fetcher := &mockFetcher{
		channelPages: [][]slack.Channel{
			{
				{GroupConversation: slack.GroupConversation{Name: "p1a", Conversation: slack.Conversation{ID: "C01"}}},
				{GroupConversation: slack.GroupConversation{Name: "p1b", Conversation: slack.Conversation{ID: "C02"}}},
			},
			{
				{GroupConversation: slack.GroupConversation{Name: "p2a", Conversation: slack.Conversation{ID: "C03"}}},
			},
		},
		users:      []slack.User{},
		usergroups: []slack.UserGroup{},
	}

	result, err := Warm(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if result.Channels != 3 {
		t.Errorf("Channels = %d, want 3", result.Channels)
	}
}

func TestWarmChannelError(t *testing.T) {
	withTempCacheDir(t)
	fetcher := &mockFetcher{channelsErr: fmt.Errorf("api error")}
	_, err := Warm(context.Background(), fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWarmUserError(t *testing.T) {
	withTempCacheDir(t)
	fetcher := &mockFetcher{channels: []slack.Channel{}, usersErr: fmt.Errorf("api error")}
	_, err := Warm(context.Background(), fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWarmUsergroupError(t *testing.T) {
	withTempCacheDir(t)
	fetcher := &mockFetcher{channels: []slack.Channel{}, users: []slack.User{}, usergroupsErr: fmt.Errorf("api error")}
	_, err := Warm(context.Background(), fetcher)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWarmSkipsEmptyNames(t *testing.T) {
	withTempCacheDir(t)
	fetcher := &mockFetcher{
		channels: []slack.Channel{
			{GroupConversation: slack.GroupConversation{Name: "general", Conversation: slack.Conversation{ID: "C01"}}},
			{GroupConversation: slack.GroupConversation{Name: "", Conversation: slack.Conversation{ID: "C02"}}},
		},
		users: []slack.User{
			{ID: "U01", Name: "poconnor", Profile: slack.UserProfile{Email: "p@so.com"}},
			{ID: "U02", Name: ""},
		},
		usergroups: []slack.UserGroup{
			{ID: "S01", Handle: "team"},
			{ID: "S02", Handle: ""},
		},
	}

	result, err := Warm(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if result.Channels != 1 {
		t.Errorf("Channels = %d, want 1", result.Channels)
	}
	if result.Users != 1 {
		t.Errorf("Users = %d, want 1", result.Users)
	}
	if result.Usergroups != 1 {
		t.Errorf("Usergroups = %d, want 1", result.Usergroups)
	}
}

func TestEnrichOnly(t *testing.T) {
	withTempCacheDir(t)

	// Pre-populate flat channels (should not be touched).
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

	// Channels untouched.
	channels, _ := LoadEntity[ChannelCache](ChannelsFileName)
	if len(channels) != 1 || channels["general"] != "C01" {
		t.Errorf("channels changed after EnrichOnly: %v", channels)
	}

	// People enriched.
	people, _ := LoadEntity[PeopleCache](PeopleFileName)
	if people["poconnor"].Email != "poconnor@stackoverflow.com" {
		t.Errorf("people not enriched: %v", people["poconnor"])
	}

	// Version bumped.
	version, _ := MetaVersion()
	if version != CurrentVersion {
		t.Errorf("version = %d, want %d", version, CurrentVersion)
	}
}

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
