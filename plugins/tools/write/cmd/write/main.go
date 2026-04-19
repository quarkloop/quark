// write is a quark tool that writes and updates regular text files.
//
// Usage as a tool:
//
//	write run --path ./notes.txt --content "hello"
//	write run --path ./notes.txt --operation replace --find hello --replace-with world
//	write run --path ./app.py --operation edit --start-line 2 --start-column 1 --end-line 2 --end-column 14 --new-text "print('hi')"
//
// Usage as an HTTP tool server:
//
//	write serve --addr 127.0.0.1:8092
//
// The HTTP server accepts POST /apply with a JSON request and returns a JSON
// result describing the file operation.
package main

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	writetool "github.com/quarkloop/plugins/tools/write/pkg/write"
	"github.com/quarkloop/supervisor/pkg/toolkit"
)

func main() {
	root := toolkit.NewToolCommand("write", "write and update regular text files")

	root.AddCommand(runCmd())
	root.AddCommand(serveCmd())

	toolkit.Execute(root)
}

func runCmd() *cobra.Command {
	var req writetool.Request

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Apply a write operation and print the JSON result",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := writetool.Apply(req)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(res)
		},
	}

	cmd.Flags().StringVar(&req.Path, "path", "", "Path to the text file")
	cmd.Flags().StringVar(&req.Operation, "operation", "", "Operation: write, append, replace, or edit")
	cmd.Flags().StringVar(&req.Content, "content", "", "Text content for write or append")
	cmd.Flags().StringVar(&req.Find, "find", "", "Text to find when operation=replace")
	cmd.Flags().StringVar(&req.ReplaceWith, "replace-with", "", "Replacement text when operation=replace")
	cmd.Flags().IntVar(&req.StartLine, "start-line", 0, "1-based start line when operation=edit")
	cmd.Flags().IntVar(&req.StartColumn, "start-column", 0, "1-based start column when operation=edit")
	cmd.Flags().IntVar(&req.EndLine, "end-line", 0, "1-based exclusive end line when operation=edit")
	cmd.Flags().IntVar(&req.EndColumn, "end-column", 0, "1-based exclusive end column when operation=edit")
	cmd.Flags().StringVar(&req.NewText, "new-text", "", "Replacement text when operation=edit")
	return cmd
}

func serveCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an HTTP server that applies write operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writetool.Serve(addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8092", "Address to listen on")
	return cmd
}
