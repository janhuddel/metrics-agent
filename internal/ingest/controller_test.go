package ingest

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// createTestConfig creates a test configuration with a temporary storage directory
func createTestConfig(t *testing.T) (*utils.AppConfig, func()) {
	// Create a temporary directory for storage
	tempDir, err := os.MkdirTemp("", "metrics-agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := &utils.AppConfig{
		Storage: struct {
			Path string `koanf:"path"`
		}{
			Path: tempDir,
		},
		Sources: map[string]interface{}{
			"mock": map[string]interface{}{
				"enabled": true,
				"name":    "test_mock",
			},
		},
		Retry: struct {
			MaxRetries int           `koanf:"max_retries"`
			BaseDelay  time.Duration `koanf:"base_delay"`
			MaxDelay   time.Duration `koanf:"max_delay"`
		}{
			MaxRetries: 3,
			BaseDelay:  time.Second,
			MaxDelay:   30 * time.Second,
		},
	}

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return config, cleanup
}

// createTestConfigWithMultipleSources creates a test configuration with multiple mock sources
func createTestConfigWithMultipleSources(t *testing.T) (*utils.AppConfig, func()) {
	// Create a temporary directory for storage
	tempDir, err := os.MkdirTemp("", "metrics-agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := &utils.AppConfig{
		Storage: struct {
			Path string `koanf:"path"`
		}{
			Path: tempDir,
		},
		Sources: map[string]interface{}{
			"mock1": map[string]interface{}{
				"enabled": true,
				"name":    "test_mock_1",
			},
			"mock2": map[string]interface{}{
				"enabled": true,
				"name":    "test_mock_2",
			},
		},
		Retry: struct {
			MaxRetries int           `koanf:"max_retries"`
			BaseDelay  time.Duration `koanf:"base_delay"`
			MaxDelay   time.Duration `koanf:"max_delay"`
		}{
			MaxRetries: 3,
			BaseDelay:  time.Second,
			MaxDelay:   30 * time.Second,
		},
	}

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return config, cleanup
}

// TestNewController tests the controller creation with different configurations
func TestNewController(t *testing.T) {
	tests := []struct {
		name        string
		config      *utils.AppConfig
		expectError bool
		description string
	}{
		{
			name: "valid_config_with_enabled_sources",
			config: &utils.AppConfig{
				Sources: map[string]interface{}{
					"mock": map[string]interface{}{
						"enabled": true,
						"name":    "test_mock",
					},
				},
				Retry: struct {
					MaxRetries int           `koanf:"max_retries"`
					BaseDelay  time.Duration `koanf:"base_delay"`
					MaxDelay   time.Duration `koanf:"max_delay"`
				}{
					MaxRetries: 3,
					BaseDelay:  time.Second,
					MaxDelay:   30 * time.Second,
				},
			},
			expectError: false,
			description: "Should create controller successfully with enabled sources",
		},
		{
			name: "no_enabled_sources",
			config: &utils.AppConfig{
				Sources: map[string]interface{}{
					"mock": map[string]interface{}{
						"enabled": false,
					},
				},
				Retry: struct {
					MaxRetries int           `koanf:"max_retries"`
					BaseDelay  time.Duration `koanf:"base_delay"`
					MaxDelay   time.Duration `koanf:"max_delay"`
				}{
					MaxRetries: 3,
					BaseDelay:  time.Second,
					MaxDelay:   30 * time.Second,
				},
			},
			expectError: true,
			description: "Should return error when no sources are enabled",
		},
		{
			name: "empty_sources_config",
			config: &utils.AppConfig{
				Sources: map[string]interface{}{},
				Retry: struct {
					MaxRetries int           `koanf:"max_retries"`
					BaseDelay  time.Duration `koanf:"base_delay"`
					MaxDelay   time.Duration `koanf:"max_delay"`
				}{
					MaxRetries: 3,
					BaseDelay:  time.Second,
					MaxDelay:   30 * time.Second,
				},
			},
			expectError: true,
			description: "Should return error when sources config is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register mock ingester for testing
			registry := NewRegistry()
			registry.Register("mock", MockIngesterCreator)

			// Temporarily replace the global registry
			originalRegistry := SourceRegistry
			SourceRegistry = registry
			defer func() {
				SourceRegistry = originalRegistry
			}()

			// Use test config with temporary storage for valid configs
			var config *utils.AppConfig
			var cleanup func()
			if !tt.expectError {
				config, cleanup = createTestConfig(t)
				defer cleanup()
			} else {
				config = tt.config
			}

			controller, err := NewController(config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none: %s", tt.description)
				}
				if controller != nil {
					t.Error("Expected controller to be nil when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v - %s", err, tt.description)
				}
				if controller == nil {
					t.Error("Expected controller to be created successfully")
					return
				}
				if controller.config != config {
					t.Error("Controller config not set correctly")
				}
				if controller.store == nil {
					t.Error("Controller store not initialized")
				}
				if len(controller.ingesters) == 0 {
					t.Error("Expected at least one ingester to be created")
				}
				// Clean up the store directory
				if controller.store != nil {
					controller.store.Cleanup()
				}
			}
		})
	}
}

