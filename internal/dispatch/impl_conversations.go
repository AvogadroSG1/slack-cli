package dispatch

import (
	"context"

	"github.com/slack-go/slack"
)

// implConversationsList implements conversations.list.
func implConversationsList(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := &slack.GetConversationsParameters{
		Cursor:          flagStr(flags, "cursor"),
		ExcludeArchived: flagBool(flags, "exclude-archived"),
		Limit:           flagInt(flags, "limit"),
		Types:           flagStrSlice(flags, "types"),
		TeamID:          flagStr(flags, "team-id"),
	}
	channels, nextCursor, err := client.GetConversationsContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"channels":    channels,
		"next_cursor": nextCursor,
	}, nil
}

// implConversationsHistory implements conversations.history.
func implConversationsHistory(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID:          flagStr(flags, "channel"),
		Cursor:             flagStr(flags, "cursor"),
		Inclusive:          flagBool(flags, "inclusive"),
		Latest:             flagStr(flags, "latest"),
		Limit:              flagInt(flags, "limit"),
		Oldest:             flagStr(flags, "oldest"),
		IncludeAllMetadata: flagBool(flags, "include-all-metadata"),
	}
	resp, err := client.GetConversationHistoryContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// implConversationsInfo implements conversations.info.
func implConversationsInfo(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	input := &slack.GetConversationInfoInput{
		ChannelID:         flagStr(flags, "channel"),
		IncludeLocale:     flagBool(flags, "include-locale"),
		IncludeNumMembers: flagBool(flags, "include-num-members"),
	}
	ch, err := client.GetConversationInfoContext(ctx, input)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// implConversationsReplies implements conversations.replies.
func implConversationsReplies(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID:          flagStr(flags, "channel"),
		Timestamp:          flagStr(flags, "ts"),
		Cursor:             flagStr(flags, "cursor"),
		Inclusive:          flagBool(flags, "inclusive"),
		Latest:             flagStr(flags, "latest"),
		Limit:              flagInt(flags, "limit"),
		Oldest:             flagStr(flags, "oldest"),
		IncludeAllMetadata: flagBool(flags, "include-all-metadata"),
	}
	msgs, hasMore, nextCursor, err := client.GetConversationRepliesContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"messages":    msgs,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	}, nil
}

// implConversationsCreate implements conversations.create.
func implConversationsCreate(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := slack.CreateConversationParams{
		ChannelName: flagStr(flags, "name"),
		IsPrivate:   flagBool(flags, "is-private"),
		TeamID:      flagStr(flags, "team-id"),
	}
	ch, err := client.CreateConversationContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// implConversationsArchive implements conversations.archive.
func implConversationsArchive(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.ArchiveConversationContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implConversationsUnarchive implements conversations.unarchive.
func implConversationsUnarchive(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.UnArchiveConversationContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implConversationsInvite implements conversations.invite.
func implConversationsInvite(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	users := flagStrSlice(flags, "users")
	ch, err := client.InviteUsersToConversationContext(ctx, flagStr(flags, "channel"), users...)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// implConversationsKick implements conversations.kick.
func implConversationsKick(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.KickUserFromConversationContext(ctx, flagStr(flags, "channel"), flagStr(flags, "user"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implConversationsLeave implements conversations.leave.
func implConversationsLeave(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	notInChannel, err := client.LeaveConversationContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"not_in_channel": notInChannel,
	}, nil
}

// implConversationsJoin implements conversations.join.
func implConversationsJoin(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ch, warning, warnings, err := client.JoinConversationContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"channel":  ch,
		"warning":  warning,
		"warnings": warnings,
	}, nil
}

// implConversationsSetPurpose implements conversations.setPurpose.
func implConversationsSetPurpose(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ch, err := client.SetPurposeOfConversationContext(ctx, flagStr(flags, "channel"), flagStr(flags, "purpose"))
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// implConversationsSetTopic implements conversations.setTopic.
func implConversationsSetTopic(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ch, err := client.SetTopicOfConversationContext(ctx, flagStr(flags, "channel"), flagStr(flags, "topic"))
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// implConversationsOpen implements conversations.open.
func implConversationsOpen(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := &slack.OpenConversationParameters{
		ChannelID: flagStr(flags, "channel"),
		ReturnIM:  flagBool(flags, "return-im"),
		Users:     flagStrSlice(flags, "users"),
	}
	ch, noOp, alreadyOpen, err := client.OpenConversationContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"channel":      ch,
		"no_op":        noOp,
		"already_open": alreadyOpen,
	}, nil
}

// implConversationsClose implements conversations.close.
func implConversationsClose(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	noOp, alreadyClosed, err := client.CloseConversationContext(ctx, flagStr(flags, "channel"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"no_op":          noOp,
		"already_closed": alreadyClosed,
	}, nil
}

// implConversationsRename implements conversations.rename.
func implConversationsRename(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	ch, err := client.RenameConversationContext(ctx, flagStr(flags, "channel"), flagStr(flags, "name"))
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// implConversationsMark implements conversations.mark.
func implConversationsMark(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	err := client.MarkConversationContext(ctx, flagStr(flags, "channel"), flagStr(flags, "ts"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// implConversationsMembers implements conversations.members.
func implConversationsMembers(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	params := &slack.GetUsersInConversationParameters{
		ChannelID: flagStr(flags, "channel"),
		Cursor:    flagStr(flags, "cursor"),
		Limit:     flagInt(flags, "limit"),
	}
	users, nextCursor, err := client.GetUsersInConversationContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"users":       users,
		"next_cursor": nextCursor,
	}, nil
}

func init() {
	RegisterDispatch("conversations.list", implConversationsList)
	RegisterDispatch("conversations.history", implConversationsHistory)
	RegisterDispatch("conversations.info", implConversationsInfo)
	RegisterDispatch("conversations.replies", implConversationsReplies)
	RegisterDispatch("conversations.create", implConversationsCreate)
	RegisterDispatch("conversations.archive", implConversationsArchive)
	RegisterDispatch("conversations.unarchive", implConversationsUnarchive)
	RegisterDispatch("conversations.invite", implConversationsInvite)
	RegisterDispatch("conversations.kick", implConversationsKick)
	RegisterDispatch("conversations.leave", implConversationsLeave)
	RegisterDispatch("conversations.join", implConversationsJoin)
	RegisterDispatch("conversations.setPurpose", implConversationsSetPurpose)
	RegisterDispatch("conversations.setTopic", implConversationsSetTopic)
	RegisterDispatch("conversations.open", implConversationsOpen)
	RegisterDispatch("conversations.close", implConversationsClose)
	RegisterDispatch("conversations.rename", implConversationsRename)
	RegisterDispatch("conversations.mark", implConversationsMark)
	RegisterDispatch("conversations.members", implConversationsMembers)
}
