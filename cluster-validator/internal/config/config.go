// Package config handles loading and validating the cluster-validator
// node list from a YAML file on disk.
package config

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// Node represents a single target the validator will probe.
type Node struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
}

// Config is the top-level structure of the nodes YAML file.
type Config struct {
	Nodes []Node `yaml:"nodes"`
}

// Load reads a YAML config file from path and returns a validated Config.
// Returns an error if the file is missing, malformed, or fails validation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the config for obvious problems before we try to use it.
// Done after parsing so error messages are clearer to the user.
func (c *Config) Validate() error {
	if len(c.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}

	seen := map[string]bool{}
	for i, n := range c.Nodes {
		if n.Name == "" {
			return fmt.Errorf("node %d: name is required", i)
		}
		if n.Endpoint == "" {
			return fmt.Errorf("node %s: endpoint is required", n.Name)
		}
		if _, err := url.Parse(n.Endpoint); err != nil {
			return fmt.Errorf("node %s: invalid endpoint URL: %w", n.Name, err)
		}
		if seen[n.Name] {
			return fmt.Errorf("duplicate node name: %s", n.Name)
		}
		seen[n.Name] = true
	}

	return nil
}