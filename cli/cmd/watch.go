package cmd

import (
        "context"
        "fmt"
        "os"
        "time"

        "github.com/quarkloop/quark/cli/internal/model"
        "github.com/spf13/cobra"
)

var watchSystemFlag string

var watchCmd = &cobra.Command{
        Use:   "watch",
        Short: "Watch for changes to a resource",
        Long: `Watch for changes to a resource.

Examples:
  # Watch events in a namespace
  quarkctl watch events -n alice

  # Watch events for a specific system
  quarkctl watch events -n alice -s monitor`,
        Args: cobra.NoArgs,
}

var watchEventsCmd = &cobra.Command{
        Use:   "events",
        Short: "Watch events in real time",
        Args:  cobra.NoArgs,
        RunE:  runWatchEvents,
}

func init() {
        rootCmd.AddCommand(watchCmd)
        watchCmd.AddCommand(watchEventsCmd)
        watchEventsCmd.Flags().StringVarP(&watchSystemFlag, "system", "s", "", "Filter by system name")
}

func runWatchEvents(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        opts := EventListOptions{
                Namespace: ns,
                System:    watchSystemFlag,
                Limit:     1000,
        }
        watchCtx := signalContext()
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        fmt.Fprintln(os.Stdout, "Watching events (Ctrl+C to stop)...")

        var lastSeenID string
        consecutiveErrors := 0

        for {
                select {
                case <-watchCtx.Done():
                        return nil
                case <-ticker.C:
                        listCtx, cancel := context.WithTimeout(watchCtx, flagTimeout)
                        events, err := c.ListEvents(listCtx, opts.Namespace, opts.System, "", "", "", "", opts.Limit, false)
                        cancel()
                        if err != nil {
                                consecutiveErrors++
                                if consecutiveErrors == 5 {
                                        fmt.Fprintf(os.Stderr, "  (5 consecutive errors — last: %v)\n", err)
                                }
                                continue
                        }
                        consecutiveErrors = 0
                        var fresh []model.Event
                        for _, e := range events {
                                if e.ID > lastSeenID {
                                        fresh = append(fresh, e)
                                }
                        }
                        if len(fresh) > 0 {
                                _ = p.PrintEventList(fresh)
                                lastSeenID = fresh[len(fresh)-1].ID
                        }
                }
        }
}
