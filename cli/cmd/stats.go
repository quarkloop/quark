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

Displays per-namespace CPU attribution, message throughput, error rate,
memory usage, node count, and health status. Updates every 2 seconds in
watch mode.

For shared namespaces (running in the same JVM), CPU % reflects the CPU
time consumed by message handlers for that namespace, measured via
ThreadMXBean.getCurrentThreadCpuTime(). For isolated namespaces (running
in a dedicated data-plane process), CPU % is exact at the process level.

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
			fmt.Print("\033[2J\033[H")
			printStatsHeader()
			fmt.Fprintln(os.Stdout, "(server unreachable)")
		} else {
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
	fmt.Fprintf(os.Stdout, "%-12s %4s %5s %7s %9s %9s %6s %6s %8s %9s\n",
		"NAMESPACE", "SYS", "NODES", "CPU%", "MSG/S", "ERR/S", "HEAP", "NONH", "OK", "BAD")
}

func printStatsRow(c interface{ GetNamespace(context.Context, string) (*model.NamespaceDetail, error) }, namespace string, ctx context.Context) {
	detail, err := c.GetNamespace(ctx, namespace)
	if err != nil {
		return
	}

	cpuPct := detail.Metrics.CPU.NamespacePercent
	msgPerSec := detail.Metrics.Throughput.MessagesReceivedPerSec
	errPerSec := detail.Metrics.Throughput.ErrorsPerSec
	heapMB := float64(detail.Metrics.Memory.HeapUsed) / (1024 * 1024)
	nonHeapMB := float64(detail.Metrics.Memory.NonHeapUsed) / (1024 * 1024)

	fmt.Fprintf(os.Stdout, "%-12s %4d %5d %6.1f%% %8.1f %8.2f %5.0fMB %6.0fMB %8d %7d\n",
		detail.Namespace,
		detail.SystemCount,
		detail.NodeCount,
		cpuPct,
		msgPerSec,
		errPerSec,
		heapMB,
		nonHeapMB,
		detail.HealthyNodes,
		detail.UnhealthyNodes,
	)
}
