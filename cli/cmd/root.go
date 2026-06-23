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

var (
	flagHost      string
	flagNamespace string
	flagJSON      bool
	flagTimeout   time.Duration
	flagOutput    string
)

var rootCmd = &cobra.Command{
	Use:   "quarkctl",
	Short: "Quark platform command-line interface",
	Long: `quarkctl — the Quark platform CLI.

A command-line client for the Quark node platform. Every command maps
to a REST API endpoint under /api/v1/. Namespace is specified via -n
or QUARK_NAMESPACE env var.

Usage:
  quarkctl get systems -n alice
  quarkctl get system monitor -n alice
  quarkctl apply -f system.quark.ts -n alice
  quarkctl delete system monitor -n alice
  quarkctl get nodes -n alice -s monitor
  quarkctl get node cpu -n alice -s monitor
  quarkctl get events -n alice --limit 10
  quarkctl watch events -n alice
  quarkctl get registry
  quarkctl get namespaces`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "Server URL (default: http://localhost:8080, or QUARK_HOST env var)")
	rootCmd.PersistentFlags().StringVarP(&flagNamespace, "namespace", "n", "", "Tenant namespace (or QUARK_NAMESPACE env var)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON (alias for -o json)")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format: json|wide|name (default: table)")
	rootCmd.PersistentFlags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "HTTP timeout")
}

func newClient() *client.Client {
	host := flagHost
	if host == "" {
		host = envOr("QUARK_HOST", "http://localhost:8080")
	}
	return client.New(host, client.WithTimeout(flagTimeout))
}

func newPrinter() output.Printer {
	if flagJSON || flagOutput == "json" {
		return output.New(os.Stdout, true)
	}
	return output.New(os.Stdout, false)
}

func resolveNamespace() string {
	if flagNamespace != "" {
		return flagNamespace
	}
	return os.Getenv("QUARK_NAMESPACE")
}

func requireNamespace() (string, error) {
	ns := resolveNamespace()
	if ns == "" {
		return "", fmt.Errorf("namespace is required (use --namespace / -n, or set QUARK_NAMESPACE env var)")
	}
	return ns, nil
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), flagTimeout)
}

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
