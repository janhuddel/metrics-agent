// Package config provides a centralized configuration system for all modules.
// It supports loading configuration from JSON files with module-specific overrides
// and common settings.
//
// The configuration system follows these principles:
// - Modules are disabled by default for security
// - Configuration can be loaded from multiple file locations
// - Module-specific settings are merged with defaults
// - Global settings like log level are applied system-wide
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/janhuddel/metrics-agent/internal/utils"
)

// GlobalConfigPath holds the path to the global configuration file.
// This is set when the application starts and used by modules to locate
// the configuration file when loading module-specific settings.
var GlobalConfigPath string

// BaseConfig represents the base configuration that all modules can embed.
// It provides common functionality for device name overrides and custom settings.
type BaseConfig struct {
	// FriendlyNameOverrides maps device IDs to human-readable names.
	// This allows modules to override device names for better readability in metrics.
	FriendlyNameOverrides map[string]string `json:"friendly_name_overrides,omitempty"`

	// Custom contains module-specific configuration settings.
	// The structure depends on the individual module's requirements.
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// GetFriendlyName returns the friendly name for a device, checking for overrides first.
// It follows this priority order:
// 1. Override from FriendlyNameOverrides map
// 2. Device's own friendly name
// 3. Device name as fallback
func (bc *BaseConfig) GetFriendlyName(deviceID string, deviceFriendlyName string, deviceName string) string {
	return GetFriendlyName(deviceID, deviceFriendlyName, deviceName, bc.FriendlyNameOverrides)
}

// ModuleConfig represents the base configuration that all modules can use.
// It includes common settings and embeds BaseConfig for device-specific functionality.
type ModuleConfig struct {
	// LogLevel sets the logging level for this specific module.
	// If not set, the global log level is used.
	LogLevel string `json:"log_level,omitempty"`

	// Enabled controls whether the module should be started.
	// Defaults to false (disabled) for security - modules must be explicitly enabled.
	Enabled bool `json:"enabled,omitempty"`

	// BaseConfig provides common functionality for device name overrides and custom settings.
	BaseConfig `json:",inline"`
}

// GlobalConfig represents the global configuration file structure.
// It contains system-wide settings and module-specific configurations.
type GlobalConfig struct {
	// LogLevel sets the global logging level for the application.
	// Valid values: "debug", "info", "warn", "error"
	LogLevel string `json:"log_level,omitempty"`

	// ModuleRestartLimit controls how many times a module can restart before the process exits.
	// - 0: unlimited restarts (not recommended for production)
	// - 1: exit on first failure
	// - 3: default, good for telegraf/systemd deployments
	// - negative values: fall back to default (3)
	ModuleRestartLimit int `json:"module_restart_limit,omitempty"`

	// Modules contains configuration for each available module.
	// Only modules with "enabled": true will be started.
	Modules map[string]ModuleConfig `json:"modules,omitempty"`
}

// Loader handles loading configuration from JSON files for specific modules.
// It provides a clean interface for loading and merging configuration data.
type Loader struct {
	configPath string
	moduleName string
}

// NewLoader creates a new configuration loader for a specific module.
// The loader will automatically discover the configuration file location.
func NewLoader(moduleName string) *Loader {
	return &Loader{
		moduleName: moduleName,
	}
}

// NewLoaderWithPath creates a new configuration loader for a specific module with a custom config path.
// This is useful when you need to load configuration from a specific file location.
func NewLoaderWithPath(moduleName string, configPath string) *Loader {
	return &Loader{
		moduleName: moduleName,
		configPath: configPath,
	}
}

// SetConfigPath sets a specific configuration file path.
// This overrides the automatic configuration file discovery.
func (l *Loader) SetConfigPath(path string) {
	l.configPath = path
}

// LoadConfig loads configuration for the module from JSON file.
// It starts with the provided default configuration and merges in any
// module-specific settings found in the configuration file.
func (l *Loader) LoadConfig(defaultConfig interface{}) (interface{}, error) {
	// Start with default configuration
	config := l.cloneConfig(defaultConfig)

	// Load from config file if available
	if err := l.loadFromFile(config); err != nil {
		return nil, fmt.Errorf("failed to load config from file: %w", err)
	}

	return config, nil
}

// loadFromFile loads configuration from a JSON file.
func (l *Loader) loadFromFile(config interface{}) error {
	configPath := l.getConfigPath()
	if configPath == "" {
		return nil // No config file specified
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // File doesn't exist, continue with defaults
	}

	// Read and parse the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var globalConfig GlobalConfig
	if err := json.Unmarshal(data, &globalConfig); err != nil {
		return err
	}

	// Extract module-specific config
	moduleConfig, exists := globalConfig.Modules[l.moduleName]
	if !exists {
		return nil // No module-specific config found
	}

	// Apply module config to the target config struct
	return l.applyModuleConfig(config, moduleConfig)
}

// applyModuleConfig applies module-specific configuration to the target config.
func (l *Loader) applyModuleConfig(config interface{}, moduleConfig ModuleConfig) error {
	// Use reflection to apply the module config to the target config struct
	configValue := reflect.ValueOf(config).Elem()
	_ = configValue.Type()

	// Apply friendly name overrides if the target config has this field
	if friendlyNameField := configValue.FieldByName("FriendlyNameOverrides"); friendlyNameField.IsValid() && friendlyNameField.CanSet() {
		if moduleConfig.FriendlyNameOverrides != nil {
			friendlyNameField.Set(reflect.ValueOf(moduleConfig.FriendlyNameOverrides))
		}
	}

	// Apply custom settings to individual fields
	if moduleConfig.Custom != nil {
		l.applyCustomSettings(configValue, moduleConfig.Custom)
	}

	return nil
}

// applyCustomSettings applies custom settings to the config struct fields.
func (l *Loader) applyCustomSettings(configValue reflect.Value, custom map[string]interface{}) {
	configType := configValue.Type()

	for i := 0; i < configValue.NumField(); i++ {
		field := configValue.Field(i)
		fieldType := configType.Field(i)

		// Skip unexported fields and embedded structs
		if !field.CanSet() || fieldType.Anonymous {
			continue
		}

		// Get JSON tag name
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Remove omitempty and other options
		jsonName := strings.Split(jsonTag, ",")[0]

		// Apply custom setting if it exists
		if customValue, exists := custom[jsonName]; exists {
			l.setFieldValue(field, customValue)
		}
	}
}

// setFieldValue sets a field value with type conversion.
func (l *Loader) setFieldValue(field reflect.Value, value interface{}) {
	fieldType := field.Type()
	valueType := reflect.TypeOf(value)

	// Direct assignment if types match
	if valueType.AssignableTo(fieldType) {
		field.Set(reflect.ValueOf(value))
		return
	}

	// Handle string to duration conversion
	if fieldType == reflect.TypeOf(time.Duration(0)) {
		if str, ok := value.(string); ok {
			if duration, err := time.ParseDuration(str); err == nil {
				field.Set(reflect.ValueOf(duration))
			}
		}
		return
	}

	// Handle string to other types
	if valueType == reflect.TypeOf("") {
		str := value.(string)
		switch fieldType.Kind() {
		case reflect.String:
			field.SetString(str)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if intVal, err := strconv.ParseInt(str, 10, 64); err == nil {
				field.SetInt(intVal)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if uintVal, err := strconv.ParseUint(str, 10, 64); err == nil {
				field.SetUint(uintVal)
			}
		case reflect.Bool:
			if boolVal, err := strconv.ParseBool(str); err == nil {
				field.SetBool(boolVal)
			}
		}
	}
}

// getConfigPath determines the configuration file path to use.
func (l *Loader) getConfigPath() string {
	// 1. Use explicitly set path
	if l.configPath != "" {
		return l.configPath
	}

	// 2. Use global config path if set
	if GlobalConfigPath != "" {
		return GlobalConfigPath
	}

	// 3. Try common locations
	possiblePaths := []string{
		"metrics-agent.json",
		filepath.Join("config", "metrics-agent.json"),
		"config.json",
		filepath.Join("config", "config.json"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "" // No config file found
}

// cloneConfig creates a deep copy of the configuration struct.
func (l *Loader) cloneConfig(config interface{}) interface{} {
	// Simple deep copy using JSON marshaling/unmarshaling
	// This works for most configuration structs
	data, err := json.Marshal(config)
	if err != nil {
		// If marshaling fails, return the original
		return config
	}

	newConfig := reflect.New(reflect.TypeOf(config).Elem()).Interface()
	if err := json.Unmarshal(data, newConfig); err != nil {
		// If unmarshaling fails, return the original
		return config
	}

	return newConfig
}

// GetFriendlyName returns the friendly name for a device, checking for overrides first.
// This is a utility function that modules can use to get human-readable device names.
// It follows this priority order:
// 1. Override from the overrides map
// 2. Device's own friendly name
// 3. Device name as fallback
func GetFriendlyName(deviceID string, deviceFriendlyName string, deviceName string, overrides map[string]string) string {
	// Check for override first
	if override, exists := overrides[deviceID]; exists {
		return override
	}

	// Fall back to device's friendly name or device name
	if deviceFriendlyName != "" {
		return deviceFriendlyName
	}
	return deviceName
}

// LoadGlobalConfig loads the global configuration and applies global settings like log level.
// It automatically discovers the configuration file location using GetGlobalConfigPath().
// If no configuration file is found, it returns a default configuration.
func LoadGlobalConfig() (*GlobalConfig, error) {
	configPath := GetGlobalConfigPath()
	if configPath == "" {
		// No config file found, return default config
		return &GlobalConfig{}, nil
	}
	return LoadGlobalConfigFromPath(configPath)
}

// LoadGlobalConfigFromPath loads the global configuration from a specific path.
// It validates that the file exists and contains valid JSON before parsing.
func LoadGlobalConfigFromPath(configPath string) (*GlobalConfig, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Read and parse the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", configPath, err)
	}

	var globalConfig GlobalConfig
	if err := json.Unmarshal(data, &globalConfig); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file %s: %w", configPath, err)
	}

	return &globalConfig, nil
}

// SetLogLevel sets the global log level based on the configuration.
// It accepts standard log level strings: "debug", "info", "warn", "error".
func SetLogLevel(level string) {
	utils.SetGlobalLogLevelFromString(level)
	utils.Debugf("Log level set to: %s", level)
}

// GetGlobalConfigPath determines the global configuration file path to use.
// It searches for configuration files in the following order:
// 1. metrics-agent.json in current directory
// 2. config/metrics-agent.json
// 3. config.json in current directory
// 4. config/config.json
// Returns an empty string if no configuration file is found.
func GetGlobalConfigPath() string {
	// Try common locations
	possiblePaths := []string{
		"metrics-agent.json",
		filepath.Join("config", "metrics-agent.json"),
		"config.json",
		filepath.Join("config", "config.json"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "" // No config file found
}
