package registry

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGroupByCategory(t *testing.T) {
	tests := []struct {
		name string
		defs []MethodDef
		want map[string][]MethodDef
	}{
		{
			name: "three methods across two categories",
			defs: []MethodDef{
				{APIMethod: "chat.postMessage", Category: "chat", Command: "post-message"},
				{APIMethod: "chat.update", Category: "chat", Command: "update"},
				{APIMethod: "users.list", Category: "users", Command: "list"},
			},
			want: map[string][]MethodDef{
				"chat": {
					{APIMethod: "chat.postMessage", Category: "chat", Command: "post-message"},
					{APIMethod: "chat.update", Category: "chat", Command: "update"},
				},
				"users": {
					{APIMethod: "users.list", Category: "users", Command: "list"},
				},
			},
		},
		{
			name: "empty input returns empty map",
			defs: []MethodDef{},
			want: map[string][]MethodDef{},
		},
		{
			name: "single category groups into one entry",
			defs: []MethodDef{
				{APIMethod: "conversations.list", Category: "conversations", Command: "list"},
				{APIMethod: "conversations.history", Category: "conversations", Command: "history"},
			},
			want: map[string][]MethodDef{
				"conversations": {
					{APIMethod: "conversations.list", Category: "conversations", Command: "list"},
					{APIMethod: "conversations.history", Category: "conversations", Command: "history"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GroupByCategory(tt.defs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GroupByCategory() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
