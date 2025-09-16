package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/janhuddel/metrics-agent/internal/config"
	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/modules"
)

// Test helper function to filter enabled modules
func filterEnabledModules(allModuleNames []string, globalConfig *config.GlobalConfig) (enabled []string, disabled []string) {
	for _, moduleName := range allModuleNames {
		enabledFlag := false
		if globalConfig != nil && globalConfig.Modules != nil {
			if moduleConfig, exists := globalConfig.Modules[moduleName]; exists {
				enabledFlag = moduleConfig.Enabled
			}
		}

		if enabledFlag {
			enabled = append(enabled, moduleName)
		} else {
			disabled = append(disabled, moduleName)
		}
	}
	return enabled, disabled
}

func TestFilterEnabledModules(t *testing.T) {
	// Register test modules
	testRegistry := modules.NewRegistry()
	testRegistry.Register("module1", func(ctx context.Context, ch chan<- metrics.Metric) error { return nil })
	testRegistry.Register("module2", func(ctx context.Context, ch chan<- metrics.Metric) error { return nil })
	testRegistry.Register("module3", func(ctx context.Context, ch chan<- metrics.Metric) error { return nil })

	allModuleNames := testRegistry.List()

	tests := []struct {
		name             string
		globalConfig     *config.GlobalConfig
		expectedEnabled  []string
		expectedDisabled []string
		description      string
	}{
		{
			name: "all modules enabled",
			globalConfig: &config.GlobalConfig{
				Modules: map[string]config.ModuleConfig{
					"module1": {Enabled: true},
					"module2": {Enabled: true},
					"module3": {Enabled: true},
				},
			},
			expectedEnabled:  []string{"module1", "module2", "module3"},
			expectedDisabled: []string{},
			description:      "All modules should be enabled when explicitly set to true",
		},
		{
			name: "some modules enabled",
			globalConfig: &config.GlobalConfig{
				Modules: map[string]config.ModuleConfig{
					"module1": {Enabled: true},
					"module2": {Enabled: false},
					"module3": {Enabled: true},
				},
			},
			expectedEnabled:  []string{"module1", "module3"},
			expectedDisabled: []string{"module2"},
			description:      "Only explicitly enabled modules should be enabled",
		},
		{
			name: "no modules enabled",
			globalConfig: &config.GlobalConfig{
				Modules: map[string]config.ModuleConfig{
					"module1": {Enabled: false},
					"module2": {Enabled: false},
					"module3": {Enabled: false},
				},
			},
			expectedEnabled:  []string{},
			expectedDisabled: []string{"module1", "module2", "module3"},
			description:      "No modules should be enabled when all set to false",
		},
		{
			name: "enabled not specified (default disabled)",
			globalConfig: &config.GlobalConfig{
				Modules: map[string]config.ModuleConfig{
					"module1": {LogLevel: "debug"}, // enabled not specified
					"module2": {Enabled: true},
					"module3": {LogLevel: "info"}, // enabled not specified
				},
			},
			expectedEnabled:  []string{"module2"},
			expectedDisabled: []string{"module1", "module3"},
			description:      "Modules without enabled field should default to disabled",
		},
		{
			name: "modules not in config (default disabled)",
			globalConfig: &config.GlobalConfig{
				Modules: map[string]config.ModuleConfig{
					"module2": {Enabled: true},
					// module1 and module3 not in config
				},
			},
			expectedEnabled:  []string{"module2"},
			expectedDisabled: []string{"module1", "module3"},
			description:      "Modules not in config should default to disabled",
		},
		{
			name:             "nil global config (all disabled)",
			globalConfig:     nil,
			expectedEnabled:  []string{},
			expectedDisabled: []string{"module1", "module2", "module3"},
			description:      "All modules should be disabled when global config is nil",
		},
		{
			name: "nil modules config (all disabled)",
			globalConfig: &config.GlobalConfig{
				Modules: nil,
			},
			expectedEnabled:  []string{},
			expectedDisabled: []string{"module1", "module2", "module3"},
			description:      "All modules should be disabled when modules config is nil",
		},
		{
			name: "empty modules config (all disabled)",
			globalConfig: &config.GlobalConfig{
				Modules: map[string]config.ModuleConfig{},
			},
			expectedEnabled:  []string{},
			expectedDisabled: []string{"module1", "module2", "module3"},
			description:      "All modules should be disabled when modules config is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, disabled := filterEnabledModules(allModuleNames, tt.globalConfig)

			// Check enabled modules
			if len(enabled) != len(tt.expectedEnabled) {
				t.Errorf("%s: Expected %d enabled modules, got %d", tt.description, len(tt.expectedEnabled), len(enabled))
			}

			// Check that all expected enabled modules are present
			enabledMap := make(map[string]bool)
			for _, module := range enabled {
				enabledMap[module] = true
			}
			for _, expectedModule := range tt.expectedEnabled {
				if !enabledMap[expectedModule] {
					t.Errorf("%s: Expected module %s to be enabled, but it wasn't", tt.description, expectedModule)
				}
			}

			// Check disabled modules
			if len(disabled) != len(tt.expectedDisabled) {
				t.Errorf("%s: Expected %d disabled modules, got %d", tt.description, len(tt.expectedDisabled), len(disabled))
			}

			// Check that all expected disabled modules are present
			disabledMap := make(map[string]bool)
			for _, module := range disabled {
				disabledMap[module] = true
			}
			for _, expectedModule := range tt.expectedDisabled {
				if !disabledMap[expectedModule] {
					t.Errorf("%s: Expected module %s to be disabled, but it wasn't", tt.description, expectedModule)
				}
			}
		})
	}
}

