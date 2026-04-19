// Package bash implements the quark bash tool — shell command execution.
package bash

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
)

// Execute runs a shell command and returns its combined output.
func Execute(command string) ([]byte, error) {
	return exec.Command("bash", "-c", command).CombinedOutput()
}

// RunHandler returns an HTTP handler that executes shell commands.
// Accepts POST with body {"cmd":"..."} and returns {"output":"...","exit_code":0}.
func RunHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		out, err := Execute(req.Cmd)
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
	}
}

// Serve starts an HTTP server for the bash tool on the given address.
func Serve(addr string) error {
	http.HandleFunc("POST /bash", RunHandler())
	fmt.Printf("bash tool listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}
