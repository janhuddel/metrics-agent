// Package utils provides utility functions for the metrics agent.
// This file contains centralized storage utilities for modules.
package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Storage provides a thread-safe key-value storage system for modules.
// It stores data in JSON files with secure permissions following the Linux
// Filesystem Hierarchy Standard (FHS) for production deployments.
type Storage struct {
	filePath string
	data     map[string]interface{}
	mutex    sync.RWMutex
}

// StorageConfig holds configuration for storage initialization.
type StorageConfig struct {
	// ModuleName is the name of the module using this storage
	ModuleName string
	// PreferredDir is the preferred directory for storage (default: "/var/lib/metrics-agent")
	PreferredDir string
	// FallbackDir is the fallback directory for development (default: ".data")
	FallbackDir string
}

// DefaultStorageConfig returns a default storage configuration.
func DefaultStorageConfig(moduleName string) *StorageConfig {
	return &StorageConfig{
		ModuleName:   moduleName,
		PreferredDir: "/var/lib/metrics-agent",
		FallbackDir:  ".data",
	}
}

// NewStorage creates a new storage instance for a specific module.
// The storage file will be created in /var/lib/metrics-agent/{moduleName}.json
// Falls back to .data/{moduleName}-storage.json if the system directory is not accessible,
// and finally to .{moduleName}-storage.json in the current directory as a last resort.
//
// This follows the Linux Filesystem Hierarchy Standard (FHS) for production deployments
// while providing graceful fallbacks for development environments.
func NewStorage(moduleName string) (*Storage, error) {
	return NewStorageWithConfig(DefaultStorageConfig(moduleName))
}

// NewStorageWithConfig creates a new storage instance with custom configuration.
// This allows for more flexible storage initialization in different environments.
func NewStorageWithConfig(config *StorageConfig) (*Storage, error) {
	filePath, err := determineStoragePath(config)
	if err != nil {
		return nil, fmt.Errorf("failed to determine storage path: %w", err)
	}

	storage := &Storage{
		filePath: filePath,
		data:     make(map[string]interface{}),
	}

	// Load existing data if file exists
	if err := storage.load(); err != nil {
		// If file doesn't exist or is corrupted, start with empty data
		storage.data = make(map[string]interface{})
	}

	return storage, nil
}

// determineStoragePath determines the best storage path based on availability and permissions.
// It follows a fallback hierarchy: preferred directory -> fallback directory -> current directory.
func determineStoragePath(config *StorageConfig) (string, error) {
	// Try preferred directory first
	if path, err := tryStorageDirectory(config.PreferredDir, config.ModuleName, false); err == nil {
		return path, nil
	}

	// Try fallback directory
	if path, err := tryStorageDirectory(config.FallbackDir, config.ModuleName, true); err == nil {
		return path, nil
	}

	// Last resort: current directory with hidden file
	fileName := fmt.Sprintf(".%s-storage.json", config.ModuleName)
	return fileName, nil
}

// tryStorageDirectory attempts to use a specific directory for storage.
// Returns the full file path if successful, or an error if the directory cannot be used.
func tryStorageDirectory(dir, moduleName string, isFallback bool) (string, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Test write permissions
	testFile := filepath.Join(dir, ".write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		// Clean up test file if it was created
		os.Remove(testFile)
		return "", fmt.Errorf("no write permission to directory %s: %w", dir, err)
	}

	// Clean up the test file
	os.Remove(testFile)

	// Generate file path
	fileName := fmt.Sprintf("%s-storage.json", moduleName)
	return filepath.Join(dir, fileName), nil
}

// Set stores a key-value pair in the storage and persists it to disk.
// The value can be any JSON-serializable type (string, number, boolean, map, slice).
// Returns an error if the data cannot be persisted to disk.
func (s *Storage) Set(key string, value interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data[key] = value
	return s.save()
}

// Get retrieves a value by key from the storage.
// Returns nil if the key doesn't exist. The returned value maintains its original type.
func (s *Storage) Get(key string) interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.data[key]
}

