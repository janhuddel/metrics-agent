package demo_test

import (
	"context"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/modules/demo"
)

// TestDemoModulePublishesMetrics tests that the demo module publishes metrics correctly.
func TestDemoModulePublishesMetrics(t *testing.T) {
	ch := make(chan metrics.Metric, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start module asynchronously
	go func() {
		_ = demo.Run(ctx, ch)
	}()

	select {
	case m := <-ch:
		if m.Name != "demo_metric" {
			t.Errorf("unexpected metric name: %s", m.Name)
		}
		if m.Tags["vendor"] != "demo" {
			t.Errorf("expected vendor=demo tag")
		}
		if _, ok := m.Fields["value"]; !ok {
			t.Errorf("expected 'value' field")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no metric received within 2s")
	}
}
