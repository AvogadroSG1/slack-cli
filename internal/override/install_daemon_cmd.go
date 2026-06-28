package override

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/spf13/cobra"
)

// defaultDaemonLabel is the reverse-DNS launchd Label (and plist filename stem)
// used when --label is not supplied.
const defaultDaemonLabel = "com.slack-cli.prime"

// plistData is the template input for the launchd agent plist. It is rendered
// by renderPlist and contains only values safe to interpolate into XML.
type plistData struct {
	Label      string
	ExecPath   string
	StdoutPath string
	StderrPath string
	RunAtLoad  bool
}

// plistTemplate is the launchd agent definition. It runs the cache-priming
// command through zsh so that ~/.zshenv (where the user exports SLACK_TOKEN) is
// sourced — launchd jobs do not inherit an interactive shell environment.
// StartCalendarInterval with Minute=0 fires at the top of every hour.
const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ .Label }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>/bin/zsh</string>
        <string>-lc</string>
        <string>{{ .ExecPath }} cache warm</string>
    </array>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Minute</key>
        <integer>0</integer>
    </dict>
    <key>RunAtLoad</key>
    <{{ if .RunAtLoad }}true{{ else }}false{{ end }}/>
    <key>StandardOutPath</key>
    <string>{{ .StdoutPath }}</string>
    <key>StandardErrorPath</key>
    <string>{{ .StderrPath }}</string>
</dict>
</plist>
`

// renderPlist renders the launchd plist XML for d. It is pure and deterministic.
// Interpolated values are XML-escaped; ExecPath is additionally single-quoted so
// that paths containing spaces survive the `zsh -lc` shell word-splitting.
func renderPlist(d plistData) (string, error) {
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return "", err
	}

	// Single-quote the executable path for the shell, escaping any embedded
	// single quotes via the standard '\'' idiom, then XML-escape the result.
	quoted := "'" + strings.ReplaceAll(d.ExecPath, "'", `'\''`) + "'"

	view := plistData{
		Label:      html.EscapeString(d.Label),
		ExecPath:   html.EscapeString(quoted),
		StdoutPath: html.EscapeString(d.StdoutPath),
		StderrPath: html.EscapeString(d.StderrPath),
		RunAtLoad:  d.RunAtLoad,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// plistPath returns the install path ~/Library/LaunchAgents/<label>.plist.
func plistPath(label string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// logPaths returns the daemon's stdout/stderr log paths under
// ~/Library/Logs/slack-cli so that a failing hourly job is visible.
func logPaths() (out, errPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, "Library", "Logs", "slack-cli")
	return filepath.Join(dir, "prime.out.log"), filepath.Join(dir, "prime.err.log"), nil
}

// loadLaunchd makes the agent active immediately. It unloads any previously
// loaded job for the same plist (ignoring errors, since it may not be loaded)
// then loads it with -w so a prior `disable` is overridden. Re-installs are
// therefore idempotent.
func loadLaunchd(ctx context.Context, path string) error {
	// Best-effort unload; a not-loaded job is not an error we care about.
	_ = exec.CommandContext(ctx, "launchctl", "unload", path).Run()

	cmd := exec.CommandContext(ctx, "launchctl", "load", "-w", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" {
			return fmt.Errorf("launchctl load: %w: %s", err, trimmed)
		}
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

// newInstallDaemonCmd builds the "install-daemon" command, which installs a
// macOS launchd agent that runs `slack-cli cache warm` at the top of every hour.
func newInstallDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-daemon",
		Short: "Install a macOS launchd agent that warms the cache hourly",
		Long: "install-daemon writes a launchd agent to ~/Library/LaunchAgents that runs " +
			"\"slack-cli cache warm\" at the top of every hour. The job runs through zsh so " +
			"that ~/.zshenv provides SLACK_TOKEN. macOS only.",
		RunE: runInstallDaemon,
	}

	cmd.Flags().String("label", defaultDaemonLabel, "launchd Label and plist filename stem")
	cmd.Flags().Bool("force", false, "Overwrite an existing plist if present")
	cmd.Flags().Bool("no-load", false, "Write the plist but do not launchctl load it")
	cmd.Flags().Bool("run-at-load", false, "Also run cache warm immediately when the agent loads")

	return cmd
}

func runInstallDaemon(cmd *cobra.Command, _ []string) error {
	if runtime.GOOS != "darwin" {
		return formatAndExit(cmd,
			fmt.Errorf("install-daemon is only supported on macOS (launchd); detected GOOS=%s", runtime.GOOS),
			exitcode.InputError)
	}

	label, _ := cmd.Flags().GetString("label")
	force, _ := cmd.Flags().GetBool("force")
	noLoad, _ := cmd.Flags().GetBool("no-load")
	runAtLoad, _ := cmd.Flags().GetBool("run-at-load")

	path, err := plistPath(label)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}

	if _, statErr := os.Stat(path); statErr == nil && !force {
		return formatAndExit(cmd,
			fmt.Errorf("plist already exists at %s; use --force to overwrite", path),
			exitcode.InputError)
	}

	exe, err := os.Executable()
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	if resolved, rErr := filepath.EvalSymlinks(exe); rErr == nil {
		exe = resolved
	}

	outLog, errLog, err := logPaths()
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	if err := os.MkdirAll(filepath.Dir(outLog), 0o755); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}

	xml, err := renderPlist(plistData{
		Label:      label,
		ExecPath:   exe,
		StdoutPath: outLog,
		StderrPath: errLog,
		RunAtLoad:  runAtLoad,
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Installed launchd agent %q\n", label)
	fmt.Fprintf(w, "  plist: %s\n", path)
	fmt.Fprintf(w, "  logs:  %s\n         %s\n", outLog, errLog)
	fmt.Fprintln(w, "  runs \"cache warm\" at :00 every hour")

	if noLoad {
		fmt.Fprintf(w, "Not loaded (--no-load). Load it with: launchctl load -w %s\n", path)
		return nil
	}

	if err := loadLaunchd(cmd.Context(), path); err != nil {
		return formatAndExit(cmd,
			fmt.Errorf("plist written to %s but loading failed (load manually with \"launchctl load -w %s\"): %w", path, path, err),
			exitcode.NetError)
	}
	fmt.Fprintln(w, "Loaded and active.")
	return nil
}
