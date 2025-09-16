package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNewStorage(t *testing.T) {
	tests := []struct {
		name        string
		moduleName  string
		expectError bool
	}{
		{
			name:        "valid module name",
			moduleName:  "test-module",
			expectError: false,
		},
		{
			name:        "empty module name",
			moduleName:  "",
			expectError: false,
		},
		{
			name:        "module name with special characters",
			moduleName:  "test_module-123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewStorage(tt.moduleName)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if storage == nil {
				t.Errorf("Expected storage instance but got nil")
				return
			}

			// Verify file path is set
			if storage.filePath == "" {
				t.Errorf("Expected file path to be set")
			}

			// Verify data map is initialized
			if storage.data == nil {
				t.Errorf("Expected data map to be initialized")
			}

			// Clean up
			if storage.filePath != "" {
				os.Remove(storage.filePath)
			}
		})
	}
}

func TestStorage_SetAndGet(t *testing.T) {
	storage, err := NewStorage("test-set-get")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	tests := []struct {
		key   string
		value interface{}
	}{
		{"string", "test value"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"map", map[string]interface{}{"nested": "value"}},
		{"slice", []string{"item1", "item2"}},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			// Set value
			if err := storage.Set(tt.key, tt.value); err != nil {
				t.Errorf("Set failed: %v", err)
				return
			}

			// Get value
			retrieved := storage.Get(tt.key)
			if retrieved == nil {
				t.Errorf("Expected value but got nil")
				return
			}

			// For complex types, compare JSON representation
			if tt.key == "map" || tt.key == "slice" {
				expectedJSON, _ := json.Marshal(tt.value)
				actualJSON, _ := json.Marshal(retrieved)
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("Expected %v, got %v", tt.value, retrieved)
				}
			} else {
				if retrieved != tt.value {
					t.Errorf("Expected %v, got %v", tt.value, retrieved)
				}
			}
		})
	}
}

