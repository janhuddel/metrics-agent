// Package main implements the metrics-agent application.
// It runs all registered modules concurrently in a single process,
// designed to work with telegraf's inputs.execd plugin.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/janhuddel/metrics-agent/internal/config"
	"github.com/janhuddel/metrics-agent/internal/metricchannel"
	"github.com/janhuddel/metrics-agent/internal/modules"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

var (
	// flagVersion prints the version and exits
	flagVersion = flag.Bool("version", false, "Print version and exit")
	// flagConfig specifies the path to the configuration file
	flagConfig = flag.String("c", "", "Path to configuration file")
)

// version can be overridden at build time with -ldflags
var version = "dev"

// main is the entry point of the metrics-agent application.
// It initializes logging, parses command-line flags, and runs all modules
// concurrently in a single process.
func main() {
	// Parse flags first to get config path
	flag.Parse()

	// Handle version flag
	if *flagVersion {
		fmt.Fprintf(os.Stderr, "metrics-agent %s (%s %s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	// Set global config path for modules to use
	if *flagConfig != "" {
		config.GlobalConfigPath = *flagConfig
	}

	// Load global configuration first to set log level
	var globalConfig *config.GlobalConfig
	var err error
	var configPath string
	if *flagConfig != "" {
		configPath = *flagConfig
		globalConfig, err = config.LoadGlobalConfigFromPath(*flagConfig)
		if err != nil {
			utils.Fatalf("Failed to load configuration from specified file '%s': %v", *flagConfig, err)
		}
		utils.Infof("Using configuration file: %s", configPath)
	} else {
		globalConfig, err = config.LoadGlobalConfig()
		// Get the discovered config path for logging
		configPath = config.GetGlobalConfigPath()
		if configPath != "" {
			utils.Infof("Using configuration file: %s", configPath)
		} else {
			utils.Infof("No configuration file found, using defaults")
		}
		if err != nil {
			// If config loading fails, continue with default logging
			utils.Warnf("Failed to load global configuration: %v", err)
		}
	}

	// Set log level from configuration (defaults to info if not set)
	if globalConfig != nil && globalConfig.LogLevel != "" {
		config.SetLogLevel(globalConfig.LogLevel)
		utils.Debugf("Log level configured from config file: %s", globalConfig.LogLevel)
	} else {
		// Default to info level
		config.SetLogLevel("info")
		utils.Debugf("Using default log level: info")
	}

	// Run all modules in a single process
	runAllModules(globalConfig)
}

// ModuleManager handles the lifecycle of all metric collection modules.
type ModuleManager struct {
	globalConfig *config.GlobalConfig
	metricCh     *metricchannel.Channel
	signalCh     chan os.Signal
}

// NewModuleManager creates a new module manager instance.
func NewModuleManager(globalConfig *config.GlobalConfig) *ModuleManager {
	return &ModuleManager{
		globalConfig: globalConfig,
		signalCh:     make(chan os.Signal, 2),
	}
}

// runAllModules starts all registered modules concurrently in a single process.
// It handles graceful shutdown on SIGTERM/SIGINT signals and module restart on SIGHUP.
// Provides panic recovery for each module to ensure the process remains stable.
func runAllModules(globalConfig *config.GlobalConfig) {
	manager := NewModuleManager(globalConfig)
	manager.run()
}

// run executes the main module management loop.
func (mm *ModuleManager) run() {
	// Set up signal handling
	signal.Notify(mm.signalCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(mm.signalCh)

	// Channel to communicate signal type to main loop
	signalType := make(chan os.Signal, 1)

	// Signal handler goroutine
	go mm.handleSignals(signalType)

	for {
		// Set up context for graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())

		// Initialize metric channel and serializer
		if err := mm.initializeMetricChannel(); err != nil {
			utils.Errorf("Failed to initialize metric channel: %v", err)
			cancel()
			return
		}

		// Filter and validate enabled modules
		enabledModules, disabledModules := mm.filterEnabledModules()
		if len(enabledModules) == 0 {
			utils.Infof("No modules enabled, exiting")
			mm.cleanup(cancel)
			return
		}

		// Log module status
		mm.logModuleStatus(enabledModules, disabledModules)

		// Get restart configuration
		maxRestarts := mm.getRestartLimit()

		// Run all modules concurrently and wait for either completion or signal
		done := make(chan struct{})
		go func() {
			mm.runModules(ctx, enabledModules, maxRestarts)
			close(done)
		}()

		// Wait for either all modules to complete or a signal
		select {
		case sig := <-signalType:
			mm.handleShutdownSignal(sig, cancel)
			if sig == syscall.SIGHUP {
				continue // Restart the loop
			}
			return // Exit the process
		case <-done:
			// All modules completed normally
			utils.Infof("All modules completed normally")
			mm.cleanup(cancel)
			return
		}
	}
}

// handleSignals processes incoming signals and forwards them to the main loop.
func (mm *ModuleManager) handleSignals(signalType chan<- os.Signal) {
	utils.WithPanicRecoveryAndContinue("Signal handler", "main", func() {
		for {
			sig := <-mm.signalCh
			utils.Infof("Received signal: %s", sig)
			signalType <- sig
		}
	})
}

// initializeMetricChannel creates and starts the metric channel and serializer.
func (mm *ModuleManager) initializeMetricChannel() error {
	mm.metricCh = metricchannel.New(100)
	utils.Debugf("Created metric channel with buffer size: 100")

	mm.metricCh.StartSerializer()
	utils.Debugf("Started metric serializer")

	return nil
}

// filterEnabledModules returns lists of enabled and disabled modules based on configuration.
func (mm *ModuleManager) filterEnabledModules() (enabled, disabled []string) {
	allModuleNames := modules.Global.List()
	utils.Debugf("Found %d registered modules: %v", len(allModuleNames), allModuleNames)

	if len(allModuleNames) == 0 {
		utils.Infof("No modules registered, exiting")
		return nil, nil
	}

	return filterEnabledModules(allModuleNames, mm.globalConfig)
}

// logModuleStatus logs the status of enabled and disabled modules.
func (mm *ModuleManager) logModuleStatus(enabled, disabled []string) {
	if len(disabled) > 0 {
		utils.Infof("Disabled modules: %v", disabled)
	}
	utils.Infof("Starting %d enabled modules: %v", len(enabled), enabled)
}

// getRestartLimit returns the configured restart limit with appropriate logging.
func (mm *ModuleManager) getRestartLimit() int {
	maxRestarts := 3 // Default value
	if mm.globalConfig != nil {
		if mm.globalConfig.ModuleRestartLimit == 0 {
			maxRestarts = 0 // 0 means unlimited restarts
		} else if mm.globalConfig.ModuleRestartLimit > 0 {
			maxRestarts = mm.globalConfig.ModuleRestartLimit
		}
		// If ModuleRestartLimit < 0, use default (3)
	}

	if maxRestarts == 0 {
		utils.Infof("Module restart limit: unlimited")
		utils.Warnf("Unlimited restarts (limit=0) is NOT recommended for telegraf/systemd deployments!")
	} else {
		utils.Infof("Module restart limit: %d", maxRestarts)
	}

	return maxRestarts
}

// runModules starts all enabled modules concurrently with restart capability.
func (mm *ModuleManager) runModules(ctx context.Context, moduleNames []string, maxRestarts int) {
	var wg sync.WaitGroup
	for _, moduleName := range moduleNames {
		wg.Add(1)
		go mm.runModule(ctx, &wg, moduleName, maxRestarts)
	}

	// Wait for all modules to complete
	wg.Wait()
}

// runModule runs a single module with restart capability.
func (mm *ModuleManager) runModule(ctx context.Context, wg *sync.WaitGroup, moduleName string, maxRestarts int) {
	defer wg.Done()

	restartCount := 0

	for {
		// Check for context cancellation before each iteration
		select {
		case <-ctx.Done():
			utils.Infof("[%s] module stopped due to context cancellation", moduleName)
			return
		default:
		}

		// Execute the module
		mm.executeModule(ctx, moduleName, restartCount, maxRestarts)

		// Check for context cancellation after module execution
		select {
		case <-ctx.Done():
			utils.Infof("[%s] module stopped due to context cancellation", moduleName)
			return
		default:
		}

		// Increment restart count and check limits
		restartCount++
		if maxRestarts > 0 && restartCount >= maxRestarts {
			utils.Errorf("[%s] module failed %d times, exiting program", moduleName, restartCount)
			return
		}

		// Log restart and wait with context cancellation support
		mm.logRestart(moduleName, restartCount, maxRestarts)

		// Use context-aware sleep instead of time.Sleep
		select {
		case <-ctx.Done():
			utils.Infof("[%s] module stopped due to context cancellation during restart delay", moduleName)
			return
		case <-time.After(1 * time.Second):
			// Continue to next iteration
		}
	}
}

// executeModule runs a single module execution with panic recovery.
func (mm *ModuleManager) executeModule(ctx context.Context, moduleName string, restartCount, maxRestarts int) {
	utils.WithPanicRecoveryAndContinue("Module execution", moduleName, func() {
		if maxRestarts == 0 {
			utils.Infof("[%s] starting module (attempt %d/unlimited)", moduleName, restartCount+1)
		} else {
			utils.Infof("[%s] starting module (attempt %d/%d)", moduleName, restartCount+1, maxRestarts+1)
		}
		if err := modules.Global.Run(ctx, moduleName, mm.metricCh.Get()); err != nil {
			utils.Errorf("[%s] module error: %v", moduleName, err)
		}
		utils.Infof("[%s] module stopped", moduleName)
	})
}

// logRestart logs module restart information.
func (mm *ModuleManager) logRestart(moduleName string, restartCount, maxRestarts int) {
	if maxRestarts == 0 {
		utils.Infof("[%s] restarting module after completion/panic (restart %d/unlimited)", moduleName, restartCount)
	} else {
		utils.Infof("[%s] restarting module after completion/panic (restart %d/%d)", moduleName, restartCount, maxRestarts)
	}
}

// handleShutdownSignal processes shutdown signals and cleans up resources.
func (mm *ModuleManager) handleShutdownSignal(sig os.Signal, cancel context.CancelFunc) {
	utils.Infof("Received %s, stopping modules...", sig)
	cancel() // Stop all modules

	// Clean up resources
	mm.cleanup(cancel)

	switch sig {
	case syscall.SIGHUP:
		utils.Infof("Restarting all modules...")
	case syscall.SIGTERM, syscall.SIGINT:
		utils.Infof("Shutting down...")
	}
}

// cleanup closes the metric channel and cancels the context.
func (mm *ModuleManager) cleanup(cancel context.CancelFunc) {
	if mm.metricCh != nil {
		mm.metricCh.Close()
	}
	cancel()
}

// filterEnabledModules filters modules based on enabled configuration.
// This function is extracted to be reusable between main.go and tests.
func filterEnabledModules(allModuleNames []string, globalConfig *config.GlobalConfig) (enabled, disabled []string) {
	for _, moduleName := range allModuleNames {
		enabledFlag := false
		if globalConfig != nil && globalConfig.Modules != nil {
			if moduleConfig, exists := globalConfig.Modules[moduleName]; exists {
				enabledFlag = moduleConfig.Enabled
				utils.Debugf("Module %s: enabled=%v (from config)", moduleName, enabledFlag)
			} else {
				utils.Debugf("Module %s: enabled=false (no config found)", moduleName)
			}
		} else {
			utils.Debugf("Module %s: enabled=false (no global config)", moduleName)
		}

		if enabledFlag {
			enabled = append(enabled, moduleName)
		} else {
			disabled = append(disabled, moduleName)
		}
	}
	return enabled, disabled
}
