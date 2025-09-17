// Package utils provides common utility functions used across multiple modules.
//
// This file contains panic recovery utilities that provide consistent
// error handling and logging across the application.
package utils

import (
	"fmt"
)

// WithPanicRecoveryAndContinue executes a function with panic recovery and logging.
// If a panic occurs, it logs the panic and continues execution (panic is "swallowed").
// This is a utility function that can be used across all modules to ensure
// consistent panic recovery behavior for non-critical operations.
//
// Parameters:
// - operation: A descriptive name for the operation being performed
// - deviceID: An identifier for the device or context where the operation is running
// - fn: The function to execute with panic recovery
func WithPanicRecoveryAndContinue(operation string, deviceID string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			Errorf("%s panic recovered for device %s: %v", operation, deviceID, r)
		}
	}()
	fn()
}

// WithPanicRecoveryAndReturnError executes a function with panic recovery and logging, returning an error.
// If a panic occurs, it logs the panic and converts it to an error that is returned to the caller.
// This is a utility function that can be used across all modules to ensure
// consistent panic recovery behavior for functions that return errors.
//
// Parameters:
// - operation: A descriptive name for the operation being performed
// - deviceID: An identifier for the device or context where the operation is running
// - fn: The function to execute with panic recovery
//
// Returns:
// - error: The original error from the function, or a panic error if a panic occurred
func WithPanicRecoveryAndReturnError(operation string, deviceID string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			Errorf("%s panic recovered for device %s: %v", operation, deviceID, r)
			err = fmt.Errorf("panic in %s: %v", operation, r)
		}
	}()
	return fn()
}
