package runtimecmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/quarkfile"
	"github.com/quarkloop/cli/pkg/util"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

// StopCLI returns the "stop" command.
func StopCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the agent for the current space via the supervisor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			name, err := quarkfile.NameFromDir(cwd)
			if err != nil {
				return err
			}
			sup := supclient.New()
			agent, err := sup.AgentBySpace(cmd.Context(), name)
			if err != nil {
				if supclient.IsNotFound(err) {
					return fmt.Errorf("no agent running for space %q", name)
				}
				return err
			}
			stopped, err := sup.StopAgent(cmd.Context(), agent.ID)
			if err != nil {
				return fmt.Errorf("stop agent: %w", err)
			}
			util.Successf("Agent stopped (space=%s, id=%s, status=%s)", stopped.Space, stopped.ID, stopped.Status)
			return nil
		},
	}
}
