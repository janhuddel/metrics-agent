// Package utils provides utility functions for the metrics agent.
// This file contains centralized OAuth2 authentication utilities for modules.
package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

// OAuth2Config represents the configuration for OAuth2 authentication.
type OAuth2Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURI  string
	Scope        string
	State        string
	Hostname     string // Optional hostname/IP for redirect URI (defaults to localhost)
}

// OAuth2Token represents an OAuth2 token response.
type OAuth2Token struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	ExpiresAt    time.Time
}

// OAuth2Client provides OAuth2 authentication functionality.
type OAuth2Client struct {
	config  OAuth2Config
	storage *Storage
}

// NewOAuth2Client creates a new OAuth2 client.
func NewOAuth2Client(config OAuth2Config, moduleName string) (*OAuth2Client, error) {
	Debugf("Creating OAuth2 client for module: %s", moduleName)
	storage, err := NewStorage(moduleName)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	Debugf("OAuth2 client created successfully for module: %s", moduleName)
	return &OAuth2Client{
		config:  config,
		storage: storage,
	}, nil
}

// GetConfig returns the OAuth2 configuration (for testing purposes).
func (c *OAuth2Client) GetConfig() OAuth2Config {
	return c.config
}

// Authenticate performs OAuth2 authentication using Authorization Code flow.
// It will try to use stored tokens first, then perform web authorization if needed.
func (c *OAuth2Client) Authenticate(ctx context.Context) (*OAuth2Token, error) {
	Debugf("Starting OAuth2 authentication process")
	// Try to load existing tokens
	if token, err := c.loadStoredToken(); err == nil && token != nil {
		Debugf("Found stored token, checking expiry")
		// Check if token is still valid
		if time.Now().Before(token.ExpiresAt.Add(-5 * time.Minute)) {
			Debugf("Stored token is still valid, using cached token")
			return token, nil
		}

		// Try to refresh the token
		Debugf("Stored token expired, attempting to refresh")
		if refreshedToken, err := c.refreshToken(token.RefreshToken); err == nil {
			return refreshedToken, nil
		} else {
			Warnf("Token refresh failed: %v", err)
		}
	}

	// Need to perform initial authorization
	Infof("Starting OAuth2 authorization flow...")
	authCode, redirectURI, err := c.performWebAuthorization(ctx)
	if err != nil {
		return nil, fmt.Errorf("web authorization failed: %w", err)
	}

	// Exchange authorization code for tokens
	token, err := c.exchangeAuthorizationCode(authCode, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Store the new token
	if err := c.storeToken(token); err != nil {
		Warnf("Failed to store token: %v", err)
	}

	return token, nil
}

// performWebAuthorization starts an embedded web server to handle OAuth2 authorization.
func (c *OAuth2Client) performWebAuthorization(ctx context.Context) (string, string, error) {
	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", "", fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	// Use configured hostname or default to localhost
	hostname := c.config.Hostname
	if hostname == "" {
		hostname = "localhost"
	}
	redirectURI := fmt.Sprintf("http://%s:%d/callback", hostname, port)

	// Create authorization URL
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&state=%s",
		c.config.AuthURL,
		c.config.ClientID,
		redirectURI,
		c.config.Scope,
		c.config.State)

	// Channel to receive the authorization code
	authCodeChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	// Create HTTP server
	mux := http.NewServeMux()

	// Landing page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>OAuth2 Authorization</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .button { background: #007bff; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; display: inline-block; margin: 10px 0; }
        .button:hover { background: #0056b3; }
        .info { background: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }
    </style>
</head>
<body>
    <h1>OAuth2 Authorization</h1>
    <div class="info">
        <p>Click the button below to authorize the application.</p>
    </div>
    <a href="{{.AuthURL}}" class="button">Authorize Application</a>
    <p><small>This will open the authorization page in a new tab.</small></p>
</body>
</html>`

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl, _ := template.New("auth").Parse(html)
		tmpl.Execute(w, map[string]string{"AuthURL": authURL})
	})

	// Callback handler
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errorParam := r.URL.Query().Get("error")

		if errorParam != "" {
			errorDesc := r.URL.Query().Get("error_description")
			errorChan <- fmt.Errorf("authorization failed: %s - %s", errorParam, errorDesc)
			return
		}

		if code == "" {
			errorChan <- fmt.Errorf("no authorization code received")
			return
		}

		// Send success response
		html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Authorization Successful</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .success { background: #d4edda; color: #155724; padding: 15px; border-radius: 4px; margin: 20px 0; }
    </style>
</head>
<body>
    <h1>Authorization Successful!</h1>
    <div class="success">
        <p>✅ Your application has been successfully authorized.</p>
        <p>You can now close this browser tab. The application will continue running.</p>
    </div>
</body>
</html>`

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))

		// Send the code to the channel
		authCodeChan <- code
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errorChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	/*
		// Open browser automatically
		Infof("Opening browser for authorization...")
		Infof("If the browser doesn't open automatically, please visit: http://%s:%d", hostname, port)

		// Try to open browser
		if err := openBrowser(fmt.Sprintf("http://%s:%d", hostname, port)); err != nil {
			Warnf("Could not open browser automatically: %v", err)
			Infof("Please manually open: http://%s:%d", hostname, port)
		}
	*/

	Infof("Please manually open: http://%s:%d", hostname, port)

	// Wait for authorization code or error with context cancellation support
	select {
	case authCode := <-authCodeChan:
		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)

		Infof("Authorization successful!")
		return authCode, redirectURI, nil

	case err := <-errorChan:
		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)

		return "", "", err

	case <-ctx.Done():
		// Context cancelled - this is the most important case for signal handling
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)

		return "", "", ctx.Err()

	case <-time.After(5 * time.Minute):
		// Timeout after 5 minutes
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)

		return "", "", fmt.Errorf("authorization timeout - please try again")
	}
}

