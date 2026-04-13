package override

import (
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newCacheCmd builds the "cache" parent command with warm, info, and
// clear subcommands.
func newCacheCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local name-to-ID cache",
	}

	cmd.AddCommand(newCacheWarmCmd(client))
	cmd.AddCommand(newCacheInfoCmd())
	cmd.AddCommand(newCacheClearCmd())

	return cmd
}

func newCacheWarmCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "warm",
		Short: "Warm the cache by fetching all channels, users, and usergroups",
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd,
					fmt.Errorf("SLACK_TOKEN is not set"),
					exitcode.AuthError)
			}

			result, err := cache.Warm(cmd.Context(), client)
			if err != nil {
				code := exitcode.Classify(err)
				return formatAndExit(cmd, err, code)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cache warmed: %d channels, %d people, %d usergroups\n",
				result.Channels, result.Users, result.Usergroups)
			return nil
		},
	}

	// --force is accepted for ergonomics but warm always does a full refresh.
	cmd.Flags().Bool("force", false, "Warm even if cache is fresh (default behavior)")

	return cmd
}

func newCacheInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show cache path, version, and entry counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := cache.Dir()
			if err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Path: %s\n", dir)

			version, err := cache.MetaVersion()
			if err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}

			if version == 0 {
				hasLegacy, _ := cache.HasLegacyCache()
				if hasLegacy {
					fmt.Fprintln(w, "Version: 1 (legacy, needs migration)")
				} else {
					fmt.Fprintln(w, "Status: no cache (run 'slack-cli cache warm')")
				}
				return nil
			}

			fmt.Fprintf(w, "Version: %d\n", version)

			stale, _ := cache.IsStale()
			if stale {
				fmt.Fprintln(w, "Status: stale (needs warm)")
			} else {
				fmt.Fprintln(w, "Status: fresh")
			}

			lock, err := cache.AcquireShared()
			if err != nil {
				return nil
			}
			defer lock.Close()

			channels, err := cache.LoadEntity[cache.ChannelCache](cache.ChannelsFileName)
			if err == nil {
				fmt.Fprintf(w, "Channels: %d\n", len(channels))
			}

			people, err := cache.LoadEntity[cache.PeopleCache](cache.PeopleFileName)
			if err == nil {
				fmt.Fprintf(w, "People: %d\n", len(people))
			}

			groups, err := cache.LoadEntity[cache.UsergroupCache](cache.UsergroupsFileName)
			if err == nil {
				fmt.Fprintf(w, "Usergroups: %d\n", len(groups))
			}

			idToName, err := cache.LoadIDToNameMap()
			if err == nil {
				fmt.Fprintf(w, "ID mappings: %d\n", len(idToName))
			}

			return nil
		},
	}
}

func newCacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Delete all cache files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cache.Clear(); err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cache cleared")
			return nil
		},
	}
}
