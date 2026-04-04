package runtimecmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/agent/pkg/infra/idgen"
	"github.com/quarkloop/agent/pkg/infra/term"
	"github.com/quarkloop/cli/pkg/plugin"
	"github.com/quarkloop/cli/pkg/quarkfile"
)

const (
	defaultAgentPort          = 7100
	waitPollInterval          = 500 * time.Millisecond
	defaultAutoInstallPlugins = true
)

// RunCLI returns the "run" command.
func RunCLI() *cobra.Command {
	var flags struct {
		name               string
		envFile            string
		restart            string
		detach             bool
		dryRun             bool
		port               int
		timeout            time.Duration
		autoInstallPlugins bool
	}

	cmd := &cobra.Command{
		Use:   "run [dir]",
		Short: "Start an agent from the current (or given) directory",
		Long: `Start an agent from a directory that contains a Quarkfile.

By default quark waits for the agent to become ready and then streams its
activity to the terminal (attached mode). Press Ctrl+C to stop the agent.

Use --detach (-d) to return immediately after the agent is confirmed running.
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
			qf, err := quarkfile.Load(absDir)
			if err != nil {
				return err
			}

			// Auto-install missing plugins.
			if flags.autoInstallPlugins {
				if err := autoInstallPlugins(absDir, qf); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: auto-install failed: %v\n", err)
				}
			}

			name := flags.name
			if name == "" {
				name = qf.Meta.Name
			}

			agentID := idgen.Short()
			port := flags.port

			env := os.Environ()
			if flags.envFile != "" {
				parsed, err := parseEnvFile(flags.envFile)
				if err != nil {
					return err
				}
				for k, v := range parsed {
					env = append(env, fmt.Sprintf("%s=%s", k, v))
				}
			}
			for _, key := range qf.Env {
				if val := os.Getenv(key); val != "" {
					env = append(env, fmt.Sprintf("%s=%s", key, val))
				}
			}
			if flags.dryRun {
				env = append(env, "QUARK_DRY_RUN=1")
			}

			agentBin, err := resolveAgentBin()
			if err != nil {
				return err
			}

			agentCmd := exec.CommandContext(cmd.Context(),
				agentBin, "run",
				"--id", agentID,
				"--dir", absDir,
				"--port", fmt.Sprintf("%d", port),
			)
			agentCmd.Env = env

			if flags.detach {
				// Detached: start process, wait for health, return.
				agentCmd.Stdout = os.Stdout
				agentCmd.Stderr = os.Stderr
				if err := agentCmd.Start(); err != nil {
					return fmt.Errorf("starting agent: %w", err)
				}

				term.Successf("Agent started (pid=%d)", agentCmd.Process.Pid)
				fmt.Printf("  ID:   %s\n  Name: %s\n  Port: %d\n", agentID, name, port)

				baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
				if err := waitForHealth(cmd.Context(), baseURL, flags.timeout); err != nil {
					return fmt.Errorf("agent failed to become ready: %w", err)
				}
				term.Successf("Agent running (detached)")
				fmt.Printf("\nUse 'quark activity --agent-url %s' to stream activity.\n", baseURL)
				fmt.Printf("Use 'quark stop --agent-url %s' to stop.\n", baseURL)
				return nil
			}

			// Attached mode: connect stdout/stderr, wait for process exit.
			agentCmd.Stdout = os.Stdout
			agentCmd.Stderr = os.Stderr
			agentCmd.Stdin = os.Stdin

			term.Infof("Starting agent %s on port %d ...", name, port)
			if err := agentCmd.Start(); err != nil {
				return fmt.Errorf("starting agent: %w", err)
			}

			err = agentCmd.Wait()
			if err != nil {
				if cmd.Context().Err() != nil {
					// User pressed Ctrl+C
					return nil
				}
				return fmt.Errorf("agent exited with error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.name, "name", "", "Name for the agent instance")
	cmd.Flags().StringVar(&flags.envFile, "env-file", "", "Path to .env file")
	cmd.Flags().StringVar(&flags.restart, "restart", "",
		"Restart policy (reserved for future use)")
	cmd.Flags().BoolVarP(&flags.detach, "detach", "d", false,
		"Return immediately after agent is running (do not attach)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false,
		"Use noop model gateway — no API key required, useful for testing the pipeline")
	cmd.Flags().IntVar(&flags.port, "port", defaultAgentPort,
		"Port for the agent HTTP API")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 30*time.Second,
		"Maximum time to wait for agent to become ready")
	cmd.Flags().BoolVar(&flags.autoInstallPlugins, "auto-install-plugins", defaultAutoInstallPlugins,
		"Auto-install missing plugins declared in the Quarkfile before starting the agent")
	return cmd
}

// waitForHealth polls the agent health endpoint until it responds or timeout.
func waitForHealth(ctx context.Context, baseURL string, timeout time.Duration) error {
	client := agentclient.New(baseURL)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := client.Health(ctx); err == nil {
			return nil
		}
		time.Sleep(waitPollInterval)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

// resolveAgentBin finds the agent binary.
// Looks in: 1. Same dir as quark binary. 2. PATH.
func resolveAgentBin() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "agent")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("agent"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("agent binary not found next to quark or in PATH")
}

// autoInstallPlugins ensures all plugins declared in the Quarkfile are installed
// into the space's .quark/plugins/ directory before the agent starts.
func autoInstallPlugins(spaceDir string, qf *quarkfile.Quarkfile) error {
	pluginsDir := filepath.Join(spaceDir, ".quark", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}

	installed, err := plugin.DiscoverInstalled(pluginsDir)
	if err != nil {
		return fmt.Errorf("discover installed plugins: %w", err)
	}
	installedSet := make(map[string]bool, len(installed))
	for _, p := range installed {
		installedSet[p.Manifest.Name] = true
	}

	for _, pRef := range qf.Plugins {
		expectedName := plugin.DeriveName(pRef.Ref)
		if installedSet[expectedName] {
			continue
		}

		log.Printf("Auto-installing plugin: %s", pRef.Ref)
		_, err := plugin.InstallPlugin(pluginsDir, pRef.Ref)
		if err != nil {
			return fmt.Errorf("auto-install %s: %w", pRef.Ref, err)
		}
		fmt.Printf("Auto-installed %s\n", pRef.Ref)
	}

	return nil
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
