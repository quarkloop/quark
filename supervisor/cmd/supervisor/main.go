// supervisor is the quark supervisor binary.
// It manages Space lifecycle and provides the exclusive Space API.
package main

import (
	"fmt"
	"os"

	"github.com/quarkloop/supervisor/pkg/commands"
)

func main() {
	root := commands.Init()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
