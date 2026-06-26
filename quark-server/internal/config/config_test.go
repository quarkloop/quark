package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any QUARK_ env vars that might leak from the parent process
	for _, k := range []string{
		"QUARK_HTTP_PORT", "QUARK_NATS_URL", "QUARK_STATE_ROOT",
		"QUARK_DATAPLANE_BINARY", "QUARK_DATAPLANE_PORT_BASE",
		"QUARK_LOG_FORMAT", "QUARK_LOG_LEVEL",
	} {
		os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("NATSURL = %q, want nats://localhost:4222", cfg.NATSURL)
	}
	if cfg.StateRoot != "./quark-state" {
		t.Errorf("StateRoot = %q, want ./quark-state", cfg.StateRoot)
	}
	if cfg.DataPlanePortBase != 9100 {
		t.Errorf("DataPlanePortBase = %d, want 9100", cfg.DataPlanePortBase)
	}
	if cfg.LogFormat != "console" {
		t.Errorf("LogFormat = %q, want console", cfg.LogFormat)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("QUARK_HTTP_PORT", "9090")
	t.Setenv("QUARK_NATS_URL", "nats://example.com:4222")
	t.Setenv("QUARK_STATE_ROOT", "/tmp/quark-test")
	t.Setenv("QUARK_LOG_FORMAT", "json")
	t.Setenv("QUARK_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
	if cfg.NATSURL != "nats://example.com:4222" {
		t.Errorf("NATSURL = %q, want nats://example.com:4222", cfg.NATSURL)
	}
	if cfg.StateRoot != "/tmp/quark-test" {
		t.Errorf("StateRoot = %q, want /tmp/quark-test", cfg.StateRoot)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("QUARK_HTTP_PORT", "not-a-number")
	_, err := Load()
	if err == nil {
		t.Error("Load() with invalid port should return error")
	}
}
