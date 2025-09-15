package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// createTestOAuth2Client creates an OAuth2Client with real storage for testing
func createTestOAuth2Client(config OAuth2Config) *OAuth2Client {
	th := NewTestHelper()
	return th.CreateOAuth2TestClient(config)
}

func TestNewOAuth2Client(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()

	tests := []struct {
		name        string
		config      OAuth2Config
		moduleName  string
		expectError bool
	}{
		{
			name:        "valid config",
			config:      tdg.CreateTestOAuth2Config(),
			moduleName:  "test-module",
			expectError: false,
		},
		{
			name:        "empty module name",
			config:      tdg.CreateTestOAuth2Config(),
			moduleName:  "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewOAuth2Client(tt.config, tt.moduleName)
			if tt.expectError {
				tah.AssertError(t, err, "Expected error but got none")
				return
			}

			tah.AssertNoError(t, err, "Unexpected error")
			tah.AssertNotNil(t, client, "Expected client but got nil")
			tah.AssertOAuth2ConfigEqual(t, tt.config, client.config, "Config mismatch")
			tah.AssertNotNil(t, client.storage, "Expected storage to be initialized")

			// Clean up
			if client.storage != nil {
				os.Remove(client.storage.GetFilePath())
			}
		})
	}
}

func TestOAuth2Client_StoreToken(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()

	client := createTestOAuth2Client(tdg.CreateTestOAuth2ConfigWithClientID("test-client-id"))
	token := tdg.CreateValidTestToken()

	err := client.storeToken(token)
	tah.AssertNoError(t, err, "storeToken failed")

	// Verify token was stored
	storedData := client.storage.Get("oauth2_token")
	tah.AssertNotNil(t, storedData, "Expected token to be stored")

	data, ok := storedData.(map[string]interface{})
	if !ok {
		t.Errorf("Expected stored data to be map[string]interface{}")
		return
	}

	if data["access_token"] != token.AccessToken {
		t.Errorf("Expected access_token %s, got %s", token.AccessToken, data["access_token"])
	}

	if data["refresh_token"] != token.RefreshToken {
		t.Errorf("Expected refresh_token %s, got %s", token.RefreshToken, data["refresh_token"])
	}

	if data["client_id"] != client.config.ClientID {
		t.Errorf("Expected client_id %s, got %s", client.config.ClientID, data["client_id"])
	}
}

func TestOAuth2Client_LoadStoredToken(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()

	tests := []struct {
		name          string
		storedData    map[string]interface{}
		clientID      string
		expectToken   bool
		expectError   bool
		expectedToken *OAuth2Token
	}{
		{
			name: "valid stored token",
			storedData: map[string]interface{}{
				"access_token":  "access-token-123",
				"refresh_token": "refresh-token-456",
				"expires_at":    time.Now().Add(time.Hour).Format(time.RFC3339),
				"client_id":     "test-client-id",
			},
			clientID:    "test-client-id",
			expectToken: true,
			expectError: false,
			expectedToken: &OAuth2Token{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-456",
			},
		},
		{
			name:        "no stored token",
			storedData:  nil,
			clientID:    "test-client-id",
			expectToken: false,
			expectError: false,
		},
		{
			name: "client ID mismatch",
			storedData: map[string]interface{}{
				"access_token":  "access-token-123",
				"refresh_token": "refresh-token-456",
				"expires_at":    time.Now().Add(time.Hour).Format(time.RFC3339),
				"client_id":     "different-client-id",
			},
			clientID:    "test-client-id",
			expectToken: false,
			expectError: false,
		},
		{
			name: "invalid token data format",
			storedData: map[string]interface{}{
				"client_id": "test-client-id", // Include client_id to pass the first check
				"invalid":   "data",
			},
			clientID:    "test-client-id",
			expectToken: false,
			expectError: true,
		},
		{
			name: "invalid expires_at format",
			storedData: map[string]interface{}{
				"access_token":  "access-token-123",
				"refresh_token": "refresh-token-456",
				"expires_at":    "invalid-date",
				"client_id":     "test-client-id",
			},
			clientID:    "test-client-id",
			expectToken: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestOAuth2Client(tdg.CreateTestOAuth2ConfigWithClientID(tt.clientID))
			defer os.Remove(client.storage.GetFilePath())

			// Clear any existing data first
			client.storage.Clear()

			// Set up stored data
			if tt.storedData != nil {
				client.storage.Set("oauth2_token", tt.storedData)
			}

			token, err := client.loadStoredToken()

			if tt.expectError {
				tah.AssertError(t, err, "Expected error but got none")
				return
			}

			tah.AssertNoError(t, err, "Unexpected error")

			if tt.expectToken {
				tah.AssertNotNil(t, token, "Expected token but got nil")
				if tt.expectedToken != nil {
					tah.AssertOAuth2TokenEqual(t, tt.expectedToken, token, "Token mismatch")
				}
			} else {
				tah.AssertNil(t, token, "Expected no token but got one")
			}
		})
	}
}

