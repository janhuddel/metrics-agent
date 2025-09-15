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
	runAllModules()
}

// runAllModules starts all registered modules concurrently in a single process.
// It handles graceful shutdown on SIGTERM/SIGINT signals and module restart on SIGHUP.
// Provides panic recovery for each module to ensure the process remains stable.
func runAllModules() {
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
		moduleNames := modules.Global.List()
		if len(moduleNames) == 0 {
			log.Printf("No modules registered, exiting")
			metricCh.Close()
			cancel()
			return
		}

		log.Printf("Starting %d modules: %v", len(moduleNames), moduleNames)

		// Run all modules concurrently
		var wg sync.WaitGroup
		for _, moduleName := range moduleNames {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				utils.WithPanicRecoveryAndContinue("Module execution", name, func() {
					log.Printf("[%s] starting module", name)
					if err := modules.Global.Run(ctx, name, metricCh.Get()); err != nil {
						log.Printf("[%s] module error: %v", name, err)
					}
					log.Printf("[%s] module stopped", name)
				})
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
