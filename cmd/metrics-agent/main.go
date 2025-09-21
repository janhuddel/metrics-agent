// Package main implements the metrics-agent application.
// It runs all registered modules concurrently in a single process,
// designed to work with telegraf's inputs.execd plugin.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/janhuddel/metrics-agent/internal/ingest"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

func main() {
	config, err := utils.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to load config:", err)
		os.Exit(1)
	}

	// Init Logger
	logger := utils.InitLogger(config)
	slog.SetDefault(logger)

	controller, err := ingest.NewController(config)
	if err != nil {
		slog.Error("unable to create controller", "err", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful vs hard shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	controller.Start(sigChan)
}
