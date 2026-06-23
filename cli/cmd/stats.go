package cmd

import (
        "context"
        "fmt"
        "os"
        "time"

        "github.com/quarkloop/quark/cli/internal/model"
        "github.com/spf13/cobra"
)

var statsWatch bool

var statsCmd = &cobra.Command{
        Use:   "stats",
        Short: "Show real-time usage statistics for all namespaces",
        Long: `Show real-time usage statistics for all namespaces.

Displays CPU load, memory usage, node count, and health status for each
active namespace. Updates every 2 seconds in watch mode.

Examples:
  quarkctl get stats
  quarkctl get stats --watch
  quarkctl get stats --json`,
        Args: cobra.NoArgs,
        RunE: runStats,
}

func init() {
        getCmd.AddCommand(statsCmd)
        statsCmd.Flags().BoolVarP(&statsWatch, "watch", "w", false, "Watch mode")
}

func runStats(cmd *cobra.Command, args []string) error {
        if statsWatch {
                return runStatsWatch()
        }
        return runStatsOnce()
}

func runStatsOnce() error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()

        namespaces, err := c.ListNamespaces(ctx)
        if err != nil {
                return newPrinter().PrintError(err)
        }

        if flagJSON || flagOutput == "json" {
                var details []*model.NamespaceDetail
                for _, ns := range namespaces {
                        d, err := c.GetNamespace(ctx, ns.Namespace)
                        if err == nil {
                                details = append(details, d)
                        }
                }
                return newPrinter().PrintRaw(details)
        }

        printStatsHeader()
        for _, ns := range namespaces {
                detailCtx, detailCancel := context.WithTimeout(ctx, 5*time.Second)
                printStatsRow(c, ns.Namespace, detailCtx)
                detailCancel()
        }
        return nil
}

func runStatsWatch() error {
        c := newClient()
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        watchCtx := signalContext()

        for {
                listCtx, listCancel := context.WithTimeout(watchCtx, 5*time.Second)
                namespaces, err := c.ListNamespaces(listCtx)
                listCancel()

                if err != nil {
                        // Server unreachable — clear screen and show empty table
                        fmt.Print("\033[2J\033[H")
                        printStatsHeader()
                        fmt.Fprintln(os.Stdout, "(server unreachable)")
                } else {
                        // Server reachable — collect details and print
                        fmt.Print("\033[2J\033[H")
                        printStatsHeader()
                        if len(namespaces) == 0 {
                                fmt.Fprintln(os.Stdout, "(no active namespaces)")
                        }
                        for _, ns := range namespaces {
                                detailCtx, detailCancel := context.WithTimeout(watchCtx, 5*time.Second)
                                printStatsRow(c, ns.Namespace, detailCtx)
                                detailCancel()
                        }
                }

                select {
                case <-watchCtx.Done():
                        return nil
                case <-ticker.C:
                }
        }
}

func printStatsHeader() {
        fmt.Fprintf(os.Stdout, "%-15s  %5s  %5s  %8s  %8s  %8s  %10s  %10s\n",
                "NAMESPACE", "SYS", "NODES", "HEALTHY", "UNHEALTHY", "CPU%", "HEAP", "NON-HEAP")
}

func printStatsRow(c interface{ GetNamespace(context.Context, string) (*model.NamespaceDetail, error) }, namespace string, ctx context.Context) {
        detail, err := c.GetNamespace(ctx, namespace)
        if err != nil {
                // Namespace detail fetch failed — skip this row (namespace may have
                // been undeployed between the list call and the detail call)
                return
        }

        heapMB := float64(detail.Metrics.Memory.HeapUsed) / (1024 * 1024)
        nonHeapMB := float64(detail.Metrics.Memory.NonHeapUsed) / (1024 * 1024)
        cpuPct := detail.Metrics.CPU.SystemLoad * 100

        fmt.Fprintf(os.Stdout, "%-15s  %5d  %5d  %8d  %8d  %7.1f%%  %9.1fMB  %9.1fMB\n",
                detail.Namespace,
                detail.SystemCount,
                detail.NodeCount,
                detail.HealthyNodes,
                detail.UnhealthyNodes,
                cpuPct,
                heapMB,
                nonHeapMB,
        )
}
