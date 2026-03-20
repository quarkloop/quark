package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/quarkloop/agent/pkg/infra/term"
	"github.com/quarkloop/api-server/pkg/api"
	"github.com/quarkloop/api-server/pkg/space"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

const waitPollInterval = 500 * time.Millisecond

// RunCLI returns the "run" command.
func RunCLI() *cobra.Command {
	var flags struct {
		name    string
		envFile string
		restart string
		detach  bool
		dryRun  bool
		timeout time.Duration
	}

	cmd := &cobra.Command{
		Use:   "run [dir]",
		Short: "Start a space from the current (or given) directory",
		Long: `Start a space from a directory that contains a Quarkfile.

By default quark waits for the space to reach 'running' status and then
streams its activity to the terminal (attached mode). Press Ctrl+C to detach
without stopping the space.

Use --detach (-d) to return immediately after the space is confirmed running.
Use --dry-run to start with the noop model gateway (no API key needed).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if !quarkfile.Exists(absDir) {
				return fmt.Errorf("no Quarkfile found in %s", absDir)
			}
			if !quarkfile.LockExists(absDir) {
				return fmt.Errorf("lock file missing — run 'quark lock' first")
			}
			qf, err := quarkfile.Load(absDir)
			if err != nil {
				return err
			}

			name := flags.name
			if name == "" {
				name = qf.Meta.Name
			}

			restart := flags.restart
			if restart == "" {
				restart = qf.Restart
			}
			if restart == "" {
				restart = "on-failure"
			}
			if err := validateRestartPolicy(restart); err != nil {
				return err
			}

			env := map[string]string{}
			if flags.envFile != "" {
				parsed, err := parseEnvFile(flags.envFile)
				if err != nil {
					return err
				}
				for k, v := range parsed {
					env[k] = v
				}
			}
			for _, key := range qf.Env {
				if val := os.Getenv(key); val != "" {
					env[key] = val
				}
			}
			// --dry-run: inject QUARK_DRY_RUN=1 into the space env so the
			// space-runtime activates the noop gateway without editing Quarkfile.
			if flags.dryRun {
				env["QUARK_DRY_RUN"] = "1"
			}

			client := api.NewClientApi(apiServerURL())
			sp, err := client.RunSpace(cmd.Context(), name, absDir, env, restart)
			if err != nil {
				return fmt.Errorf("starting space: %w", err)
			}

			term.Successf("Space created")
			fmt.Printf("  ID:      %s\n  Name:    %s\n  Restart: %s\n", sp.ID, sp.Name, sp.RestartPolicy)

			timeout := flags.timeout
			if timeout <= 0 {
				timeout = 30 * time.Second
			}

			sp, err = waitForRunning(cmd, client, sp.ID, timeout)
			if err != nil {
				return err
			}

			if flags.detach {
				term.Successf("Space running (detached)")
				fmt.Printf("  ID:   %s\n  Port: %d\n", sp.ID, sp.Port)
				fmt.Printf("\nUse 'quark logs %s' to stream output.\n", sp.ID)
				return nil
			}

			// Attached mode: stream activity until Ctrl+C.
			term.Infof("Streaming activity (Ctrl+C to detach) ...")
			fmt.Println()
			return streamAgentActivity(cmd.Context(), client.Agent(sp.ID), func(line string) {
				fmt.Println(line)
			})
		},
	}

	cmd.Flags().StringVar(&flags.name, "name", "", "Name for the space instance")
	cmd.Flags().StringVar(&flags.envFile, "env-file", "", "Path to .env file")
	cmd.Flags().StringVar(&flags.restart, "restart", "",
		"Restart policy: on-failure (default), always, never")
	cmd.Flags().BoolVarP(&flags.detach, "detach", "d", false,
		"Return immediately after space is running (do not stream activity)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false,
		"Use noop model gateway — no API key required, useful for testing the pipeline")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 30*time.Second,
		"Maximum time to wait for space to reach running status")
	return cmd
}

// waitForRunning polls until the space is running, failed, or the timeout elapses.
// On failure it fetches and prints the first captured log line inline.
func waitForRunning(cmd *cobra.Command, client *api.ClientApi, id string, timeout time.Duration) (*space.Space, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sp, err := client.GetSpace(cmd.Context(), id)
		if err != nil {
			return nil, err
		}
		switch sp.Status {
		case space.StatusRunning:
			return sp, nil
		case space.StatusFailed:
			hint := firstLogLine(sp)
			if hint != "" {
				return nil, fmt.Errorf("space failed to start: %s\n\nUse 'quark logs %s' for full output.", hint, id)
			}
			return nil, fmt.Errorf("space failed to start.\n\nUse 'quark logs %s' for details.", id)
		}
		time.Sleep(waitPollInterval)
	}
	return nil, fmt.Errorf(
		"timed out after %s waiting for space to start (still in 'starting' state).\n"+
			"The process may be slow to initialise. Check 'quark logs %s' or increase --timeout.",
		timeout, id,
	)
}

// firstLogLine returns the first entry of sp.LastLogs, stripped of its
// timestamp prefix, or empty string if no logs are captured yet.
func firstLogLine(sp *space.Space) string {
	if len(sp.LastLogs) == 0 {
		return ""
	}
	line := sp.LastLogs[0]
	// Logs are stored as "HH:MM:SS <message>" — strip the timestamp.
	if len(line) > 9 && line[8] == ' ' {
		return line[9:]
	}
	return line
}

func validateRestartPolicy(p string) error {
	switch p {
	case "on-failure", "always", "never":
		return nil
	}
	return fmt.Errorf("invalid restart policy %q — must be: on-failure, always, never", p)
}

func parseEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '='); i >= 0 {
			env[line[:i]] = line[i+1:]
		}
	}
	return env, nil
}
