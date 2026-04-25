package main

import (
	"github.com/quarkloop/plugins/tools/fs/pkg/fs"
	"github.com/quarkloop/pkg/toolkit"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&fs.Tool{}))
}
