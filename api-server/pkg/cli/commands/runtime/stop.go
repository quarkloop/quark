package runtime

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/space"
	"github.com/quarkloop/agent/pkg/infra/term"
)

const (
	stopPollInterval = 500 * time.Millisecond
	stopTimeout      = 15 * time.Second
)

// StopCLI returns the "stop" command.
func StopCLI() *cobra.Command {
	var flags struct {
		force   bool
		timeout time.Duration
		noWait  bool
	}

	cmd := &cobra.Command{
		Use:   "stop <id>",
		Short: "Gracefully stop a running space",
		Long: `Send SIGINT to the space-runtime process and wait for it to exit.

If the space does not stop within --timeout, the command exits with an error
and suggests using --force (SIGKILL). Use --no-wait to return immediately
after sending the signal without polling.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			client := api.NewClientApi(apiServerURL())

			if err := client.StopSpace(cmd.Context(), id, flags.force); err != nil {
				return err
			}

			signal := "SIGINT"
			if flags.force {
				signal = "SIGKILL"
			}
			term.Infof("Signal %s sent to space %s", signal, id)

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

	cmd.Flags().BoolVarP(&flags.force, "force", "f", false,
		"Send SIGKILL instead of SIGINT (immediate kill)")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", stopTimeout,
		"Maximum time to wait for the space to stop")
	cmd.Flags().BoolVar(&flags.noWait, "no-wait", false,
		"Return immediately after sending the signal (do not poll for completion)")
	return cmd
}

// waitForStopped polls until the space reaches stopped or failed, or timeout.
func waitForStopped(cmd *cobra.Command, client *api.ClientApi, id string, timeout time.Duration, alreadyForced bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sp, err := client.GetSpace(cmd.Context(), id)
		if err != nil {
			return err
		}
		switch sp.Status {
		case space.StatusStopped:
			term.Successf("Space %s stopped", id)
			return nil
		case space.StatusFailed:
			term.Successf("Space %s exited (failed)", id)
			return nil
		}
		time.Sleep(stopPollInterval)
	}

	if !alreadyForced {
		return fmt.Errorf(
			"space %s did not stop within %s.\n"+
				"Use 'quark stop --force %s' to send SIGKILL.",
			id, timeout, id,
		)
	}
	return fmt.Errorf("space %s did not exit within %s after SIGKILL — process may be stuck", id, timeout)
}
