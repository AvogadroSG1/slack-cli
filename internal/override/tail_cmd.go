package override

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// tailMaxConsecutiveFailures is how many polls in a row may fail before
// tail gives up.
const tailMaxConsecutiveFailures = 3

// tailSeenLimit caps the dedupe set size.
const tailSeenLimit = 1000

// tailSkipSubtypes are message subtypes that are not new conversation
// content and should not be emitted.
var tailSkipSubtypes = map[string]bool{
	"message_changed": true,
	"message_deleted": true,
	"message_replied": true,
	"channel_join":    true,
	"channel_leave":   true,
	"group_join":      true,
	"group_leave":     true,
}

// tailState tracks the polling high-water mark and recently emitted ts.
type tailState struct {
	lastTS string
	seen   map[string]struct{}
}

func newTailState() *tailState {
	return &tailState{seen: map[string]struct{}{}}
}

// newTailCmd builds the semantic "tail" command: stream new channel
// messages to stdout by polling conversations.history.
func newTailCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail <channel>",
		Short: "Stream new channel messages to stdout (like tail -f)",
		Long: `Poll a channel and print new messages as they arrive, until
interrupted (Ctrl-C exits cleanly). Uses conversations.history polling so it
works with a plain SLACK_TOKEN; Slack's Socket Mode would need a separate
xapp- app token. Thread replies do not appear in channel history and are
not streamed. With --json, each message is printed as one JSON object per
line (NDJSON); --template renders each message through the template.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runTail(cmd, client, args[0])
		},
	}

	cmd.Flags().Duration("interval", 3*time.Second, "Poll interval (minimum 1s)")
	cmd.Flags().String("since", "", "Backfill messages after this time before streaming (2h, 30m, …)")
	addOutputFlags(cmd)

	return cmd
}

func runTail(cmd *cobra.Command, client readClient, target string) error {
	interval, _ := cmd.Flags().GetDuration("interval")
	since, _ := cmd.Flags().GetString("since")
	opts := getOutputOpts(cmd)

	if interval < time.Second {
		return formatAndExit(cmd, fmt.Errorf("--interval must be at least 1s"), exitcode.InputError)
	}

	warnIfCacheNotReady(cmd)

	channelID, channelName, err := readTargetChannel(cmd.Context(), client, target)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	st := newTailState()
	if since != "" {
		st.lastTS, err = parseTimeSpec(since, time.Now())
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		// Emit the backfill window on the first poll below.
	} else {
		// Establish the high-water mark: latest message ts, nothing emitted.
		resp, err := client.GetConversationHistoryContext(cmd.Context(), &slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Limit:     1,
		})
		if err != nil {
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		if len(resp.Messages) > 0 {
			st.lastTS = resp.Messages[0].Timestamp
		}
	}

	out := cmd.OutOrStdout()
	emit := func(msgs []slack.Message) error {
		for _, m := range toSemMessages(msgs, channelName, idMap) {
			if opts.Template != "" || opts.JSON {
				if err := renderTailRow(out, opts, m); err != nil {
					return err
				}
				continue
			}
			if err := writeSemMessageText(m, false, opts.Plain, idMap, out); err != nil {
				return err
			}
		}
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failures := 0
	for {
		newMsgs, err := pollTail(cmd.Context(), client, channelID, st)
		switch {
		case err == nil:
			failures = 0
			if err := emit(newMsgs); err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}
		case errors.Is(err, context.Canceled):
			return nil
		default:
			failures++
			fmt.Fprintf(cmd.ErrOrStderr(), "tail: poll failed (%d/%d): %v\n", failures, tailMaxConsecutiveFailures, err)
			if failures >= tailMaxConsecutiveFailures {
				return formatAndExit(cmd, err, exitcode.Classify(err))
			}
		}

		select {
		case <-cmd.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

// renderTailRow writes one message in NDJSON or template form.
func renderTailRow(w io.Writer, opts outputOpts, m semMessage) error {
	if opts.Template != "" {
		if err := renderOutput(w, opts, m, nil); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w)
		return err
	}
	enc := json.NewEncoder(w)
	return enc.Encode(m)
}

// pollTail fetches messages newer than st.lastTS, oldest-first, deduped
// against st.seen and with non-content subtypes skipped. It advances the
// state and returns only the messages to emit. Rate limits are waited out.
func pollTail(ctx context.Context, c historyFetcher, channelID string, st *tailState) ([]slack.Message, error) {
	for {
		resp, err := c.GetConversationHistoryContext(ctx, &slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Oldest:    st.lastTS,
			Inclusive: false,
			Limit:     historyPageLimit,
		})
		if err != nil {
			if retried, rerr := sleepOnRateLimit(ctx, err); retried {
				continue
			} else if rerr != nil {
				return nil, rerr
			}
			return nil, err
		}

		msgs := append([]slack.Message(nil), resp.Messages...)
		reverseMessages(msgs)

		var out []slack.Message
		for _, m := range msgs {
			if _, dup := st.seen[m.Timestamp]; dup {
				continue
			}
			st.seen[m.Timestamp] = struct{}{}
			if m.Timestamp > st.lastTS {
				st.lastTS = m.Timestamp
			}
			if tailSkipSubtypes[m.SubType] {
				continue
			}
			out = append(out, m)
		}

		if len(st.seen) > tailSeenLimit {
			st.seen = map[string]struct{}{}
		}
		return out, nil
	}
}
