// Package config loads the gateway's YAML configuration: which backends to
// route to, which load balancing strategy to use, and settings for the
// health checker, rate limiter, and circuit breaker.
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// BackendConfig describes a single upstream server.
type BackendConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

// Config is the top-level gateway configuration.
type Config struct {
	ListenAddr string          `yaml:"listen_addr"`
	Strategy   string          `yaml:"strategy"`
	Backends   []BackendConfig `yaml:"backends"`
}

// Load reads and parses a YAML config file from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for i := range cfg.Backends {
		if cfg.Backends[i].Weight == 0 {
			cfg.Backends[i].Weight = 1
		}
	}
	if cfg.Strategy == "" {
		cfg.Strategy = "round_robin"
	}

	return &cfg, nil
}
