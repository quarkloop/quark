// Package space provides the shared space directory model and Quarkfile schema.
//
// It defines Metadata (the core space metadata struct), Quarkfile parsing
// and validation, and filesystem I/O helpers (ReadMetadata, WriteMetadata,
// ReadQuarkfile, WriteQuarkfile). Both the CLI and supervisor
// depend on this package for Quarkfile handling.
package space
