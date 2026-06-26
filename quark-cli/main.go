// Command quarkctl is the Quark platform CLI.
//
// It talks to the Quark server's REST API only — no shared code with the
// server. Every command maps 1:1 to a REST endpoint. Use --json on any
// command to get raw JSON output (for AI agents / scripting).
package main

import "github.com/quarkloop/quark/cli/cmd"

func main() {
	cmd.Execute()
}
