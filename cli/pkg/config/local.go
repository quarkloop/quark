package config

import (
	"context"
	"fmt"

	"github.com/quarkloop/cli/pkg/kb"
)

const namespace = "config"

// LocalStore implements Service using the local KB filesystem store.
type LocalStore struct {
	kb kb.Store
}

// NewLocalService creates a config service backed by the space's KB.
func NewLocalService(spaceDir string) (Service, error) {
	k, err := kb.Open(spaceDir)
	if err != nil {
		return nil, fmt.Errorf("open kb for config: %w", err)
	}
	return &LocalStore{kb: k}, nil
}

func (s *LocalStore) Get(_ context.Context, key string) (string, error) {
	data, err := s.kb.Get(namespace, key)
	if err != nil {
		return "", fmt.Errorf("config key %s not found", key)
	}
	return string(data), nil
}

func (s *LocalStore) Set(_ context.Context, key string, value string) error {
	return s.kb.Set(namespace, key, []byte(value))
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	return s.kb.Delete(namespace, key)
}

func (s *LocalStore) List(_ context.Context) (map[string]string, error) {
	keys, err := s.kb.List(namespace)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		data, err := s.kb.Get(namespace, k)
		if err != nil {
			continue
		}
		out[k] = string(data)
	}
	return out, nil
}

func (s *LocalStore) Close() error {
	return s.kb.Close()
}
