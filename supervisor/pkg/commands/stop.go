package commands

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

// StopCmd creates the "supervisor stop" command.
func StopCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running supervisor server",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("http://127.0.0.1:%d/v1/health", port)
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("supervisor does not appear to be running on port %d: %w", port, err)
			}
			defer resp.Body.Close()
			fmt.Printf("supervisor on port %d is running; send SIGTERM to its process to stop it\n", port)
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 7200, "Supervisor port")

	return cmd
}