func TestOAuth2Client_ExchangeAuthorizationCode(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()

	tests := []struct {
		name           string
		authCode       string
		redirectURI    string
		serverResponse string
		serverStatus   int
		expectError    bool
		expectedToken  *OAuth2Token
	}{
		{
			name:        "successful token exchange",
			authCode:    "auth-code-123",
			redirectURI: "http://localhost:8080/callback",
			serverResponse: `{
				"access_token": "access-token-123",
				"refresh_token": "refresh-token-456",
				"expires_in": 3600,
				"scope": ["read", "write"]
			}`,
			serverStatus: http.StatusOK,
			expectError:  false,
			expectedToken: &OAuth2Token{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-456",
				ExpiresIn:    3600,
				Scope:        []string{"read", "write"},
			},
		},
		{
			name:        "server error response",
			authCode:    "auth-code-123",
			redirectURI: "http://localhost:8080/callback",
			serverResponse: `{
				"error": "invalid_grant",
				"error_description": "Authorization code expired"
			}`,
			serverStatus: http.StatusBadRequest,
			expectError:  true,
		},
		{
			name:           "invalid JSON response",
			authCode:       "auth-code-123",
			redirectURI:    "http://localhost:8080/callback",
			serverResponse: `invalid json`,
			serverStatus:   http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and content type
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
				}

				// Parse form data
				err := r.ParseForm()
				if err != nil {
					t.Errorf("Failed to parse form: %v", err)
					return
				}

				// Verify form data
				if r.Form.Get("grant_type") != "authorization_code" {
					t.Errorf("Expected grant_type authorization_code, got %s", r.Form.Get("grant_type"))
				}
				if r.Form.Get("code") != tt.authCode {
					t.Errorf("Expected code %s, got %s", tt.authCode, r.Form.Get("code"))
				}
				if r.Form.Get("redirect_uri") != tt.redirectURI {
					t.Errorf("Expected redirect_uri %s, got %s", tt.redirectURI, r.Form.Get("redirect_uri"))
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := createTestOAuth2Client(tdg.CreateTestOAuth2ConfigWithTokenURL(server.URL))

			token, err := client.exchangeAuthorizationCode(tt.authCode, tt.redirectURI)

			if tt.expectError {
				tah.AssertError(t, err, "Expected error but got none")
				return
			}

			tah.AssertNoError(t, err, "Unexpected error")
			tah.AssertNotNil(t, token, "Expected token but got nil")

			if tt.expectedToken != nil {
				tah.AssertOAuth2TokenEqual(t, tt.expectedToken, token, "Token mismatch")

				// Check that ExpiresAt is set (should be approximately now + expires_in)
				expectedExpiry := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
				if token.ExpiresAt.Before(expectedExpiry.Add(-time.Minute)) || token.ExpiresAt.After(expectedExpiry.Add(time.Minute)) {
					t.Errorf("Expected ExpiresAt to be approximately %v, got %v", expectedExpiry, token.ExpiresAt)
				}
			}
		})
	}
}

