package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

// captureLogOutput captures log output for testing
func captureLogOutput(fn func()) string {
	tlc := NewTestLogCapture()
	return tlc.CaptureLogOutput(fn)
}

func TestWithPanicRecoveryAndContinue(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		deviceID    string
		fn          func()
		expectPanic bool
		expectLog   bool
	}{
		{
			name:      "normal execution without panic",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() {
				// Normal execution
			},
			expectPanic: false,
			expectLog:   false,
		},
		{
			name:      "panic with string message",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() {
				panic("test panic message")
			},
			expectPanic: true,
			expectLog:   true,
		},
		{
			name:      "panic with error",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() {
				panic(errors.New("test error"))
			},
			expectPanic: true,
			expectLog:   true,
		},
		{
			name:      "panic with integer",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() {
				panic(42)
			},
			expectPanic: true,
			expectLog:   true,
		},
		{
			name:      "panic with nil",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() {
				panic(nil)
			},
			expectPanic: true,
			expectLog:   true,
		},
		{
			name:      "empty operation name",
			operation: "",
			deviceID:  "device-123",
			fn: func() {
				panic("test panic")
			},
			expectPanic: true,
			expectLog:   true,
		},
		{
			name:      "empty device ID",
			operation: "test-operation",
			deviceID:  "",
			fn: func() {
				panic("test panic")
			},
			expectPanic: true,
			expectLog:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logOutput string
			var panicOccurred bool

			// Capture log output
			logOutput = captureLogOutput(func() {
				// Use defer to catch any panic that might escape
				defer func() {
					if r := recover(); r != nil {
						panicOccurred = true
					}
				}()

				// Call the function with panic recovery
				WithPanicRecoveryAndContinue(tt.operation, tt.deviceID, tt.fn)
			})

			// Check if panic was handled correctly
			if tt.expectPanic && panicOccurred {
				t.Errorf("Panic was not properly recovered - it escaped the recovery function")
			}

			// Check log output
			if tt.expectLog {
				if logOutput == "" {
					t.Errorf("Expected log output but got none")
				} else {
					// Check that the log contains expected elements
					if !strings.Contains(logOutput, tt.operation) {
						t.Errorf("Expected log to contain operation '%s', got: %s", tt.operation, logOutput)
					}
					if !strings.Contains(logOutput, tt.deviceID) {
						t.Errorf("Expected log to contain device ID '%s', got: %s", tt.deviceID, logOutput)
					}
					if !strings.Contains(logOutput, "panic recovered") {
						t.Errorf("Expected log to contain 'panic recovered', got: %s", logOutput)
					}
				}
			} else {
				if logOutput != "" {
					t.Errorf("Expected no log output but got: %s", logOutput)
				}
			}
		})
	}
}

func TestWithPanicRecoveryAndReturnError(t *testing.T) {
	tah := NewTestAssertionHelper()

	tests := []struct {
		name           string
		operation      string
		deviceID       string
		fn             func() error
		expectError    bool
		expectPanic    bool
		expectLog      bool
		expectedErrMsg string
	}{
		{
			name:      "normal execution without panic",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() error {
				return nil
			},
			expectError: false,
			expectPanic: false,
			expectLog:   false,
		},
		{
			name:      "normal execution with error",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() error {
				return errors.New("normal error")
			},
			expectError: true,
			expectPanic: false,
			expectLog:   false,
		},
		{
			name:      "panic with string message",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() error {
				panic("test panic message")
			},
			expectError:    true,
			expectPanic:    true,
			expectLog:      true,
			expectedErrMsg: "panic in test-operation: test panic message",
		},
		{
			name:      "panic with error",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() error {
				panic(errors.New("test error"))
			},
			expectError:    true,
			expectPanic:    true,
			expectLog:      true,
			expectedErrMsg: "panic in test-operation: test error",
		},
		{
			name:      "panic with integer",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() error {
				panic(42)
			},
			expectError:    true,
			expectPanic:    true,
			expectLog:      true,
			expectedErrMsg: "panic in test-operation: 42",
		},
		{
			name:      "panic with nil",
			operation: "test-operation",
			deviceID:  "device-123",
			fn: func() error {
				panic(nil)
			},
			expectError:    true,
			expectPanic:    true,
			expectLog:      true,
			expectedErrMsg: "panic in test-operation: panic called with nil argument",
		},
		{
			name:      "empty operation name",
			operation: "",
			deviceID:  "device-123",
			fn: func() error {
				panic("test panic")
			},
			expectError:    true,
			expectPanic:    true,
			expectLog:      true,
			expectedErrMsg: "panic in : test panic",
		},
		{
			name:      "empty device ID",
			operation: "test-operation",
			deviceID:  "",
			fn: func() error {
				panic("test panic")
			},
			expectError:    true,
			expectPanic:    true,
			expectLog:      true,
			expectedErrMsg: "panic in test-operation: test panic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logOutput string
			var panicOccurred bool

			// Capture log output
			logOutput = captureLogOutput(func() {
				// Use defer to catch any panic that might escape
				defer func() {
					if r := recover(); r != nil {
						panicOccurred = true
					}
				}()

				// Call the function with panic recovery
				err := WithPanicRecoveryAndReturnError(tt.operation, tt.deviceID, tt.fn)

				// Check error result
				if tt.expectError {
					tah.AssertError(t, err, "Expected error but got none")
					if tt.expectedErrMsg != "" && err != nil && err.Error() != tt.expectedErrMsg {
						t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, err.Error())
					}
				} else {
					tah.AssertNoError(t, err, "Expected no error but got one")
				}
			})

			// Check if panic was handled correctly
			if tt.expectPanic && panicOccurred {
				t.Errorf("Panic was not properly recovered - it escaped the recovery function")
			}

			// Check log output
			if tt.expectLog {
				if logOutput == "" {
					t.Errorf("Expected log output but got none")
				} else {
					// Check that the log contains expected elements
					if !strings.Contains(logOutput, tt.operation) {
						t.Errorf("Expected log to contain operation '%s', got: %s", tt.operation, logOutput)
					}
					if !strings.Contains(logOutput, tt.deviceID) {
						t.Errorf("Expected log to contain device ID '%s', got: %s", tt.deviceID, logOutput)
					}
					if !strings.Contains(logOutput, "panic recovered") {
						t.Errorf("Expected log to contain 'panic recovered', got: %s", logOutput)
					}
				}
			} else {
				if logOutput != "" {
					t.Errorf("Expected no log output but got: %s", logOutput)
				}
			}
		})
	}
}

