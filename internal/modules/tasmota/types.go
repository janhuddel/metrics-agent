// Package tasmota provides a metric collection module for Tasmota devices.
// It connects to an MQTT broker to discover Tasmota devices and collect sensor data.
package tasmota

import (
	"time"

	"github.com/janhuddel/metrics-agent/internal/config"
)

// Config holds the configuration for the Tasmota module.
type Config struct {
	// Embed the base configuration for common functionality
	config.BaseConfig

	// Tasmota-specific settings
	Broker      string        `json:"broker"`       // MQTT broker address (e.g., "tcp://localhost:1883")
	Username    string        `json:"username"`     // MQTT username (optional)
	Password    string        `json:"password"`     // MQTT password (optional)
	ClientID    string        `json:"client_id"`    // MQTT client ID (optional, defaults to hostname)
	Timeout     time.Duration `json:"timeout"`      // Connection timeout (defaults to 30s)
	KeepAlive   time.Duration `json:"keep_alive"`   // Keep-alive interval (defaults to 60s)
	PingTimeout time.Duration `json:"ping_timeout"` // Ping timeout (defaults to 10s)
}

// DeviceInfo represents a discovered Tasmota device.
type DeviceInfo struct {
	IP    string         `json:"ip"`
	DN    string         `json:"dn"`    // Device name
	FN    []string       `json:"fn"`    // Friendly names
	HN    string         `json:"hn"`    // Hostname
	MAC   string         `json:"mac"`   // MAC address
	MD    string         `json:"md"`    // Module name
	TY    int            `json:"ty"`    // Type
	IF    int            `json:"if"`    // Interface
	OFLN  string         `json:"ofln"`  // Offline message
	ONLN  string         `json:"onln"`  // Online message
	State []string       `json:"state"` // State options
	SW    string         `json:"sw"`    // Software version
	T     string         `json:"t"`     // Topic (used for sensor subscription)
	FT    string         `json:"ft"`    // Full topic
	TP    []string       `json:"tp"`    // Topic prefixes
	RL    []int          `json:"rl"`    // Relay states
	SWC   []int          `json:"swc"`   // Switch states
	SWN   []string       `json:"swn"`   // Switch names
	BTN   []int          `json:"btn"`   // Button states
	SO    map[string]int `json:"so"`    // Set options
	LK    int            `json:"lk"`    // Lock
	LT_ST int            `json:"lt_st"` // Light state
	BAT   int            `json:"bat"`   // Battery
	DSLP  int            `json:"dslp"`  // Deep sleep
	SHO   []string       `json:"sho"`   // Shutter options
	SHT   []string       `json:"sht"`   // Shutter states
	VER   int            `json:"ver"`   // Version
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		BaseConfig: config.BaseConfig{
			FriendlyNameOverrides: make(map[string]string),
		},
		Broker:      "tcp://localhost:1883",
		Username:    "",
		Password:    "",
		ClientID:    "",
		Timeout:     30 * time.Second,
		KeepAlive:   60 * time.Second,
		PingTimeout: 10 * time.Second,
	}
}

// GetFriendlyName returns the friendly name for a device, checking for overrides first.
func (c *Config) GetFriendlyName(device *DeviceInfo, suffix string) string {
	deviceFriendlyName := ""
	if len(device.FN) > 0 && device.FN[0] != "" {
		deviceFriendlyName = device.FN[0]
	}
	return c.BaseConfig.GetFriendlyName(device.T+suffix, deviceFriendlyName, device.DN)
}

// LoadConfig loads configuration using the centralized configuration system.
func LoadConfig() Config {
	loader := config.NewLoader("tasmota")
	defaultConfig := DefaultConfig()

	loadedConfig, err := loader.LoadConfig(&defaultConfig)
	if err != nil {
		// If loading fails, return default config
		return defaultConfig
	}

	return *loadedConfig.(*Config)
}
