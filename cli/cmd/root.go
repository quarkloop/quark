// Package cmd implements the cobra command tree for quarkctl.
//
// Each command is in its own file (system.go, node.go, etc.) but all
// share the root command's persistent flags (--host, -n/--namespace, --json).
// The package exposes a single Execute() function called from main.go.
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quarkloop/quark/cli/internal/client"
	"github.com/quarkloop/quark/cli/internal/output"
	"github.com/spf13/cobra"
)

// Global flag values, populated by the root command's persistent flags.
var (
	flagHost      string
	flagNamespace string
	flagJSON      bool
	flagTimeout   time.Duration
)

// rootCmd is the top-level cobra command.
var rootCmd = &cobra.Command{
	Use:   "quarkctl",
	Short: "Quark platform command-line interface",
	Long: `quarkctl — the Quark platform CLI.

A kubectl-style client for the Quark node platform. Every command maps
1:1 to a server REST endpoint. Multi-tenancy is enforced via the --namespace
flag (or QUARK_NAMESPACE env var).

Use --json on any command to get the raw API response as formatted JSON
(for AI agents and scripting).`,
	SilenceUsage: true,
}

// Execute runs the root command. Called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra prints errors to stderr by default; exit non-zero.
		os.Exit(1)
	}
}

func init() {
	// Persistent flags — inherited by all subcommands.
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "Server URL (default: http://localhost:8080, or QUARK_HOST env var)")
	rootCmd.PersistentFlags().StringVarP(&flagNamespace, "namespace", "n", "", "Tenant namespace (required for tenant-scoped commands; or QUARK_NAMESPACE env var)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON (for AI agents / scripting)")
	rootCmd.PersistentFlags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "HTTP timeout")
}

// newClient constructs a Client using the current global flags + env vars.
func newClient() *client.Client {
	host := flagHost
	if host == "" {
		host = envOr("QUARK_HOST", "http://localhost:8080")
	}
	return client.New(host, client.WithTimeout(flagTimeout))
}

// newPrinter constructs a Printer based on the --json flag.
func newPrinter() output.Printer {
	return output.New(os.Stdout, flagJSON)
}

// resolveNamespace returns the namespace to use for a command, checking
// the --namespace flag first and falling back to the QUARK_NAMESPACE env var.
// Returns an empty string if neither is set — callers should validate.
func resolveNamespace() string {
	if flagNamespace != "" {
		return flagNamespace
	}
	return os.Getenv("QUARK_NAMESPACE")
}

// requireNamespace returns the namespace or an error if it's not set.
// Use this for commands that MUST have a namespace (all tenant-scoped commands).
func requireNamespace() (string, error) {
	ns := resolveNamespace()
	if ns == "" {
		return "", fmt.Errorf("namespace is required (use --namespace / -n, or set QUARK_NAMESPACE env var)")
	}
	return ns, nil
}

// envOr returns the env var value or the default if the env var is empty/unset.
func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// ctx returns a context with the global timeout. Subcommands can use this
// for their API calls.
func ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), flagTimeout)
	// We can't defer cancel here because the caller uses the context after
	// this function returns. The timeout will fire automatically.
	_ = cancel
	return ctx
}

// stdout returns os.Stdout — abstracted so tests can replace it.
func stdout() io.Writer {
	return os.Stdout
}

// signalContext returns a context that is cancelled on SIGINT/SIGTERM.
// Used by long-running commands like `event watch`.
func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	return ctx
}
