package metricchannel

import (
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

func TestChannel(t *testing.T) {
	// Create a new channel
	ch := New(10)
	defer ch.Close()

	// Test that we can get the underlying channel
	metricCh := ch.Get()
	if metricCh == nil {
		t.Fatal("Get() returned nil channel")
	}

	// Test that we can send a metric
	testMetric := metrics.Metric{
		Name:      "test_metric",
		Tags:      map[string]string{"host": "test"},
		Fields:    map[string]interface{}{"value": 42},
		Timestamp: time.Now(),
	}

	select {
	case metricCh <- testMetric:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Failed to send metric to channel")
	}

	// Test context
	ctx := ch.Context()
	if ctx == nil {
		t.Fatal("Context() returned nil")
	}

	// Test that context is not cancelled initially
	select {
	case <-ctx.Done():
		t.Fatal("Context should not be cancelled initially")
	default:
		// Expected
	}
}

func TestChannelClose(t *testing.T) {
	ch := New(10)

	// Start serializer
	ch.StartSerializer()

	// Send a metric
	metricCh := ch.Get()
	testMetric := metrics.Metric{
		Name:      "test_metric",
		Tags:      map[string]string{"host": "test"},
		Fields:    map[string]interface{}{"value": 42},
		Timestamp: time.Now(),
	}

	metricCh <- testMetric

	// Close the channel
	ch.Close()

	// Verify context is cancelled
	ctx := ch.Context()
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be cancelled after Close()")
	}
}
