// Package config loads a migration configuration. JSON is used for the
// skeleton to keep the build dependency-free; a YAML loader can be added later
// without changing these structs.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the top-level migration configuration.
type Config struct {
	Source    Endpoint  `json:"source"`
	Target    Endpoint  `json:"target"`
	Migration Migration `json:"migration"`
}

// Endpoint identifies a database engine and how to connect to it. Engine must
// match a registered source/target adapter name (e.g. "mssql", "postgres").
type Endpoint struct {
	Engine string `json:"engine"`
	// DSN is the connection string. Prefer providing it via an environment
	// variable reference resolved by the caller rather than hardcoding secrets.
	DSN string `json:"dsn"`
}

// Migration holds run options.
type Migration struct {
	DryRun      bool     `json:"dry_run"`
	Parallelism int      `json:"parallelism"`
	Tables      []string `json:"tables"`
}

// Load reads and parses a JSON config file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &c, nil
}
