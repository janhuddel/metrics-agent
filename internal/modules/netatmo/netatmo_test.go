package netatmo

import (
	"context"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

func TestNetatmoModule(t *testing.T) {
	// Create a test configuration
	config := Config{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Timeout:      "10s",
		Interval:     "1m",
	}

	// Create module instance
	module, err := NewNetatmoModule(config)
	if err != nil {
		t.Fatalf("Failed to create Netatmo module: %v", err)
	}

	// Verify configuration is set correctly
	if module.config.ClientID != "test_client_id" {
		t.Errorf("Expected ClientID to be 'test_client_id', got '%s'", module.config.ClientID)
	}

	if module.config.Interval != "1m" {
		t.Errorf("Expected Interval to be '1m', got '%s'", module.config.Interval)
	}

	// Verify base URL is set correctly
	if module.baseURL != "https://api.netatmo.com" {
		t.Errorf("Expected baseURL to be 'https://api.netatmo.com', got '%s'", module.baseURL)
	}

	// Verify HTTP client timeout is set
	if module.httpClient.Timeout != 10*time.Second {
		t.Errorf("Expected HTTP client timeout to be 10s, got %v", module.httpClient.Timeout)
	}
}

func TestLoadConfig(t *testing.T) {
	// Test loading default configuration
	config := LoadConfig()

	// Verify default values
	if config.Timeout != "30s" {
		t.Errorf("Expected default timeout to be '30s', got '%s'", config.Timeout)
	}

	if config.Interval != "5m" {
		t.Errorf("Expected default interval to be '5m', got '%s'", config.Interval)
	}
}

func TestSendDeviceMetrics(t *testing.T) {
	// Create a test module
	config := Config{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	}
	module, err := NewNetatmoModule(config)
	if err != nil {
		t.Fatalf("Failed to create Netatmo module: %v", err)
	}

	// Create a metrics channel
	metricsCh := make(chan metrics.Metric, 10)
	module.metricsCh = metricsCh

	// Create test dashboard data
	dashboard := &Dashboard{
		Temperature: 22.5,
		Humidity:    65,
		CO2:         450,
		Pressure:    1013.25,
	}

	// Send metrics
	module.sendDeviceMetrics("test_device_id", "Test Device", dashboard, time.Now())

	// Verify metric was sent
	select {
	case metric := <-metricsCh:
		if metric.Name != "climate" {
			t.Errorf("Expected metric name to be 'netatmo_sensor', got '%s'", metric.Name)
		}

		if metric.Tags["vendor"] != "netatmo" {
			t.Errorf("Expected vendor tag to be 'netatmo', got '%s'", metric.Tags["vendor"])
		}

		if metric.Tags["device"] != "test_device_id" {
			t.Errorf("Expected device tag to be 'test_device_id', got '%s'", metric.Tags["device"])
		}

		// Check fields
		if temp, ok := metric.Fields["temperature"]; !ok || temp != 22.5 {
			t.Errorf("Expected temperature field to be 22.5, got %v", temp)
		}

		if humidity, ok := metric.Fields["humidity"]; !ok || humidity != 65 {
			t.Errorf("Expected humidity field to be 65, got %v", humidity)
		}

		if co2, ok := metric.Fields["co2"]; !ok || co2 != 450 {
			t.Errorf("Expected co2 field to be 450, got %v", co2)
		}

		if pressure, ok := metric.Fields["pressure"]; !ok || pressure != 1013.25 {
			t.Errorf("Expected pressure field to be 1013.25, got %v", pressure)
		}

	case <-time.After(1 * time.Second):
		t.Error("Expected metric to be sent within 1 second")
	}
}

func TestRunWithCancellation(t *testing.T) {
	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create a metrics channel
	metricsCh := make(chan metrics.Metric, 10)

	// This should return quickly due to context cancellation
	// We expect it to fail during authentication since we don't have real credentials
	err := Run(ctx, metricsCh)
	if err == nil {
		t.Error("Expected Run to return an error due to authentication failure")
	}
}
