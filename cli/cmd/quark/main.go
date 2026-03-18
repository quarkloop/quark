package main

import (
	"fmt"
	"os"

	"github.com/quarkloop/cli/pkg/cli"
)

func main() {
	root := cli.Root()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
