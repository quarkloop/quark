package main

import (
	"github.com/quarkloop/plugins/tools/web-search/pkg/websearch"
	"github.com/quarkloop/pkg/toolkit"
)

func main() {
	toolkit.Execute(toolkit.BuildCLI(&websearch.Tool{}))
}
