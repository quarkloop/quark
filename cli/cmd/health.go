package cmd

import (
        "github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
        Use:   "health",
        Short: "Platform, namespace, system, and node health",
        Long:  `Inspect health at various scopes. Platform-wide health is admin-only; the others require --namespace.`,
}

var healthPlatformCmd = &cobra.Command{
        Use:   "platform",
        Short: "Platform-wide health summary (ADMIN — aggregates across all namespaces)",
        Args:  cobra.NoArgs,
        RunE:  runHealthPlatform,
}

var healthNamespaceCmd = &cobra.Command{
        Use:   "namespace NAMESPACE",
        Short: "Per-namespace health summary",
        Args:  cobra.ExactArgs(1),
        RunE:  runHealthNamespace,
}

var healthSystemCmd = &cobra.Command{
        Use:   "system NAME",
        Short: "Per-system health breakdown (requires --namespace)",
        Args:  cobra.ExactArgs(1),
        RunE:  runHealthSystem,
}

var healthNodeCmd = &cobra.Command{
        Use:   "node NAME",
        Short: "Per-node health with recent events (requires --namespace and --system)",
        Args:  cobra.ExactArgs(1),
        RunE:  runHealthNode,
}

var healthNodeSystem string

func init() {
        healthCmd.AddCommand(healthPlatformCmd, healthNamespaceCmd, healthSystemCmd, healthNodeCmd)
        rootCmd.AddCommand(healthCmd)

        healthNodeCmd.Flags().StringVarP(&healthNodeSystem, "system", "s", "", "System name (required)")
        _ = healthNodeCmd.MarkFlagRequired("system")
}

func runHealthPlatform(cmd *cobra.Command, args []string) error {
        c := newClient()
        p := newPrinter()
        ctx, cancel := ctx()
        defer cancel()
        h, err := c.PlatformHealth(ctx)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintHealthSummary(h)
}

func runHealthNamespace(cmd *cobra.Command, args []string) error {
        c := newClient()
        p := newPrinter()
        ctx, cancel := ctx()
        defer cancel()
        h, err := c.NamespaceHealth(ctx, args[0])
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintHealthSummary(h)
}

func runHealthSystem(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        ctx, cancel := ctx()
        defer cancel()
        h, err := c.SystemHealth(ctx, args[0], ns)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintSystemHealth(h)
}

func runHealthNode(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        ctx, cancel := ctx()
        defer cancel()
        h, err := c.NodeHealth(ctx, args[0], ns, healthNodeSystem)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintNodeHealth(h)
}
