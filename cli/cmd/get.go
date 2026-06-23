package cmd

import (
        "github.com/spf13/cobra"
)

// getCmd is the main "get" command (for listing resources).
// Subcommands: namespaces, systems, system, nodes, node, events, registry
var getCmd = &cobra.Command{
        Use:   "get",
        Short: "Display one or many resources",
        Long: `Display one or many resources.

Examples:
  # List all namespaces
  quarkctl get namespaces

  # List all systems in a namespace
  quarkctl get systems -n alice

  # Get details of a specific system
  quarkctl get system monitor -n alice

  # List all nodes in a system
  quarkctl get nodes -n alice -s monitor

  # Get details of a specific node
  quarkctl get node cpu -n alice -s monitor

  # List events in a namespace
  quarkctl get events -n alice --limit 10

  # List registry entries
  quarkctl get registry`,
        Args: cobra.NoArgs,
}

var getSystemFlag string

func init() {
        rootCmd.AddCommand(getCmd)
        getCmd.AddCommand(getNamespacesCmd)
        getCmd.AddCommand(getNamespaceCmd)
        getCmd.AddCommand(getSystemsCmd)
        getCmd.AddCommand(getSystemCmd)
        getCmd.AddCommand(getNodesCmd)
        getCmd.AddCommand(getNodeCmd)
        getCmd.AddCommand(getEventsCmd)
        getCmd.AddCommand(getRegistryCmd)

        // -s/--system flag for commands that need it
        getNodesCmd.Flags().StringVarP(&getSystemFlag, "system", "s", "", "System name")
        getNodeCmd.Flags().StringVarP(&getSystemFlag, "system", "s", "", "System name (required)")
        _ = getNodeCmd.MarkFlagRequired("system")
        getEventsCmd.Flags().StringVarP(&getSystemFlag, "system", "s", "", "Filter by system name")
        getEventsCmd.Flags().Int("limit", 100, "Maximum number of events to return")
}

// get namespaces
var getNamespacesCmd = &cobra.Command{
        Use:   "namespaces",
        Short: "List all active namespaces",
        Args:  cobra.NoArgs,
        RunE:  runGetNamespaces,
}

// get namespace NAME
var getNamespaceCmd = &cobra.Command{
        Use:   "namespace NAME",
        Short: "Get namespace details and metrics",
        Args:  cobra.ExactArgs(1),
        RunE:  runGetNamespace,
}

// get systems
var getSystemsCmd = &cobra.Command{
        Use:   "systems",
        Short: "List all systems in a namespace",
        Args:  cobra.NoArgs,
        RunE:  runGetSystems,
}

// get system NAME
var getSystemCmd = &cobra.Command{
        Use:   "system NAME",
        Short: "Get details of a specific system",
        Args:  cobra.ExactArgs(1),
        RunE:  runGetSystem,
}

// get nodes
var getNodesCmd = &cobra.Command{
        Use:   "nodes",
        Short: "List all nodes in a system",
        Args:  cobra.NoArgs,
        RunE:  runGetNodes,
}

// get node NAME
var getNodeCmd = &cobra.Command{
        Use:   "node NAME",
        Short: "Get details of a specific node",
        Args:  cobra.ExactArgs(1),
        RunE:  runGetNode,
}

// get events
var getEventsCmd = &cobra.Command{
        Use:   "events",
        Short: "List events in a namespace",
        Args:  cobra.NoArgs,
        RunE:  runGetEvents,
}

// get registry
var getRegistryCmd = &cobra.Command{
        Use:   "registry",
        Short: "List all registered node implementations",
        Args:  cobra.NoArgs,
        RunE:  runGetRegistry,
}

func runGetNamespaces(cmd *cobra.Command, args []string) error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        list, err := c.ListNamespaces(ctx)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintNamespaceList(list)
}

func runGetNamespace(cmd *cobra.Command, args []string) error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        detail, err := c.GetNamespace(ctx, args[0])
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintNamespaceDetail(detail)
}

func runGetSystems(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        list, err := c.ListSystems(ctx, ns)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintSystemList(list)
}

func runGetSystem(cmd *cobra.Command, args []string) error {
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

func runGetNodes(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        list, err := c.ListNodes(ctx, ns, getSystemFlag)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintNodeList(list)
}

func runGetNode(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        detail, err := c.GetNode(ctx, args[0], ns, getSystemFlag)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintNodeDetail(detail)
}

func runGetEvents(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        limit, _ := cmd.Flags().GetInt("limit")
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        events, err := c.ListEvents(ctx, ns, getSystemFlag, "", "", "", "", limit, false)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintEventList(events)
}

func runGetRegistry(cmd *cobra.Command, args []string) error {
        c := newClient()
        ctx, cancel := ctx()
        defer cancel()
        list, err := c.ListRegistry(ctx, "", "")
        if err != nil {
                return newPrinter().PrintError(err)
        }
        return newPrinter().PrintRegistryList(list)
}
