package dispatch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slack-go/slack"
)

// buildChatMsgOptions constructs a slice of slack.MsgOption from the
// flags map. It covers the common options shared across chat methods:
// text, thread-ts, reply-broadcast, icon-emoji, icon-url, username, and
// blocks. Callers append method-specific options after this call.
func buildChatMsgOptions(flags map[string]any) ([]slack.MsgOption, error) {
	var opts []slack.MsgOption

	if text := flagStr(flags, "text"); text != "" {
		opts = append(opts, slack.MsgOptionText(text, false))
	}
	if ts := flagStr(flags, "thread-ts"); ts != "" {
		opts = append(opts, slack.MsgOptionTS(ts))
	}
	if flagBool(flags, "reply-broadcast") {
		opts = append(opts, slack.MsgOptionBroadcast())
	}
	if flagBool(flags, "unfurl-links") {
		opts = append(opts, slack.MsgOptionEnableLinkUnfurl())
	}
	if flagBool(flags, "unfurl-media") {
		// SDK only exposes a disable toggle; skip when the flag is true
		// because link unfurl is enabled by default.
	}
	if emoji := flagStr(flags, "icon-emoji"); emoji != "" {
		opts = append(opts, slack.MsgOptionIconEmoji(emoji))
	}
	if url := flagStr(flags, "icon-url"); url != "" {
		opts = append(opts, slack.MsgOptionIconURL(url))
	}
	if user := flagStr(flags, "username"); user != "" {
		opts = append(opts, slack.MsgOptionAsUser(false))
		opts = append(opts, slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{Username: user}))
	}

	if raw := flagStr(flags, "blocks"); raw != "" {
		var b slack.Blocks
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			return nil, fmt.Errorf("parsing blocks JSON: %v", err)
		}
		opts = append(opts, slack.MsgOptionBlocks(b.BlockSet...))
	}

	return opts, nil
}

// dispatchPostMessage implements chat.postMessage.
func dispatchPostMessageImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	channel := flagStr(flags, "channel")
	if channel == "" {
		return nil, fmt.Errorf("channel flag is required")
	}

	opts, err := buildChatMsgOptions(flags)
	if err != nil {
		return nil, err
	}

	ch, ts, err := client.PostMessageContext(ctx, channel, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat.postMessage: %v", err)
	}

	return map[string]string{"channel": ch, "timestamp": ts}, nil
}

// dispatchPostEphemeralImpl implements chat.postEphemeral.
func dispatchPostEphemeralImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	channel := flagStr(flags, "channel")
	if channel == "" {
		return nil, fmt.Errorf("channel flag is required")
	}
	user := flagStr(flags, "user")
	if user == "" {
		return nil, fmt.Errorf("user flag is required")
	}

	opts, err := buildChatMsgOptions(flags)
	if err != nil {
		return nil, err
	}

	ts, err := client.PostEphemeralContext(ctx, channel, user, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat.postEphemeral: %v", err)
	}

	return map[string]string{"timestamp": ts}, nil
}

// dispatchUpdateMessageImpl implements chat.update.
func dispatchUpdateMessageImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	channel := flagStr(flags, "channel")
	if channel == "" {
		return nil, fmt.Errorf("channel flag is required")
	}
	ts := flagStr(flags, "ts")
	if ts == "" {
		return nil, fmt.Errorf("ts flag is required")
	}

	opts, err := buildChatMsgOptions(flags)
	if err != nil {
		return nil, err
	}

	ch, newTS, text, err := client.UpdateMessageContext(ctx, channel, ts, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat.update: %v", err)
	}

	return map[string]string{"channel": ch, "timestamp": newTS, "text": text}, nil
}

// dispatchDeleteMessageImpl implements chat.delete.
func dispatchDeleteMessageImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	channel := flagStr(flags, "channel")
	if channel == "" {
		return nil, fmt.Errorf("channel flag is required")
	}
	ts := flagStr(flags, "ts")
	if ts == "" {
		return nil, fmt.Errorf("ts flag is required")
	}

	ch, respTS, err := client.DeleteMessageContext(ctx, channel, ts)
	if err != nil {
		return nil, fmt.Errorf("chat.delete: %v", err)
	}

	return map[string]string{"channel": ch, "timestamp": respTS}, nil
}

// dispatchGetPermalinkImpl implements chat.getPermalink.
func dispatchGetPermalinkImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	channel := flagStr(flags, "channel")
	if channel == "" {
		return nil, fmt.Errorf("channel flag is required")
	}
	ts := flagStr(flags, "ts")
	if ts == "" {
		return nil, fmt.Errorf("ts flag is required")
	}

	permalink, err := client.GetPermalinkContext(ctx, &slack.PermalinkParameters{
		Channel: channel,
		Ts:      ts,
	})
	if err != nil {
		return nil, fmt.Errorf("chat.getPermalink: %v", err)
	}

	return map[string]string{"permalink": permalink}, nil
}

// dispatchScheduleMessageImpl implements chat.scheduleMessage.
func dispatchScheduleMessageImpl(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
	channel := flagStr(flags, "channel")
	if channel == "" {
		return nil, fmt.Errorf("channel flag is required")
	}
	postAt := flagStr(flags, "post-at")
	if postAt == "" {
		return nil, fmt.Errorf("post-at flag is required")
	}

	opts, err := buildChatMsgOptions(flags)
	if err != nil {
		return nil, err
	}

	ch, scheduledID, err := client.ScheduleMessageContext(ctx, channel, postAt, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat.scheduleMessage: %v", err)
	}

	return map[string]string{"channel": ch, "scheduled_message_id": scheduledID}, nil
}

func init() {
	RegisterDispatch("chat.postMessage", dispatchPostMessageImpl)
	RegisterDispatch("chat.postEphemeral", dispatchPostEphemeralImpl)
	RegisterDispatch("chat.update", dispatchUpdateMessageImpl)
	RegisterDispatch("chat.delete", dispatchDeleteMessageImpl)
	RegisterDispatch("chat.getPermalink", dispatchGetPermalinkImpl)
	RegisterDispatch("chat.scheduleMessage", dispatchScheduleMessageImpl)
}
