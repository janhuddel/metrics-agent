// Package dummy provides a dummy metric source for testing and development.
package dummy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/janhuddel/metrics-agent/internal/sources"
)

// Source implements a dummy metric source that generates fake temperature data.
type Source struct{}

// New creates a new dummy source.
func New() *Source {
	return &Source{}
}

// Name returns the name of this source.
func (s *Source) Name() string {
	return "dummy"
}

// Start begins generating dummy metrics every 5 seconds.
// It sends temperature metrics in InfluxDB line protocol format.
func (s *Source) Start(ctx context.Context, out chan<- string, gracefulShutdown <-chan struct{}, hardShutdown <-chan struct{}) error {
	// Simplified example: every 5 seconds a measurement
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Panic simulation
	panicChan := make(chan struct{})
	go s.watchPanicFile(ctx, panicChan)

	// Simulate connection to external service (e.g., MQTT)
	connected := true
	slog.Info("dummy source: connected to external service")

	for {
		select {
		case <-gracefulShutdown:
			slog.Info("dummy source: received graceful shutdown, disconnecting from service...")
			// Simulate cleanup time (disconnect from MQTT, close connections, etc.)
			time.Sleep(2 * time.Second)
			connected = false
			slog.Info("dummy source: gracefully disconnected from service")
			return nil
		case <-hardShutdown:
			slog.Info("dummy source: received hard shutdown, terminating immediately")
			return nil
		case <-panicChan:
			panic("Demo module panic triggered by /tmp/metrics-agent-panic-demo file")
		case t := <-ticker.C:
			if connected {
				line := fmt.Sprintf("temperature,source=dummy value=42 %d", t.UnixNano())
				out <- line
			}
		}
	}
}

// watchPanicFile watches for the panic file and sends a signal when found.
func (s *Source) watchPanicFile(ctx context.Context, panicChan chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if _, err := os.Stat("/tmp/metrics-agent-panic-demo"); err == nil {
				panicChan <- struct{}{}
				return
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}
}

// Ensure Source implements the sources.Source interface.
var _ sources.Source = (*Source)(nil)
