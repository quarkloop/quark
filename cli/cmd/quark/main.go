package main

import (
	"fmt"
	"os"

	"github.com/quarkloop/cli/pkg"
)

func main() {
	root := pkg.Root()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
