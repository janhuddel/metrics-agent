package supervisor_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/supervisor"
)

// TestSupervisor_PanicRecovery tests that panics in modules are properly recovered
func TestSupervisor_PanicRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sup := supervisor.New(true) // Use in-process mode for testing

	// Start a module that will panic
	if err := sup.Start(ctx, supervisor.VendorSpec{Name: "panic-test"}); err != nil {
		t.Fatalf("failed to start panic-test module: %v", err)
	}

	// Wait for events
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-sup.Events():
			t.Logf("got event: %s", ev)
			if contains(ev, "panic") || contains(ev, "error") {
				// Panic was handled gracefully
				return
			}
		case <-timeout:
			t.Fatal("no panic recovery event within 5s")
		}
	}
}

// TestSupervisor_EventChannelOverflow tests that event channel overflow is handled gracefully
func TestSupervisor_EventChannelOverflow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sup := supervisor.New(true)

	// Start multiple modules to generate many events
	for i := 0; i < 10; i++ {
		spec := supervisor.VendorSpec{Name: "test-module"}
		if err := sup.Start(ctx, spec); err != nil {
			t.Logf("failed to start module %d: %v", i, err)
		}
	}

	// Wait a bit for events to accumulate
	time.Sleep(100 * time.Millisecond)

	// Stop all modules
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	sup.StopAll(stopCtx)

	// Should not block or panic
	t.Log("Event channel overflow test completed successfully")
}

// TestSupervisor_ConcurrentOperations tests concurrent start/stop operations
func TestSupervisor_ConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sup := supervisor.New(true)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Start multiple goroutines doing concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			spec := supervisor.VendorSpec{Name: "concurrent-test"}
			if err := sup.Start(ctx, spec); err != nil {
				t.Logf("goroutine %d: failed to start: %v", id, err)
			}

			time.Sleep(50 * time.Millisecond)

			// Try to restart
			sup.RestartAll(ctx)
		}(i)
	}

	wg.Wait()

	// Stop all modules
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	sup.StopAll(stopCtx)

	t.Log("Concurrent operations test completed successfully")
}

// TestSupervisor_GracefulShutdown tests graceful shutdown under various conditions
func TestSupervisor_GracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sup := supervisor.New(true)

	// Start multiple modules
	for i := 0; i < 5; i++ {
		spec := supervisor.VendorSpec{Name: "shutdown-test"}
		if err := sup.Start(ctx, spec); err != nil {
			t.Logf("failed to start module %d: %v", i, err)
		}
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Test graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	start := time.Now()
	sup.StopAll(shutdownCtx)
	duration := time.Since(start)

	if duration > 2*time.Second {
		t.Errorf("shutdown took too long: %v", duration)
	}

	t.Logf("Graceful shutdown completed in %v", duration)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
