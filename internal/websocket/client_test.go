package websocket

import (
	"context"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	config := Config{
		URL: "ws://localhost:8080/ws",
	}

	client, err := NewClient(config, func(message []byte) error { return nil })
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check that defaults are set
	if client.config.ReconnectInterval == 0 {
		t.Error("ReconnectInterval should have a default value")
	}
	if client.config.MaxReconnectAttempts == 0 {
		t.Error("MaxReconnectAttempts should have a default value")
	}
	if client.config.ConnectionTimeout == 0 {
		t.Error("ConnectionTimeout should have a default value")
	}
	if client.config.ReadTimeout == 0 {
		t.Error("ReadTimeout should have a default value")
	}
	if client.config.WriteTimeout == 0 {
		t.Error("WriteTimeout should have a default value")
	}
	if client.config.MaxBackoffInterval == 0 {
		t.Error("MaxBackoffInterval should have a default value")
	}
	if client.config.BackoffMultiplier == 0 {
		t.Error("BackoffMultiplier should have a default value")
	}

	// Check specific default values
	if client.config.ReconnectInterval != 5*time.Second {
		t.Errorf("Expected ReconnectInterval to be 5s, got %v", client.config.ReconnectInterval)
	}
	if client.config.MaxReconnectAttempts != 10 {
		t.Errorf("Expected MaxReconnectAttempts to be 10, got %d", client.config.MaxReconnectAttempts)
	}
	if client.config.ConnectionTimeout != 10*time.Second {
		t.Errorf("Expected ConnectionTimeout to be 10s, got %v", client.config.ConnectionTimeout)
	}
	if client.config.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout to be 30s, got %v", client.config.ReadTimeout)
	}
	if client.config.WriteTimeout != 10*time.Second {
		t.Errorf("Expected WriteTimeout to be 10s, got %v", client.config.WriteTimeout)
	}
	if client.config.MaxBackoffInterval != 5*time.Minute {
		t.Errorf("Expected MaxBackoffInterval to be 5m, got %v", client.config.MaxBackoffInterval)
	}
	if client.config.BackoffMultiplier != 2.0 {
		t.Errorf("Expected BackoffMultiplier to be 2.0, got %f", client.config.BackoffMultiplier)
	}
}

func TestConnectionState(t *testing.T) {
	config := Config{
		URL: "ws://localhost:8080/ws",
	}

	client, err := NewClient(config, func(message []byte) error { return nil })
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test initial state
	if client.GetState() != StateDisconnected {
		t.Errorf("Expected initial state to be StateDisconnected, got %v", client.GetState())
	}

	// Test state transitions (using reflection to access private setState method)
	// Since setState is private, we'll test through the public interface
	// by checking that the state changes appropriately during operations
}

func TestIsUnrecoverableError(t *testing.T) {
	config := Config{
		URL: "ws://localhost:8080/ws",
	}

	client, err := NewClient(config, func(message []byte) error { return nil })
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test context cancellation (unrecoverable)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !client.isUnrecoverableError(ctx.Err()) {
		t.Error("Context cancellation should be unrecoverable")
	}

	// Test authentication errors (unrecoverable)
	authErr := &mockError{msg: "401 Unauthorized"}
	if !client.isUnrecoverableError(authErr) {
		t.Error("Authentication errors should be unrecoverable")
	}

	// Test EOF errors (recoverable)
	eofErr := &mockError{msg: "EOF"}
	if client.isUnrecoverableError(eofErr) {
		t.Error("EOF errors should be recoverable")
	}

	// Test nil error (recoverable)
	if client.isUnrecoverableError(nil) {
		t.Error("Nil errors should be recoverable")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s          string
		substrings []string
		expected   bool
	}{
		{"hello world", []string{"world"}, true},
		{"hello world", []string{"foo"}, false},
		{"401 Unauthorized", []string{"401", "403"}, true},
		{"invalid URL", []string{"invalid", "malformed"}, true},
		{"connection refused", []string{"timeout", "refused"}, true},
		{"", []string{"test"}, false},
		{"test", []string{""}, false},
		{"test", []string{}, false}, // Empty slice should return false
	}

	for _, test := range tests {
		result := containsAny(test.s, test.substrings)
		if result != test.expected {
			t.Errorf("containsAny(%q, %v) = %v, expected %v", test.s, test.substrings, result, test.expected)
		}
	}
}

func TestNewClientValidation(t *testing.T) {
	// Test empty URL
	_, err := NewClient(Config{}, func(message []byte) error { return nil })
	if err == nil {
		t.Error("Should return error for empty URL")
	}

	// Test nil handler
	_, err = NewClient(Config{URL: "ws://localhost:8080/ws"}, nil)
	if err == nil {
		t.Error("Should return error for nil handler")
	}

	// Test valid config
	_, err = NewClient(Config{URL: "ws://localhost:8080/ws"}, func(message []byte) error { return nil })
	if err != nil {
		t.Errorf("Should not return error for valid config, got: %v", err)
	}
}

// mockError is a simple error implementation for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