func TestOAuth2Client_RefreshToken(t *testing.T) {
	tests := []struct {
		name           string
		refreshToken   string
		serverResponse string
		serverStatus   int
		expectError    bool
		expectedToken  *OAuth2Token
	}{
		{
			name:         "successful token refresh",
			refreshToken: "refresh-token-456",
			serverResponse: `{
				"access_token": "new-access-token-789",
				"refresh_token": "new-refresh-token-101",
				"expires_in": 3600
			}`,
			serverStatus: http.StatusOK,
			expectError:  false,
			expectedToken: &OAuth2Token{
				AccessToken:  "new-access-token-789",
				RefreshToken: "new-refresh-token-101",
				ExpiresIn:    3600,
			},
		},
		{
			name:         "server error response",
			refreshToken: "invalid-refresh-token",
			serverResponse: `{
				"error": "invalid_grant",
				"error_description": "Refresh token expired"
			}`,
			serverStatus: http.StatusBadRequest,
			expectError:  true,
		},
		{
			name:           "invalid JSON response",
			refreshToken:   "refresh-token-456",
			serverResponse: `invalid json`,
			serverStatus:   http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and content type
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
				}

				// Parse form data
				err := r.ParseForm()
				if err != nil {
					t.Errorf("Failed to parse form: %v", err)
					return
				}

				// Verify form data
				if r.Form.Get("grant_type") != "refresh_token" {
					t.Errorf("Expected grant_type refresh_token, got %s", r.Form.Get("grant_type"))
				}
				if r.Form.Get("refresh_token") != tt.refreshToken {
					t.Errorf("Expected refresh_token %s, got %s", tt.refreshToken, r.Form.Get("refresh_token"))
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := createTestOAuth2Client(OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				TokenURL:     server.URL,
			})

			token, err := client.refreshToken(tt.refreshToken)

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

			if token == nil {
				t.Errorf("Expected token but got nil")
				return
			}

			if tt.expectedToken != nil {
				if token.AccessToken != tt.expectedToken.AccessToken {
					t.Errorf("Expected access_token %s, got %s", tt.expectedToken.AccessToken, token.AccessToken)
				}
				if token.RefreshToken != tt.expectedToken.RefreshToken {
					t.Errorf("Expected refresh_token %s, got %s", tt.expectedToken.RefreshToken, token.RefreshToken)
				}
				if token.ExpiresIn != tt.expectedToken.ExpiresIn {
					t.Errorf("Expected expires_in %d, got %d", tt.expectedToken.ExpiresIn, token.ExpiresIn)
				}

				// Check that ExpiresAt is set
				expectedExpiry := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
				if token.ExpiresAt.Before(expectedExpiry.Add(-time.Minute)) || token.ExpiresAt.After(expectedExpiry.Add(time.Minute)) {
					t.Errorf("Expected ExpiresAt to be approximately %v, got %v", expectedExpiry, token.ExpiresAt)
				}
			}
		})
	}
}

func TestOAuth2Client_PerformWebAuthorization(t *testing.T) {
	client := createTestOAuth2Client(OAuth2Config{
		ClientID: "test-client-id",
		AuthURL:  "https://example.com/auth",
		Scope:    "read write",
		State:    "test-state",
	})

	// Test that the function starts a server and returns proper URLs
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This test is limited because we can't easily test the full web flow
	// We mainly test that the function doesn't crash and returns appropriate errors
	_, _, err := client.performWebAuthorization(ctx)
	if err == nil {
		t.Errorf("Expected timeout error due to context cancellation")
	}

	// Verify the error is context-related
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context-related error, got: %v", err)
	}
}

func TestOAuth2Client_Authenticate_WithValidStoredToken(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()

	client := createTestOAuth2Client(tdg.CreateTestOAuth2ConfigWithClientID("test-client-id"))

	// Store a valid token
	validToken := tdg.CreateValidTestToken()
	client.storeToken(validToken)

	ctx := context.Background()
	token, err := client.Authenticate(ctx)

	tah.AssertNoError(t, err, "Unexpected error")
	tah.AssertNotNil(t, token, "Expected token but got nil")

	if token.AccessToken != validToken.AccessToken {
		t.Errorf("Expected access_token %s, got %s", validToken.AccessToken, token.AccessToken)
	}
}

func TestOAuth2Client_Authenticate_WithExpiredStoredToken(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()

	// Create test server for token refresh
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"access_token": "new-access-token-789",
			"refresh_token": "new-refresh-token-101",
			"expires_in": 3600
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestOAuth2Client(tdg.CreateTestOAuth2ConfigWithTokenURL(server.URL))

	// Store an expired token
	expiredToken := tdg.CreateExpiredTestToken()
	client.storeToken(expiredToken)

	ctx := context.Background()
	token, err := client.Authenticate(ctx)

	tah.AssertNoError(t, err, "Unexpected error")
	tah.AssertNotNil(t, token, "Expected token but got nil")

	// Should get the refreshed token
	if token.AccessToken != "new-access-token-789" {
		t.Errorf("Expected refreshed access_token, got %s", token.AccessToken)
	}
}

