// Package activitycmd provides CLI commands for managing the activity log.
package activitycmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/cli/pkg/activity"
	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/resolve"
)

func NewActivityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Manage the activity log",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
	}
	cmd.AddCommand(newActivityAppendCmd())
	cmd.AddCommand(newActivityQueryCmd())
	return cmd
}

func resolveClient(cmd *cobra.Command) (*activity.Client, error) {
	if url := resolve.AgentURL(cmd); url != "" {
		return activity.NewHTTP(url), nil
	}
	dir, err := resolve.SpaceDir()
	if err != nil {
		return nil, err
	}
	return activity.NewLocal(dir), nil
}

func newActivityAppendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "append <event-json>",
		Short: "Append an event to the activity log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()

			var record agentapi.ActivityRecord
			if err := json.Unmarshal([]byte(args[0]), &record); err != nil {
				return fmt.Errorf("invalid event JSON: %w", err)
			}
			if record.Timestamp.IsZero() {
				record.Timestamp = time.Now()
			}
			return c.Append(cmd.Context(), record)
		},
	}
}

func newActivityQueryCmd() *cobra.Command {
	var eventType, since string
	var limit int
	var follow bool
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query activity log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()

			if follow {
				fmt.Println("Streaming live activity... (Ctrl+C to stop)")
				return c.Stream(cmd.Context(), func(record agentapi.ActivityRecord) {
					if eventType != "" && record.Type != eventType {
						return
					}
					fmt.Printf("%s  %-30s  %s  %s\n", record.Timestamp, record.Type, record.SessionID, record.Data)
				})
			}

			records, err := c.Query(cmd.Context(), activity.QueryOptions{
				Type:  eventType,
				Since: since,
				Limit: limit,
			})
			if err != nil {
				return fmt.Errorf("query activity: %w", err)
			}
			if len(records) == 0 {
				fmt.Println("No activity.")
				return nil
			}
			for _, rec := range records {
				fmt.Printf("%s  %-30s  %s  %s\n",
					rec.Timestamp.Format(time.RFC3339), rec.Type, rec.SessionID, rec.Data)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&eventType, "type", "", "Filter by event type")
	cmd.Flags().StringVar(&since, "since", "", "Only show events after this timestamp (RFC3339)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of entries")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream live activity (requires --agent-url)")
	return cmd
}
