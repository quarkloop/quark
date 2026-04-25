package main

import (
	"github.com/quarkloop/plugins/tools/bash/pkg/bash"
	"github.com/quarkloop/pkg/toolkit"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&bash.Tool{}))
}