func TestOAuth2Client_Authenticate_WithInvalidStoredToken(t *testing.T) {
	tdg := NewTestDataGenerator()
	tah := NewTestAssertionHelper()
	tch := NewTestContextHelper()

	client := createTestOAuth2Client(tdg.CreateTestOAuth2ConfigWithClientID("test-client-id"))

	// Store invalid token data
	client.storage.Set("oauth2_token", "invalid-data")

	ctx, cancel := tch.CreateTestContextWithTimeout(100 * time.Millisecond)
	defer cancel()

	// Should attempt web authorization (which will timeout due to context)
	_, err := client.Authenticate(ctx)

	// Should get context-related error
	tah.AssertError(t, err, "Expected error due to context timeout")
	tah.AssertErrorContains(t, err, "context", "Expected context-related error")
}

// TestOAuth2Client_ForceRefresh tests the ForceRefresh method
func TestOAuth2Client_ForceRefresh(t *testing.T) {
	tests := []struct {
		name           string
		setupStorage   func(*OAuth2Client)
		expectedError  bool
		expectedTokens bool
	}{
		{
			name: "successful_force_refresh",
			setupStorage: func(client *OAuth2Client) {
				// Store a valid token with refresh token
				token := &OAuth2Token{
					AccessToken:  "old-access-token",
					RefreshToken: "valid-refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			expectedError:  false,
			expectedTokens: true,
		},
		{
			name: "no_stored_token",
			setupStorage: func(client *OAuth2Client) {
				// No token stored - clear storage
				client.storage.Clear()
			},
			expectedError:  true,
			expectedTokens: false,
		},
		{
			name: "no_refresh_token",
			setupStorage: func(client *OAuth2Client) {
				// Store token without refresh token
				token := &OAuth2Token{
					AccessToken:  "access-token",
					RefreshToken: "",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			expectedError:  true,
			expectedTokens: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server that returns successful refresh response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" && r.Method == "POST" {
					// Simulate successful token refresh
					response := `{
						"access_token": "new-access-token",
						"refresh_token": "new-refresh-token",
						"expires_in": 3600,
						"scope": ["read", "write"]
					}`
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(response))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			// Create OAuth2Client
			client := createTestOAuth2Client(OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				TokenURL:     server.URL + "/token",
			})

			// Setup storage
			tt.setupStorage(client)

			// Test ForceRefresh
			ctx := context.Background()
			token, err := client.ForceRefresh(ctx)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if token == nil {
					t.Errorf("Expected token but got nil")
					return
				}
				if token.AccessToken != "new-access-token" {
					t.Errorf("Expected new access token, got: %s", token.AccessToken)
				}
			}
		})
	}
}