func TestModuleFilteringEdgeCases(t *testing.T) {
	t.Run("no registered modules", func(t *testing.T) {
		emptyRegistry := modules.NewRegistry()
		allModuleNames := emptyRegistry.List()

		globalConfig := &config.GlobalConfig{
			Modules: map[string]config.ModuleConfig{
				"nonexistent": {Enabled: true},
			},
		}

		enabled, disabled := filterEnabledModules(allModuleNames, globalConfig)

		if len(enabled) != 0 {
			t.Errorf("Expected 0 enabled modules, got %d", len(enabled))
		}
		if len(disabled) != 0 {
			t.Errorf("Expected 0 disabled modules, got %d", len(disabled))
		}
	})

	t.Run("module in config but not registered", func(t *testing.T) {
		testRegistry := modules.NewRegistry()
		testRegistry.Register("registered-module", func(ctx context.Context, ch chan<- metrics.Metric) error { return nil })
		allModuleNames := testRegistry.List()

		globalConfig := &config.GlobalConfig{
			Modules: map[string]config.ModuleConfig{
				"registered-module":   {Enabled: true},
				"unregistered-module": {Enabled: true}, // This won't be in allModuleNames
			},
		}

		enabled, disabled := filterEnabledModules(allModuleNames, globalConfig)

		// Only the registered module should be considered
		if len(enabled) != 1 || enabled[0] != "registered-module" {
			t.Errorf("Expected only 'registered-module' to be enabled, got %v", enabled)
		}
		if len(disabled) != 0 {
			t.Errorf("Expected 0 disabled modules, got %d", len(disabled))
		}
	})
}

// Benchmark the module filtering function
func BenchmarkFilterEnabledModules(b *testing.B) {
	// Create a large number of modules
	testRegistry := modules.NewRegistry()
	for i := 0; i < 100; i++ {
		moduleName := fmt.Sprintf("module%d", i)
		testRegistry.Register(moduleName, func(ctx context.Context, ch chan<- metrics.Metric) error { return nil })
	}
	allModuleNames := testRegistry.List()

	// Create config with half enabled, half disabled
	globalConfig := &config.GlobalConfig{
		Modules: make(map[string]config.ModuleConfig),
	}
	for i, moduleName := range allModuleNames {
		globalConfig.Modules[moduleName] = config.ModuleConfig{
			Enabled: i%2 == 0, // Alternate between enabled/disabled
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filterEnabledModules(allModuleNames, globalConfig)
	}
}
