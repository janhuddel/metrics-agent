// Package main implements the metrics-agent application.
// It runs all registered modules concurrently in a single process,
// designed to work with telegraf's inputs.execd plugin.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	// Configure logging to stderr since stdout is reserved for metrics (Line Protocol)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[metrics-agent] ")

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
			log.Fatalf("Failed to load configuration from specified file '%s': %v", *flagConfig, err)
		}
		log.Printf("Using configuration file: %s", configPath)
	} else {
		globalConfig, err = config.LoadGlobalConfig()
		// Get the discovered config path for logging
		configPath = config.GetGlobalConfigPath()
		if configPath != "" {
			log.Printf("Using configuration file: %s", configPath)
		} else {
			log.Printf("No configuration file found, using defaults")
		}
		if err != nil {
			// If config loading fails, continue with default logging
			log.Printf("Warning: Failed to load global configuration: %v", err)
		}
	}

	// Set log level from configuration (defaults to info if not set)
	if globalConfig != nil && globalConfig.LogLevel != "" {
		config.SetLogLevel(globalConfig.LogLevel)
	} else {
		// Default to info level
		config.SetLogLevel("info")
	}

	// Run all modules in a single process
	runAllModules(globalConfig)
}

// runAllModules starts all registered modules concurrently in a single process.
// It handles graceful shutdown on SIGTERM/SIGINT signals and module restart on SIGHUP.
// Provides panic recovery for each module to ensure the process remains stable.
func runAllModules(globalConfig *config.GlobalConfig) {
	// Set up signal handling once
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Channel to communicate signal type to main loop
	signalType := make(chan os.Signal, 1)

	// Signal handler goroutine
	go func() {
		utils.WithPanicRecoveryAndContinue("Signal handler", "main", func() {
			for {
				sig := <-sigCh
				log.Printf("Received signal: %s", sig)
				signalType <- sig
			}
		})
	}()

	for {
		// Set up context for graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())

		// Create metric channel for all modules to share
		metricCh := metricchannel.New(100)

		// Start metric serializer
		metricCh.StartSerializer()

		// Get list of all registered modules
		allModuleNames := modules.Global.List()
		if len(allModuleNames) == 0 {
			log.Printf("No modules registered, exiting")
			metricCh.Close()
			cancel()
			return
		}

		// Filter modules based on enabled configuration
		var moduleNames []string
		var disabledModules []string

		for _, moduleName := range allModuleNames {
			enabled := false
			if globalConfig != nil && globalConfig.Modules != nil {
				if moduleConfig, exists := globalConfig.Modules[moduleName]; exists {
					enabled = moduleConfig.Enabled
				}
			}

			if enabled {
				moduleNames = append(moduleNames, moduleName)
			} else {
				disabledModules = append(disabledModules, moduleName)
			}
		}

		if len(disabledModules) > 0 {
			log.Printf("Disabled modules: %v", disabledModules)
		}

		if len(moduleNames) == 0 {
			log.Printf("No modules enabled, exiting")
			metricCh.Close()
			cancel()
			return
		}

		log.Printf("Starting %d enabled modules: %v", len(moduleNames), moduleNames)

		// Log restart limit configuration
		maxRestarts := 3 // Default value
		if globalConfig != nil {
			if globalConfig.ModuleRestartLimit == 0 {
				maxRestarts = 0 // 0 means unlimited restarts
			} else if globalConfig.ModuleRestartLimit > 0 {
				maxRestarts = globalConfig.ModuleRestartLimit
			}
			// If ModuleRestartLimit < 0, use default (3)
		}

		if maxRestarts == 0 {
			log.Printf("Module restart limit: unlimited")
			log.Printf("WARNING: Unlimited restarts (limit=0) is NOT recommended for telegraf/systemd deployments!")
		} else {
			log.Printf("Module restart limit: %d", maxRestarts)
		}

		// Run all modules concurrently with individual restart capability
		var wg sync.WaitGroup
		for _, moduleName := range moduleNames {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()

				// Individual module restart loop with limit
				restartCount := 0

				for {
					select {
					case <-ctx.Done():
						log.Printf("[%s] module stopped due to context cancellation", name)
						return
					default:
						utils.WithPanicRecoveryAndContinue("Module execution", name, func() {
							if maxRestarts == 0 {
								log.Printf("[%s] starting module (attempt %d/unlimited)", name, restartCount+1)
							} else {
								log.Printf("[%s] starting module (attempt %d/%d)", name, restartCount+1, maxRestarts+1)
							}
							if err := modules.Global.Run(ctx, name, metricCh.Get()); err != nil {
								log.Printf("[%s] module error: %v", name, err)
							}
							log.Printf("[%s] module stopped", name)
						})

						// Check if we should restart or exit
						select {
						case <-ctx.Done():
							return
						default:
							restartCount++
							if maxRestarts > 0 && restartCount >= maxRestarts {
								log.Printf("[%s] module failed %d times, exiting program", name, restartCount)
								// Signal other modules to stop and exit
								cancel()
								return
							}
							if maxRestarts == 0 {
								log.Printf("[%s] restarting module after completion/panic (restart %d/unlimited)", name, restartCount)
							} else {
								log.Printf("[%s] restarting module after completion/panic (restart %d/%d)", name, restartCount, maxRestarts)
							}
							time.Sleep(1 * time.Second) // Brief delay before restart
						}
					}
				}
			}(moduleName)
		}

		// Wait for either all modules to complete or a signal
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case sig := <-signalType:
			log.Printf("Received %s, stopping modules...", sig)
			cancel()  // Stop all modules
			wg.Wait() // Wait for modules to stop

			// Clean up resources
			metricCh.Close()

			switch sig {
			case syscall.SIGHUP:
				log.Printf("Restarting all modules...")
				continue // Restart the loop
			case syscall.SIGTERM, syscall.SIGINT:
				log.Printf("Shutting down...")
				return // Exit the process
			}
		case <-done:
			// All modules completed normally
			log.Printf("All modules completed normally")
			metricCh.Close()
			cancel()
			return
		}
	}
}
