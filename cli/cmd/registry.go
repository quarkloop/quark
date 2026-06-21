package cmd

import (
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Browse available node implementations",
	Long:  `List and inspect registered node providers (sources, functions, stores, endpoints, policies). The registry is platform-global — no namespace required.`,
}

var (
	registryCategory string
	registryQuery    string
)

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered implementations (optionally filtered by --category or --query)",
	Args:  cobra.NoArgs,
	RunE:  runRegistryList,
}

var registryGetCmd = &cobra.Command{
	Use:   "get URI",
	Short: "Look up a specific implementation by URI (e.g. source/timer:v1)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRegistryGet,
}

func init() {
	registryCmd.AddCommand(registryListCmd, registryGetCmd)
	rootCmd.AddCommand(registryCmd)

	registryListCmd.Flags().StringVar(&registryCategory, "category", "", "Filter by category (source/function/store/endpoint/policy)")
	registryListCmd.Flags().StringVar(&registryQuery, "query", "", "Free-text search across URI and description")
}

func runRegistryList(cmd *cobra.Command, args []string) error {
	c := newClient()
	p := newPrinter()
	ctx, cancel := ctx()
	defer cancel()
	list, err := c.ListRegistry(ctx, registryCategory, registryQuery)
	if err != nil {
		return p.PrintError(err)
	}
	return p.PrintRegistryList(list)
}

func runRegistryGet(cmd *cobra.Command, args []string) error {
	c := newClient()
	p := newPrinter()
	ctx, cancel := ctx()
	defer cancel()
	entry, err := c.GetRegistryEntry(ctx, args[0])
	if err != nil {
		return p.PrintError(err)
	}
	return p.PrintRegistryEntry(entry)
}
