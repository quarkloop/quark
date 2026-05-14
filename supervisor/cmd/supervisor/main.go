// supervisor is the quark supervisor binary.
// It manages Space lifecycle and provides the exclusive Space API.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/quarkloop/supervisor/pkg/commands"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil).WithAttrs([]slog.Attr{
		slog.String("process", "supervisor"),
	})))

	root := commands.Init()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
