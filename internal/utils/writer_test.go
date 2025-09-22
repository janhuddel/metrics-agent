package utils

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
)

func TestLineProtocolWriter(t *testing.T) {
	writer := NewLineProtocolWriter()

	// Create test channels
	metricChan := make(chan *types.Metric, 10)

	// Start the writer in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go writer.Start(ctx, metricChan)

	// Create a test metric
	metric := types.NewMetric("temperature", "dummy", time.Now())
	metric.AddTag("source", "dummy")
	metric.AddField("value", 42)

	// Send the metric
	metricChan <- metric

	// Wait a bit for processing
	time.Sleep(10 * time.Millisecond)

	// Note: Since the writer now writes directly to stdout, we can't easily capture
	// the output in a test without redirecting stdout. For now, we just verify
	// that the writer processes the metric without panicking.

	// Test explicit stop
	writer.Stop()

	// Wait a bit for the stop to take effect
	time.Sleep(10 * time.Millisecond)
}

func TestConvertToLineProtocol(t *testing.T) {
	writer := NewLineProtocolWriter()

	// Create a test metric
	metric := types.NewMetric("temperature", "dummy", time.Unix(1234567890, 0))
	metric.AddTag("source", "dummy")
	metric.AddTag("location", "server1")
	metric.AddField("value", 42)
	metric.AddField("humidity", 65.5)

	// Convert to line protocol
	line := writer.ConvertToLineProtocol(metric)

	// Basic validation
	if line == "" {
		t.Error("Expected non-empty line protocol output")
	}

	// Check that it contains expected elements
	if !strings.Contains(line, "temperature") {
		t.Error("Expected line to contain measurement name 'temperature'")
	}

	if !strings.Contains(line, "source=dummy") {
		t.Error("Expected line to contain tag 'source=dummy'")
	}

	if !strings.Contains(line, "value=42i") {
		t.Error("Expected line to contain field 'value=42i'")
	}

	if !strings.Contains(line, "humidity=65.5") {
		t.Error("Expected line to contain field 'humidity=65.5'")
	}

	if !strings.Contains(line, "1234567890000000000") {
		t.Error("Expected line to contain timestamp")
	}

	t.Logf("Generated line protocol: %s", line)
}

func TestDrain(t *testing.T) {
	writer := NewLineProtocolWriter()

	// Create a test channel with some metrics
	metricChan := make(chan *types.Metric, 5)

	// Add some test metrics to the channel
	for i := 0; i < 3; i++ {
		metric := types.NewMetric("test", "dummy", time.Now())
		metric.AddField("value", i)
		metricChan <- metric
	}

	// Test draining the metrics
	writer.Drain(metricChan)

	// Verify the channel is empty
	select {
	case <-metricChan:
		t.Error("Expected channel to be empty after draining")
	default:
		// Channel is empty, which is what we expect
	}
}

func TestStopMultipleTimes(t *testing.T) {
	writer := NewLineProtocolWriter()

	// Stop multiple times - should not panic
	writer.Stop()
	writer.Stop()
	writer.Stop()

	// This should not cause any issues
	t.Log("Multiple stops completed without panic")
}
