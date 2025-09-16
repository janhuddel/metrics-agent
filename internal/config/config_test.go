package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestModuleConfig_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected bool
	}{
		{
			name:     "enabled true",
			json:     `{"enabled": true}`,
			expected: true,
		},
		{
			name:     "enabled false",
			json:     `{"enabled": false}`,
			expected: false,
		},
		{
			name:     "enabled not specified (default)",
			json:     `{}`,
			expected: false,
		},
		{
			name:     "enabled with other fields",
			json:     `{"enabled": true, "log_level": "debug", "friendly_name_overrides": {"device1": "Device 1"}}`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config ModuleConfig
			err := json.Unmarshal([]byte(tt.json), &config)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			if config.Enabled != tt.expected {
				t.Errorf("Expected Enabled to be %v, got %v", tt.expected, config.Enabled)
			}
		})
	}
}

func TestGlobalConfig_ModuleEnabled(t *testing.T) {
	tests := []struct {
		name        string
		config      GlobalConfig
		moduleName  string
		expected    bool
		description string
	}{
		{
			name: "module enabled explicitly",
			config: GlobalConfig{
				Modules: map[string]ModuleConfig{
					"test-module": {
						Enabled: true,
					},
				},
			},
			moduleName:  "test-module",
			expected:    true,
			description: "Module with enabled: true should return true",
		},
		{
			name: "module disabled explicitly",
			config: GlobalConfig{
				Modules: map[string]ModuleConfig{
					"test-module": {
						Enabled: false,
					},
				},
			},
			moduleName:  "test-module",
			expected:    false,
			description: "Module with enabled: false should return false",
		},
		{
			name: "module not in config (default disabled)",
			config: GlobalConfig{
				Modules: map[string]ModuleConfig{
					"other-module": {
						Enabled: true,
					},
				},
			},
			moduleName:  "test-module",
			expected:    false,
			description: "Module not in config should default to disabled",
		},
		{
			name: "no modules config (default disabled)",
			config: GlobalConfig{
				Modules: nil,
			},
			moduleName:  "test-module",
			expected:    false,
			description: "Module should default to disabled when no modules config exists",
		},
		{
			name: "module with enabled not specified (default disabled)",
			config: GlobalConfig{
				Modules: map[string]ModuleConfig{
					"test-module": {
						LogLevel: "debug",
					},
				},
			},
			moduleName:  "test-module",
			expected:    false,
			description: "Module with enabled field not specified should default to disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a helper function to check if module is enabled
			enabled := false
			if tt.config.Modules != nil {
				if moduleConfig, exists := tt.config.Modules[tt.moduleName]; exists {
					enabled = moduleConfig.Enabled
				}
			}

			if enabled != tt.expected {
				t.Errorf("%s: Expected %v, got %v", tt.description, tt.expected, enabled)
			}
		})
	}
}

func TestLoadGlobalConfigFromPath_WithEnabled(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "metrics-agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name            string
		configContent   string
		expectedTasmota bool
		expectedNetatmo bool
		expectError     bool
	}{
		{
			name: "both modules enabled",
			configContent: `{
				"log_level": "info",
				"modules": {
					"tasmota": {
						"enabled": true,
						"custom": {"broker": "tcp://localhost:1883"}
					},
					"netatmo": {
						"enabled": true,
						"custom": {"client_id": "test"}
					}
				}
			}`,
			expectedTasmota: true,
			expectedNetatmo: true,
			expectError:     false,
		},
		{
			name: "only tasmota enabled",
			configContent: `{
				"log_level": "info",
				"modules": {
					"tasmota": {
						"enabled": true,
						"custom": {"broker": "tcp://localhost:1883"}
					},
					"netatmo": {
						"enabled": false,
						"custom": {"client_id": "test"}
					}
				}
			}`,
			expectedTasmota: true,
			expectedNetatmo: false,
			expectError:     false,
		},
		{
			name: "no enabled fields (default disabled)",
			configContent: `{
				"log_level": "info",
				"modules": {
					"tasmota": {
						"custom": {"broker": "tcp://localhost:1883"}
					},
					"netatmo": {
						"custom": {"client_id": "test"}
					}
				}
			}`,
			expectedTasmota: false,
			expectedNetatmo: false,
			expectError:     false,
		},
		{
			name: "no modules section",
			configContent: `{
				"log_level": "info"
			}`,
			expectedTasmota: false,
			expectedNetatmo: false,
			expectError:     false,
		},
		{
			name: "invalid JSON",
			configContent: `{
				"log_level": "info",
				"modules": {
					"tasmota": {
						"enabled": true,
						"custom": {"broker": "tcp://localhost:1883"}
					}
				}
			`, // Missing closing brace
			expectedTasmota: false,
			expectedNetatmo: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config file
			configPath := filepath.Join(tempDir, "test-config.json")
			err := os.WriteFile(configPath, []byte(tt.configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config file: %v", err)
			}

			// Load config
			config, err := LoadGlobalConfigFromPath(configPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error loading config: %v", err)
			}

			// Check tasmota module
			tasmotaEnabled := false
			if config.Modules != nil {
				if moduleConfig, exists := config.Modules["tasmota"]; exists {
					tasmotaEnabled = moduleConfig.Enabled
				}
			}
			if tasmotaEnabled != tt.expectedTasmota {
				t.Errorf("Tasmota enabled: expected %v, got %v", tt.expectedTasmota, tasmotaEnabled)
			}

			// Check netatmo module
			netatmoEnabled := false
			if config.Modules != nil {
				if moduleConfig, exists := config.Modules["netatmo"]; exists {
					netatmoEnabled = moduleConfig.Enabled
				}
			}
			if netatmoEnabled != tt.expectedNetatmo {
				t.Errorf("Netatmo enabled: expected %v, got %v", tt.expectedNetatmo, netatmoEnabled)
			}
		})
	}
}

func TestModuleConfig_JSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		config   ModuleConfig
		expected string
	}{
		{
			name: "enabled true with other fields",
			config: ModuleConfig{
				LogLevel: "debug",
				Enabled:  true,
				BaseConfig: BaseConfig{
					FriendlyNameOverrides: map[string]string{
						"device1": "Device 1",
					},
					Custom: map[string]interface{}{
						"broker": "tcp://localhost:1883",
					},
				},
			},
			expected: `{"log_level":"debug","enabled":true,"friendly_name_overrides":{"device1":"Device 1"},"custom":{"broker":"tcp://localhost:1883"}}`,
		},
		{
			name: "enabled false",
			config: ModuleConfig{
				Enabled: false,
			},
			expected: `{}`, // omitempty means false values are omitted
		},
		{
			name: "enabled not set (should not appear in JSON)",
			config: ModuleConfig{
				LogLevel: "info",
			},
			expected: `{"log_level":"info"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("Failed to marshal config: %v", err)
			}

			// Parse and re-marshal to normalize the JSON (handle map ordering)
			var parsed map[string]interface{}
			err = json.Unmarshal(jsonData, &parsed)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			normalized, err := json.Marshal(parsed)
			if err != nil {
				t.Fatalf("Failed to re-marshal JSON: %v", err)
			}

			var expectedParsed map[string]interface{}
			err = json.Unmarshal([]byte(tt.expected), &expectedParsed)
			if err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			expectedNormalized, err := json.Marshal(expectedParsed)
			if err != nil {
				t.Fatalf("Failed to marshal expected JSON: %v", err)
			}

			if string(normalized) != string(expectedNormalized) {
				t.Errorf("JSON serialization mismatch.\nExpected: %s\nGot: %s", string(expectedNormalized), string(normalized))
			}
		})
	}
}