func TestStorage_GetString(t *testing.T) {
	storage, err := NewStorage("test-get-string")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected string
	}{
		{"valid string", "str", "hello", "hello"},
		{"non-string value", "int", 42, ""},
		{"missing key", "missing", nil, ""},
		{"empty string", "empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != nil {
				storage.Set(tt.key, tt.value)
			}

			result := storage.GetString(tt.key)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStorage_GetInt(t *testing.T) {
	storage, err := NewStorage("test-get-int")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected int
	}{
		{"valid int", "int", 42, 42},
		{"float to int", "float", 3.14, 3},
		{"non-numeric value", "str", "hello", 0},
		{"missing key", "missing", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != nil {
				storage.Set(tt.key, tt.value)
			}

			result := storage.GetInt(tt.key)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestStorage_GetFloat64(t *testing.T) {
	storage, err := NewStorage("test-get-float")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected float64
	}{
		{"valid float64", "float", 3.14, 3.14},
		{"int to float64", "int", 42, 42.0},
		{"non-numeric value", "str", "hello", 0.0},
		{"missing key", "missing", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != nil {
				storage.Set(tt.key, tt.value)
			}

			result := storage.GetFloat64(tt.key)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestStorage_GetBool(t *testing.T) {
	storage, err := NewStorage("test-get-bool")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected bool
	}{
		{"valid true", "true", true, true},
		{"valid false", "false", false, false},
		{"non-bool value", "str", "hello", false},
		{"missing key", "missing", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != nil {
				storage.Set(tt.key, tt.value)
			}

			result := storage.GetBool(tt.key)
			if result != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestStorage_Delete(t *testing.T) {
	storage, err := NewStorage("test-delete")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Set some data
	storage.Set("key1", "value1")
	storage.Set("key2", "value2")

	// Verify data exists
	if !storage.Exists("key1") {
		t.Errorf("Expected key1 to exist")
	}

	// Delete key1
	if err := storage.Delete("key1"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify key1 is deleted
	if storage.Exists("key1") {
		t.Errorf("Expected key1 to be deleted")
	}

	// Verify key2 still exists
	if !storage.Exists("key2") {
		t.Errorf("Expected key2 to still exist")
	}

	// Delete non-existent key (should not error)
	if err := storage.Delete("non-existent"); err != nil {
		t.Errorf("Delete of non-existent key should not error: %v", err)
	}
}

func TestStorage_Exists(t *testing.T) {
	storage, err := NewStorage("test-exists")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Initially no keys should exist
	if storage.Exists("key1") {
		t.Errorf("Expected key1 to not exist initially")
	}

	// Set a key
	storage.Set("key1", "value1")

	// Now it should exist
	if !storage.Exists("key1") {
		t.Errorf("Expected key1 to exist after setting")
	}
}

func TestStorage_Keys(t *testing.T) {
	storage, err := NewStorage("test-keys")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Initially no keys
	keys := storage.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected no keys initially, got %v", keys)
	}

	// Add some keys
	storage.Set("key1", "value1")
	storage.Set("key2", "value2")
	storage.Set("key3", "value3")

	// Get keys
	keys = storage.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Verify all expected keys are present
	expectedKeys := map[string]bool{"key1": true, "key2": true, "key3": true}
	for _, key := range keys {
		if !expectedKeys[key] {
			t.Errorf("Unexpected key: %s", key)
		}
	}
}

func TestStorage_Clear(t *testing.T) {
	storage, err := NewStorage("test-clear")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Add some data
	storage.Set("key1", "value1")
	storage.Set("key2", "value2")

	// Verify data exists
	if len(storage.Keys()) != 2 {
		t.Errorf("Expected 2 keys before clear")
	}

	// Clear storage
	if err := storage.Clear(); err != nil {
		t.Errorf("Clear failed: %v", err)
	}

	// Verify data is cleared
	if len(storage.Keys()) != 0 {
		t.Errorf("Expected no keys after clear")
	}

	if storage.Exists("key1") {
		t.Errorf("Expected key1 to be cleared")
	}
}

func TestStorage_Persistence(t *testing.T) {
	// Create first storage instance
	storage1, err := NewStorage("test-persistence")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage1.filePath)

	// Set some data
	storage1.Set("key1", "value1")
	storage1.Set("key2", 42)

	// Create second storage instance (should load existing data)
	// Use the same file path to ensure persistence test works
	storage2 := &Storage{
		filePath: storage1.filePath,
		data:     make(map[string]interface{}),
	}

	// Load existing data
	if err := storage2.load(); err != nil {
		t.Fatalf("Failed to load existing data: %v", err)
	}

	// Verify data was loaded
	if !storage2.Exists("key1") {
		t.Errorf("Expected key1 to exist in second storage")
	}

	if storage2.GetString("key1") != "value1" {
		t.Errorf("Expected key1 to have value 'value1'")
	}

	if storage2.GetInt("key2") != 42 {
		t.Errorf("Expected key2 to have value 42")
	}
}

func TestStorage_ConcurrentAccess(t *testing.T) {
	storage, err := NewStorage("test-concurrent")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()
			key := fmt.Sprintf("key%d", i)
			value := fmt.Sprintf("value%d", i)
			storage.Set(key, value)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all data was written
	if len(storage.Keys()) != 10 {
		t.Errorf("Expected 10 keys, got %d", len(storage.Keys()))
	}

	// Test concurrent reads
	done = make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()
			key := fmt.Sprintf("key%d", i)
			storage.Get(key)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStorage_FileOperations(t *testing.T) {
	// Test with invalid file path (should fallback to current directory)
	storage, err := NewStorage("test-file-ops")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Test file path is accessible
	filePath := storage.GetFilePath()
	if filePath == "" {
		t.Errorf("Expected file path to be set")
	}

	// Test that file is created when data is saved
	storage.Set("test", "value")

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Expected storage file to exist")
	}

	// Test file permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Errorf("Failed to stat file: %v", err)
	}

	// Check file permissions based on location
	mode := info.Mode()
	if strings.HasPrefix(filePath, "/var/") {
		// System directory - should be 0600 (readable only by owner)
		if mode&0077 != 0 {
			t.Errorf("File in system directory should not be readable by group or others, mode: %o", mode)
		}
	} else {
		// Development fallback - should be 0644 (readable by owner and group)
		expectedMode := os.FileMode(0644)
		if mode&0777 != expectedMode {
			t.Errorf("File in development mode should have mode %o, got %o", expectedMode, mode&0777)
		}
	}
}

func TestStorage_CorruptedFile(t *testing.T) {
	// Create a storage instance
	storage, err := NewStorage("test-corrupted")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Write some data first
	storage.Set("key1", "value1")

	// Manually corrupt the file
	corruptedData := []byte("invalid json data")
	if err := os.WriteFile(storage.filePath, corruptedData, 0600); err != nil {
		t.Fatalf("Failed to write corrupted data: %v", err)
	}

	// Create new storage instance (should handle corrupted file gracefully)
	newStorage, err := NewStorage("test-corrupted")
	if err != nil {
		t.Fatalf("Failed to create storage with corrupted file: %v", err)
	}

	// Should start with empty data
	if len(newStorage.Keys()) != 0 {
		t.Errorf("Expected empty storage after corrupted file, got %d keys", len(newStorage.Keys()))
	}

	// Should be able to write new data
	newStorage.Set("newkey", "newvalue")
	if !newStorage.Exists("newkey") {
		t.Errorf("Expected to be able to write new data after corrupted file")
	}
}

func TestStorage_EmptyFile(t *testing.T) {
	// Create a storage instance
	storage, err := NewStorage("test-empty")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Create an empty file
	if err := os.WriteFile(storage.filePath, []byte(""), 0600); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Create new storage instance (should handle empty file gracefully)
	newStorage, err := NewStorage("test-empty")
	if err != nil {
		t.Fatalf("Failed to create storage with empty file: %v", err)
	}

	// Should start with empty data
	if len(newStorage.Keys()) != 0 {
		t.Errorf("Expected empty storage after empty file, got %d keys", len(newStorage.Keys()))
	}
}

// Benchmark tests
func BenchmarkStorage_Set(b *testing.B) {
	storage, err := NewStorage("bench-set")
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		storage.Set(key, value)
	}
}

func BenchmarkStorage_Get(b *testing.B) {
	storage, err := NewStorage("bench-get")
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		storage.Set(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		storage.Get(key)
	}
}

func BenchmarkStorage_ConcurrentSet(b *testing.B) {
	storage, err := NewStorage("bench-concurrent-set")
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer os.Remove(storage.filePath)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			value := fmt.Sprintf("value%d", i)
			storage.Set(key, value)
			i++
		}
	})
}
