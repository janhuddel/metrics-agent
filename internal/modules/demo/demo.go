// Package demo provides a demonstration metric collection module.
// It generates sample metrics at regular intervals for testing purposes.
package demo

import (
	"context"
	"math/rand/v2"
	"os"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// Run generates demo metrics every 5 seconds and sends them through the channel.
// It runs until the context is cancelled.
// Panic simulation: If file "/tmp/metrics-agent-panic-demo" exists, the module will panic.
func Run(ctx context.Context, ch chan<- metrics.Metric) error {
	host, _ := os.Hostname()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send first metric immediately on start
	ch <- makeMetric(host)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Check for panic trigger file before sending metric
			if _, err := os.Stat("/tmp/metrics-agent-panic-demo"); err == nil {
				panic("Demo module panic triggered by /tmp/metrics-agent-panic-demo file")
			}
			ch <- makeMetric(host)
		}
	}
}

// makeMetric creates a demo metric with random values.
func makeMetric(host string) metrics.Metric {
	return metrics.Metric{
		Name: "demo_metric",
		Tags: map[string]string{
			"vendor": "demo",
			"host":   host,
		},
		Fields: map[string]interface{}{
			"value": 10 + rand.IntN(90),
		},
		Timestamp: time.Now(),
	}
}
