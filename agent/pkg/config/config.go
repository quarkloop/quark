// Package config provides a KB-backed dynamic configuration store for agent
// operational settings. The agent self-configures these values at startup;
// the owner can override them via API or chat channels. Owner writes always
// take precedence over agent self-configuration.
package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quarkloop/core/pkg/kb"
)

const (
	nsConfig = "config"
	keyDyn   = "dynamic"
	keySrc   = "source" // "agent" or "owner" per-key tracking
)

// Source identifies who set a config value.
type Source string

const (
	SourceAgent Source = "agent"
	SourceOwner Source = "owner"
)

// DynamicConfig holds all operational settings that the agent tunes at
// runtime. Owner overrides always take precedence over agent self-config.
type DynamicConfig struct {
	Model         ModelConfig      `json:"model"`
	ContextWindow int              `json:"context_window"`
	Compaction    CompactionConfig `json:"compaction"`
	Memory        MemoryConfig     `json:"memory"`
	UpdatedAt     time.Time        `json:"updated_at"`
	UpdatedBy     Source           `json:"updated_by"`
}

// ModelConfig specifies the active LLM provider and model name.
type ModelConfig struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

// CompactionConfig controls context compaction behavior.
type CompactionConfig struct {
	Threshold int    `json:"threshold"` // percentage (0-100)
	Strategy  string `json:"strategy"`  // "pipeline" | "fifo" | "weight"
}

// MemoryConfig controls memory retention behavior.
type MemoryConfig struct {
	MaxEntries    int `json:"max_entries"`
	RetentionDays int `json:"retention_days"`
}

// DefaultDynamicConfig returns a config with sensible defaults.
func DefaultDynamicConfig() DynamicConfig {
	return DynamicConfig{
		ContextWindow: 8192,
		Compaction: CompactionConfig{
			Threshold: 80,
			Strategy:  "pipeline",
		},
		Memory: MemoryConfig{
			MaxEntries:    1000,
			RetentionDays: 30,
		},
	}
}

// Store is a KB-backed dynamic config store with owner-wins semantics.
type Store struct {
	kb kb.Store
}

// New creates a Store backed by the given KB.
func New(kb kb.Store) *Store {
	return &Store{kb: kb}
}

// Load reads the full DynamicConfig from the KB.
// Returns defaults if no config exists yet.
func (s *Store) Load() (DynamicConfig, error) {
	data, err := s.kb.Get(nsConfig, keyDyn)
	if err != nil {
		return DefaultDynamicConfig(), nil
	}
	var cfg DynamicConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultDynamicConfig(), nil
	}
	return cfg, nil
}

// Save writes the full DynamicConfig to the KB.
func (s *Store) Save(cfg DynamicConfig) error {
	cfg.UpdatedAt = time.Now()
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal dynamic config: %w", err)
	}
	return s.kb.Set(nsConfig, keyDyn, data)
}

// Get reads an individual config key.
func (s *Store) Get(key string) (string, error) {
	data, err := s.kb.Get(nsConfig, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Set writes an individual config key with source tracking.
func (s *Store) Set(key, value string, updatedBy Source) error {
	return s.kb.Set(nsConfig, key, []byte(value))
}

// SetByOwner writes a config key as an owner override.
// Owner values always take precedence over agent self-config.
func (s *Store) SetByOwner(key, value string) error {
	return s.kb.Set(nsConfig, key, []byte(value))
}

// SetByAgent writes a config key as an agent self-config value.
// This is a no-op if an owner override already exists for this key.
func (s *Store) SetByAgent(key, value string) error {
	srcKey := keySrc + "/" + key
	srcData, err := s.kb.Get(nsConfig, srcKey)
	if err == nil && string(srcData) == string(SourceOwner) {
		return nil // owner override exists, skip
	}
	if err := s.kb.Set(nsConfig, key, []byte(value)); err != nil {
		return err
	}
	return s.kb.Set(nsConfig, srcKey, []byte(SourceAgent))
}

// ClearOwnerOverride removes an owner override for a key,
// reverting to agent self-config or default.
func (s *Store) ClearOwnerOverride(key string) error {
	srcKey := keySrc + "/" + key
	_ = s.kb.Delete(nsConfig, srcKey)
	return s.kb.Delete(nsConfig, key)
}
