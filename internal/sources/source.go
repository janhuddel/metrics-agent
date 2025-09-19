// Package sources defines interfaces and implementations for metric sources.
package sources

import "context"

// Source represents a metric source that can generate metrics.
type Source interface {
	// Name returns the unique name of this source.
	Name() string

	// Start begins generating metrics and sends them to the output channel.
	// It should run until graceful or hard shutdown signal is received, or an error occurs.
	// gracefulShutdown: allows time for cleanup (disconnect from services, close connections)
	// hardShutdown: immediate termination, no cleanup time
	Start(ctx context.Context, out chan<- string, gracefulShutdown <-chan struct{}, hardShutdown <-chan struct{}) error
}