// TestControllerStart_SIGTERM tests graceful shutdown with SIGTERM signal
func TestControllerStart_SIGTERM(t *testing.T) {
	// Create test configuration with temporary storage
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", MockIngesterCreator)

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait a bit for controller to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM signal
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestControllerStart_SIGINT tests hard shutdown with SIGINT signal
func TestControllerStart_SIGINT(t *testing.T) {
	// Create test configuration with temporary storage
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", MockIngesterCreator)

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait a bit for controller to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGINT signal
	sigChan <- syscall.SIGINT

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestControllerStart_IngesterRetry tests ingester retry logic
func TestControllerStart_IngesterRetry(t *testing.T) {
	// Create test configuration with temporary storage and low retry settings
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Override retry settings for this test
	config.Retry.MaxRetries = 2
	config.Retry.BaseDelay = 50 * time.Millisecond
	config.Retry.MaxDelay = 200 * time.Millisecond

	// Create a mock ingester that will error after a short time
	mockIngester := NewMockIngester("test_mock").WithError(100 * time.Millisecond)

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester
	})

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait for one retry to happen, then stop before exhaustion
	time.Sleep(200 * time.Millisecond)

	// Send SIGTERM to stop the controller before retries are exhausted
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestControllerStart_IngesterPanic tests panic recovery in ingester
func TestControllerStart_IngesterPanic(t *testing.T) {
	// Create test configuration with temporary storage
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Override retry settings for this test
	config.Retry.MaxRetries = 2
	config.Retry.BaseDelay = 50 * time.Millisecond
	config.Retry.MaxDelay = 200 * time.Millisecond

	// Create a mock ingester that will panic after a short time
	mockIngester := NewMockIngester("test_mock").WithPanic(100 * time.Millisecond)

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester
	})

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait for panic and one retry to happen, then stop before exhaustion
	time.Sleep(200 * time.Millisecond)

	// Send SIGTERM to stop the controller before retries are exhausted
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestControllerStart_GracefulShutdownTimeout tests graceful shutdown timeout
func TestControllerStart_GracefulShutdownTimeout(t *testing.T) {
	// Create test configuration with temporary storage
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Create a mock ingester with long cleanup time to trigger timeout
	mockIngester := NewMockIngester("test_mock").WithCleanupTime(35 * time.Second)

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester
	})

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait a bit for controller to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM signal
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish (should timeout and force hard shutdown)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(40 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestControllerStart_ConcurrentIngesters tests multiple ingesters running concurrently
func TestControllerStart_ConcurrentIngesters(t *testing.T) {
	// Create test configuration with multiple ingesters and temporary storage
	config, cleanup := createTestConfigWithMultipleSources(t)
	defer cleanup()

	// Create mock ingesters
	mockIngester1 := NewMockIngester("test_mock_1").WithMetrics(5, 50*time.Millisecond)
	mockIngester2 := NewMockIngester("test_mock_2").WithMetrics(3, 75*time.Millisecond)

	// Register mock ingesters
	registry := NewRegistry()
	registry.Register("mock1", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester1
	})
	registry.Register("mock2", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester2
	})

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait for ingesters to send some metrics
	time.Sleep(500 * time.Millisecond)

	// Send SIGTERM signal
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Verify both ingesters were started
		if !mockIngester1.IsStarted() {
			t.Error("Mock ingester 1 was not started")
		}
		if !mockIngester2.IsStarted() {
			t.Error("Mock ingester 2 was not started")
		}
		// Verify metrics were sent
		if mockIngester1.MetricsSent() == 0 {
			t.Error("Mock ingester 1 did not send any metrics")
		}
		if mockIngester2.MetricsSent() == 0 {
			t.Error("Mock ingester 2 did not send any metrics")
		}
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestControllerStart_MetricChannel tests that metrics are properly sent through the channel
func TestControllerStart_MetricChannel(t *testing.T) {
	// Create test configuration with temporary storage
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Create a mock ingester that sends a specific number of metrics
	mockIngester := NewMockIngester("test_mock").WithMetrics(3, 100*time.Millisecond)

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester
	})

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait for metrics to be sent
	time.Sleep(500 * time.Millisecond)

	// Send SIGTERM signal
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify metrics were sent
		if mockIngester.MetricsSent() == 0 {
			t.Error("No metrics were sent")
		}
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// TestGetEnabledSources tests the getEnabledSources function
func TestGetEnabledSources(t *testing.T) {
	// Create test configuration
	config := &utils.AppConfig{
		Sources: map[string]interface{}{
			"mock1": map[string]interface{}{
				"enabled": true,
				"name":    "test_mock_1",
			},
			"mock2": map[string]interface{}{
				"enabled": false,
				"name":    "test_mock_2",
			},
			"mock3": map[string]interface{}{
				"enabled": true,
				"name":    "test_mock_3",
			},
		},
	}

	// Create temporary directory for test storage
	tempDir, err := os.MkdirTemp("", "metrics-agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mock store
	store := utils.NewStore(tempDir)

	// Register mock ingesters
	registry := NewRegistry()
	registry.Register("mock1", MockIngesterCreator)
	registry.Register("mock2", MockIngesterCreator)
	registry.Register("mock3", MockIngesterCreator)

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	ingesters := getEnabledSources(config, store)

	// Should have 2 enabled ingesters (mock1 and mock3)
	if len(ingesters) != 2 {
		t.Errorf("Expected 2 enabled ingesters, got %d", len(ingesters))
	}

	// Verify the ingesters are the correct ones
	names := make(map[string]bool)
	for _, ingester := range ingesters {
		names[ingester.Name()] = true
	}

	if !names["test_mock_1"] {
		t.Error("Expected mock1 ingester to be enabled")
	}
	if !names["test_mock_3"] {
		t.Error("Expected mock3 ingester to be enabled")
	}
	if names["test_mock_2"] {
		t.Error("Expected mock2 ingester to be disabled")
	}

	// Clean up the store directory
	store.Cleanup()
}

// TestControllerStart_ContextCancellation tests context cancellation handling
func TestControllerStart_ContextCancellation(t *testing.T) {
	// Create test configuration with temporary storage
	config, cleanup := createTestConfig(t)
	defer cleanup()

	// Create a mock ingester that respects context cancellation
	mockIngester := NewMockIngester("test_mock").WithMetrics(10, 50*time.Millisecond)

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", func(config map[string]interface{}, store *utils.Store) types.Ingester {
		return mockIngester
	})

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	controller, err := NewController(config)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)

	// Start controller in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Start(context.Background(), sigChan)
	}()

	// Wait a bit for controller to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM signal
	sigChan <- syscall.SIGTERM

	// Wait for controller to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Controller finished successfully
		// Clean up the store directory
		if controller.store != nil {
			controller.store.Cleanup()
		}
	case <-time.After(5 * time.Second):
		t.Error("Controller did not finish within timeout")
	}
}

