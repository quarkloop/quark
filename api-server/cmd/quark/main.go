package main

import (
	"fmt"
	"os"

	"github.com/quarkloop/api-server/pkg/cli"
)

func main() {
	root := cli.Root()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
