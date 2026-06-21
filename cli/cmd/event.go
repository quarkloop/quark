package cmd

import (
        "context"
        "fmt"
        "os"
        "time"

        "github.com/quarkloop/quark/cli/internal/client"
        "github.com/quarkloop/quark/cli/internal/model"
        "github.com/spf13/cobra"
)

var eventCmd = &cobra.Command{
        Use:   "event",
        Short: "Query the platform event log",
        Long:  `List, count, and watch events. All commands require --namespace (or --all for admin mode).`,
}

var (
        eventSystem   string
        eventNode   string
        eventKinds      string
        eventSince      string
        eventUntil      string
        eventLimit      int
        eventAll        bool
        eventWatch      bool
        eventWatchEvery time.Duration
)

var eventListCmd = &cobra.Command{
        Use:   "list",
        Short: "List events in a namespace (with optional filters)",
        Args:  cobra.NoArgs,
        RunE:  runEventList,
}

var eventCountCmd = &cobra.Command{
        Use:   "count",
        Short: "Count events matching the given filters",
        Args:  cobra.NoArgs,
        RunE:  runEventCount,
}

var eventWatchCmd = &cobra.Command{
        Use:   "watch",
        Short: "Watch events in real time (polls every 2s by default)",
        Args:  cobra.NoArgs,
        RunE:  runEventWatch,
}

func init() {
        eventCmd.AddCommand(eventListCmd, eventCountCmd, eventWatchCmd)
        rootCmd.AddCommand(eventCmd)

        // Common filters for list/count.
        for _, c := range []*cobra.Command{eventListCmd, eventCountCmd} {
                c.Flags().StringVarP(&eventSystem, "system", "s", "", "Filter by system name")
                c.Flags().StringVarP(&eventNode, "node", "r", "", "Filter by node name")
                c.Flags().StringVar(&eventKinds, "kinds", "", "Comma-separated event kinds (e.g. NODE_STATE_CHANGED,NODE_EXECUTION_FAILED)")
                c.Flags().StringVar(&eventSince, "since", "", "Only events since this ISO-8601 timestamp")
                c.Flags().StringVar(&eventUntil, "until", "", "Only events until this ISO-8601 timestamp")
                c.Flags().IntVar(&eventLimit, "limit", 100, "Max events to return")
                c.Flags().BoolVar(&eventAll, "all", false, "Admin mode: query across ALL namespaces")
        }

        eventWatchCmd.Flags().StringVarP(&eventSystem, "system", "s", "", "Filter by system name")
        eventWatchCmd.Flags().StringVarP(&eventNode, "node", "r", "", "Filter by node name")
        eventWatchCmd.Flags().StringVar(&eventKinds, "kinds", "", "Comma-separated event kinds")
        eventWatchCmd.Flags().BoolVar(&eventAll, "all", false, "Admin mode: query across ALL namespaces")
        eventWatchCmd.Flags().DurationVar(&eventWatchEvery, "every", 2*time.Second, "Poll interval")
}

func eventOpts() client.EventListOptions {
        return client.EventListOptions{
                Namespace:     resolveNamespace(),
                System:      eventSystem,
                Node:      eventNode,
                Kinds:         eventKinds,
                Since:         eventSince,
                Until:         eventUntil,
                Limit:         eventLimit,
                AllNamespaces: eventAll,
        }
}

func runEventList(cmd *cobra.Command, args []string) error {
        opts := eventOpts()
        if !opts.AllNamespaces && opts.Namespace == "" {
                return fmt.Errorf("namespace is required (use --namespace / -n, or --all for admin mode)")
        }
        c := newClient()
        p := newPrinter()
        ctx, cancel := ctx()
        defer cancel()
        events, err := c.ListEvents(ctx, opts)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintEventList(events)
}

func runEventCount(cmd *cobra.Command, args []string) error {
        opts := eventOpts()
        if !opts.AllNamespaces && opts.Namespace == "" {
                return fmt.Errorf("namespace is required (use --namespace / -n, or --all for admin mode)")
        }
        c := newClient()
        p := newPrinter()
        ctx, cancel := ctx()
        defer cancel()
        count, err := c.CountEvents(ctx, opts)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintRaw(count)
}

func runEventWatch(cmd *cobra.Command, args []string) error {
        opts := eventOpts()
        if !opts.AllNamespaces && opts.Namespace == "" {
                return fmt.Errorf("namespace is required (use --namespace / -n, or --all for admin mode)")
        }
        c := newClient()
        p := newPrinter()
        opts.Limit = 1000 // fetch up to 1000 per poll; we filter client-side
        watchCtx := signalContext()
        ticker := time.NewTicker(eventWatchEvery)
        defer ticker.Stop()
        fmt.Fprintln(stdout(), "Watching events (Ctrl+C to stop)...")

        // Track the last-seen event ID so we only print NEW events on each
        // poll. Without this, every poll reprints the most recent event.
        var lastSeenID string
        consecutiveErrors := 0

        for {
                select {
                case <-watchCtx.Done():
                        return nil
                case <-ticker.C:
                        listCtx, cancel := context.WithTimeout(watchCtx, flagTimeout)
                        events, err := c.ListEvents(listCtx, opts)
                        cancel()
                        if err != nil {
                                consecutiveErrors++
                                if consecutiveErrors == 5 {
                                        fmt.Fprintf(os.Stderr, "  (5 consecutive errors — last: %v)\n", err)
                                }
                                continue
                        }
                        consecutiveErrors = 0

                        // Filter to events newer than lastSeenID.
                        // Event IDs are UUIDs generated in timestamp order by the
                        // server, so string comparison is a reasonable proxy for
                        // ordering. For a robust solution the server would expose
                        // a sequence number, but UUIDs work in practice.
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
