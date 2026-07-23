package override

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fileIDRe matches Slack file IDs.
var fileIDRe = regexp.MustCompile(`^F[A-Z0-9]{8,}$`)

// downloadClient abstracts the file APIs download needs.
type downloadClient interface {
	GetFileInfoContext(ctx context.Context, fileID string, count, page int) (*slack.File, []slack.Comment, *slack.Paging, error)
	GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error
}

// newDownloadCmd builds the semantic "download" command: fetch a file with
// the token's auth headers.
func newDownloadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <file-id|link>",
		Short: "Download a Slack file or attachment",
		Long: `Download a file using the token for authentication. Accepts a
file ID (F…), a file permalink (https://…/files/<user>/<id>/<name>), or a
direct files.slack.com URL. The file is written to --output (default: the
file's own name in the current directory; use "-" for stdout).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runDownload(cmd, client, args[0])
		},
	}

	cmd.Flags().String("output", "", "Output path (\"-\" for stdout; default: the file's name)")

	return cmd
}

func runDownload(cmd *cobra.Command, client downloadClient, target string) error {
	output, _ := cmd.Flags().GetString("output")

	downloadURL, name, err := resolveDownloadTarget(cmd.Context(), client, target)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	if output == "-" {
		if err := client.GetFileContext(cmd.Context(), downloadURL, cmd.OutOrStdout()); err != nil {
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		return nil
	}

	path := output
	if path == "" {
		path = name
	}
	if err := downloadTo(cmd.Context(), client, downloadURL, path); err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "saved %s\n", path)
	return nil
}

// resolveDownloadTarget turns the argument into a download URL and a
// suggested file name. Accepted forms: a file ID, a file permalink
// containing /files/<user>/<id>/<name>, or a direct files.slack.com URL.
func resolveDownloadTarget(ctx context.Context, client downloadClient, target string) (downloadURL, name string, err error) {
	if fileIDRe.MatchString(target) {
		return fileInfoURL(ctx, client, target)
	}

	u, uerr := url.Parse(target)
	if uerr != nil || u.Scheme == "" {
		return "", "", fmt.Errorf("expected a file ID (F…) or a Slack file URL, got %q", target)
	}

	// Direct file-host URL: download as-is.
	if strings.HasPrefix(u.Host, "files.slack.com") {
		return target, filepath.Base(u.Path), nil
	}

	// Permalink: /files/<user>/<fileID>[/<name>]
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "files" && i+2 < len(parts) && fileIDRe.MatchString(parts[i+2]) {
			return fileInfoURL(ctx, client, parts[i+2])
		}
	}

	return "", "", fmt.Errorf("could not find a file ID in %q (expected a permalink like https://…/files/<user>/<F…>/<name>)", target)
}

// fileInfoURL looks up a file's download URL and name via files.info.
func fileInfoURL(ctx context.Context, client downloadClient, fileID string) (string, string, error) {
	file, _, _, err := client.GetFileInfoContext(ctx, fileID, 0, 0)
	if err != nil {
		return "", "", err
	}
	if file.URLPrivateDownload == "" {
		return "", "", fmt.Errorf("file %s has no downloadable content", fileID)
	}
	name := file.Name
	if name == "" {
		name = fileID
	}
	return file.URLPrivateDownload, filepath.Base(name), nil
}
