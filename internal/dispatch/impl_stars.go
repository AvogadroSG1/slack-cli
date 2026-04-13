package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implStarsAdd implements stars.add.
func implStarsAdd(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ref := slack.ItemRef{
		Channel:   flagStr(flags, "channel"),
		Timestamp: flagStr(flags, "timestamp"),
		File:      flagStr(flags, "file"),
	}
	err := client.AddStarContext(ctx, flagStr(flags, "channel"), ref)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implStarsRemove implements stars.remove.
func implStarsRemove(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ref := slack.ItemRef{
		Channel:   flagStr(flags, "channel"),
		Timestamp: flagStr(flags, "timestamp"),
		File:      flagStr(flags, "file"),
	}
	err := client.RemoveStarContext(ctx, flagStr(flags, "channel"), ref)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implStarsList implements stars.list.
func implStarsList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.StarsParameters{
		User:   flagStr(flags, "user"),
		Cursor: flagStr(flags, "cursor"),
		Limit:  flagInt(flags, "limit"),
	}
	items, nextCursor, err := client.ListStarsContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	}, nil
}

func init() {
	RegisterDispatch("stars.add", implStarsAdd)
	RegisterDispatch("stars.remove", implStarsRemove)
	RegisterDispatch("stars.list", implStarsList)
}
