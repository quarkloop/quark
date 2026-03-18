package server

import "time"

// Config holds api-server tuning parameters.
// Obtain a ready-to-use value with DefaultConfig and override fields as needed.
type Config struct {
	Host                string        `yaml:"host"`
	Port                int           `yaml:"port"`
	DataDir             string        `yaml:"data_dir"`
	LogLevel            string        `yaml:"log_level"`
	SpacePortRangeStart int           `yaml:"space_port_range_start"`
	SpacePortRangeEnd   int           `yaml:"space_port_range_end"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
}

// DefaultConfig returns a Config with production-suitable defaults
// (bind on localhost:7070, data in ~/.quark/apiserver, ports 7100–7999).
// DefaultConfig returns the default api-server configuration:
//   - Listens on 127.0.0.1:7070
//   - Stores data in ~/.quark/apiserver
//   - Allocates space-runtime ports in the range 7100–7999
//   - Health-check interval of 15 seconds
func DefaultConfig() *Config {
	return &Config{
		Host:                "127.0.0.1",
		Port:                7070,
		DataDir:             "~/.quark/apiserver",
		LogLevel:            "info",
		SpacePortRangeStart: 7100,
		SpacePortRangeEnd:   7999,
		HealthCheckInterval: 15 * time.Second,
	}
}
