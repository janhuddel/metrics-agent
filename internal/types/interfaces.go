// Package sources defines interfaces for metric sources.
package types

import (
	"context"
	"time"
)

// RetryConfig holds configuration for retry logic.
type RetryConfig struct {
	MaxRetries int           // Maximum number of retries before crashing
	BaseDelay  time.Duration // Base delay between retries, e.g. 1s
	MaxDelay   time.Duration // Maximum delay between retries, e.g. 30s
}

// Ingester represents a metric source that can generate metrics.
type Ingester interface {
	// Name returns the unique name of this source.
	Name() string

	// Start begins generating metrics and sends them to the output channel.
	// It should run until graceful or hard shutdown signal is received, or an error occurs.
	// gracefulShutdown: allows time for cleanup (disconnect from services, close connections)
	// hardShutdown: immediate termination, no cleanup time
	Start(ctx context.Context, out chan<- string, gracefulShutdown <-chan struct{}, hardShutdown <-chan struct{}) error
}
