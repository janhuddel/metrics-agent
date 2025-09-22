// Package dummy provides a dummy metric source for testing and development.
package dummy

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// DummySource implements a dummy metric source that generates fake temperature data.
type DummySource struct {
	interval time.Duration
	store    *utils.Store
}

// CreateInstance creates a new DummySource instance with the provided configuration.
// This function is used by the registry system.
func CreateInstance(config map[string]interface{}, store *utils.Store) types.Ingester {
	interval := 5 * time.Second // default interval
	store.Set("dummy", "interval", interval)

	// Parse interval from config if provided
	if intervalStr, ok := config["interval"].(string); ok {
		if parsed, err := time.ParseDuration(intervalStr); err == nil {
			interval = parsed
		}
	}

	return &DummySource{
		interval: interval,
		store:    store,
	}
}

// Name returns the name of this source.
func (s *DummySource) Name() string {
	return "dummy"
}

// Start begins generating dummy metrics at the configured interval.
// It sends structured temperature metrics.
func (s *DummySource) Start(ctx context.Context, out chan<- *types.Metric, gracefulShutdown <-chan struct{}, hardShutdown <-chan struct{}) error {
	// Use configured interval for measurements
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Panic simulation
	panicChan := make(chan struct{})
	go s.watchPanicFile(ctx, panicChan)

	// Simulate connection to external service (e.g., MQTT)
	connected := true
	slog.Debug("dummy source: connected to external service")

	for {
		select {
		case <-gracefulShutdown:
			slog.Debug("dummy source: received graceful shutdown, disconnecting from service...")
			// Simulate cleanup time (disconnect from MQTT, close connections, etc.)
			time.Sleep(2 * time.Second)
			connected = false
			slog.Debug("dummy source: gracefully disconnected from service")
			return nil
		case <-hardShutdown:
			slog.Debug("dummy source: received hard shutdown, terminating immediately")
			return nil
		case <-panicChan:
			panic("Demo module panic triggered by /tmp/metrics-agent-panic-demo file")
		case t := <-ticker.C:
			if connected {
				metric := types.NewMetric("temperature", s.Name(), t)
				metric.AddTag("source", "dummy")
				metric.AddField("value", 42)
				out <- metric
			}
		}
	}
}

// watchPanicFile watches for the panic file and sends a signal when found.
func (s *DummySource) watchPanicFile(ctx context.Context, panicChan chan<- struct{}) {
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
