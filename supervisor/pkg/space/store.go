package space

import (
	"errors"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/kb"
	"github.com/quarkloop/supervisor/pkg/pluginmanager"
	"github.com/quarkloop/supervisor/pkg/sessions"
)

// ErrNotFound is returned by Store implementations when a space does not exist.
var ErrNotFound = errors.New("space not found")

// ErrAlreadyExists is returned by Create when a space with the same name exists.
var ErrAlreadyExists = errors.New("space already exists")

// Store is the supervisor's persistent space registry. All spaces are
// identified by Name (meta.name from the Quarkfile). Quarkfile versions are
// tracked by the store; the latest version is always returned.
//
// Implementations must be safe for concurrent use.
type Store interface {
	// Create registers a new space with the given name and initial Quarkfile
	// contents. Returns ErrAlreadyExists if a space with that name is
	// already registered.
	Create(name string, quarkfile []byte) (*Space, error)

	// UpdateQuarkfile stores a new Quarkfile version for the named space and
	// increments the version counter.
	UpdateQuarkfile(name string, quarkfile []byte) (*Space, error)

	// Get returns the metadata for the named space.
	Get(name string) (*Space, error)

	// List returns every registered space.
	List() ([]*Space, error)

	// Delete permanently removes the named space and all of its data.
	Delete(name string) error

	// Quarkfile returns the latest stored Quarkfile contents and its version.
	Quarkfile(name string) (contents []byte, version int, err error)

	// KB opens the knowledge-base store scoped to the named space.
	KB(name string) (kb.Store, error)

	// Plugins returns the plugin manager scoped to the named space.
	Plugins(name string) (*pluginmanager.Manager, error)

	// Sessions returns the session store scoped to the named space.
	Sessions(name string) (*sessions.Store, error)

	// Doctor runs health checks against the named space's Quarkfile and
	// installed plugins.
	Doctor(name string) (api.DoctorResponse, error)
}
