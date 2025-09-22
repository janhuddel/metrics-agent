// Package main implements the metrics-agent application.
// It runs all registered modules concurrently in a single process,
// designed to work with telegraf's inputs.execd plugin.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/janhuddel/metrics-agent/internal/ingest"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

func main() {
	// Create a root context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration with proper error handling
	config, err := utils.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize logger early
	logger := utils.InitLogger(config)
	slog.SetDefault(logger)
	slog.Info("metrics-agent starting", "version", "1.0.0")

	// Create controller with proper error handling
	controller, err := ingest.NewController(config)
	if err != nil {
		slog.Error("failed to create controller", "error", err)
		os.Exit(1)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Start controller with context and signal channel
	if err := controller.Start(ctx, sigChan); err != nil {
		slog.Error("controller failed", "error", err)
		os.Exit(1)
	}

	slog.Info("metrics-agent shutdown complete")
}
