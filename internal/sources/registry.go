// Package modules provides a registry system for metric collection modules.
// It allows dynamic registration and execution of different metric collection
// modules through a unified interface.
//
// The package supports:
// - Module registration and discovery
// - Unified module execution interface
// - Panic recovery for module execution
// - Configurable module support
package sources

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// SourceCreatorFunc represents a function that creates a metric collection source.
// It receives a configuration map and returns a Source instance.
type SourceCreatorFunc func(config map[string]any) types.Source

// Registry holds all available metric collection modules.
// It provides thread-safe access to registered modules and their execution.
type Registry struct {
	modules map[string]SourceCreatorFunc
}

// NewRegistry creates a new module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]SourceCreatorFunc),
	}
}

// Register adds a module to the registry.
// If a module with the same name already exists, it will be overwritten.
func (r *Registry) Register(name string, fn SourceCreatorFunc) {
	r.modules[name] = fn
}

// GetEnabledSources returns all enabled sources from the registry.
func (r *Registry) GetEnabledSources(config *utils.AppConfig) []types.Source {
	sources := []types.Source{}
	for sourceName, sourceCreator := range r.modules {
		slog.Debug("source", "name", sourceName, "enabled", config.IsSourceEnabled(sourceName))
		if config.IsSourceEnabled(sourceName) {
			sources = append(sources, sourceCreator(config.GetSourceConfig(sourceName)))
		}
	}
	return sources
}

// StartSource executes a source with retry logic and panic recovery.
// It will restart the source on failure up to MaxRetries times,
// then crash the process if all retries are exhausted.
// The shutdown channels provide centralized shutdown control.
func (r *Registry) StartSource(
	ctx context.Context,
	source types.Source,
	out chan<- string,
	gracefulShutdown <-chan struct{},
	hardShutdown <-chan struct{},
	cfg types.RetryConfig) {

	var attempt int

	for {
		// Panic protection
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in %s: %v", source.Name(), r)
				}
			}()
			return source.Start(ctx, out, gracefulShutdown, hardShutdown)
		}()

		// Normal exit due to context cancellation
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			slog.Info("stopping source (context canceled)", "name", source.Name())
			return
		}

		// Successful graceful shutdown (source returned nil)
		if err == nil {
			slog.Info("source completed gracefully", "name", source.Name())
			return
		}

		// Error case → Retry
		attempt++
		if attempt > cfg.MaxRetries {
			slog.Error("source retries exhausted, crashing",
				"name", source.Name(), "err", err)
			os.Exit(1) // Crash → Control back to Telegraf
		}

		// Calculate backoff
		backoff := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if backoff > cfg.MaxDelay {
			backoff = cfg.MaxDelay
		}

		slog.Warn("source failed, will retry",
			"name", source.Name(), "attempt", attempt, "err", err, "backoff", backoff)

		select {
		case <-time.After(backoff):
			// Continue retry loop
		case <-gracefulShutdown:
			slog.Info("stopping source during backoff (graceful shutdown)", "name", source.Name())
			return
		case <-hardShutdown:
			slog.Info("stopping source during backoff (hard shutdown)", "name", source.Name())
			return
		}
	}
}
