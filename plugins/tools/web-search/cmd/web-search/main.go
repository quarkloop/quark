package main

import (
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/web-search/pkg/websearch"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&websearch.Tool{}))
}
