package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// itemRefFromFlags builds a slack.ItemRef from the standard flag set.
func itemRefFromFlags(flags map[string]any) slack.ItemRef {
	return slack.ItemRef{
		Channel:   flagStr(flags, "channel"),
		Timestamp: flagStr(flags, "timestamp"),
		File:      flagStr(flags, "file"),
		Comment:   flagStr(flags, "comment"),
	}
}

// implReactionsAdd implements reactions.add.
func implReactionsAdd(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.AddReactionContext(ctx, flagStr(flags, "name"), itemRefFromFlags(flags))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implReactionsRemove implements reactions.remove.
func implReactionsRemove(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.RemoveReactionContext(ctx, flagStr(flags, "name"), itemRefFromFlags(flags))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implReactionsGet implements reactions.get.
func implReactionsGet(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.GetReactionsParameters{
		Full: flagBool(flags, "full"),
	}
	item, err := client.GetReactionsContext(ctx, itemRefFromFlags(flags), params)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// implReactionsList implements reactions.list.
func implReactionsList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.ListReactionsParameters{
		User:   flagStr(flags, "user"),
		Cursor: flagStr(flags, "cursor"),
		Limit:  flagInt(flags, "limit"),
		Full:   flagBool(flags, "full"),
	}
	items, nextCursor, err := client.ListReactionsContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	}, nil
}

func init() {
	RegisterDispatch("reactions.add", implReactionsAdd)
	RegisterDispatch("reactions.remove", implReactionsRemove)
	RegisterDispatch("reactions.get", implReactionsGet)
	RegisterDispatch("reactions.list", implReactionsList)
}
