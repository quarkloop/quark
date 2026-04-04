// read is a quark tool that reads regular text files.
//
// Usage as a tool:
//
//	read run --path ./notes.txt
//	read run --path ./app.py --start-line 10 --end-line 20
//
// Usage as an HTTP tool server:
//
//	read serve --addr 127.0.0.1:8093
//
// The HTTP server accepts POST /read with a JSON request and returns a JSON
// result describing the file content that was read.
package main

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/core/pkg/toolkit"
	readtool "github.com/quarkloop/plugins/tool-read/pkg/read"
)

func main() {
	root := toolkit.NewToolCommand("read", "read regular text files")

	root.AddCommand(runCmd())
	root.AddCommand(serveCmd())

	toolkit.Execute(root)
}

func runCmd() *cobra.Command {
	var req readtool.Request

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Read a text file and print the JSON result",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := readtool.Apply(req)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(res)
		},
	}

	cmd.Flags().StringVar(&req.Path, "path", "", "Path to the text file")
	cmd.Flags().IntVar(&req.StartLine, "start-line", 0, "1-based start line for a partial read")
	cmd.Flags().IntVar(&req.EndLine, "end-line", 0, "1-based inclusive end line for a partial read")
	return cmd
}

func serveCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an HTTP server that reads text files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return readtool.Serve(addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8093", "Address to listen on")
	return cmd
}
