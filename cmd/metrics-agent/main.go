// Package main implements the metrics-agent application.
// It runs all registered modules concurrently in a single process,
// designed to work with telegraf's inputs.execd plugin.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/janhuddel/metrics-agent/internal/runner"
	"github.com/janhuddel/metrics-agent/internal/sources"
	"github.com/janhuddel/metrics-agent/internal/sources/dummy"
)

func main() {
	// Logging auf STDERR
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Set up signal handling for graceful vs hard shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Shared channels
	out := make(chan string, 100)
	errs := make(chan error, 10)
	gracefulShutdown := make(chan struct{}) // Graceful shutdown signal
	hardShutdown := make(chan struct{})     // Hard shutdown signal

	retryCfg := runner.RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}

	// Register sources
	sourceList := []sources.Source{
		dummy.New(),
	}

	// WaitGroup to track source completion
	var wg sync.WaitGroup
	completionChan := make(chan struct{})

	// Start all sources with shutdown channels
	for _, s := range sourceList {
		wg.Add(1)
		go func(source sources.Source) {
			defer wg.Done()
			runner.SafeRun(context.Background(), source, out, gracefulShutdown, hardShutdown, retryCfg)
		}(s)
		logger.Info("started source", "name", s.Name())
	}

	// Goroutine to signal completion when all sources finish
	go func() {
		wg.Wait()
		close(completionChan)
	}()

	// Main loop: metrics to STDOUT, errors to STDERR, signal handling
	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGTERM:
				logger.Info("received SIGTERM, initiating graceful shutdown")
				close(gracefulShutdown)

				// Wait for graceful shutdown with timeout
				gracefulTimeout := 30 * time.Second
				select {
				case <-time.After(gracefulTimeout):
					logger.Warn("graceful shutdown timeout exceeded, forcing hard shutdown")
					close(hardShutdown)
					return
				case <-completionChan:
					logger.Info("all sources completed gracefully")
					return
				}
			case syscall.SIGINT:
				logger.Info("received SIGINT, initiating hard shutdown")
				close(hardShutdown)
				return
			}
		case line := <-out:
			fmt.Println(line) // only metrics → STDOUT
		case err := <-errs:
			logger.Error("source error", "err", err)
		}
	}
}
