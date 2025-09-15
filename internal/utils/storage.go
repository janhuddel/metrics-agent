// Package utils provides utility functions for the metrics agent.
// This file contains centralized storage utilities for modules.
package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage provides a thread-safe key-value storage system for modules.
// It stores data in JSON files with secure permissions.
type Storage struct {
	filePath string
	data     map[string]interface{}
	mutex    sync.RWMutex
}

// NewStorage creates a new storage instance for a specific module.
// The storage file will be created in ~/.config/metrics-agent/{moduleName}.json
func NewStorage(moduleName string) (*Storage, error) {
	// Use XDG Base Directory specification for secure storage
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory is not accessible
		fileName := fmt.Sprintf(".%s-storage.json", moduleName)
		return &Storage{
			filePath: fileName,
			data:     make(map[string]interface{}),
		}, nil
	}

	// Create .config directory if it doesn't exist
	configDir := filepath.Join(homeDir, ".config", "metrics-agent")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	fileName := fmt.Sprintf("%s-storage.json", moduleName)
	filePath := filepath.Join(configDir, fileName)

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

// Set stores a key-value pair in the storage.
func (s *Storage) Set(key string, value interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data[key] = value
	return s.save()
}

// Get retrieves a value by key from the storage.
// Returns nil if the key doesn't exist.
func (s *Storage) Get(key string) interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.data[key]
}

// GetString retrieves a string value by key from the storage.
// Returns empty string if the key doesn't exist or value is not a string.
func (s *Storage) GetString(key string) string {
	value := s.Get(key)
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

// GetInt retrieves an integer value by key from the storage.
// Returns 0 if the key doesn't exist or value is not an integer.
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
// Returns 0.0 if the key doesn't exist or value is not a float64.
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
// Returns false if the key doesn't exist or value is not a boolean.
func (s *Storage) GetBool(key string) bool {
	value := s.Get(key)
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

// Delete removes a key from the storage.
func (s *Storage) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.data, key)
	return s.save()
}

// Exists checks if a key exists in the storage.
func (s *Storage) Exists(key string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	_, exists := s.data[key]
	return exists
}

// Keys returns all keys in the storage.
func (s *Storage) Keys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	keys := make([]string, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	return keys
}

// Clear removes all data from the storage.
func (s *Storage) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data = make(map[string]interface{})
	return s.save()
}

// load reads data from the storage file.
func (s *Storage) load() error {
	// Check if file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, that's okay
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	if len(data) == 0 {
		return nil // Empty file, that's okay
	}

	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("failed to parse storage file: %w", err)
	}

	return nil
}

// save writes data to the storage file.
func (s *Storage) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage data: %w", err)
	}

	// Write with secure permissions (readable only by owner)
	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	return nil
}

// GetFilePath returns the path to the storage file.
func (s *Storage) GetFilePath() string {
	return s.filePath
}
