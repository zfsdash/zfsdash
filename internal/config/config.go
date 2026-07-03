package config

import (
	"fmt"
	"os"

	"github.com/zfsdash/zfsdash/internal/alerts"
	"gopkg.in/yaml.v3"
)

// Config is the top-level ZFSdash configuration.
type Config struct {
	Server ServerConfig  `yaml:"server"`
	DB     DBConfig      `yaml:"db"`
	Hosts  []HostConfig  `yaml:"hosts"`
	Alerts alerts.Config `yaml:"alerts"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

// DBConfig holds database settings.
type DBConfig struct {
	Path string `yaml:"path"`
}

// HostConfig holds per-host collection settings.
type HostConfig struct {
	Name    string       `yaml:"name"`
	Mode    string       `yaml:"mode"`
	SSH     SSHConfig    `yaml:"ssh"`
	TrueNAS TrueNASConfig `yaml:"truenas"`
}

// SSHConfig holds SSH connection settings.
type SSHConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	PrivateKeyPath string `yaml:"private_key_path"`
	PrivateKeyPEM  string `yaml:"private_key_pem"`
}

// TrueNASConfig holds TrueNAS API settings.
type TrueNASConfig struct {
	URL      string `yaml:"url"`
	APIKey   string `yaml:"api_key"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Insecure bool   `yaml:"insecure"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// Apply defaults
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = "/var/lib/zfsdash/history.db"
	}
	// Default local host if none configured
	if len(cfg.Hosts) == 0 {
		cfg.Hosts = []HostConfig{{Name: "local", Mode: "local"}}
	}
	return cfg, nil
}
