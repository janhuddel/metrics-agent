package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHelper provides utility functions for testing
type TestHelper struct{}

// NewTestHelper creates a new test helper instance
func NewTestHelper() *TestHelper {
	return &TestHelper{}
}

// CreateTempStorage creates a temporary storage instance for testing
func (th *TestHelper) CreateTempStorage(moduleName string) (*Storage, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "metrics-agent-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Create storage file path
	fileName := fmt.Sprintf("%s-storage.json", moduleName)
	filePath := filepath.Join(tempDir, fileName)

	// Create storage instance
	storage := &Storage{
		filePath: filePath,
		data:     make(map[string]interface{}),
	}

	return storage, nil
}

// CleanupTempStorage cleans up temporary storage files
func (th *TestHelper) CleanupTempStorage(storage *Storage) error {
	if storage == nil {
		return nil
	}

	// Remove the storage file
	if err := os.Remove(storage.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove storage file: %w", err)
	}

	// Remove the parent directory if it's empty
	parentDir := filepath.Dir(storage.filePath)
	if err := os.Remove(parentDir); err != nil && !os.IsNotExist(err) {
		// Ignore error if directory is not empty
	}

	return nil
}

// MockHTTPClient is a mock HTTP client for testing
type MockHTTPClient struct {
	responses map[string]*MockResponse
	requests  []*MockRequest
}

// MockResponse represents a mock HTTP response
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Delay      time.Duration
}

// MockRequest represents a captured HTTP request
type MockRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*MockResponse),
		requests:  make([]*MockRequest, 0),
	}
}

// SetResponse sets a mock response for a given URL pattern
func (m *MockHTTPClient) SetResponse(urlPattern string, response *MockResponse) {
	m.responses[urlPattern] = response
}

// GetRequests returns all captured requests
func (m *MockHTTPClient) GetRequests() []*MockRequest {
	return m.requests
}

// ClearRequests clears all captured requests
func (m *MockHTTPClient) ClearRequests() {
	m.requests = make([]*MockRequest, 0)
}

// Do performs a mock HTTP request
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Capture the request
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()

	headers := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	mockReq := &MockRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    string(body),
	}
	m.requests = append(m.requests, mockReq)

	// Find matching response
	var response *MockResponse
	for pattern, resp := range m.responses {
		if strings.Contains(req.URL.String(), pattern) {
			response = resp
			break
		}
	}

	if response == nil {
		// Default 404 response
		response = &MockResponse{
			StatusCode: http.StatusNotFound,
			Body:       "Not Found",
		}
	}

	// Apply delay if specified
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	}

	// Create mock response
	resp := &http.Response{
		StatusCode: response.StatusCode,
		Body:       io.NopCloser(strings.NewReader(response.Body)),
		Header:     make(http.Header),
	}

	// Set headers
	for key, value := range response.Headers {
		resp.Header.Set(key, value)
	}

	return resp, nil
}

// TestDataGenerator provides methods to generate test data
type TestDataGenerator struct{}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{}
}

// GenerateOAuth2Config generates a test OAuth2 configuration
func (tdg *TestDataGenerator) GenerateOAuth2Config() OAuth2Config {
	return OAuth2Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		RedirectURI:  "http://localhost:8080/callback",
		Scope:        "read write",
		State:        "test-state",
	}
}

// GenerateOAuth2Token generates a test OAuth2 token
func (tdg *TestDataGenerator) GenerateOAuth2Token() *OAuth2Token {
	return &OAuth2Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresIn:    3600,
		Scope:        []string{"read", "write"},
		ExpiresAt:    time.Now().Add(time.Hour),
	}
}

// GenerateExpiredOAuth2Token generates an expired OAuth2 token
func (tdg *TestDataGenerator) GenerateExpiredOAuth2Token() *OAuth2Token {
	return &OAuth2Token{
		AccessToken:  "expired-access-token",
		RefreshToken: "expired-refresh-token",
		ExpiresIn:    3600,
		Scope:        []string{"read", "write"},
		ExpiresAt:    time.Now().Add(-time.Hour), // Expired 1 hour ago
	}
}

// GenerateTokenResponseJSON generates a JSON response for token exchange
func (tdg *TestDataGenerator) GenerateTokenResponseJSON() string {
	token := tdg.GenerateOAuth2Token()
	data, _ := json.Marshal(token)
	return string(data)
}

// GenerateErrorResponseJSON generates a JSON error response
func (tdg *TestDataGenerator) GenerateErrorResponseJSON(errorCode, description string) string {
	errorResp := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}
	data, _ := json.Marshal(errorResp)
	return string(data)
}

