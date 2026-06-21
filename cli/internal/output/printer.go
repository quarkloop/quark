// Package output handles formatting of API responses for human and machine consumption.
//
// Two implementations:
//   - PrettyPrinter: colored tables and aligned columns (default, for interactive use)
//   - JSONPrinter: raw JSON, indented (for --json flag, AI agents, scripting)
//
// The Printer interface is the abstraction the cmd/ layer depends on.
package output

import "io"

// Printer abstracts output formatting. Every command takes a Printer and
// calls the appropriate method based on the data type it wants to render.
type Printer interface {
	// PrintSystemList renders a list of system summaries.
	PrintSystemList(systems interface{}) error
	// PrintSystemDetail renders a single system with node states.
	PrintSystemDetail(system interface{}) error
	// PrintNodeList renders a list of node summaries.
	PrintNodeList(nodes interface{}) error
	// PrintNodeDetail renders a single node with connections.
	PrintNodeDetail(node interface{}) error
	// PrintRegistryList renders a list of registry entries.
	PrintRegistryList(entries interface{}) error
	// PrintRegistryEntry renders a single registry entry.
	PrintRegistryEntry(entry interface{}) error
	// PrintEventList renders a list of events.
	PrintEventList(events interface{}) error
	// PrintHealthSummary renders a platform or namespace health summary.
	PrintHealthSummary(health interface{}) error
	// PrintSystemHealth renders a per-system health breakdown.
	PrintSystemHealth(health interface{}) error
	// PrintNodeHealth renders a per-node health with recent events.
	PrintNodeHealth(health interface{}) error
	// PrintDeployResult renders a deploy response (or failure).
	PrintDeployResult(result interface{}) error
	// PrintRaw prints an arbitrary value as-is (used for counts, plain strings, etc.).
	PrintRaw(value interface{}) error
	// PrintSuccess prints a simple success message (used for 204 No Content responses).
	PrintSuccess(message string) error
	// PrintError prints an error to stderr (used for API errors, validation failures).
	PrintError(err error) error
}

// New returns the appropriate printer based on the json flag.
func New(w io.Writer, json bool) Printer {
	if json {
		return &JSONPrinter{w: w}
	}
	return &PrettyPrinter{w: w}
}
