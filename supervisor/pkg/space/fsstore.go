package space

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/kb"
	"github.com/quarkloop/supervisor/pkg/pluginmanager"
	"github.com/quarkloop/supervisor/pkg/quarkfile"
	"github.com/quarkloop/supervisor/pkg/sessions"
)

// FSStore is a filesystem-backed Store implementation.
//
// Layout:
//
//	<root>/<name>/meta.json               — Space metadata (name, version, timestamps).
//	<root>/<name>/quarkfiles/v<N>.yaml    — Every Quarkfile version ever stored.
//	<root>/<name>/data/kb/                — KB collection directory.
//	<root>/<name>/data/plugins/           — Installed plugins directory.
type FSStore struct {
	root string

	mu    sync.Mutex             // guards meta.json writes and version bumps
	locks map[string]*sync.Mutex // per-space serialization
}

// DefaultRoot returns the default filesystem root for spaces:
// $QUARK_SPACES_ROOT when set, else $HOME/.quarkloop/spaces.
func DefaultRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("QUARK_SPACES_ROOT")); v != "" {
		return filepath.Abs(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".quarkloop", "spaces"), nil
}

// NewFSStore returns a filesystem-backed Store rooted at root. The root
// directory is created if it does not exist.
func NewFSStore(root string) (*FSStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create spaces root: %w", err)
	}
	return &FSStore{
		root:  root,
		locks: make(map[string]*sync.Mutex),
	}, nil
}

// spaceLock returns a mutex scoped to a single space name.
func (s *FSStore) spaceLock(name string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.locks[name]
	if !ok {
		m = &sync.Mutex{}
		s.locks[name] = m
	}
	return m
}

// spaceDir returns the on-disk directory for a space name.
func (s *FSStore) spaceDir(name string) string {
	return filepath.Join(s.root, name)
}

// quarkfilesDir returns the directory holding quarkfile versions.
func (s *FSStore) quarkfilesDir(name string) string {
	return filepath.Join(s.spaceDir(name), "quarkfiles")
}

// dataKBDir returns the directory holding the KB collection.
func (s *FSStore) dataKBDir(name string) string {
	return filepath.Join(s.spaceDir(name), "data", "kb")
}

// dataPluginsDir returns the directory holding installed plugins.
func (s *FSStore) dataPluginsDir(name string) string {
	return filepath.Join(s.spaceDir(name), "data", "plugins")
}

// dataSessionsPath returns the JSONL file that stores sessions.
func (s *FSStore) dataSessionsPath(name string) string {
	return filepath.Join(s.spaceDir(name), "data", "sessions.jsonl")
}

// metaPath returns the path to meta.json for a space.
func (s *FSStore) metaPath(name string) string {
	return filepath.Join(s.spaceDir(name), "meta.json")
}

// readMeta loads the persisted Space metadata.
func (s *FSStore) readMeta(name string) (*Space, error) {
	data, err := os.ReadFile(s.metaPath(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read meta.json: %w", err)
	}
	var sp Space
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("parse meta.json: %w", err)
	}
	return &sp, nil
}

// writeMeta persists the Space metadata atomically.
func (s *FSStore) writeMeta(sp *Space) error {
	data, err := json.MarshalIndent(sp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta.json: %w", err)
	}
	path := s.metaPath(sp.Name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write meta.json: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename meta.json: %w", err)
	}
	return nil
}

// writeQuarkfileVersion writes the quarkfile contents as v<N>.yaml and
// returns the version number written.
func (s *FSStore) writeQuarkfileVersion(name string, version int, contents []byte) error {
	dir := s.quarkfilesDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir quarkfiles dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("v%d.yaml", version))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, contents, 0o644); err != nil {
		return fmt.Errorf("write quarkfile v%d: %w", version, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename quarkfile v%d: %w", version, err)
	}
	return nil
}

// parseQuarkfile parses raw YAML into a Quarkfile struct.
func parseQuarkfile(data []byte) (*quarkfile.Quarkfile, error) {
	var qf quarkfile.Quarkfile
	if err := yaml.Unmarshal(data, &qf); err != nil {
		return nil, fmt.Errorf("parse quarkfile: %w", err)
	}
	return &qf, nil
}

// validateQuarkfile parses and semantically validates the raw bytes. The
// expectedName, if non-empty, must match qf.Meta.Name.
func validateQuarkfile(data []byte, expectedName string) (*quarkfile.Quarkfile, error) {
	qf, err := parseQuarkfile(data)
	if err != nil {
		return nil, err
	}
	if err := quarkfile.Validate("", qf); err != nil {
		return nil, fmt.Errorf("invalid quarkfile: %w", err)
	}
	if expectedName != "" && qf.Meta.Name != expectedName {
		return nil, fmt.Errorf("quarkfile meta.name %q does not match space name %q", qf.Meta.Name, expectedName)
	}
	return qf, nil
}

