// bash is a quark tool that executes a shell command and returns its output.
//
// Usage as a tool (called by agent worker):
//
//	bash run --cmd "ls -la"
//
// Usage as an HTTP tool server:
//
//	bash serve --addr 127.0.0.1:8091
//
// The HTTP server accepts POST /run with body {"cmd":"..."} and returns
// {"output":"...","exit_code":0}.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/core/pkg/toolkit"
	"github.com/quarkloop/plugins/tool-bash/pkg/bash"
)

func main() {
	root := toolkit.NewToolCommand("bash", "execute shell commands")

	root.AddCommand(runCmd())
	root.AddCommand(serveCmd())

	toolkit.Execute(root)
}

func runCmd() *cobra.Command {
	var command string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a shell command and print its output",
		RunE: func(cmd *cobra.Command, args []string) error {
			if command == "" {
				return fmt.Errorf("--cmd is required")
			}
			out, err := bash.Execute(command)
			fmt.Print(string(out))
			return err
		},
	}
	cmd.Flags().StringVar(&command, "cmd", "", "Shell command to execute")
	return cmd
}

func serveCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an HTTP server that executes shell commands on request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bash.Serve(addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8091", "Address to listen on")
	return cmd
}
