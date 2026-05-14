package main

import (
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/build-release/pkg/buildrelease"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&buildrelease.Tool{}))
}
