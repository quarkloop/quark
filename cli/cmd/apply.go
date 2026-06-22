package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Declaratively apply a .quark.ts system definition",
	Long:  `Apply a .quark.ts file to the server. Creates or reconciles the system to match the desired state.`,
	Args:  cobra.NoArgs,
	RunE:  runApply,
}

var applyFile string

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "Path to .quark.ts file (or '-' for stdin)")
	_ = applyCmd.MarkFlagRequired("file")
}

func runApply(cmd *cobra.Command, args []string) error {
	ns, err := requireNamespace()
	if err != nil {
		return err
	}
	source, err := readFileOrStdin(applyFile)
	if err != nil {
		return err
	}
	systemName, err := extractSystemName(source)
	if err != nil {
		return fmt.Errorf("could not determine system name from source: %w", err)
	}

	c := newClient()
	ctx, cancel := ctx()
	defer cancel()

	result, err := c.ApplySystem(ctx, systemName, source, ns)
	if err != nil {
		return newPrinter().PrintError(err)
	}

	if result.Created {
		fmt.Printf("✓ System %s/%s created.\n", result.Namespace, result.Name)
	} else if result.Changed {
		fmt.Printf("✓ System %s/%s updated.\n", result.Namespace, result.Name)
		for _, change := range result.Changes {
			fmt.Printf("  %s %s: %s\n", change.Type, change.Node, change.Details)
		}
	} else {
		fmt.Printf("✓ System %s/%s unchanged.\n", result.Namespace, result.Name)
	}
	return nil
}

func extractSystemName(source string) (string, error) {
	for _, q := range []string{"\"", "'"} {
		prefix := "name: " + q
		idx := indexOf(source, prefix)
		if idx >= 0 {
			start := idx + len(prefix)
			end := indexOfFrom(source, q, start)
			if end > start {
				return source[start:end], nil
			}
		}
	}
	return "", fmt.Errorf("name field not found")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func indexOfFrom(s, substr string, from int) int {
	for i := from; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