// GetString retrieves a string value by key from the storage.
// Returns empty string if the key doesn't exist or the value is not a string.
// This is a convenience method for type-safe string retrieval.
func (s *Storage) GetString(key string) string {
	value := s.Get(key)
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

// GetInt retrieves an integer value by key from the storage.
// Returns 0 if the key doesn't exist or the value is not a number.
// Handles both int and float64 types (common when unmarshaling JSON).
func (s *Storage) GetInt(key string) int {
	value := s.Get(key)
	if i, ok := value.(int); ok {
		return i
	}
	if f, ok := value.(float64); ok {
		return int(f)
	}
	return 0
}

// GetFloat64 retrieves a float64 value by key from the storage.
// Returns 0.0 if the key doesn't exist or the value is not a number.
// Handles both int and float64 types (common when unmarshaling JSON).
func (s *Storage) GetFloat64(key string) float64 {
	value := s.Get(key)
	if f, ok := value.(float64); ok {
		return f
	}
	if i, ok := value.(int); ok {
		return float64(i)
	}
	return 0.0
}

// GetBool retrieves a boolean value by key from the storage.
// Returns false if the key doesn't exist or the value is not a boolean.
// This is a convenience method for type-safe boolean retrieval.
func (s *Storage) GetBool(key string) bool {
	value := s.Get(key)
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

// Delete removes a key from the storage and persists the change to disk.
// Returns an error if the data cannot be persisted to disk.
// No error is returned if the key doesn't exist.
func (s *Storage) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.data, key)
	return s.save()
}

// Exists checks if a key exists in the storage.
// Returns true if the key exists, false otherwise.
// This is a read-only operation that doesn't modify the storage.
func (s *Storage) Exists(key string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	_, exists := s.data[key]
	return exists
}

// Keys returns all keys currently stored in the storage.
// Returns an empty slice if the storage is empty.
// The order of keys is not guaranteed as it depends on the underlying map iteration.
func (s *Storage) Keys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	keys := make([]string, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	return keys
}

// Clear removes all data from the storage and persists the change to disk.
// Returns an error if the data cannot be persisted to disk.
// After calling Clear, the storage will be empty but still functional.
func (s *Storage) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data = make(map[string]interface{})
	return s.save()
}

// load reads data from the storage file into memory.
// If the file doesn't exist or is empty, no error is returned.
// If the file exists but contains invalid JSON, an error is returned.
func (s *Storage) load() error {
	// Check if file exists - this is not an error condition
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, start with empty storage
	}

	// Read the entire file
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	// Empty file is valid - start with empty storage
	if len(data) == 0 {
		return nil
	}

	// Parse JSON data into the storage map
	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("failed to parse storage file: %w", err)
	}

	return nil
}

// save writes the current storage data to disk as formatted JSON.
// Uses appropriate file permissions based on the storage location:
// - 0600 (owner read/write only) for system directories like /var/lib
// - 0644 (owner read/write, group/other read) for development directories
func (s *Storage) save() error {
	// Marshal data with pretty-printing for human readability
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage data: %w", err)
	}

	// Determine appropriate file permissions based on location
	perm := s.getFilePermissions()

	// Write data to file with atomic operation
	if err := os.WriteFile(s.filePath, data, perm); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	return nil
}

// getFilePermissions returns the appropriate file permissions based on storage location.
// Uses stricter permissions (0600) for system directories and more permissive (0644) for development.
func (s *Storage) getFilePermissions() os.FileMode {
	// Use strict permissions for system directories
	if filepath.IsAbs(s.filePath) && strings.HasPrefix(s.filePath, "/var/") {
		return 0600 // Owner read/write only
	}
	// More permissive for development directories
	return 0644 // Owner read/write, group/other read
}

// GetFilePath returns the absolute path to the storage file.
// This is useful for debugging and logging purposes.
func (s *Storage) GetFilePath() string {
	return s.filePath
}
