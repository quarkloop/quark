package main

import (
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/bash/pkg/bash"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&bash.Tool{}))
}
