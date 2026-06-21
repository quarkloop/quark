package cmd

import (
        "fmt"

        "github.com/spf13/cobra"
)

// nodeCmd groups all node subcommands.
var nodeCmd = &cobra.Command{
        Use:   "node",
        Short: "Manage individual nodes",
        Long:  `Inspect and operate on nodes within deployed systems. All commands require --namespace and --system.`,
}

var nodeSystem string

var nodeListCmd = &cobra.Command{
        Use:   "list",
        Short: "List nodes in a namespace (optionally within a specific system)",
        Args:  cobra.NoArgs,
        RunE:  runNodeList,
}

var nodeGetCmd = &cobra.Command{
        Use:   "get NAME",
        Short: "Get node details (state, health, connections, config)",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeGet,
}

var nodePauseCmd = &cobra.Command{
        Use:   "pause NAME",
        Short: "Pause a node (ACTIVE → PAUSED)",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeLifecycle("pause"),
}

var nodeResumeCmd = &cobra.Command{
        Use:   "resume NAME",
        Short: "Resume a paused node (PAUSED → ACTIVE)",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeLifecycle("resume"),
}

var nodeDrainCmd = &cobra.Command{
        Use:   "drain NAME",
        Short: "Drain a node (ACTIVE → DRAINING)",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeLifecycle("drain"),
}

var nodeArchiveCmd = &cobra.Command{
        Use:   "archive NAME",
        Short: "Archive a drained or errored node",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeLifecycle("archive"),
}

var nodeRecoverCmd = &cobra.Command{
        Use:   "recover NAME",
        Short: "Attempt to recover an errored node",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeLifecycle("recover"),
}

var nodeDeleteCmd = &cobra.Command{
        Use:   "delete NAME",
        Short: "Delete an archived node (ARCHIVED → DELETED)",
        Args:  cobra.ExactArgs(1),
        RunE:  runNodeLifecycle("delete"),
}

func init() {
        nodeCmd.AddCommand(
                nodeListCmd, nodeGetCmd,
                nodePauseCmd, nodeResumeCmd, nodeDrainCmd,
                nodeArchiveCmd, nodeRecoverCmd, nodeDeleteCmd,
        )
        rootCmd.AddCommand(nodeCmd)

        // --system / -s is required for single-node operations.
        for _, c := range []*cobra.Command{nodeGetCmd, nodePauseCmd, nodeResumeCmd,
                nodeDrainCmd, nodeArchiveCmd, nodeRecoverCmd, nodeDeleteCmd} {
                c.Flags().StringVarP(&nodeSystem, "system", "s", "", "System name (required)")
                _ = c.MarkFlagRequired("system")
        }
        // list also accepts --system but it's optional.
        nodeListCmd.Flags().StringVarP(&nodeSystem, "system", "s", "", "Filter to a specific system")
}

func runNodeList(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        list, err := c.ListNodes(ctx(), ns, nodeSystem)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintNodeList(list)
}

func runNodeGet(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        detail, err := c.GetNode(ctx(), args[0], ns, nodeSystem)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintNodeDetail(detail)
}

// runNodeLifecycle returns a RunE function for the given lifecycle action.
// Captures action in a closure so all 6 lifecycle commands share this helper.
func runNodeLifecycle(action string) func(*cobra.Command, []string) error {
        return func(cmd *cobra.Command, args []string) error {
                ns, err := requireNamespace()
                if err != nil {
                        return err
                }
                c := newClient()
                p := newPrinter()
                if err := c.NodeLifecycle(ctx(), action, args[0], ns, nodeSystem); err != nil {
                        return p.PrintError(err)
                }
                return p.PrintSuccess(fmt.Sprintf("Node %s/%s/%s %s.", ns, nodeSystem, args[0], action))
        }
}
