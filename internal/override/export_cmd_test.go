package override

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeExportClient wraps fakeFetcher with channel listing and file downloads.
type fakeExportClient struct {
	fakeFetcher
	channels   []slack.Channel
	downloaded []string
}

func (f *fakeExportClient) GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	return f.channels, "", nil
}

func (f *fakeExportClient) GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error {
	f.downloaded = append(f.downloaded, downloadURL)
	_, err := io.WriteString(writer, "file-bytes")
	return err
}

func execExport(t *testing.T, client exportClient, args ...string) (stderr string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "export", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runExport(cmd, client, a[0])
	}
	cmd.Flags().String("format", "json", "")
	cmd.Flags().String("output", "slack-export", "")
	cmd.Flags().String("since", "", "")
	cmd.Flags().Int("limit", 0, "")
	cmd.Flags().Bool("include-threads", true, "")
	cmd.Flags().Bool("include-files", false, "")
	cmd.Flags().Bool("private", false, "")

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return errBuf.String(), err
}

func TestExportSingleChannelJSON(t *testing.T) {
	seedResolverCache(t)
	dir := t.TempDir()
	f := &fakeExportClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("2.0", "second"), msg("1.0", "first")}},
	}}}

	_, err := execExport(t, f, "#general", "--output", dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "general.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"text": "first"`) {
		t.Errorf("export content = %s", data)
	}
	// Chronological order in the file.
	if strings.Index(string(data), "first") > strings.Index(string(data), "second") {
		t.Error("messages not chronological")
	}
}

func TestExportMarkdown(t *testing.T) {
	seedResolverCache(t)
	dir := t.TempDir()
	f := &fakeExportClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("1.0", "hello world")}},
	}}}

	_, err := execExport(t, f, "#general", "--output", dir, "--format", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "general.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# #general") || !strings.Contains(string(data), "hello world") {
		t.Errorf("markdown = %s", data)
	}
}

func TestExportCSV(t *testing.T) {
	seedResolverCache(t)
	dir := t.TempDir()
	f := &fakeExportClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("1.0", "a,b \"quoted\"")}},
	}}}

	_, err := execExport(t, f, "#general", "--output", dir, "--format", "csv")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "general.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "user,ts,time,thread_ts,text") {
		t.Errorf("csv header missing: %s", data)
	}
	if !strings.Contains(string(data), `"a,b ""quoted"""`) {
		t.Errorf("csv escaping wrong: %s", data)
	}
}

func TestExportAllEnumeratesChannels(t *testing.T) {
	seedResolverCache(t)
	dir := t.TempDir()
	ch1 := listChannel("general", false)
	ch2 := listChannel("dev", false)
	ch2.ID = "C0BBBBBBBBB"
	f := &fakeExportClient{
		channels: []slack.Channel{ch1, ch2},
		fakeFetcher: fakeFetcher{historyPages: []historyPage{
			{messages: []slack.Message{msg("1.0", "x")}},
			{messages: []slack.Message{msg("2.0", "y")}},
		}},
	}

	stderr, err := execExport(t, f, "all", "--output", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "exported 2 channel(s)") {
		t.Errorf("stderr = %q", stderr)
	}
	for _, name := range []string{"general.json", "dev.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestExportIncludeFiles(t *testing.T) {
	seedResolverCache(t)
	dir := t.TempDir()
	m := msg("1.0", "with attachment")
	m.Files = []slack.File{{ID: "F0AAAAAAAAA", Name: "report.pdf", URLPrivateDownload: "https://files.slack.com/x/report.pdf"}}
	f := &fakeExportClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{m}},
	}}}

	_, err := execExport(t, f, "#general", "--output", dir, "--include-files")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.downloaded) != 1 {
		t.Fatalf("downloaded %d files, want 1", len(f.downloaded))
	}
	saved := filepath.Join(dir, "files", "general", "F0AAAAAAAAA-report.pdf")
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "file-bytes" {
		t.Errorf("attachment content = %q", data)
	}
}

func TestExportInvalidFormat(t *testing.T) {
	seedResolverCache(t)
	f := &fakeExportClient{}
	if _, err := execExport(t, f, "#general", "--format", "xml"); err == nil {
		t.Error("expected input error for bad format")
	}
}
