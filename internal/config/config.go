// Package config provides a centralized configuration system for all modules.
// It supports loading configuration from JSON files with module-specific overrides
// and common settings.
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// BaseConfig represents the base configuration that all modules can embed.
type BaseConfig struct {
	// Module-specific overrides (device topic/ID -> friendly name)
	FriendlyNameOverrides map[string]string `json:"friendly_name_overrides,omitempty"`

	// Module-specific custom settings
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// GetFriendlyName returns the friendly name for a device, checking for overrides first.
func (bc *BaseConfig) GetFriendlyName(deviceID string, deviceFriendlyName string, deviceName string) string {
	return GetFriendlyName(deviceID, deviceFriendlyName, deviceName, bc.FriendlyNameOverrides)
}

// ModuleConfig represents the base configuration that all modules can use.
type ModuleConfig struct {
	// Common settings that apply to all modules
	LogLevel string `json:"log_level,omitempty"`

	// Base configuration that modules can embed
	BaseConfig `json:",inline"`
}

// GlobalConfig represents the global configuration file structure.
type GlobalConfig struct {
	// Global settings
	LogLevel string `json:"log_level,omitempty"`

	// Module-specific configurations
	Modules map[string]ModuleConfig `json:"modules,omitempty"`
}

// Loader handles loading configuration from JSON files.
type Loader struct {
	configPath string
	moduleName string
}

// NewLoader creates a new configuration loader for a specific module.
func NewLoader(moduleName string) *Loader {
	return &Loader{
		moduleName: moduleName,
	}
}

// SetConfigPath sets a specific configuration file path.
func (l *Loader) SetConfigPath(path string) {
	l.configPath = path
}

// LoadConfig loads configuration for the module from JSON file.
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

	// 2. Try common locations
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
// This is a utility function that modules can use.
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
func LoadGlobalConfig() (*GlobalConfig, error) {
	configPath := getGlobalConfigPath()
	if configPath == "" {
		// No config file found, return default config
		return &GlobalConfig{}, nil
	}

	// Read and parse the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var globalConfig GlobalConfig
	if err := json.Unmarshal(data, &globalConfig); err != nil {
		return nil, err
	}

	// Apply global settings
	if globalConfig.LogLevel != "" {
		SetLogLevel(globalConfig.LogLevel)
	}

	return &globalConfig, nil
}

// SetLogLevel sets the global log level based on the configuration.
func SetLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	case "info":
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	case "warn", "warning":
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	case "error":
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	default:
		// Default to info level
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}
}

// getGlobalConfigPath determines the global configuration file path to use.
func getGlobalConfigPath() string {
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
