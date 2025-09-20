package utils

import (
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// AppConfig represents the application configuration structure
type AppConfig struct {
	Logging struct {
		Level string `koanf:"level"`
	} `koanf:"logging"`
	Sources map[string]interface{} `koanf:"sources"`
	Retry   struct {
		MaxRetries int           `koanf:"max_retries"`
		BaseDelay  time.Duration `koanf:"base_delay"`
		MaxDelay   time.Duration `koanf:"max_delay"`
	} `koanf:"retry"`
}

// getEnvironment returns the current environment, defaulting to "dev"
func getEnvironment() string {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("APP_ENV")
	}
	if env == "" {
		env = "dev" // default to development
	}
	return strings.ToLower(env)
}

// LoadConfig loads configuration with environment-specific overrides
func LoadConfig() (*AppConfig, error) {
	k := koanf.New(".")

	// Load default configuration first
	if err := k.Load(file.Provider("./configs/defaults.yaml"), yaml.Parser()); err != nil {
		return nil, err
	}

	// Load environment-specific configuration to override defaults
	env := getEnvironment()
	envConfigPath := "./configs/" + env + ".yaml"

	// Check if environment-specific config exists
	if _, err := os.Stat(envConfigPath); err == nil {
		if err := k.Load(file.Provider(envConfigPath), yaml.Parser()); err != nil {
			return nil, err
		}
	}

	var cfg AppConfig
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetSourceConfig returns the configuration for a specific source as a map
func (c *AppConfig) GetSourceConfig(sourceName string) map[string]interface{} {
	if c.Sources == nil {
		return make(map[string]interface{})
	}

	if sourceConfig, exists := c.Sources[sourceName]; exists {
		if configMap, ok := sourceConfig.(map[string]interface{}); ok {
			return configMap
		}
	}

	return make(map[string]interface{})
}

// IsSourceEnabled checks if a source is enabled in the configuration
func (c *AppConfig) IsSourceEnabled(sourceName string) bool {
	config := c.GetSourceConfig(sourceName)
	if enabled, exists := config["enabled"]; exists {
		if enabledBool, ok := enabled.(bool); ok {
			return enabledBool
		}
	}
	return false // Default to disabled if not specified
}