// TestFileManager provides utilities for managing test files
type TestFileManager struct {
	tempDir string
}

// NewTestFileManager creates a new test file manager
func NewTestFileManager() (*TestFileManager, error) {
	tempDir, err := os.MkdirTemp("", "metrics-agent-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	return &TestFileManager{
		tempDir: tempDir,
	}, nil
}

// CreateTestFile creates a test file with the given content
func (tfm *TestFileManager) CreateTestFile(filename, content string) (string, error) {
	filePath := filepath.Join(tfm.tempDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create test file: %w", err)
	}
	return filePath, nil
}

// CreateCorruptedJSONFile creates a corrupted JSON file for testing
func (tfm *TestFileManager) CreateCorruptedJSONFile(filename string) (string, error) {
	return tfm.CreateTestFile(filename, "invalid json content")
}

// CreateEmptyFile creates an empty file for testing
func (tfm *TestFileManager) CreateEmptyFile(filename string) (string, error) {
	return tfm.CreateTestFile(filename, "")
}

// GetTempDir returns the temporary directory path
func (tfm *TestFileManager) GetTempDir() string {
	return tfm.tempDir
}

// Cleanup removes all test files and directories
func (tfm *TestFileManager) Cleanup() error {
	return os.RemoveAll(tfm.tempDir)
}

// TestAssertionHelper provides assertion utilities for testing
type TestAssertionHelper struct{}

// NewTestAssertionHelper creates a new test assertion helper
func NewTestAssertionHelper() *TestAssertionHelper {
	return &TestAssertionHelper{}
}

// AssertStringContains checks if a string contains a substring
func (tah *TestAssertionHelper) AssertStringContains(t testing.TB, str, substr string, msg string) {
	if !strings.Contains(str, substr) {
		t.Errorf("%s: expected string to contain '%s', got: %s", msg, substr, str)
	}
}

// AssertStringNotContains checks if a string does not contain a substring
func (tah *TestAssertionHelper) AssertStringNotContains(t testing.TB, str, substr string, msg string) {
	if strings.Contains(str, substr) {
		t.Errorf("%s: expected string to not contain '%s', got: %s", msg, substr, str)
	}
}

// AssertTimeApproximatelyEqual checks if two times are approximately equal
func (tah *TestAssertionHelper) AssertTimeApproximatelyEqual(t testing.TB, expected, actual time.Time, tolerance time.Duration, msg string) {
	diff := expected.Sub(actual)
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Errorf("%s: expected time %v, got %v (diff: %v, tolerance: %v)", msg, expected, actual, diff, tolerance)
	}
}

// AssertMapContains checks if a map contains the expected key-value pairs
func (tah *TestAssertionHelper) AssertMapContains(t testing.TB, actual map[string]interface{}, expected map[string]interface{}, msg string) {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("%s: expected key '%s' to exist in map", msg, key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("%s: expected key '%s' to have value %v, got %v", msg, key, expectedValue, actualValue)
		}
	}
}

// TestContextHelper provides utilities for managing test contexts
type TestContextHelper struct{}

// NewTestContextHelper creates a new test context helper
func NewTestContextHelper() *TestContextHelper {
	return &TestContextHelper{}
}

// CreateTimeoutContext creates a context with a timeout for testing
func (tch *TestContextHelper) CreateTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// CreateCancelledContext creates a cancelled context for testing
func (tch *TestContextHelper) CreateCancelledContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	return ctx, cancel
}

// TestLogCapture provides utilities for capturing log output during tests
type TestLogCapture struct {
	originalOutput io.Writer
	capturedOutput *strings.Builder
}

// NewTestLogCapture creates a new log capture instance
func NewTestLogCapture() *TestLogCapture {
	return &TestLogCapture{
		capturedOutput: &strings.Builder{},
	}
}

// StartCapture starts capturing log output
func (tlc *TestLogCapture) StartCapture() {
	// Note: This is a simplified version. In a real implementation,
	// you might want to use a more sophisticated log capture mechanism
	tlc.originalOutput = os.Stdout
	// In practice, you'd redirect log output here
}

// StopCapture stops capturing log output and returns the captured content
func (tlc *TestLogCapture) StopCapture() string {
	// Restore original output
	if file, ok := tlc.originalOutput.(*os.File); ok {
		os.Stdout = file
	}
	return tlc.capturedOutput.String()
}

// GetCapturedOutput returns the currently captured output
func (tlc *TestLogCapture) GetCapturedOutput() string {
	return tlc.capturedOutput.String()
}
