package cmd

import (
        "fmt"

        "github.com/spf13/cobra"
)

var lifecycleSystemFlag string

var pauseCmd = &cobra.Command{
        Use:   "pause node NAME",
        Short: "Pause a node (ACTIVE to PAUSED)",
        Args:  cobra.ExactArgs(2),
        RunE:  runLifecycle("pause"),
}

var resumeCmd = &cobra.Command{
        Use:   "resume node NAME",
        Short: "Resume a paused node (PAUSED to ACTIVE)",
        Args:  cobra.ExactArgs(2),
        RunE:  runLifecycle("resume"),
}

var drainCmd = &cobra.Command{
        Use:   "drain node NAME",
        Short: "Drain a node (ACTIVE to DRAINING)",
        Args:  cobra.ExactArgs(2),
        RunE:  runLifecycle("drain"),
}

var archiveCmd = &cobra.Command{
        Use:   "archive node NAME",
        Short: "Archive a drained or errored node",
        Args:  cobra.ExactArgs(2),
        RunE:  runLifecycle("archive"),
}

var recoverCmd = &cobra.Command{
        Use:   "recover node NAME",
        Short: "Attempt to recover an errored node",
        Args:  cobra.ExactArgs(2),
        RunE:  runLifecycle("recover"),
}

func init() {
        for _, c := range []*cobra.Command{pauseCmd, resumeCmd, drainCmd, archiveCmd, recoverCmd} {
                c.Flags().StringVarP(&lifecycleSystemFlag, "system", "s", "", "System name (required)")
                _ = c.MarkFlagRequired("system")
                rootCmd.AddCommand(c)
        }
}

func runLifecycle(action string) func(*cobra.Command, []string) error {
        return func(cmd *cobra.Command, args []string) error {
                ns, err := requireNamespace()
                if err != nil {
                        return err
                }
                // args[0] is "node", args[1] is the node name
                nodeName := args[1]
                c := newClient()
                ctx, cancel := ctx()
                defer cancel()
                if err := c.NodeLifecycle(ctx, action, nodeName, ns, lifecycleSystemFlag); err != nil {
                        return newPrinter().PrintError(err)
                }
                fmt.Printf("✓ Node %s/%s/%s %s.\n", ns, lifecycleSystemFlag, nodeName, action)
                return nil
        }
}
