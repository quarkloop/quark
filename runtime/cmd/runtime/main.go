// runtime is the quark runtime binary.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/quarkloop/runtime/pkg/commands"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil).WithAttrs([]slog.Attr{
		slog.String("process", "runtime"),
	})))

	root := commands.Init()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
