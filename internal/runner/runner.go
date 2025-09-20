// Package runner provides safe execution and retry logic for metric sources.
package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
)

// RetryConfig holds configuration for retry logic.
type RetryConfig struct {
	MaxRetries int           // Maximum number of retries before crashing
	BaseDelay  time.Duration // Base delay between retries, e.g. 1s
	MaxDelay   time.Duration // Maximum delay between retries, e.g. 30s
}

// SafeRun executes a source with retry logic and panic recovery.
// It will restart the source on failure up to MaxRetries times,
// then crash the process if all retries are exhausted.
// The shutdown channels provide centralized shutdown control.
func SafeRun(ctx context.Context, s types.Source, out chan<- string, gracefulShutdown <-chan struct{}, hardShutdown <-chan struct{}, cfg RetryConfig) {
	var attempt int

	for {
		// Panic protection
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in %s: %v", s.Name(), r)
				}
			}()
			return s.Start(ctx, out, gracefulShutdown, hardShutdown)
		}()

		// Normal exit due to context cancellation
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			slog.Info("stopping source (context canceled)", "name", s.Name())
			return
		}

		// Successful graceful shutdown (source returned nil)
		if err == nil {
			slog.Info("source completed gracefully", "name", s.Name())
			return
		}

		// Error case → Retry
		attempt++
		if attempt > cfg.MaxRetries {
			slog.Error("source retries exhausted, crashing",
				"name", s.Name(), "err", err)
			os.Exit(1) // Crash → Control back to Telegraf
		}

		// Calculate backoff
		backoff := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if backoff > cfg.MaxDelay {
			backoff = cfg.MaxDelay
		}

		slog.Warn("source failed, will retry",
			"name", s.Name(), "attempt", attempt, "err", err, "backoff", backoff)

		select {
		case <-time.After(backoff):
			// Continue retry loop
		case <-gracefulShutdown:
			slog.Info("stopping source during backoff (graceful shutdown)", "name", s.Name())
			return
		case <-hardShutdown:
			slog.Info("stopping source during backoff (hard shutdown)", "name", s.Name())
			return
		}
	}
}
