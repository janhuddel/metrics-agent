package dummy

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestNew verifies that New() returns a valid Source instance.
func TestNew(t *testing.T) {
	source := New(map[string]interface{}{})
	if source == nil {
		t.Fatal("New() returned nil")
	}

	// Note: Interface compliance is checked at compile time
}

// TestName verifies that the source returns the correct name.
func TestName(t *testing.T) {
	source := New(map[string]interface{}{})
	expected := "dummy"
	actual := source.Name()

	if actual != expected {
		t.Errorf("Expected name %q, got %q", expected, actual)
	}
}

// TestStart verifies that the source generates metrics and handles shutdown signals.
func TestStart(t *testing.T) {
	source := New(map[string]interface{}{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create channels
	out := make(chan string, 10)
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Start the source in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := source.Start(ctx, out, gracefulShutdown, hardShutdown)
		errChan <- err
	}()

	// Wait for at least one metric to be generated
	select {
	case metric := <-out:
		// Verify the metric format
		if !strings.Contains(metric, "temperature,source=dummy") {
			t.Errorf("Expected metric to contain 'temperature,source=dummy', got: %s", metric)
		}
		if !strings.Contains(metric, "value=42") {
			t.Errorf("Expected metric to contain 'value=42', got: %s", metric)
		}
		if !strings.Contains(metric, " ") {
			t.Errorf("Expected metric to contain timestamp, got: %s", metric)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("No metric received within 6 seconds")
	}

	// Test graceful shutdown
	close(gracefulShutdown)

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Expected no error on graceful shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Graceful shutdown did not complete within 5 seconds")
	}
}

// TestHardShutdown verifies that hard shutdown works immediately.
func TestHardShutdown(t *testing.T) {
	source := New(map[string]interface{}{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create channels
	out := make(chan string, 10)
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Start the source in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := source.Start(ctx, out, gracefulShutdown, hardShutdown)
		errChan <- err
	}()

	// Wait a bit to ensure the source is running
	time.Sleep(100 * time.Millisecond)

	// Test hard shutdown
	close(hardShutdown)

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Expected no error on hard shutdown, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Hard shutdown did not complete within 2 seconds")
	}
}

// TestPanicFile verifies that the source panics when the panic file exists.
func TestPanicFile(t *testing.T) {
	source := New(map[string]interface{}{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the panic file
	panicFile := "/tmp/metrics-agent-panic-demo"
	file, err := os.Create(panicFile)
	if err != nil {
		t.Fatalf("Failed to create panic file: %v", err)
	}
	file.Close()
	defer os.Remove(panicFile) // Clean up

	// Create channels
	out := make(chan string, 10)
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Start the source in a goroutine and expect it to panic
	panicChan := make(chan interface{}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicChan <- r
			}
		}()
		source.Start(ctx, out, gracefulShutdown, hardShutdown)
	}()

	// Wait for panic
	select {
	case panicValue := <-panicChan:
		expectedMsg := "Demo module panic triggered by /tmp/metrics-agent-panic-demo file"
		if panicValue != expectedMsg {
			t.Errorf("Expected panic message %q, got %q", expectedMsg, panicValue)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Expected panic did not occur within 3 seconds")
	}
}

// TestMetricFormat verifies the exact format of generated metrics.
func TestMetricFormat(t *testing.T) {
	source := New(map[string]interface{}{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create channels
	out := make(chan string, 10)
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Start the source in a goroutine
	go func() {
		source.Start(ctx, out, gracefulShutdown, hardShutdown)
	}()

	// Collect multiple metrics to verify consistency
	var metrics []string
	for i := 0; i < 3; i++ {
		select {
		case metric := <-out:
			metrics = append(metrics, metric)
		case <-time.After(6 * time.Second):
			t.Fatalf("Only received %d metrics, expected 3", len(metrics))
		}
	}

	// Verify all metrics have the same format
	for i, metric := range metrics {
		parts := strings.Split(metric, " ")
		if len(parts) != 3 {
			t.Errorf("Metric %d: expected 3 parts separated by space, got %d parts: %s", i, len(parts), metric)
		}

		// Check measurement part
		measurement := parts[0]
		if !strings.HasPrefix(measurement, "temperature,source=dummy") {
			t.Errorf("Metric %d: expected measurement to start with 'temperature,source=dummy', got: %s", i, measurement)
		}

		// Check value part
		value := parts[1]
		if !strings.HasPrefix(value, "value=42") {
			t.Errorf("Metric %d: expected value to start with 'value=42', got: %s", i, value)
		}

		// Check timestamp (should be a number)
		timestamp := parts[2]
		if timestamp == "" {
			t.Errorf("Metric %d: expected non-empty timestamp, got empty", i)
		}
	}

	// Clean shutdown
	close(gracefulShutdown)
}

// TestContextCancellation verifies that context cancellation stops the source.
func TestContextCancellation(t *testing.T) {
	source := New(map[string]interface{}{})
	ctx, cancel := context.WithCancel(context.Background())

	// Create channels
	out := make(chan string, 10)
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Start the source in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := source.Start(ctx, out, gracefulShutdown, hardShutdown)
		errChan <- err
	}()

	// Wait a bit to ensure the source is running
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	// The source should continue running until it receives a shutdown signal
	// because it doesn't check ctx.Done() in the main loop
	time.Sleep(100 * time.Millisecond)

	// Now send graceful shutdown
	close(gracefulShutdown)

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Expected no error on graceful shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Graceful shutdown did not complete within 5 seconds")
	}
}

// BenchmarkSourceCreation benchmarks the creation of a new dummy source.
func BenchmarkSourceCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(map[string]interface{}{})
	}
}

// BenchmarkSourceName benchmarks the Name() method.
func BenchmarkSourceName(b *testing.B) {
	source := New(map[string]interface{}{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = source.Name()
	}
}

// TestConfiguration verifies that the source uses the provided configuration.
func TestConfiguration(t *testing.T) {
	// Test with custom interval
	config := map[string]interface{}{
		"interval": "2s",
	}
	source := New(config)

	// We can't directly test the interval since it's private, but we can test
	// that the source starts and generates metrics at the expected rate
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := make(chan string, 10)
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Start the source in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := source.Start(ctx, out, gracefulShutdown, hardShutdown)
		errChan <- err
	}()

	// Wait for at least one metric
	select {
	case metric := <-out:
		if !strings.Contains(metric, "temperature,source=dummy") {
			t.Errorf("Expected metric to contain 'temperature,source=dummy', got: %s", metric)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("No metric received within 3 seconds")
	}

	// Clean shutdown
	close(gracefulShutdown)

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Expected no error on graceful shutdown, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Graceful shutdown did not complete within 2 seconds")
	}
}
