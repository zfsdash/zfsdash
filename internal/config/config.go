package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/zfsdash/zfsdash/internal/alerts"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// Config is the top-level ZFSdash configuration
type Config struct {
	Server ServerConfig   `yaml:"server"`
	Hosts  []zfs.CollectorConfig `yaml:"hosts"`
	Alerts alerts.Config  `yaml:"alerts"`
	DB     DBConfig       `yaml:"db"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Listen   string `yaml:"listen"`   // default: :8080
	APIKey   string `yaml:"api_key"`  // optional static API key
	StaticDir string `yaml:"static_dir"` // override embedded static files
}

// DBConfig holds database settings
type DBConfig struct {
	Path string `yaml:"path"` // default: /var/lib/zfsdash/history.db
}

// Load reads and parses the config file at the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// DefaultConfig returns a config with sensible defaults (local mode)
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.applyDefaults()
	return cfg
}

func (c *Config) applyDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.DB.Path == "" {
		c.DB.Path = "/var/lib/zfsdash/history.db"
	}
	if c.Alerts.CapacityWarning == 0 {
		c.Alerts.CapacityWarning = 80
	}
	if c.Alerts.CapacityCritical == 0 {
		c.Alerts.CapacityCritical = 90
	}
	if c.Alerts.CooldownMinutes == 0 {
		c.Alerts.CooldownMinutes = 60
	}
	// If no hosts configured, default to local mode
	if len(c.Hosts) == 0 {
		c.Hosts = []zfs.CollectorConfig{
			{
				Mode:     "local",
				Hostname: "local",
				Timeout:  30,
			},
		}
	}
}
