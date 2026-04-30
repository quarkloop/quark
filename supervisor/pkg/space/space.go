// Package space is the supervisor-owned space layer. A space is a persistent
// data namespace identified by name (taken from the Quarkfile's meta.name).
// Storage is pluggable via Store; the default implementation is directory-
// backed under an internal root. User working directories (the directories
// where Quarkfile files live on a user's machine) are unrelated to space storage -
// they are passed to the agent at start time via AgentInfo.WorkingDir.
package space

import spacemodel "github.com/quarkloop/pkg/space"

// Space is the supervisor's view of a stored space.
// It embeds spacemodel.Metadata to avoid a type alias and maintain
// an explicit boundary between the supervisor domain and the shared model.
// Fields of Metadata are promoted to Space (e.g., sp.Name, sp.Version).
type Space struct {
	spacemodel.Metadata
}

// ToModel converts Space to a spacemodel.Metadata value.
func (s *Space) ToModel() spacemodel.Metadata {
	return s.Metadata
}

// Meta returns a pointer to the embedded Metadata for calling spacemodel functions.
func (s *Space) Meta() *spacemodel.Metadata {
	return &s.Metadata
}

// FromModel creates a Space from a spacemodel.Metadata.
func FromModel(m spacemodel.Metadata) *Space {
	return &Space{Metadata: m}
}
