package dispatch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slack-go/slack"
)

// implViewsOpen implements views.open.
func implViewsOpen(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	var view slack.ModalViewRequest
	if err := json.Unmarshal([]byte(flagStr(flags, "view")), &view); err != nil {
		return nil, fmt.Errorf("invalid view JSON: %w", err)
	}
	resp, err := client.OpenViewContext(ctx, flagStr(flags, "trigger-id"), view)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// implViewsPush implements views.push.
func implViewsPush(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	var view slack.ModalViewRequest
	if err := json.Unmarshal([]byte(flagStr(flags, "view")), &view); err != nil {
		return nil, fmt.Errorf("invalid view JSON: %w", err)
	}
	resp, err := client.PushViewContext(ctx, flagStr(flags, "trigger-id"), view)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// implViewsUpdate implements views.update.
func implViewsUpdate(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	var view slack.ModalViewRequest
	if err := json.Unmarshal([]byte(flagStr(flags, "view")), &view); err != nil {
		return nil, fmt.Errorf("invalid view JSON: %w", err)
	}
	resp, err := client.UpdateViewContext(ctx, view, flagStr(flags, "external-id"), flagStr(flags, "hash"), flagStr(flags, "view-id"))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// implViewsPublish implements views.publish.
func implViewsPublish(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	var view slack.HomeTabViewRequest
	if err := json.Unmarshal([]byte(flagStr(flags, "view")), &view); err != nil {
		return nil, fmt.Errorf("invalid view JSON: %w", err)
	}
	req := slack.PublishViewContextRequest{
		UserID: flagStr(flags, "user-id"),
		View:   view,
	}
	resp, err := client.PublishViewContext(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func init() {
	RegisterDispatch("views.open", implViewsOpen)
	RegisterDispatch("views.push", implViewsPush)
	RegisterDispatch("views.update", implViewsUpdate)
	RegisterDispatch("views.publish", implViewsPublish)
}
