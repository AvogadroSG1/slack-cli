package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implDNDEnd implements dnd.endDnd.
func implDNDEnd(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	err := client.EndDNDContext(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implDNDEndSnooze implements dnd.endSnooze.
func implDNDEndSnooze(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	status, err := client.EndSnoozeContext(ctx)
	if err != nil {
		return nil, err
	}
	return status, nil
}

// implDNDInfo implements dnd.info.
func implDNDInfo(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	var userPtr *string
	if v := flagStr(flags, "user"); v != "" {
		userPtr = &v
	}
	status, err := client.GetDNDInfoContext(ctx, userPtr)
	if err != nil {
		return nil, err
	}
	return status, nil
}

// implDNDSetSnooze implements dnd.setSnooze.
func implDNDSetSnooze(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	status, err := client.SetSnoozeContext(ctx, flagInt(flags, "minutes"))
	if err != nil {
		return nil, err
	}
	return status, nil
}

// implDNDTeamInfo implements dnd.teamInfo.
func implDNDTeamInfo(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	users := flagStrSlice(flags, "users")
	statuses, err := client.GetDNDTeamInfoContext(ctx, users)
	if err != nil {
		return nil, err
	}
	return statuses, nil
}

func init() {
	RegisterDispatch("dnd.endDnd", implDNDEnd)
	RegisterDispatch("dnd.endSnooze", implDNDEndSnooze)
	RegisterDispatch("dnd.info", implDNDInfo)
	RegisterDispatch("dnd.setSnooze", implDNDSetSnooze)
	RegisterDispatch("dnd.teamInfo", implDNDTeamInfo)
}
