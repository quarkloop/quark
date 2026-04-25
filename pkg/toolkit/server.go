package toolkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// RunServer starts an HTTP server with all standard endpoints.
func RunServer(tool AgentTool, addr string) error {
	mux := BuildServer(tool)
	fmt.Printf("%s tool listening on %s\n", tool.Name(), addr)
	return http.ListenAndServe(addr, mux)
}

// BuildServer returns an http.ServeMux with standard tool endpoints.
func BuildServer(tool AgentTool) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"name":    tool.Name(),
			"version": tool.Version(),
		})
	})

	mux.HandleFunc("GET /schema", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(tool.Schema())
	})

	mux.HandleFunc("GET /skill", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("SKILL.md")
		if err != nil {
			http.Error(w, "SKILL.md not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(data)
	})

	// Universal invoke endpoint
	mux.HandleFunc("POST /invoke", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Command string            `json:"command"`
			Args    map[string]string `json:"args"`
			Flags   map[string]any    `json:"flags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "invalid request: "+err.Error())
			return
		}

		cmd, ok := findCommand(tool, req.Command)
		if !ok {
			writeError(w, "unknown command: "+req.Command)
			return
		}

		input := Input{Args: make(map[string]string), Flags: make(map[string]any)}
		for _, arg := range cmd.Args {
			if v, ok := req.Args[arg.Name]; ok {
				input.Args[arg.Name] = v
			} else {
				input.Args[arg.Name] = arg.Default
			}
		}
		for k, v := range req.Flags {
			input.Flags[k] = v
		}

		out, err := cmd.Handler(r.Context(), input)
		if err != nil {
			out.Error = err.Error()
		}
		writeOutput(w, out)
	})

	// Per-command endpoints
	for _, cmdDef := range tool.Commands() {
		cmdDef := cmdDef
		mux.HandleFunc("POST /"+cmdDef.Name, func(w http.ResponseWriter, r *http.Request) {
			// For per-command endpoints, the body IS the args map directly
			var reqBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				writeError(w, "invalid request: "+err.Error())
				return
			}

			input := Input{Args: make(map[string]string), Flags: make(map[string]any)}
			for _, arg := range cmdDef.Args {
				if v, ok := reqBody[arg.Name]; ok {
					input.Args[arg.Name] = v.(string)
				} else {
					input.Args[arg.Name] = arg.Default
				}
			}
			// Remaining keys are flags
			for k, v := range reqBody {
				if _, isArg := input.Args[k]; !isArg {
					input.Flags[k] = v
				}
			}

			out, err := cmdDef.Handler(r.Context(), input)
			if err != nil {
				out.Error = err.Error()
			}
			writeOutput(w, out)
		})
	}

	return mux
}

func writeOutput(w http.ResponseWriter, out Output) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func writeError(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(Output{Error: msg})
}