// TestOAuth2Client_AuthenticatedRequest tests the AuthenticatedRequest method
func TestOAuth2Client_AuthenticatedRequest(t *testing.T) {
	tests := []struct {
		name            string
		setupStorage    func(*OAuth2Client)
		serverResponse  func(w http.ResponseWriter, r *http.Request)
		expectedError   bool
		expectedStatus  int
		expectedRetries int
	}{
		{
			name: "successful_request",
			setupStorage: func(client *OAuth2Client) {
				// Store a valid token
				token := &OAuth2Token{
					AccessToken:  "valid-access-token",
					RefreshToken: "valid-refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Check authorization header
				auth := r.Header.Get("Authorization")
				if auth != "Bearer valid-access-token" {
					t.Errorf("Expected Bearer valid-access-token, got: %s", auth)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "success"}`))
			},
			expectedError:   false,
			expectedStatus:  http.StatusOK,
			expectedRetries: 0,
		},
		{
			name: "unauthorized_with_successful_refresh",
			setupStorage: func(client *OAuth2Client) {
				// Store a valid token
				token := &OAuth2Token{
					AccessToken:  "old-access-token",
					RefreshToken: "valid-refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if strings.Contains(auth, "old-access-token") {
					// First request with old token - return 401
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error": "invalid_token"}`))
					return
				}
				if strings.Contains(auth, "new-access-token") {
					// Second request with new token - return success
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "success"}`))
					return
				}
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedError:   false,
			expectedStatus:  http.StatusOK,
			expectedRetries: 1,
		},
		{
			name: "unauthorized_with_failed_refresh",
			setupStorage: func(client *OAuth2Client) {
				// Store a token with invalid refresh token
				token := &OAuth2Token{
					AccessToken:  "old-access-token",
					RefreshToken: "invalid-refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" && r.Method == "POST" {
					// Token refresh fails
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "invalid_grant"}`))
					return
				}
				// API request returns 401
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "invalid_token"}`))
			},
			expectedError:   true,
			expectedStatus:  0,
			expectedRetries: 2, // Max retries reached
		},
		{
			name: "forbidden_with_successful_refresh",
			setupStorage: func(client *OAuth2Client) {
				// Store a valid token
				token := &OAuth2Token{
					AccessToken:  "old-access-token",
					RefreshToken: "valid-refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if strings.Contains(auth, "old-access-token") {
					// First request with old token - return 403
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"error": "insufficient_scope"}`))
					return
				}
				if strings.Contains(auth, "new-access-token") {
					// Second request with new token - return success
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "success"}`))
					return
				}
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedError:   false,
			expectedStatus:  http.StatusOK,
			expectedRetries: 1,
		},
		{
			name: "other_http_error",
			setupStorage: func(client *OAuth2Client) {
				// Store a valid token
				token := &OAuth2Token{
					AccessToken:  "valid-access-token",
					RefreshToken: "valid-refresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				client.storeToken(token)
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Return 500 error (not auth-related)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "server_error"}`))
			},
			expectedError:   false,
			expectedStatus:  http.StatusInternalServerError,
			expectedRetries: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" && r.Method == "POST" {
					// Token refresh endpoint
					response := `{
						"access_token": "new-access-token",
						"refresh_token": "new-refresh-token",
						"expires_in": 3600,
						"scope": ["read", "write"]
					}`
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(response))
					return
				}

				// API endpoint
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			// Create OAuth2Client
			client := createTestOAuth2Client(OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				TokenURL:     server.URL + "/token",
			})

			// Setup storage
			tt.setupStorage(client)

			// Create HTTP request
			req, err := http.NewRequest("GET", server.URL+"/api/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Test AuthenticatedRequest
			ctx := context.Background()
			httpClient := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.AuthenticatedRequest(ctx, httpClient, req)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if resp != nil {
					resp.Body.Close()
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp == nil {
					t.Errorf("Expected response but got nil")
				} else {
					defer resp.Body.Close()
					if resp.StatusCode != tt.expectedStatus {
						t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
					}
				}
			}
		})
	}
}

// TestOAuth2Client_AuthenticatedRequest_ContextCancellation tests context cancellation
func TestOAuth2Client_AuthenticatedRequest_ContextCancellation(t *testing.T) {
	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	// Create OAuth2Client
	client := createTestOAuth2Client(OAuth2Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TokenURL:     server.URL + "/token",
	})

	// Store a valid token
	token := &OAuth2Token{
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	client.storeToken(token)

	// Create HTTP request
	req, err := http.NewRequest("GET", server.URL+"/api/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	httpClient := &http.Client{Timeout: 5 * time.Second}
	_, err = client.AuthenticatedRequest(ctx, httpClient, req)

	if err == nil {
		t.Errorf("Expected error due to context cancellation")
		return
	}

	if !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context-related error, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkOAuth2Client_StoreToken(b *testing.B) {
	client := createTestOAuth2Client(OAuth2Config{
		ClientID: "test-client-id",
	})

	token := &OAuth2Token{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.storeToken(token)
	}
}

func BenchmarkOAuth2Client_LoadStoredToken(b *testing.B) {
	client := createTestOAuth2Client(OAuth2Config{
		ClientID: "test-client-id",
	})

	// Pre-populate with token data
	tokenData := map[string]interface{}{
		"access_token":  "access-token-123",
		"refresh_token": "refresh-token-456",
		"expires_at":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"client_id":     "test-client-id",
	}
	client.storage.Set("oauth2_token", tokenData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.loadStoredToken()
	}
}
