package override

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// exportClient is the API surface export needs: history/replies plus channel
// enumeration and file downloads.
type exportClient interface {
	historyRepliesFetcher
	GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) ([]slack.Channel, string, error)
	GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error
}

// exportFormats maps --format values to file extensions.
var exportFormats = map[string]string{
	"json":     "json",
	"markdown": "md",
	"csv":      "csv",
}

// newExportCmd builds the semantic "export" command: bulk history dumps for
// backups or offline analysis.
func newExportCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <channel|all>",
		Short: "Export channel history to JSON, Markdown, or CSV files",
		Long: `Dump the message history of one channel (name, #name, or ID) or
every channel the token can see ("all") into files under --output, one file
per channel. Thread replies are included by default (disable with
--include-threads=false). With --include-files, message attachments are
downloaded into <output>/files/<channel>/. Use --since to bound the window
and --limit to cap messages per channel.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runExport(cmd, client, args[0])
		},
	}

	cmd.Flags().String("format", "json", "Export format: json, markdown, or csv")
	cmd.Flags().String("output", "slack-export", "Output directory")
	cmd.Flags().String("since", "", "Only messages after this time (2h, 3d, 2026-07-01, …)")
	cmd.Flags().Int("limit", 0, "Maximum messages per channel (0 = all)")
	cmd.Flags().Bool("include-threads", true, "Include thread replies")
	cmd.Flags().Bool("include-files", false, "Download message attachments")
	cmd.Flags().Bool("private", false, "With \"all\": include private channels")

	return cmd
}

func runExport(cmd *cobra.Command, client exportClient, target string) error {
	format, _ := cmd.Flags().GetString("format")
	outputDir, _ := cmd.Flags().GetString("output")
	since, _ := cmd.Flags().GetString("since")
	limit, _ := cmd.Flags().GetInt("limit")
	includeThreads, _ := cmd.Flags().GetBool("include-threads")
	includeFiles, _ := cmd.Flags().GetBool("include-files")
	private, _ := cmd.Flags().GetBool("private")

	if _, ok := exportFormats[format]; !ok {
		return formatAndExit(cmd, fmt.Errorf("invalid --format %q (valid: json, markdown, csv)", format), exitcode.InputError)
	}

	oldest := ""
	if since != "" {
		var err error
		oldest, err = parseTimeSpec(since, time.Now())
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
	}

	warnIfCacheNotReady(cmd)
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	// Build the channel work list: (id, name) pairs.
	type exportTarget struct{ id, name string }
	var targets []exportTarget
	if target == "all" {
		types := []string{"public_channel"}
		if private {
			types = append(types, "private_channel")
		}
		cursor := ""
		for {
			channels, next, err := client.GetConversationsContext(cmd.Context(), &slack.GetConversationsParameters{
				Types:           types,
				ExcludeArchived: true,
				Cursor:          cursor,
				Limit:           historyPageLimit,
			})
			if err != nil {
				if retried, rerr := sleepOnRateLimit(cmd.Context(), err); retried {
					continue
				} else if rerr != nil {
					return formatAndExit(cmd, rerr, exitcode.NetError)
				}
				return formatAndExit(cmd, err, exitcode.Classify(err))
			}
			for _, ch := range channels {
				targets = append(targets, exportTarget{ch.ID, ch.Name})
			}
			cursor = next
			if cursor == "" {
				break
			}
		}
	} else {
		id, err := resolveChannelArg(target)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		targets = append(targets, exportTarget{id, exportChannelName(target, id)})
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}

	exported := 0
	for i, t := range targets {
		fmt.Fprintf(cmd.ErrOrStderr(), "exporting %s (%d/%d)…\n", displayChannel(t.name, t.id), i+1, len(targets))

		msgs, err := fetchChannelMessages(cmd.Context(), client, t.id, oldest, limit, includeThreads)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		if len(msgs) == 0 && target == "all" {
			continue
		}

		rows := toSemMessages(msgs, "", idMap)
		path := filepath.Join(outputDir, exportFileName(t.name, t.id)+"."+exportFormats[format])
		if err := writeExportFile(path, format, displayChannel(t.name, t.id), rows); err != nil {
			return formatAndExit(cmd, err, exitcode.NetError)
		}

		if includeFiles {
			if err := exportAttachments(cmd, client, msgs, filepath.Join(outputDir, "files", exportFileName(t.name, t.id))); err != nil {
				return formatAndExit(cmd, err, exitcode.Classify(err))
			}
		}
		exported++
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "exported %d channel(s) to %s\n", exported, outputDir)
	return nil
}

// exportChannelName derives a channel name from the user's argument when it
// was a name, or "" when it was a raw ID.
func exportChannelName(target, id string) string {
	if target == id {
		return ""
	}
	return strings.TrimPrefix(target, "#")
}

// displayChannel shows "#name" when a name is known, else the ID.
func displayChannel(name, id string) string {
	if name != "" {
		return "#" + name
	}
	return id
}

// exportFileName picks a stable file base name for a channel.
func exportFileName(name, id string) string {
	if name != "" {
		return name
	}
	return id
}

// writeExportFile renders rows in the requested format to path.
func writeExportFile(path, format, channelDisplay string, rows []semMessage) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	switch format {
	case "json":
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	case "markdown":
		return writeExportMarkdown(f, channelDisplay, rows)
	case "csv":
		return writeExportCSV(f, rows)
	}
	return fmt.Errorf("unknown format %q", format)
}

func writeExportMarkdown(w io.Writer, channelDisplay string, rows []semMessage) error {
	if _, err := fmt.Fprintf(w, "# %s\n\n", channelDisplay); err != nil {
		return err
	}
	for _, r := range rows {
		prefix := ""
		if r.ThreadTs != "" && r.ThreadTs != r.Ts {
			prefix = "> " // thread replies as blockquotes
		}
		timeStr := r.Time
		if t, err := time.Parse(time.RFC3339, r.Time); err == nil {
			timeStr = t.Local().Format("2006-01-02 15:04")
		}
		if _, err := fmt.Fprintf(w, "%s**%s** (%s):\n%s%s\n\n", prefix, r.User, timeStr, prefix, r.Text); err != nil {
			return err
		}
	}
	return nil
}

func writeExportCSV(w io.Writer, rows []semMessage) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"user", "ts", "time", "thread_ts", "text"}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write([]string{r.User, r.Ts, r.Time, r.ThreadTs, r.Text}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// exportAttachments downloads every attachment in msgs into dir.
func exportAttachments(cmd *cobra.Command, client exportClient, msgs []slack.Message, dir string) error {
	made := false
	for _, m := range msgs {
		for _, file := range m.Files {
			if file.URLPrivateDownload == "" {
				continue
			}
			if !made {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				made = true
			}
			// Prefix with the file ID: names are user-controlled and can clash.
			name := file.ID + "-" + filepath.Base(file.Name)
			path := filepath.Join(dir, name)
			if err := downloadTo(cmd.Context(), client, file.URLPrivateDownload, path); err != nil {
				return fmt.Errorf("download %s: %w", file.Name, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  saved %s\n", path)
		}
	}
	return nil
}

// downloadTo streams downloadURL into a file at path, removing the partial
// file on failure.
func downloadTo(ctx context.Context, client interface {
	GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error
}, downloadURL, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := client.GetFileContext(ctx, downloadURL, f); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}
