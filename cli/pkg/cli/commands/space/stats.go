package space

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/cli/pkg/cli/config"
)

// spaceStatsCmd shows runtime stats for the agent attached to a running space.
func spaceStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats <id>",
		Short: "Show runtime stats for a running agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stats, err := api.NewClientApi(config.APIServerURL()).GetAgentStats(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
