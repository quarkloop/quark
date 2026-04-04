// Package configsvc defines the config service interface and a local
// KB-backed implementation.
package config

import "context"

// Service defines operations for managing agent configuration.
type Service interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) (map[string]string, error)
	Close() error
}
