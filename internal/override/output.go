package override

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// outputOpts captures the shared output flags of semantic commands.
type outputOpts struct {
	JSON     bool
	Plain    bool
	Template string // Go text/template source; "" = disabled
}

// addOutputFlags registers the shared --json, --plain, and --template flags
// on a semantic command. --json is mutually exclusive with the other two.
func addOutputFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("plain", false, "Strip Slack markup and emoji from text output")
	cmd.Flags().String("template", "", "Render output through a Go text/template (receives the same value as --json)")
	cmd.MarkFlagsMutuallyExclusive("json", "plain")
	cmd.MarkFlagsMutuallyExclusive("json", "template")
}

// getOutputOpts reads the shared output flags back from cmd.
func getOutputOpts(cmd *cobra.Command) outputOpts {
	jsonOut, _ := cmd.Flags().GetBool("json")
	plain, _ := cmd.Flags().GetBool("plain")
	tmpl, _ := cmd.Flags().GetString("template")
	return outputOpts{JSON: jsonOut, Plain: plain, Template: tmpl}
}

// renderOutput writes v to w according to opts. Template mode executes the
// template over v (the same value --json would emit). JSON mode writes
// two-space-indented JSON. Otherwise textFn writes the command's
// human-readable format.
func renderOutput(w io.Writer, opts outputOpts, v any, textFn func(io.Writer) error) error {
	if opts.Template != "" {
		tmpl, err := template.New("output").Parse(opts.Template)
		if err != nil {
			return fmt.Errorf("invalid --template: %w", err)
		}
		return tmpl.Execute(w, v)
	}
	if opts.JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	return textFn(w)
}

// Patterns for stripMrkdwn, applied in order.
var (
	userMentionRe    = regexp.MustCompile(`<@([UW][A-Z0-9]+)(?:\|[^>]*)?>`)
	channelMentionRe = regexp.MustCompile(`<#[CDG][A-Z0-9]+\|([^>]*)>`)
	channelBareRe    = regexp.MustCompile(`<#([CDG][A-Z0-9]+)>`)
	specialMentionRe = regexp.MustCompile(`<!(here|channel|everyone)(?:\|[^>]*)?>`)
	labelledLinkRe   = regexp.MustCompile(`<(https?://[^|>]+)\|([^>]*)>`)
	bareLinkRe       = regexp.MustCompile(`<(https?://[^>]+)>`)
	emojiRe          = regexp.MustCompile(`:[a-z0-9_+-]+:`)
	boldRe           = regexp.MustCompile(`\*([^*\n]+)\*`)
	italicRe         = regexp.MustCompile(`\b_([^_\n]+)_\b`)
	strikeRe         = regexp.MustCompile(`~([^~\n]+)~`)
	codeRe           = regexp.MustCompile("`([^`\n]+)`")
)

// stripMrkdwn converts Slack mrkdwn to plain text: mentions become @name /
// #name (user IDs resolved through idMap when possible), links keep their
// label and URL, formatting markers and emoji shortcodes are removed, and
// HTML entities are unescaped.
func stripMrkdwn(s string, idMap map[string]string) string {
	s = userMentionRe.ReplaceAllStringFunc(s, func(m string) string {
		id := userMentionRe.FindStringSubmatch(m)[1]
		if name, ok := idMap[id]; ok {
			return "@" + name
		}
		return "@" + id
	})
	s = channelMentionRe.ReplaceAllString(s, "#$1")
	s = channelBareRe.ReplaceAllString(s, "#$1")
	s = specialMentionRe.ReplaceAllString(s, "@$1")
	s = labelledLinkRe.ReplaceAllString(s, "$2 ($1)")
	s = bareLinkRe.ReplaceAllString(s, "$1")
	s = boldRe.ReplaceAllString(s, "$1")
	s = italicRe.ReplaceAllString(s, "$1")
	s = strikeRe.ReplaceAllString(s, "$1")
	s = codeRe.ReplaceAllString(s, "$1")
	s = emojiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}

// semMessage is the JSON and template row for semantic message output.
type semMessage struct {
	Channel   string `json:"channel,omitempty"` // "#name" when known
	User      string `json:"user"`
	Ts        string `json:"ts"`   // raw Slack ts (stable join key)
	Time      string `json:"time"` // RFC3339, local
	Text      string `json:"text"`
	ThreadTs  string `json:"thread_ts,omitempty"`
	Permalink string `json:"permalink,omitempty"` // search only
}

// formatSemMessagesText writes msgs in the human-readable line format
// "Name [YYYY-MM-DD HH:MM]: text" (local time, matching formatMessagesText).
// When indentReplies is true, messages carrying a ThreadTs different from
// their own Ts are indented as thread replies. When plain is true, message
// text is run through stripMrkdwn.
func formatSemMessagesText(msgs []semMessage, indentReplies, plain bool, idMap map[string]string, w io.Writer) error {
	for _, m := range msgs {
		if err := writeSemMessageText(m, indentReplies, plain, idMap, w); err != nil {
			return err
		}
	}
	return nil
}

// writeSemMessageText writes a single message line (used by tail for
// per-message streaming output).
func writeSemMessageText(m semMessage, indentReplies, plain bool, idMap map[string]string, w io.Writer) error {
	text := m.Text
	if plain {
		text = stripMrkdwn(text, idMap)
	}
	prefix := ""
	if indentReplies && m.ThreadTs != "" && m.ThreadTs != m.Ts {
		prefix = "    ↳ "
	}
	timeStr := ""
	if t, err := time.Parse(time.RFC3339, m.Time); err == nil {
		timeStr = t.Local().Format("2006-01-02 15:04")
	}
	_, err := fmt.Fprintf(w, "%s%s [%s]: %s\n", prefix, m.User, timeStr, text)
	return err
}

// toSemMessages converts SDK messages to semantic rows, resolving user IDs
// to display names via idMap. channelName ("#name" or "") is attached to
// every row.
func toSemMessages(msgs []slack.Message, channelName string, idMap map[string]string) []semMessage {
	out := make([]semMessage, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, semMessage{
			Channel:  channelName,
			User:     resolveUser(msg.User, msg.BotID, idMap),
			Ts:       msg.Timestamp,
			Time:     parseSlackTimestamp(msg.Timestamp).Format(time.RFC3339),
			Text:     msg.Text,
			ThreadTs: msg.ThreadTimestamp,
		})
	}
	return out
}
