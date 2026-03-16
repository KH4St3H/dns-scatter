package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Mapping struct {
	Original     string   `yaml:"original"`
	Replacements []string `yaml:"replacements"`
}

type ClientConfig struct {
	Listen   string    `yaml:"listen"`
	Server   string    `yaml:"server"`
	Mappings []Mapping `yaml:"mappings"`
}

type ServerConfig struct {
	Listen   string    `yaml:"listen"`
	Upstream string    `yaml:"upstream"`
	Mappings []Mapping `yaml:"mappings"`
}

func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg ClientConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address is required")
	}
	if cfg.Server == "" {
		return nil, fmt.Errorf("server address is required")
	}
	for i, m := range cfg.Mappings {
		if m.Original == "" {
			return nil, fmt.Errorf("mapping %d: original domain is required", i)
		}
		if len(m.Replacements) == 0 {
			return nil, fmt.Errorf("mapping %d: at least one replacement is required", i)
		}
	}
	return &cfg, nil
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address is required")
	}
	if cfg.Upstream == "" {
		return nil, fmt.Errorf("upstream address is required")
	}
	for i, m := range cfg.Mappings {
		if m.Original == "" {
			return nil, fmt.Errorf("mapping %d: original domain is required", i)
		}
		if len(m.Replacements) == 0 {
			return nil, fmt.Errorf("mapping %d: at least one replacement is required", i)
		}
	}
	return &cfg, nil
}
