package override

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// inspectClient abstracts the info calls inspect needs.
type inspectClient interface {
	GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error)
	GetUserInfoContext(ctx context.Context, user string) (*slack.User, error)
}

// newInspectCmd builds the semantic "inspect" command: detailed metadata
// about a channel, user, or usergroup.
func newInspectCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <entity>",
		Short: "Show metadata about a channel, user, or usergroup",
		Long: `Look up detailed metadata for a workspace entity. The entity may
be a channel (#name or C… ID), a user (@name or U… ID), a usergroup handle
or S… ID, or a bare name (channels are tried first, then users, then
usergroups). Usergroup details come from the local cache.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runInspect(cmd, client, args[0])
		},
	}

	addOutputFlags(cmd)
	return cmd
}

func runInspect(cmd *cobra.Command, client inspectClient, target string) error {
	opts := getOutputOpts(cmd)
	warnIfCacheNotReady(cmd)

	id, kind, err := resolveTarget(target)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	switch kind {
	case targetChannel:
		return inspectChannel(cmd, client, id, opts)
	case targetUser:
		return inspectUser(cmd, client, id, opts)
	default:
		return inspectUsergroup(cmd, target, id, opts)
	}
}

func inspectChannel(cmd *cobra.Command, client inspectClient, id string, opts outputOpts) error {
	ch, err := client.GetConversationInfoContext(cmd.Context(), &slack.GetConversationInfoInput{
		ChannelID:         id,
		IncludeNumMembers: true,
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	err = renderOutput(cmd.OutOrStdout(), opts, ch, func(w io.Writer) error {
		return writeKV(w, [][2]string{
			{"Name", "#" + ch.Name},
			{"ID", ch.ID},
			{"Type", channelType(ch)},
			{"Topic", ch.Topic.Value},
			{"Purpose", ch.Purpose.Value},
			{"Members", fmt.Sprintf("%d", ch.NumMembers)},
			{"Created", ch.Created.Time().Local().Format("2006-01-02 15:04")},
			{"Archived", fmt.Sprintf("%t", ch.IsArchived)},
			{"Private", fmt.Sprintf("%t", ch.IsPrivate)},
		})
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}

func inspectUser(cmd *cobra.Command, client inspectClient, id string, opts outputOpts) error {
	u, err := client.GetUserInfoContext(cmd.Context(), id)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	err = renderOutput(cmd.OutOrStdout(), opts, u, func(w io.Writer) error {
		return writeKV(w, [][2]string{
			{"Name", u.Name},
			{"ID", u.ID},
			{"Real name", u.RealName},
			{"Display name", u.Profile.DisplayName},
			{"Email", u.Profile.Email},
			{"Title", u.Profile.Title},
			{"Timezone", u.TZ},
			{"Status", u.Profile.StatusText},
			{"Bot", fmt.Sprintf("%t", u.IsBot)},
			{"Deleted", fmt.Sprintf("%t", u.Deleted)},
		})
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}

// inspectUsergroup renders the cached usergroup entry. target is the
// original argument (a handle unless an ID was given).
func inspectUsergroup(cmd *cobra.Command, target, id string, opts outputOpts) error {
	handle, entry, err := lookupUsergroup(target, id)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	v := struct {
		Handle      string   `json:"handle"`
		ID          string   `json:"id"`
		Description string   `json:"description"`
		Members     []string `json:"members"`
	}{handle, entry.ID, entry.Description, entry.Members}

	err = renderOutput(cmd.OutOrStdout(), opts, v, func(w io.Writer) error {
		return writeKV(w, [][2]string{
			{"Handle", "@" + handle},
			{"ID", entry.ID},
			{"Description", entry.Description},
			{"Members", fmt.Sprintf("%d", entry.MemberCount())},
		})
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}

// lookupUsergroup finds a cached usergroup by handle or ID.
func lookupUsergroup(target, id string) (string, cache.UsergroupEntry, error) {
	groups, err := cache.LoadEntity[cache.UsergroupCache](cache.UsergroupsFileName)
	if err != nil {
		return "", cache.UsergroupEntry{}, cacheMissHint(err)
	}
	for handle, entry := range groups {
		if entry.ID == id || handle == target {
			return handle, entry, nil
		}
	}
	return "", cache.UsergroupEntry{}, cacheMissHint(fmt.Errorf("no cached usergroup %q", target))
}

// channelType describes what kind of conversation a channel is.
func channelType(ch *slack.Channel) string {
	switch {
	case ch.IsIM:
		return "im"
	case ch.IsMpIM:
		return "mpim"
	case ch.IsPrivate:
		return "private_channel"
	default:
		return "public_channel"
	}
}

// writeKV writes aligned key: value lines through a tabwriter, skipping
// empty values.
func writeKV(w io.Writer, pairs [][2]string) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, p := range pairs {
		if p[1] == "" {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s:\t%s\n", p[0], p[1]); err != nil {
			return err
		}
	}
	return tw.Flush()
}
