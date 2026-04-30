// Package server implements the supervisor HTTP API using Fiber v2.
//
// It exposes endpoints for space management, agent lifecycle, sessions,
// knowledge base, and plugin operations. Handlers are organized by
// concern: space_handler, agent_handler, session_handler, kb_handler,
// plugin_handler, and the core handler (health, utilities).
package server