// exchangeAuthorizationCode exchanges an authorization code for access and refresh tokens.
func (c *OAuth2Client) exchangeAuthorizationCode(authCode, redirectURI string) (*OAuth2Token, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("code", authCode)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", c.config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		Errorf("OAuth2 token exchange failed - Status: %d, Response: %s", resp.StatusCode, string(body))

		// Try to parse error response for better error messages
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}

		if err := json.Unmarshal(body, &errorResp); err == nil {
			switch errorResp.Error {
			case "invalid_grant":
				Errorf("Authorization code is invalid, expired, or already used")
				Errorf("Common causes:")
				Errorf("1. Authorization code expired (they expire within 10 minutes)")
				Errorf("2. Authorization code already used (can only be used once)")
				Errorf("3. Wrong redirect URI in authorization vs token exchange")
				Errorf("4. Incorrect client_id or client_secret")
				Errorf("Please get a fresh authorization code and try again")
			case "invalid_client":
				Errorf("Invalid client credentials - check your client_id and client_secret")
			case "invalid_request":
				Errorf("Invalid request - check your authorization code and redirect URI")
			default:
				Errorf("Error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
			}
		}

		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token OAuth2Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	Infof("Successfully obtained OAuth2 tokens")
	Debugf("Access token expires at: %s", token.ExpiresAt.Format(time.RFC3339))
	Debugf("Scope: %v", token.Scope)

	return &token, nil
}

// refreshToken refreshes an OAuth2 token using the refresh token.
func (c *OAuth2Client) refreshToken(refreshToken string) (*OAuth2Token, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequest("POST", c.config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		Errorf("OAuth2 token refresh failed - Status: %d, Response: %s", resp.StatusCode, string(body))

		// Try to parse error response for better error messages
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}

		if err := json.Unmarshal(body, &errorResp); err == nil {
			switch errorResp.Error {
			case "invalid_grant":
				Errorf("Refresh token is invalid, expired, or revoked")
				Errorf("Common causes:")
				Errorf("1. Refresh token expired (refresh tokens can expire)")
				Errorf("2. User revoked access to the application")
				Errorf("3. Application credentials changed")
				Errorf("4. Refresh token was already used (some providers invalidate after use)")
				Errorf("Full re-authentication will be required")
			case "invalid_client":
				Errorf("Invalid client credentials - check your client_id and client_secret")
			case "invalid_request":
				Errorf("Invalid refresh request - check refresh token format")
			case "unsupported_grant_type":
				Errorf("Refresh token grant type not supported by this provider")
			default:
				Errorf("OAuth2 error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
			}
		}

		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token OAuth2Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	Infof("Successfully refreshed OAuth2 token")
	Debugf("New token expires at: %s", token.ExpiresAt.Format(time.RFC3339))

	// Store the refreshed token
	if err := c.storeToken(&token); err != nil {
		Warnf("Failed to store refreshed token: %v", err)
	}

	return &token, nil
}

// storeToken stores an OAuth2 token in the storage.
func (c *OAuth2Client) storeToken(token *OAuth2Token) error {
	tokenData := map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_at":    token.ExpiresAt.Format(time.RFC3339),
		"client_id":     c.config.ClientID,
		"last_updated":  time.Now().Format(time.RFC3339),
	}

	return c.storage.Set("oauth2_token", tokenData)
}

