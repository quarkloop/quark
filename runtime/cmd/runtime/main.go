// agent is the quark agent binary.
package main

import (
	"fmt"
	"os"

	"github.com/quarkloop/agent/pkg/commands"
)

func main() {
	root := commands.Init()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
