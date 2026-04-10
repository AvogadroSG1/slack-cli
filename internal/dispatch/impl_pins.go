package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implPinsAdd implements pins.add.
func implPinsAdd(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ref := slack.ItemRef{
		Channel:   flagStr(flags, "channel"),
		Timestamp: flagStr(flags, "timestamp"),
	}
	err := client.AddPinContext(ctx, flagStr(flags, "channel"), ref)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implPinsRemove implements pins.remove.
func implPinsRemove(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ref := slack.ItemRef{
		Channel:   flagStr(flags, "channel"),
		Timestamp: flagStr(flags, "timestamp"),
	}
	err := client.RemovePinContext(ctx, flagStr(flags, "channel"), ref)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implPinsList implements pins.list.
func implPinsList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	items, paging, err := client.ListPinsContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items":  items,
		"paging": paging,
	}, nil
}

func init() {
	RegisterDispatch("pins.add", implPinsAdd)
	RegisterDispatch("pins.remove", implPinsRemove)
	RegisterDispatch("pins.list", implPinsList)
}
