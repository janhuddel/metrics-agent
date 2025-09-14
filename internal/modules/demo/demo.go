package demo

import (
	"context"
	"math/rand/v2"
	"os"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// Run erzeugt alle 10 Sekunden eine Demo-Metrik
// und sendet sie über den Channel an den Supervisor.
func Run(ctx context.Context, ch chan<- metrics.Metric) error {
	host, _ := os.Hostname()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Erste Metrik direkt beim Start
	ch <- makeMetric(host)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			ch <- makeMetric(host)
		}
	}
}

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
