package cache

import (
	"strings"
	"testing"
)

func TestResolveChannel(t *testing.T) {
	withTempCacheDir(t)
	if err := SaveEntity(ChannelsFileName, testChannels()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "hit", input: "general", want: "C01GENERAL"},
		{name: "hit second", input: "platform-engineering", want: "C02PLATFORM"},
		{name: "miss", input: "nonexistent", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveChannel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveChannel(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveChannel(%q) error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveChannel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveUser(t *testing.T) {
	withTempCacheDir(t)
	if err := SaveEntity(PeopleFileName, testPeople()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		user    string
		field   string
		want    string
		wantErr bool
	}{
		{name: "id default", user: "poconnor", field: "", want: "U01POCONNOR"},
		{name: "id explicit", user: "poconnor", field: "id", want: "U01POCONNOR"},
		{name: "email", user: "poconnor", field: "email", want: "poconnor@stackoverflow.com"},
		{name: "display_name", user: "poconnor", field: "display_name", want: "Peter O'Connor"},
		{name: "title", user: "poconnor", field: "title", want: "Sr. Director"},
		{name: "all is JSON", user: "poconnor", field: "all"},
		{name: "miss", user: "nobody", field: "", wantErr: true},
		{name: "bad field", user: "poconnor", field: "phone", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUser(tt.user, tt.field)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveUser(%q, %q) = %q, want error", tt.user, tt.field, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveUser(%q, %q) error: %v", tt.user, tt.field, err)
				return
			}
			if tt.field == "all" {
				if !strings.Contains(got, "U01POCONNOR") {
					t.Errorf("all output missing ID: %s", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("ResolveUser(%q, %q) = %q, want %q", tt.user, tt.field, got, tt.want)
			}
		})
	}
}

func TestResolveUserBadFieldListsValid(t *testing.T) {
	withTempCacheDir(t)
	if err := SaveEntity(PeopleFileName, testPeople()); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveUser("poconnor", "phone")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, f := range []string{"id", "email", "display_name", "title", "all"} {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("error message missing valid field %q: %s", f, err.Error())
		}
	}
}

func TestResolveUsergroup(t *testing.T) {
	withTempCacheDir(t)
	if err := SaveEntity(UsergroupsFileName, testUsergroups()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		handle  string
		field   string
		want    string
		wantErr bool
	}{
		{name: "id default", handle: "platform-team", field: "", want: "S01PLATTEAM"},
		{name: "id explicit", handle: "platform-team", field: "id", want: "S01PLATTEAM"},
		{name: "description", handle: "platform-team", field: "description", want: "Platform Engineering"},
		{name: "members is JSON", handle: "platform-team", field: "members"},
		{name: "all is JSON", handle: "platform-team", field: "all"},
		{name: "miss", handle: "no-group", field: "", wantErr: true},
		{name: "bad field", handle: "platform-team", field: "email", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUsergroup(tt.handle, tt.field)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveUsergroup(%q, %q) = %q, want error", tt.handle, tt.field, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveUsergroup(%q, %q) error: %v", tt.handle, tt.field, err)
				return
			}
			if tt.field == "members" {
				if !strings.Contains(got, "U01POCONNOR") {
					t.Errorf("members missing U01POCONNOR: %s", got)
				}
				return
			}
			if tt.field == "all" {
				if !strings.Contains(got, "S01PLATTEAM") {
					t.Errorf("all output missing ID: %s", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("ResolveUsergroup(%q, %q) = %q, want %q", tt.handle, tt.field, got, tt.want)
			}
		})
	}
}

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