// Create registers a new space with the supervised layout and writes
// v1 of the Quarkfile.
func (s *FSStore) Create(name string, quarkfileBytes []byte) (*Space, error) {
	if name == "" {
		return nil, fmt.Errorf("space name is required")
	}
	if _, err := validateQuarkfile(quarkfileBytes, name); err != nil {
		return nil, err
	}
	lock := s.spaceLock(name)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(s.metaPath(name)); err == nil {
		return nil, ErrAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat meta: %w", err)
	}

	if err := os.MkdirAll(s.dataKBDir(name), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data/kb: %w", err)
	}
	if err := os.MkdirAll(s.dataPluginsDir(name), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data/plugins: %w", err)
	}
	if err := s.writeQuarkfileVersion(name, 1, quarkfileBytes); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sp := &Space{
		Name:      name,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.writeMeta(sp); err != nil {
		return nil, err
	}
	return sp, nil
}

// UpdateQuarkfile stores a new Quarkfile version and bumps the counter.
func (s *FSStore) UpdateQuarkfile(name string, quarkfileBytes []byte) (*Space, error) {
	if _, err := validateQuarkfile(quarkfileBytes, name); err != nil {
		return nil, err
	}
	lock := s.spaceLock(name)
	lock.Lock()
	defer lock.Unlock()

	sp, err := s.readMeta(name)
	if err != nil {
		return nil, err
	}
	next := sp.Version + 1
	if err := s.writeQuarkfileVersion(name, next, quarkfileBytes); err != nil {
		return nil, err
	}
	sp.Version = next
	sp.UpdatedAt = time.Now().UTC()
	if err := s.writeMeta(sp); err != nil {
		return nil, err
	}
	return sp, nil
}

// Get returns the metadata for the named space.
func (s *FSStore) Get(name string) (*Space, error) {
	return s.readMeta(name)
}

// List returns every registered space.
func (s *FSStore) List() ([]*Space, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("read spaces root: %w", err)
	}
	out := make([]*Space, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sp, err := s.readMeta(e.Name())
		if err != nil {
			continue // skip malformed entries silently
		}
		out = append(out, sp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Delete permanently removes the named space and all of its data.
func (s *FSStore) Delete(name string) error {
	lock := s.spaceLock(name)
	lock.Lock()
	defer lock.Unlock()

	dir := s.spaceDir(name)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove space dir: %w", err)
	}
	return nil
}

// Quarkfile returns the latest Quarkfile contents and its version number.
func (s *FSStore) Quarkfile(name string) ([]byte, int, error) {
	sp, err := s.readMeta(name)
	if err != nil {
		return nil, 0, err
	}
	path := filepath.Join(s.quarkfilesDir(name), fmt.Sprintf("v%d.yaml", sp.Version))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read quarkfile v%d: %w", sp.Version, err)
	}
	return data, sp.Version, nil
}

// KB opens the KB store scoped to the named space.
func (s *FSStore) KB(name string) (kb.Store, error) {
	if _, err := s.readMeta(name); err != nil {
		return nil, err
	}
	return kb.Open(s.dataKBDir(name))
}

// Plugins returns a plugin manager scoped to the named space.
func (s *FSStore) Plugins(name string) (*pluginmanager.Manager, error) {
	if _, err := s.readMeta(name); err != nil {
		return nil, err
	}
	return pluginmanager.NewManager(s.dataPluginsDir(name)), nil
}

// Sessions returns the session store scoped to the named space.
func (s *FSStore) Sessions(name string) (*sessions.Store, error) {
	if _, err := s.readMeta(name); err != nil {
		return nil, err
	}
	return sessions.Open(s.dataSessionsPath(name), name)
}

// Doctor runs health checks against the named space's Quarkfile and
// installed plugins.
func (s *FSStore) Doctor(name string) (api.DoctorResponse, error) {
	qfBytes, _, err := s.Quarkfile(name)
	if err != nil {
		return api.DoctorResponse{}, err
	}
	mgr, err := s.Plugins(name)
	if err != nil {
		return api.DoctorResponse{}, err
	}
	installed, err := mgr.List()
	if err != nil {
		return api.DoctorResponse{}, err
	}
	return Doctor(qfBytes, installed), nil
}
