package cmd

import (
        "fmt"

        "github.com/spf13/cobra"
)

var describeSystemFlag string

var describeCmd = &cobra.Command{
        Use:   "describe",
        Short: "Show details of a specific resource or group of resources",
        Long: `Show details of a specific resource.

Examples:
  # Describe a system
  quarkctl describe system monitor -n alice

  # Describe a node
  quarkctl describe node cpu -n alice -s monitor`,
        Args: cobra.NoArgs,
}

var describeSystemCmd = &cobra.Command{
        Use:   "system NAME",
        Short: "Describe a system",
        Args:  cobra.ExactArgs(1),
        RunE:  runDescribeSystem,
}

var describeNodeCmd = &cobra.Command{
        Use:   "node NAME",
        Short: "Describe a node",
        Args:  cobra.ExactArgs(1),
        RunE:  runDescribeNode,
}

func init() {
        rootCmd.AddCommand(describeCmd)
        describeCmd.AddCommand(describeSystemCmd)
        describeCmd.AddCommand(describeNodeCmd)
        describeNodeCmd.Flags().StringVarP(&describeSystemFlag, "system", "s", "", "System name (required)")
        _ = describeNodeCmd.MarkFlagRequired("system")
}

func runDescribeSystem(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        detail, err := c.GetSystem(ctx, args[0], ns)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintSystemDetail(detail)
}

func runDescribeNode(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        detail, err := c.GetNode(ctx, args[0], ns, describeSystemFlag)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintNodeDetail(detail)
}

// Ensure fmt is used
var _ = fmt.Sprintf
