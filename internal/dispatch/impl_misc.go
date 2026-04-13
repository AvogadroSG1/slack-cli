package dispatch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slack-go/slack"
)

// implAuthTest implements auth.test.
func implAuthTest(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// implTeamInfo implements team.info.
func implTeamInfo(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	info, err := client.GetTeamInfoContext(ctx)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// implEmojiList implements emoji.list.
func implEmojiList(ctx context.Context, client *slack.Client, _ map[string]any) (any, error) {
	emoji, err := client.GetEmojiContext(ctx)
	if err != nil {
		return nil, err
	}
	return emoji, nil
}

// implBotsInfo implements bots.info.
func implBotsInfo(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.GetBotInfoParameters{
		Bot:    flagStr(flags, "bot"),
		TeamID: flagStr(flags, "team-id"),
	}
	bot, err := client.GetBotInfoContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return bot, nil
}

// implDialogOpen implements dialog.open.
func implDialogOpen(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	var dialog slack.Dialog
	if err := json.Unmarshal([]byte(flagStr(flags, "dialog")), &dialog); err != nil {
		return nil, fmt.Errorf("invalid dialog JSON: %w", err)
	}
	err := client.OpenDialogContext(ctx, flagStr(flags, "trigger-id"), dialog)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func init() {
	RegisterDispatch("auth.test", implAuthTest)
	RegisterDispatch("team.info", implTeamInfo)
	RegisterDispatch("emoji.list", implEmojiList)
	RegisterDispatch("bots.info", implBotsInfo)
	RegisterDispatch("dialog.open", implDialogOpen)
}
