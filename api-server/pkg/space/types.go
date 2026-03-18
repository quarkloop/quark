// Package space implements the full lifecycle of a quark space: creation,
// process management, log capture, REST API, and JSONL persistence.
//
// Space lifecycle:
//
//	created → starting → running → stopping → stopped
//	                  ↘ failed
//
// The Controller manages OS processes; the Handler exposes the REST API;
// the Store (JSONL Collection) persists Space records across server restarts.
package space

import "time"

// Status is the lifecycle state of a space.
// Status represents the lifecycle state of a space managed by the api-server.
type Status string

// Valid Status values in lifecycle order.
const (
	StatusCreated  Status = "created"  // Record exists; process not yet started.
	StatusStarting Status = "starting" // Process is launching; not yet ready.
	StatusRunning  Status = "running"  // Process is up and serving requests.
	StatusStopping Status = "stopping" // SIGINT sent; waiting for graceful exit.
	StatusStopped  Status = "stopped"  // Process exited with code 0.
	StatusFailed   Status = "failed"   // Process exited non-zero or failed to launch.
)

// Space is the persisted record for a space instance managed by the api-server.
type Space struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Dir       string    `json:"dir"`
	Status    Status    `json:"status"`
	PID       int       `json:"pid,omitempty"`
	Port      int       `json:"port,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Task 7: restart policy, read from Quarkfile at launch time.
	// Values: "on-failure" (default), "always", "never".
	RestartPolicy string `json:"restart_policy,omitempty"`

	// Task 8: environment saved at launch so the controller can re-use it on restart.
	LastEnv map[string]string `json:"last_env,omitempty"`

	// LastLogs holds the final tail of stdout/stderr captured from the space-runtime process.
	// Populated on process exit so "quark logs" can show output even after the space has stopped.
	LastLogs []string `json:"last_logs,omitempty"`

	// RestartCount is the total number of times this space has been restarted by the controller.
	// Persisted so the maxRestarts limit survives api-server restarts.
	RestartCount int `json:"restart_count,omitempty"`

	// LastRestartAt is the wall-clock time of the most recent restart attempt.
	// Persisted so the cooldown check survives api-server restarts.
	LastRestartAt *time.Time `json:"last_restart_at,omitempty"`
}

// RunRequest is the HTTP request body for POST /api/v1/spaces.
// The CLI builds it from Quarkfile fields and forwarded environment variables.
type RunRequest struct {
	Name  string            `json:"name"`
	Dir   string            `json:"dir"`
	Env   map[string]string `json:"env,omitempty"`
	Ports []PortMapping     `json:"ports,omitempty"`
	// RestartPolicy overrides the Quarkfile default when provided by the CLI.
	RestartPolicy string `json:"restart_policy,omitempty"`
}

// StopRequest is the HTTP request body for POST /api/v1/spaces/{id}/stop.
// Force=true sends SIGKILL; false sends SIGINT (graceful).
type StopRequest struct {
	Force bool `json:"force"`
}

// HealthReport is periodically POSTed by space-runtime to the api-server
// at POST /api/v1/spaces/{id}/health to confirm liveness.
type HealthReport struct {
	SpaceID    string `json:"space_id"`
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	AgentCount int    `json:"agent_count"`
	TokensUsed int64  `json:"tokens_used"`
}

// PortMapping describes a single host-to-space TCP/UDP forwarding rule.
// Reserved for future network configuration; not currently enforced.
type PortMapping struct {
	HostPort  int    `json:"host_port"`
	SpacePort int    `json:"space_port"`
	Protocol  string `json:"protocol"`
}
