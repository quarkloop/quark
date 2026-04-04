// Package activity provides CLI commands for managing the activity log.
// The activity log is an append-only JSONL stream in .quark/activity/.
package activity

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/core/pkg/space"
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

func spaceDir(cmd *cobra.Command) (string, error) {
	dir, err := space.FindRoot(".")
	if err != nil {
		return "", err
	}
	return dir, nil
}

func activityDir(spaceDir string) string {
	return space.ActivityDir(spaceDir)
}

func logPath(spaceDir string) string {
	return space.ActivityLogPath(spaceDir)
}

type activityRecord struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func newActivityAppendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "append <event-json>",
		Short: "Append an event to the activity log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := spaceDir(cmd)
			if err != nil {
				return err
			}
			aDir := activityDir(dir)
			if err := os.MkdirAll(aDir, 0755); err != nil {
				return err
			}
			path := logPath(dir)
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("open activity log: %w", err)
			}
			defer f.Close()

			var record activityRecord
			if err := json.Unmarshal([]byte(args[0]), &record); err != nil {
				return fmt.Errorf("invalid event JSON: %w", err)
			}
			if record.Timestamp.IsZero() {
				record.Timestamp = time.Now()
			}

			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			if _, err := f.Write(append(data, '\n')); err != nil {
				return fmt.Errorf("write activity: %w", err)
			}
			return nil
		},
	}
}

func newActivityQueryCmd() *cobra.Command {
	var eventType, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query activity log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := spaceDir(cmd)
			if err != nil {
				return err
			}
			path := logPath(dir)
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No activity.")
					return nil
				}
				return err
			}

			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			count := 0
			var sinceTime time.Time
			if since != "" {
				sinceTime, _ = time.Parse(time.RFC3339, since)
			}

			// Print newest first.
			for i := len(lines) - 1; i >= 0; i-- {
				line := strings.TrimSpace(lines[i])
				if line == "" {
					continue
				}
				var rec activityRecord
				if err := json.Unmarshal([]byte(line), &rec); err != nil {
					continue
				}
				if eventType != "" && rec.Type != eventType {
					continue
				}
				if !sinceTime.IsZero() && !rec.Timestamp.After(sinceTime) {
					continue
				}
				fmt.Printf("%s  %-30s  %s  %s\n", rec.Timestamp.Format(time.RFC3339), rec.Type, rec.SessionID, rec.Data)
				count++
				if limit > 0 && count >= limit {
					break
				}
			}
			if count == 0 {
				fmt.Println("No matching activity.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&eventType, "type", "", "Filter by event type")
	cmd.Flags().StringVar(&since, "since", "", "Only show events after this timestamp (RFC3339)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of entries")
	return cmd
}
