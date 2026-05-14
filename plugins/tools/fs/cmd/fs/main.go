package main

import (
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/fs/pkg/fs"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&fs.Tool{}))
}
