package config

// Config holds runtime configuration.
type Config struct {
	DataDir string
}

// New creates a new Config.
func New(dataDir string) *Config {
	return &Config{DataDir: dataDir}
}
