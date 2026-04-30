// Package space is the supervisor-owned space layer.
//
// A space is a persistent data namespace identified by name (from Quarkfile).
// Storage is pluggable via the Store interface; the default implementation
// is FSStore — a filesystem-backed store under QUARK_SPACES_ROOT.
// The supervisor domain type (Space) wraps the shared spacemodel.Metadata.
package space
