package runtime

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/agent/pkg/infra/term"
	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/space"
)

const (
	stopPollInterval = 500 * time.Millisecond
	stopTimeout      = 15 * time.Second
)

// StopCLI returns the "stop" command.
func StopCLI() *cobra.Command {
	var flags struct {
		agentURL string
		force    bool
		timeout  time.Duration
		noWait   bool
	}

	cmd := &cobra.Command{
		Use:   "stop [id]",
		Short: "Gracefully stop a running agent or direct agent URL",
		Long: `Request a graceful stop from the running agent and wait for it to exit.

If the agent does not stop within --timeout, the command exits with an error
and suggests using --force (SIGKILL via the space controller). Use --no-wait
to return immediately after sending the stop request without polling.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.agentURL != "" {
				return cobra.MaximumNArgs(0)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.agentURL != "" {
				if err := agentclient.New(flags.agentURL).Stop(cmd.Context()); err != nil {
					return err
				}
				term.Successf("Direct agent stop requested")
				return nil
			}

			id := args[0]
			client := api.NewClientApi(apiServerURL())

			if flags.force {
				if err := client.StopSpace(cmd.Context(), id, true); err != nil {
					return err
				}
			} else {
				if err := client.Agent(id).Stop(cmd.Context()); err != nil {
					return err
				}
			}

			signal := "stop request"
			if flags.force {
				signal = "SIGKILL"
			}
			term.Infof("%s sent to agent %s", signal, id)

			if flags.noWait {
				return nil
			}

			timeout := flags.timeout
			if timeout <= 0 {
				timeout = stopTimeout
			}

			return waitForStopped(cmd, client, id, timeout, flags.force)
		},
	}

	cmd.Flags().StringVar(&flags.agentURL, "agent-url", "", "Direct agent base URL")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", false,
		"Send SIGKILL instead of SIGINT (immediate kill)")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", stopTimeout,
		"Maximum time to wait for the agent to stop")
	cmd.Flags().BoolVar(&flags.noWait, "no-wait", false,
		"Return immediately after sending the stop request (do not poll for completion)")
	return cmd
}

// waitForStopped polls until the agent process reaches stopped or failed, or timeout.
func waitForStopped(cmd *cobra.Command, client *api.ClientApi, id string, timeout time.Duration, alreadyForced bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sp, err := client.GetSpace(cmd.Context(), id)
		if err != nil {
			return err
		}
		switch sp.Status {
		case space.StatusStopped:
			term.Successf("Agent %s stopped", id)
			return nil
		case space.StatusFailed:
			term.Successf("Agent %s exited (failed)", id)
			return nil
		}
		time.Sleep(stopPollInterval)
	}

	if !alreadyForced {
		return fmt.Errorf(
			"agent %s did not stop within %s.\n"+
				"Use 'quark stop --force %s' to send SIGKILL.",
			id, timeout, id,
		)
	}
	return fmt.Errorf("agent %s did not exit within %s after SIGKILL — process may be stuck", id, timeout)
}
