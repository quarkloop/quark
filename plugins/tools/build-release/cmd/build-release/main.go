package main

import (
	"github.com/quarkloop/plugins/tools/build-release/pkg/buildrelease"
	"github.com/quarkloop/pkg/toolkit"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&buildrelease.Tool{}))
}
