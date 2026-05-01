// Package agent provides the core agent with typed message loop.
//
// The Agent is the main entry point that wires together sessions, plans,
// LLM models, plugins, hierarchy management, and execution runtime.
// It communicates via a typed message loop (loop.Loop) and supports
// multiple concurrent chat sessions within a single agent process.
package agent
