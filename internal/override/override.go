// Package override provides a simple map registry for hand-written Cobra
// commands that replace the auto-generated command for a given Slack API method.
package override

import "github.com/spf13/cobra"

// Overrides maps a Slack API method name (e.g. "chat.postMessage") to a
// hand-written Cobra command that should be used instead of the generated one.
var Overrides = map[string]*cobra.Command{}

// Register associates a hand-written Cobra command with the given Slack API
// method name.  When BuildCommands encounters this method it will use cmd
// instead of generating a generic command from the method definition.
func Register(apiMethod string, cmd *cobra.Command) {
	Overrides[apiMethod] = cmd
}
