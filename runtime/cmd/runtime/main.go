// runtime is the quark runtime binary.
package main

import (
	"fmt"
	"os"

	"github.com/quarkloop/runtime/pkg/commands"
)

func main() {
	root := commands.Init()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
