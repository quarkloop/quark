// Package space is the supervisor-owned space layer. A space is a persistent
// data namespace identified by name (taken from the Quarkfile's meta.name).
// Storage is pluggable via Store; the default implementation is directory-
// backed under an internal root. User working directories (the directories
// where Quarkfiles live on a user's machine) are unrelated to space storage —
// they are passed to the agent at start time via AgentInfo.WorkingDir.
package space

import "time"

// Space is the supervisor's view of a stored space.
type Space struct {
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
