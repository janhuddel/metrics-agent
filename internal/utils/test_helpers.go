package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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

// CaptureLogOutput captures log output for testing (similar to the one in recovery_test.go)
func (tlc *TestLogCapture) CaptureLogOutput(fn func()) string {
	// Create a pipe to capture log output
	r, w, _ := os.Pipe()
	originalOutput := log.Writer()
	log.SetOutput(w)

	// Run the function
	fn()

	// Close the writer and restore original output
	w.Close()
	log.SetOutput(originalOutput)

	// Read the captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	return string(buf[:n])
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

// TestHTTPServer provides utilities for creating test HTTP servers
type TestHTTPServer struct {
	server *httptest.Server
}

// NewTestHTTPServer creates a new test HTTP server
func NewTestHTTPServer(handler http.HandlerFunc) *TestHTTPServer {
	return &TestHTTPServer{
		server: httptest.NewServer(handler),
	}
}

// GetURL returns the server URL
func (ths *TestHTTPServer) GetURL() string {
	return ths.server.URL
}

// Close closes the test server
func (ths *TestHTTPServer) Close() {
	ths.server.Close()
}

// CreateOAuth2TestClient creates an OAuth2Client with real storage for testing
func (th *TestHelper) CreateOAuth2TestClient(config OAuth2Config) *OAuth2Client {
	// Create a real storage instance for testing
	storage, _ := NewStorage("test-oauth2")
	return &OAuth2Client{
		config:  config,
		storage: storage,
	}
}

// CreateTestToken creates a test OAuth2 token with specified properties
func (tdg *TestDataGenerator) CreateTestToken(accessToken, refreshToken string, expiresIn int, expiresAt time.Time) *OAuth2Token {
	return &OAuth2Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		ExpiresAt:    expiresAt,
		Scope:        []string{"read", "write"},
	}
}

// CreateValidTestToken creates a valid test OAuth2 token
func (tdg *TestDataGenerator) CreateValidTestToken() *OAuth2Token {
	return tdg.CreateTestToken(
		"access-token-123",
		"refresh-token-456",
		3600,
		time.Now().Add(time.Hour),
	)
}

// CreateExpiredTestToken creates an expired test OAuth2 token
func (tdg *TestDataGenerator) CreateExpiredTestToken() *OAuth2Token {
	return tdg.CreateTestToken(
		"expired-access-token",
		"expired-refresh-token",
		3600,
		time.Now().Add(-time.Hour),
	)
}

// CreateTestOAuth2Config creates a test OAuth2 configuration
func (tdg *TestDataGenerator) CreateTestOAuth2Config() OAuth2Config {
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

// CreateTestOAuth2ConfigWithTokenURL creates a test OAuth2 configuration with custom token URL
func (tdg *TestDataGenerator) CreateTestOAuth2ConfigWithTokenURL(tokenURL string) OAuth2Config {
	config := tdg.CreateTestOAuth2Config()
	config.TokenURL = tokenURL
	return config
}

// CreateTestOAuth2ConfigWithClientID creates a test OAuth2 configuration with custom client ID
func (tdg *TestDataGenerator) CreateTestOAuth2ConfigWithClientID(clientID string) OAuth2Config {
	config := tdg.CreateTestOAuth2Config()
	config.ClientID = clientID
	return config
}

// CreateTestContextWithTimeout creates a context with timeout for testing
func (tch *TestContextHelper) CreateTestContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// CreateTestContextWithCancel creates a context with cancel for testing
func (tch *TestContextHelper) CreateTestContextWithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// CreateTestContextWithDeadline creates a context with deadline for testing
func (tch *TestContextHelper) CreateTestContextWithDeadline(deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), deadline)
}

