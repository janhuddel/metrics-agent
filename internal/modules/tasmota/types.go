// Package tasmota provides a metric collection module for Tasmota devices.
// It connects to an MQTT broker to discover Tasmota devices and collect sensor data.
package tasmota

import (
	"os"
	"time"
)

// Config holds the configuration for the Tasmota module.
type Config struct {
	Broker   string        // MQTT broker address (e.g., "tcp://localhost:1883")
	Username string        // MQTT username (optional)
	Password string        // MQTT password (optional)
	ClientID string        // MQTT client ID (optional, defaults to hostname)
	Timeout  time.Duration // Connection timeout (defaults to 30s)
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

// DefaultConfig returns a default configuration using environment variables.
func DefaultConfig() Config {
	hostname, _ := os.Hostname()
	return Config{
		Broker:   getEnvOrDefault("TASMOTA_MQTT_BROKER", "tcp://localhost:1883"),
		Username: os.Getenv("TASMOTA_MQTT_USERNAME"),
		Password: os.Getenv("TASMOTA_MQTT_PASSWORD"),
		ClientID: getEnvOrDefault("TASMOTA_MQTT_CLIENT_ID", hostname+"-tasmota"),
		Timeout:  30 * time.Second,
	}
}

// getEnvOrDefault returns the environment variable value or a default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
