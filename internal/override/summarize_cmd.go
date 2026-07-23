package override

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// summarizeDefaultModel is the Claude model used when --model is not set.
const summarizeDefaultModel = "claude-opus-4-8"

// summarizeMaxTokens bounds the summary length.
const summarizeMaxTokens = 2048

// summarizeSystemPrompt instructs the model on summary shape.
const summarizeSystemPrompt = `You summarize Slack conversations. Output concise markdown with these sections, omitting any that don't apply: **Key decisions**, **Action items** (one bullet per item, naming the owner when identifiable), **Discussion points**. Be brief; do not restate the transcript.`

// summarizer abstracts the LLM call so tests can provide a fake.
type summarizer interface {
	Summarize(ctx context.Context, model, transcript string) (string, error)
}

// anthropicSummarizer calls the Claude API.
type anthropicSummarizer struct {
	client anthropic.Client
}

func (s *anthropicSummarizer) Summarize(ctx context.Context, model, transcript string) (string, error) {
	resp, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: summarizeMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: summarizeSystemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(transcript)),
		},
	})
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			out.WriteString(t.Text)
		}
	}
	return out.String(), nil
}

// summarizeResult is the --json output shape.
type summarizeResult struct {
	Summary      string `json:"summary"`
	Model        string `json:"model"`
	MessageCount int    `json:"message_count"`
	Channel      string `json:"channel,omitempty"`
}

// newSummarizeCmd builds the semantic "summarize" command.
func newSummarizeCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize <channel|thread-permalink>",
		Short: "Summarize a channel or thread with Claude",
		Long: `Collect recent messages from a channel (or a full thread from a
permalink) and summarize the key decisions, action items, and discussion
points using the Claude API. Requires ANTHROPIC_API_KEY in addition to
SLACK_TOKEN.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			// The SDK can resolve credentials from other sources, but this
			// CLI requires the env var for a fast, clear error.
			if os.Getenv("ANTHROPIC_API_KEY") == "" {
				return formatAndExit(cmd, fmt.Errorf("ANTHROPIC_API_KEY is not set"), exitcode.AuthError)
			}
			return runSummarize(cmd, client, &anthropicSummarizer{client: anthropic.NewClient()}, args[0])
		},
	}

	cmd.Flags().String("since", "24h", "Time window for channel summaries (2h, 3d, 2026-07-01, …)")
	cmd.Flags().Int("limit", 200, "Maximum messages to include in the transcript")
	cmd.Flags().Bool("include-threads", false, "Expand thread replies inline (channel summaries)")
	cmd.Flags().String("model", summarizeDefaultModel, "Claude model to use")
	cmd.Flags().Bool("json", false, "Output as JSON")

	return cmd
}

func runSummarize(cmd *cobra.Command, client readClient, llm summarizer, target string) error {
	since, _ := cmd.Flags().GetString("since")
	limit, _ := cmd.Flags().GetInt("limit")
	includeThreads, _ := cmd.Flags().GetBool("include-threads")
	model, _ := cmd.Flags().GetString("model")
	asJSON, _ := cmd.Flags().GetBool("json")

	warnIfCacheNotReady(cmd)
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	var msgs []slack.Message
	channelName := ""
	window := ""
	if channel, ts, err := parseSlackURL(target); err == nil {
		msgs, err = fetchThreadMessages(cmd.Context(), client, channel, ts)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		window = "thread " + ts
	} else {
		channelID, name, err := readTargetChannel(cmd.Context(), client, target)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		channelName = name
		oldest, err := parseTimeSpec(since, time.Now())
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		msgs, err = fetchChannelMessages(cmd.Context(), client, channelID, oldest, limit, includeThreads)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		window = "since " + since
	}

	if len(msgs) == 0 {
		return formatAndExit(cmd, fmt.Errorf("nothing to summarize in %s (%s)", target, window), exitcode.InputError)
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}

	transcript := buildTranscript(msgs, idMap)
	summary, err := llm.Summarize(cmd.Context(), model, transcript)
	if err != nil {
		return formatAndExit(cmd, err, anthropicErrCode(err))
	}

	if asJSON {
		return renderOutput(cmd.OutOrStdout(), outputOpts{JSON: true}, summarizeResult{
			Summary:      summary,
			Model:        model,
			MessageCount: len(msgs),
			Channel:      channelName,
		}, nil)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(summary, "\n"))
	return err
}

// buildTranscript renders messages as mrkdwn-stripped "Name [time]: text"
// lines for the model.
func buildTranscript(msgs []slack.Message, idMap map[string]string) string {
	var b strings.Builder
	for _, m := range msgs {
		name := resolveUser(m.User, m.BotID, idMap)
		ts := parseSlackTimestamp(m.Timestamp).Local().Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "%s [%s]: %s\n", name, ts, stripMrkdwn(m.Text, idMap))
	}
	return b.String()
}

// anthropicErrCode maps Claude API errors to exit codes: auth failures to
// AuthError, other API responses to APIError, transport errors to NetError.
func anthropicErrCode(err error) int {
	var apierr *anthropic.Error
	if errors.As(err, &apierr) {
		switch apierr.StatusCode {
		case 401, 403:
			return exitcode.AuthError
		default:
			return exitcode.APIError
		}
	}
	return exitcode.NetError
}
