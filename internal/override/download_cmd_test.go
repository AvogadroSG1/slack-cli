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

// fakeDownloadClient serves file info and content.
type fakeDownloadClient struct {
	file       *slack.File
	gotInfoIDs []string
	gotURLs    []string
}

func (f *fakeDownloadClient) GetFileInfoContext(ctx context.Context, fileID string, count, page int) (*slack.File, []slack.Comment, *slack.Paging, error) {
	f.gotInfoIDs = append(f.gotInfoIDs, fileID)
	return f.file, nil, nil, nil
}

func (f *fakeDownloadClient) GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error {
	f.gotURLs = append(f.gotURLs, downloadURL)
	_, err := io.WriteString(writer, "content")
	return err
}

func execDownload(t *testing.T, client downloadClient, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "download", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runDownload(cmd, client, a[0])
	}
	cmd.Flags().String("output", "", "")

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func testFile() *slack.File {
	return &slack.File{
		ID:                 "F0AAAAAAAAA",
		Name:               "report.pdf",
		URLPrivateDownload: "https://files.slack.com/x/report.pdf",
	}
}

func TestDownloadByID(t *testing.T) {
	dir := t.TempDir()
	f := &fakeDownloadClient{file: testFile()}

	out := filepath.Join(dir, "out.pdf")
	_, stderr, err := execDownload(t, f, "F0AAAAAAAAA", "--output", out)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.gotInfoIDs) != 1 || f.gotInfoIDs[0] != "F0AAAAAAAAA" {
		t.Errorf("info IDs = %v", f.gotInfoIDs)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q", data)
	}
	if !strings.Contains(stderr, "saved") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestDownloadByPermalink(t *testing.T) {
	dir := t.TempDir()
	f := &fakeDownloadClient{file: testFile()}

	out := filepath.Join(dir, "out.pdf")
	_, _, err := execDownload(t, f, "https://myteam.slack.com/files/U0AAAAAAAAA/F0AAAAAAAAA/report.pdf", "--output", out)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.gotInfoIDs) != 1 || f.gotInfoIDs[0] != "F0AAAAAAAAA" {
		t.Errorf("info IDs = %v", f.gotInfoIDs)
	}
}

func TestDownloadDirectURLSkipsInfo(t *testing.T) {
	dir := t.TempDir()
	f := &fakeDownloadClient{}

	out := filepath.Join(dir, "raw.bin")
	_, _, err := execDownload(t, f, "https://files.slack.com/files-pri/T01-F01/raw.bin", "--output", out)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.gotInfoIDs) != 0 {
		t.Errorf("files.info called for direct URL: %v", f.gotInfoIDs)
	}
	if len(f.gotURLs) != 1 || f.gotURLs[0] != "https://files.slack.com/files-pri/T01-F01/raw.bin" {
		t.Errorf("download URLs = %v", f.gotURLs)
	}
}

func TestDownloadToStdout(t *testing.T) {
	f := &fakeDownloadClient{file: testFile()}

	stdout, _, err := execDownload(t, f, "F0AAAAAAAAA", "--output", "-")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "content" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestDownloadBadTarget(t *testing.T) {
	f := &fakeDownloadClient{}
	for _, target := range []string{"not-a-file", "https://example.com/nope"} {
		t.Run(target, func(t *testing.T) {
			if _, _, err := execDownload(t, f, target); err == nil {
				t.Error("expected error")
			}
		})
	}
}
