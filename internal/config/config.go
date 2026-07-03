package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Port     int          `yaml:"port"`
	DataDir  string       `yaml:"data_dir"`
	LogLevel string       `yaml:"log_level"`
	Hosts    []Host       `yaml:"hosts"`
	Alerts   AlertConfig  `yaml:"alerts"`
	DB       DBConfig     `yaml:"db"`
	Server   ServerConfig `yaml:"server"`
}

// DBConfig holds database configuration.
type DBConfig struct {
	Path string `yaml:"path"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  int    `yaml:"read_timeout"`  // seconds
	WriteTimeout int    `yaml:"write_timeout"` // seconds
}

// SSHConfig holds SSH connection configuration for a host.
type SSHConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	PrivateKey     string `yaml:"private_key"`
	PrivateKeyPEM  string `yaml:"private_key_pem"`
	PrivateKeyPath string `yaml:"private_key_path"`
	KeyPath        string `yaml:"key_path"` // backward compat alias
}

// TrueNASHostConfig holds TrueNAS API connection parameters.
type TrueNASHostConfig struct {
	URL       string `yaml:"url"`
	APIKey    string `yaml:"api_key"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Insecure  bool   `yaml:"insecure"`
	TLSVerify bool   `yaml:"tls_verify"`
}

// TrueNASConfig is an alias for TrueNASHostConfig (backward compat).
type TrueNASConfig = TrueNASHostConfig

// Host describes a monitored ZFS host.
type Host struct {
	Name string `yaml:"name"`
	Mode string `yaml:"mode"` // "local", "ssh", "truenas"

	// Flat SSH fields (backward compat)
	Address  string `yaml:"address"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	KeyPath  string `yaml:"key_path"`
	Password string `yaml:"password"`

	// Nested SSH config
	SSH SSHConfig `yaml:"ssh"`

	// TrueNAS nested config
	TrueNAS TrueNASHostConfig `yaml:"truenas"`

	// Flat TrueNAS fields (backward compat)
	APIURL string            `yaml:"api_url"`
	APIKey string            `yaml:"api_key"`
	TLS    TrueNASHostConfig `yaml:"tls"`

	// Enterprise
	LicenseKey string `yaml:"license_key"`
}

// HostConfig is an alias for Host.
type HostConfig = Host

// AlertConfig holds all alerting configuration.
type AlertConfig struct {
	Enabled         bool          `yaml:"enabled"`
	CapacityWarnPct float64       `yaml:"capacity_warn_pct"`
	CapacityCritPct float64       `yaml:"capacity_crit_pct"`
	CooldownMins    int           `yaml:"cooldown_mins"`
	Email           EmailConfig   `yaml:"email"`
	Webhook         WebhookConfig `yaml:"webhook"`
}

// Alerts is an alias for AlertConfig.
type Alerts = AlertConfig

// EmailConfig holds SMTP alerting configuration.
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
}

// WebhookConfig holds webhook alerting configuration.
type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

// Load reads and parses the YAML config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = cfg.DataDir + "/zfsdash.db"
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = fmt.Sprintf(":%d", cfg.Port)
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 15
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 60
	}
	if cfg.Alerts.CapacityWarnPct == 0 {
		cfg.Alerts.CapacityWarnPct = 80
	}
	if cfg.Alerts.CapacityCritPct == 0 {
		cfg.Alerts.CapacityCritPct = 90
	}
	if cfg.Alerts.CooldownMins == 0 {
		cfg.Alerts.CooldownMins = 60
	}
	return &cfg, nil
}
