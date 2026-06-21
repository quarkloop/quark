package cmd

import (
        "fmt"
        "io"
        "os"

        "github.com/quarkloop/quark/cli/internal/client"
        "github.com/quarkloop/quark/cli/internal/output"
        "github.com/spf13/cobra"
)

// systemCmd groups all system subcommands.
var systemCmd = &cobra.Command{
        Use:   "system",
        Short: "Manage systems",
        Long:  `Deploy, inspect, and delete systems. All commands require --namespace.`,
}

var systemDeployCmd = &cobra.Command{
        Use:   "deploy",
        Short: "Deploy a system from a .quark.ts file",
        Long:  `Reads the TypeScript source from --file (or stdin if --file=-), sends it to the server for deployment.`,
        Args:  cobra.NoArgs,
        RunE:  runSystemDeploy,
}

var systemDeployFile string

var systemListCmd = &cobra.Command{
        Use:   "list",
        Short: "List systems in a namespace",
        Args:  cobra.NoArgs,
        RunE:  runSystemList,
}

var systemGetCmd = &cobra.Command{
        Use:   "get NAME",
        Short: "Get system details (node states, flow, health)",
        Args:  cobra.ExactArgs(1),
        RunE:  runSystemGet,
}

var systemSourceCmd = &cobra.Command{
        Use:   "source NAME",
        Short: "Get the original .quark.ts source of a system",
        Args:  cobra.ExactArgs(1),
        RunE:  runSystemSource,
}

var systemDeleteCmd = &cobra.Command{
        Use:   "delete NAME",
        Short: "Undeploy a system",
        Args:  cobra.ExactArgs(1),
        RunE:  runSystemDelete,
}

func init() {
        systemCmd.AddCommand(systemDeployCmd, systemListCmd, systemGetCmd, systemSourceCmd, systemDeleteCmd)
        rootCmd.AddCommand(systemCmd)

        systemDeployCmd.Flags().StringVarP(&systemDeployFile, "file", "f", "", "Path to .quark.ts file (or '-' for stdin)")
        _ = systemDeployCmd.MarkFlagRequired("file")
}

func runSystemDeploy(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        source, err := readFileOrStdin(systemDeployFile)
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        resp, err := c.DeploySystem(ctx(), source, ns)
        if err != nil {
                if failureErr, ok := err.(*client.DeployFailureError); ok {
                        return p.PrintDeployResult(&output.DeployFailurePayload{
                                Message: failureErr.Failure.Message,
                                Errors:  failureErr.Failure.Errors,
                        })
                }
                return p.PrintError(err)
        }
        return p.PrintDeployResult(resp)
}

func runSystemList(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        list, err := c.ListSystems(ctx(), ns)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintSystemList(list)
}

func runSystemGet(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        detail, err := c.GetSystem(ctx(), args[0], ns)
        if err != nil {
                return p.PrintError(err)
        }
        return p.PrintSystemDetail(detail)
}

func runSystemSource(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        source, err := c.GetSystemSource(ctx(), args[0], ns)
        if err != nil {
                return newPrinter().PrintError(err)
        }
        fmt.Print(source)
        return nil
}

func runSystemDelete(cmd *cobra.Command, args []string) error {
        ns, err := requireNamespace()
        if err != nil {
                return err
        }
        c := newClient()
        p := newPrinter()
        if err := c.DeleteSystem(ctx(), args[0], ns); err != nil {
                return p.PrintError(err)
        }
        return p.PrintSuccess(fmt.Sprintf("System %s/%s undeployed.", ns, args[0]))
}

// readFileOrStdin reads from the file at path, or from stdin if path is "-".
func readFileOrStdin(path string) (string, error) {
        if path == "-" {
                bs, err := io.ReadAll(os.Stdin)
                return string(bs), err
        }
        bs, err := os.ReadFile(path)
        if err != nil {
                return "", fmt.Errorf("read file %s: %w", path, err)
        }
        return string(bs), nil
}
