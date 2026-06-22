// Package config loads and saves a migration configuration. The format is YAML
// (the documented format); since YAML is a superset of JSON, existing JSON
// config files also parse. The struct carries both yaml and json tags so the
// same type serializes correctly for files (yaml) and for the Wails UI (json).
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level migration configuration.
type Config struct {
	Source    Endpoint  `yaml:"source" json:"source"`
	Target    Endpoint  `yaml:"target" json:"target"`
	Migration Migration `yaml:"migration" json:"migration"`
}

// Endpoint identifies a database engine and how to connect to it. Engine must
// match a registered source/target adapter name (e.g. "mssql", "postgres").
type Endpoint struct {
	Engine string `yaml:"engine" json:"engine"`
	// DSN is the connection string. Prefer providing it via an environment
	// variable reference resolved by the caller rather than hardcoding secrets.
	DSN string `yaml:"dsn" json:"dsn"`
}

// Migration holds run options.
type Migration struct {
	DryRun      bool     `yaml:"dry_run" json:"dry_run"`
	Parallelism int      `yaml:"parallelism" json:"parallelism"`
	Tables      []string `yaml:"tables" json:"tables"`
}

// Load reads and parses a YAML (or JSON) config file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	c, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return c, nil
}

// Parse decodes config bytes (YAML or JSON).
func Parse(b []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Marshal encodes a config as YAML — used by the UI's "save config" feature.
func Marshal(c *Config) ([]byte, error) {
	return yaml.Marshal(c)
}
