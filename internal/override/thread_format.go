package override

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

type threadReaction struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type threadMessage struct {
	User      string
	Time      time.Time
	SlackTS   string
	Text      string
	Reactions []threadReaction
	Metadata  *slack.SlackMetadata
}

type threadMessageJSON struct {
	User      string               `json:"user"`
	TS        string               `json:"ts"`
	SlackTS   string               `json:"slack_ts"`
	Text      string               `json:"text"`
	Reactions []threadReaction     `json:"reactions"`
	Metadata  *slack.SlackMetadata `json:"metadata,omitempty"`
}

type threadIncompleteStatus struct {
	Complete   bool   `json:"complete"`
	Reason     string `json:"reason"`
	NextCursor string `json:"next_cursor"`
}

func normalizeThreadMessages(
	messages []slack.Message,
	idMap map[string]string,
	includeMetadata bool,
) []threadMessage {
	normalized := make([]threadMessage, 0, len(messages))
	for _, message := range messages {
		reactions := make([]threadReaction, len(message.Reactions))
		for i, reaction := range message.Reactions {
			reactions[i] = threadReaction{Name: reaction.Name, Count: reaction.Count}
		}
		sort.Slice(reactions, func(i, j int) bool {
			return reactions[i].Name < reactions[j].Name
		})

		var metadata *slack.SlackMetadata
		if includeMetadata && hasSlackMetadata(message.Metadata) {
			copy := message.Metadata
			metadata = &copy
		}
		normalized = append(normalized, threadMessage{
			User:      resolveUser(message.User, message.BotID, idMap),
			Time:      parseSlackTimestamp(message.Timestamp),
			SlackTS:   message.Timestamp,
			Text:      message.Text,
			Reactions: reactions,
			Metadata:  metadata,
		})
	}
	return normalized
}

func hasSlackMetadata(metadata slack.SlackMetadata) bool {
	return metadata.EventType != "" || metadata.EventPayload != nil
}

func formatThreadMessages(messages []threadMessage, asJSON bool, writer io.Writer) error {
	if asJSON {
		out := make([]threadMessageJSON, len(messages))
		for i, message := range messages {
			out[i] = threadMessageJSON{
				User: message.User, TS: message.Time.Format(time.RFC3339),
				SlackTS: message.SlackTS, Text: message.Text,
				Reactions: message.Reactions, Metadata: message.Metadata,
			}
		}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(out)
	}

	for _, message := range messages {
		localTime := message.Time.Local().Format("2006-01-02 15:04")
		if _, err := fmt.Fprintf(writer, "%s [%s]: %s\n", message.User, localTime, message.Text); err != nil {
			return err
		}
		if len(message.Reactions) == 0 {
			continue
		}
		parts := make([]string, len(message.Reactions))
		for i, reaction := range message.Reactions {
			parts[i] = fmt.Sprintf(":%s: %d", reaction.Name, reaction.Count)
		}
		if _, err := fmt.Fprintf(writer, "  Reactions: %s\n", strings.Join(parts, ", ")); err != nil {
			return err
		}
	}
	return nil
}

func writeThreadIncompleteStatus(writer io.Writer, asJSON bool, nextCursor string) error {
	if asJSON {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(threadIncompleteStatus{
			Complete: false, Reason: "max_results", NextCursor: nextCursor,
		})
	}
	_, err := fmt.Fprintf(
		writer,
		"Warning: result limited by --max-results; resume with --cursor %s\n",
		nextCursor,
	)
	return err
}