func TestWithPanicRecoveryAndContinue_ComplexPanic(t *testing.T) {
	// Test with complex panic values
	complexPanics := []interface{}{
		map[string]interface{}{"key": "value"},
		[]string{"item1", "item2"},
		struct{ Field string }{"test"},
		fmt.Errorf("formatted error: %s", "test"),
	}

	for i, panicValue := range complexPanics {
		t.Run(fmt.Sprintf("complex_panic_%d", i), func(t *testing.T) {
			logOutput := captureLogOutput(func() {
				WithPanicRecoveryAndContinue("test-op", "device-123", func() {
					panic(panicValue)
				})
			})

			if logOutput == "" {
				t.Errorf("Expected log output for complex panic value")
			}

			if !strings.Contains(logOutput, "panic recovered") {
				t.Errorf("Expected log to contain 'panic recovered', got: %s", logOutput)
			}
		})
	}
}

func TestWithPanicRecoveryAndReturnError_ComplexPanic(t *testing.T) {
	// Test with complex panic values
	complexPanics := []interface{}{
		map[string]interface{}{"key": "value"},
		[]string{"item1", "item2"},
		struct{ Field string }{"test"},
		fmt.Errorf("formatted error: %s", "test"),
	}

	for i, panicValue := range complexPanics {
		t.Run(fmt.Sprintf("complex_panic_%d", i), func(t *testing.T) {
			logOutput := captureLogOutput(func() {
				err := WithPanicRecoveryAndReturnError("test-op", "device-123", func() error {
					panic(panicValue)
				})

				if err == nil {
					t.Errorf("Expected error for complex panic value")
				}

				if !strings.Contains(err.Error(), "panic in test-op") {
					t.Errorf("Expected error message to contain 'panic in test-op', got: %s", err.Error())
				}
			})

			if logOutput == "" {
				t.Errorf("Expected log output for complex panic value")
			}

			if !strings.Contains(logOutput, "panic recovered") {
				t.Errorf("Expected log to contain 'panic recovered', got: %s", logOutput)
			}
		})
	}
}

// Benchmark tests
func BenchmarkWithPanicRecoveryAndContinue_NoPanic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		WithPanicRecoveryAndContinue("bench-op", "device-123", func() {
			// Normal operation
		})
	}
}

func BenchmarkWithPanicRecoveryAndContinue_WithPanic(b *testing.B) {
	// Suppress log output during benchmark
	originalOutput := log.Writer()
	log.SetOutput(os.NewFile(0, os.DevNull))
	defer log.SetOutput(originalOutput)

	for i := 0; i < b.N; i++ {
		WithPanicRecoveryAndContinue("bench-op", "device-123", func() {
			panic("benchmark panic")
		})
	}
}

func BenchmarkWithPanicRecoveryAndReturnError_NoPanic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		WithPanicRecoveryAndReturnError("bench-op", "device-123", func() error {
			return nil
		})
	}
}

func BenchmarkWithPanicRecoveryAndReturnError_WithPanic(b *testing.B) {
	// Suppress log output during benchmark
	originalOutput := log.Writer()
	log.SetOutput(os.NewFile(0, os.DevNull))
	defer log.SetOutput(originalOutput)

	for i := 0; i < b.N; i++ {
		WithPanicRecoveryAndReturnError("bench-op", "device-123", func() error {
			panic("benchmark panic")
		})
	}
}