// AssertOAuth2TokenEqual checks if two OAuth2 tokens are equal
func (tah *TestAssertionHelper) AssertOAuth2TokenEqual(t testing.TB, expected, actual *OAuth2Token, msg string) {
	if expected == nil && actual == nil {
		return
	}
	if expected == nil || actual == nil {
		t.Errorf("%s: expected token %v, got %v", msg, expected, actual)
		return
	}
	if expected.AccessToken != actual.AccessToken {
		t.Errorf("%s: expected AccessToken %s, got %s", msg, expected.AccessToken, actual.AccessToken)
	}
	if expected.RefreshToken != actual.RefreshToken {
		t.Errorf("%s: expected RefreshToken %s, got %s", msg, expected.RefreshToken, actual.RefreshToken)
	}
	if expected.ExpiresIn != actual.ExpiresIn {
		t.Errorf("%s: expected ExpiresIn %d, got %d", msg, expected.ExpiresIn, actual.ExpiresIn)
	}
}

// AssertOAuth2ConfigEqual checks if two OAuth2 configurations are equal
func (tah *TestAssertionHelper) AssertOAuth2ConfigEqual(t testing.TB, expected, actual OAuth2Config, msg string) {
	if expected.ClientID != actual.ClientID {
		t.Errorf("%s: expected ClientID %s, got %s", msg, expected.ClientID, actual.ClientID)
	}
	if expected.ClientSecret != actual.ClientSecret {
		t.Errorf("%s: expected ClientSecret %s, got %s", msg, expected.ClientSecret, actual.ClientSecret)
	}
	if expected.AuthURL != actual.AuthURL {
		t.Errorf("%s: expected AuthURL %s, got %s", msg, expected.AuthURL, actual.AuthURL)
	}
	if expected.TokenURL != actual.TokenURL {
		t.Errorf("%s: expected TokenURL %s, got %s", msg, expected.TokenURL, actual.TokenURL)
	}
	if expected.RedirectURI != actual.RedirectURI {
		t.Errorf("%s: expected RedirectURI %s, got %s", msg, expected.RedirectURI, actual.RedirectURI)
	}
	if expected.Scope != actual.Scope {
		t.Errorf("%s: expected Scope %s, got %s", msg, expected.Scope, actual.Scope)
	}
	if expected.State != actual.State {
		t.Errorf("%s: expected State %s, got %s", msg, expected.State, actual.State)
	}
}

// AssertHTTPResponseEqual checks if HTTP response matches expected values
func (tah *TestAssertionHelper) AssertHTTPResponseEqual(t testing.TB, expectedStatus int, actualStatus int, msg string) {
	if expectedStatus != actualStatus {
		t.Errorf("%s: expected status %d, got %d", msg, expectedStatus, actualStatus)
	}
}

// AssertErrorContains checks if an error contains a specific substring
func (tah *TestAssertionHelper) AssertErrorContains(t testing.TB, err error, substr string, msg string) {
	if err == nil {
		t.Errorf("%s: expected error but got none", msg)
		return
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("%s: expected error to contain '%s', got: %v", msg, substr, err)
	}
}

// AssertNoError checks if there's no error
func (tah *TestAssertionHelper) AssertNoError(t testing.TB, err error, msg string) {
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}

// AssertError checks if there's an error
func (tah *TestAssertionHelper) AssertError(t testing.TB, err error, msg string) {
	if err == nil {
		t.Errorf("%s: expected error but got none", msg)
	}
}

// AssertNotNil checks if a value is not nil
func (tah *TestAssertionHelper) AssertNotNil(t testing.TB, value interface{}, msg string) {
	if value == nil {
		t.Errorf("%s: expected non-nil value but got nil", msg)
	}
}

// AssertNil checks if a value is nil
func (tah *TestAssertionHelper) AssertNil(t testing.TB, value interface{}, msg string) {
	if value != nil {
		// Check if it's a nil pointer (common Go gotcha)
		if reflect.ValueOf(value).IsNil() {
			return // It's a nil pointer, which is what we want
		}
		t.Errorf("%s: expected nil value but got %v", msg, value)
	}
}
