// bash is a quark tool that executes a shell command and returns its output.
//
// Usage as a tool (called by agent worker):
//
//	bash run --cmd "ls -la"
//
// Usage as an HTTP skill server:
//
//	bash serve --addr 127.0.0.1:8091
//
// The HTTP server accepts POST /run with body {"cmd":"..."} and returns
// {"output":"...","exit_code":0}.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "bash",
		Short:        "quark bash tool — execute shell commands",
		SilenceUsage: true,
	}

	root.AddCommand(runCmd())
	root.AddCommand(serveCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
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
			out, err := exec.Command("bash", "-c", command).CombinedOutput()
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
			http.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Cmd string `json:"cmd"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
					return
				}
				if req.Cmd == "" {
					http.Error(w, `{"error":"cmd is required"}`, http.StatusBadRequest)
					return
				}
				out, err := exec.Command("bash", "-c", req.Cmd).CombinedOutput()
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"output":    string(out),
					"exit_code": exitCode,
				})
			})
			fmt.Printf("bash tool listening on %s\n", addr)
			return http.ListenAndServe(addr, nil)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8091", "Address to listen on")
	return cmd
}