// AuthenticatedRequest makes an HTTP request with automatic token refresh and retry logic.
// It handles authentication errors (401/403) by refreshing tokens and retrying the request.
func (c *OAuth2Client) AuthenticatedRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	const maxRetries = 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Get current token (will refresh if needed)
		token, err := c.Authenticate(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get valid token: %w", err)
		}

		// Set authorization header
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		// Make the request with context
		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			return nil, err
		}

		// Check for authentication errors that might be resolved by token refresh
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close() // Close the response body before retrying

			if attempt < maxRetries {
				Warnf("Authentication failed (status %d), attempting token refresh and retry (attempt %d/%d)",
					resp.StatusCode, attempt+1, maxRetries)

				// Force token refresh
				_, err := c.ForceRefresh(ctx)
				if err != nil {
					Errorf("Token refresh failed: %v", err)
					// Continue to next attempt or return error
					if attempt == maxRetries {
						return nil, fmt.Errorf("API request failed with status %d after %d attempts (token refresh failed)",
							resp.StatusCode, maxRetries+1)
					}
					continue
				}

				// Continue to next attempt with refreshed token
				continue
			} else {
				// Max retries reached
				return nil, fmt.Errorf("API request failed with status %d after %d attempts",
					resp.StatusCode, maxRetries+1)
			}
		}

		// Return the response (success or other error)
		return resp, nil
	}

	// This should never be reached, but just in case
	return nil, fmt.Errorf("unexpected error in authenticated request")
}

// ForceRefresh forces a token refresh regardless of expiration time.
// This is useful when API calls fail with authentication errors.
func (c *OAuth2Client) ForceRefresh(ctx context.Context) (*OAuth2Token, error) {
	// Load stored token to get refresh token
	token, err := c.loadStoredToken()
	if err != nil || token == nil {
		Warnf("ForceRefresh: No stored token available for refresh")
		return nil, fmt.Errorf("no stored token available for refresh: %w", err)
	}

	if token.RefreshToken == "" {
		Warnf("ForceRefresh: No refresh token available in stored token")
		return nil, fmt.Errorf("no refresh token available")
	}

	Infof("ForceRefresh: Forcing token refresh due to API authentication failure")
	Debugf("ForceRefresh: Current token expires at: %s", token.ExpiresAt.Format(time.RFC3339))

	refreshedToken, err := c.refreshToken(token.RefreshToken)
	if err != nil {
		Errorf("ForceRefresh: Token refresh failed: %v", err)
		return nil, fmt.Errorf("forced token refresh failed: %w", err)
	}

	Infof("ForceRefresh: Successfully refreshed token, new expiry: %s", refreshedToken.ExpiresAt.Format(time.RFC3339))
	return refreshedToken, nil
}

// loadStoredToken loads an OAuth2 token from the storage.
func (c *OAuth2Client) loadStoredToken() (*OAuth2Token, error) {
	tokenData := c.storage.Get("oauth2_token")
	if tokenData == nil {
		return nil, nil // No token stored
	}

	data, ok := tokenData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid token data format")
	}

	// Verify client_id matches
	if clientID, ok := data["client_id"].(string); !ok || clientID != c.config.ClientID {
		Warnf("Stored token client_id mismatch, ignoring stored token")
		return nil, nil
	}

	// Parse expires_at
	expiresAtStr, ok := data["expires_at"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid expires_at in stored token")
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expires_at: %w", err)
	}

	// Create token from stored data
	token := &OAuth2Token{
		AccessToken:  data["access_token"].(string),
		RefreshToken: data["refresh_token"].(string),
		ExpiresAt:    expiresAt,
	}

	return token, nil
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) error {
	// Try different commands for different operating systems
	commands := []string{
		"xdg-open", // Linux
		"open",     // macOS
		"start",    // Windows
		"xdg-open", // Fallback
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd, url).Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("could not open browser with any available command")
}
