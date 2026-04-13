package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implBookmarksAdd implements bookmarks.add.
func implBookmarksAdd(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.AddBookmarkParameters{
		Title: flagStr(flags, "title"),
		Type:  flagStr(flags, "type"),
		Link:  flagStr(flags, "link"),
		Emoji: flagStr(flags, "emoji"),
	}
	bm, err := client.AddBookmarkContext(ctx, flagStr(flags, "channel"), params)
	if err != nil {
		return nil, err
	}
	return bm, nil
}

// implBookmarksEdit implements bookmarks.edit.
func implBookmarksEdit(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.EditBookmarkParameters{
		Link: flagStr(flags, "link"),
	}
	if v, ok := flags["title"].(string); ok {
		params.Title = &v
	}
	if v, ok := flags["emoji"].(string); ok {
		params.Emoji = &v
	}
	bm, err := client.EditBookmarkContext(ctx, flagStr(flags, "channel"), flagStr(flags, "bookmark-id"), params)
	if err != nil {
		return nil, err
	}
	return bm, nil
}

// implBookmarksList implements bookmarks.list.
func implBookmarksList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	bms, err := client.ListBookmarksContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return bms, nil
}

// implBookmarksRemove implements bookmarks.remove.
func implBookmarksRemove(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.RemoveBookmarkContext(ctx, flagStr(flags, "channel"), flagStr(flags, "bookmark-id"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func init() {
	RegisterDispatch("bookmarks.add", implBookmarksAdd)
	RegisterDispatch("bookmarks.edit", implBookmarksEdit)
	RegisterDispatch("bookmarks.list", implBookmarksList)
	RegisterDispatch("bookmarks.remove", implBookmarksRemove)
}