// BenchmarkControllerStart benchmarks the controller start performance
func BenchmarkControllerStart(b *testing.B) {
	// Create temporary directory for benchmark storage
	tempDir, err := os.MkdirTemp("", "metrics-agent-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &utils.AppConfig{
		Storage: struct {
			Path string `koanf:"path"`
		}{
			Path: tempDir,
		},
		Sources: map[string]interface{}{
			"mock": map[string]interface{}{
				"enabled": true,
				"name":    "test_mock",
			},
		},
		Retry: struct {
			MaxRetries int           `koanf:"max_retries"`
			BaseDelay  time.Duration `koanf:"base_delay"`
			MaxDelay   time.Duration `koanf:"max_delay"`
		}{
			MaxRetries: 3,
			BaseDelay:  time.Second,
			MaxDelay:   30 * time.Second,
		},
	}

	// Register mock ingester
	registry := NewRegistry()
	registry.Register("mock", MockIngesterCreator)

	// Temporarily replace the global registry
	originalRegistry := SourceRegistry
	SourceRegistry = registry
	defer func() {
		SourceRegistry = originalRegistry
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		controller, err := NewController(config)
		if err != nil {
			b.Fatalf("Failed to create controller: %v", err)
		}
		if controller == nil {
			b.Error("Controller is nil")
			continue
		}
		// Clean up the store directory for each iteration
		if controller.store != nil {
			controller.store.Cleanup()
		}
	}
}
