package websocket

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/janhuddel/metrics-agent/internal/utils"
	"golang.org/x/net/websocket"
)

// ConnectionState represents the current state of the websocket connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateFailed
)

// Config represents the configuration for the websocket client
type Config struct {
	URL                  string        `json:"url"`
	ReconnectInterval    time.Duration `json:"reconnect_interval,omitempty"`
	MaxReconnectAttempts int           `json:"max_reconnect_attempts,omitempty"`
	ConnectionTimeout    time.Duration `json:"connection_timeout,omitempty"`
	ReadTimeout          time.Duration `json:"read_timeout,omitempty"`
	WriteTimeout         time.Duration `json:"write_timeout,omitempty"`
	MaxBackoffInterval   time.Duration `json:"max_backoff_interval,omitempty"`
	BackoffMultiplier    float64       `json:"backoff_multiplier,omitempty"`
	Origin               string        `json:"origin,omitempty"`
}

// MessageHandler is a function that processes incoming websocket messages
type MessageHandler func(message []byte) error

// Client represents a robust websocket client with automatic reconnection
type Client struct {
	config            Config
	handler           MessageHandler
	conn              *websocket.Conn
	state             ConnectionState
	stateMutex        sync.RWMutex
	reconnectAttempts int
	lastError         error
}

// NewClient creates a new websocket client with the given configuration and message handler
func NewClient(config Config, handler MessageHandler) (*Client, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("websocket URL is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("message handler is required")
	}

	// Set default values
	if config.ReconnectInterval == 0 {
		config.ReconnectInterval = 5 * time.Second
	}
	if config.MaxReconnectAttempts == 0 {
		config.MaxReconnectAttempts = 10
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 10 * time.Second
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.MaxBackoffInterval == 0 {
		config.MaxBackoffInterval = 5 * time.Minute
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.Origin == "" {
		config.Origin = "http://localhost"
	}

	return &Client{
		config:  config,
		handler: handler,
		state:   StateDisconnected,
	}, nil
}

// Run starts the websocket client with robust reconnection handling
func (c *Client) Run(ctx context.Context) error {
	return utils.WithPanicRecoveryAndReturnError("WebSocket client", "main", func() error {
		for {
			select {
			case <-ctx.Done():
				c.setState(StateDisconnected)
				return ctx.Err()
			default:
				// Attempt to connect
				if err := c.connect(ctx); err != nil {
					if c.isUnrecoverableError(err) {
						c.setState(StateFailed)
						return fmt.Errorf("unrecoverable connection error: %w", err)
					}

					// Wait before retrying
					if err := c.waitForReconnect(ctx); err != nil {
						return err
					}
					continue
				}

				// Connected successfully, start message processing
				if err := c.processMessages(ctx); err != nil {
					c.closeConnection()

					if c.isUnrecoverableError(err) {
						c.setState(StateFailed)
						return fmt.Errorf("unrecoverable processing error: %w", err)
					}

					// Wait before retrying
					if err := c.waitForReconnect(ctx); err != nil {
						return err
					}
				}
			}
		}
	})
}

// GetState returns the current connection state
func (c *Client) GetState() ConnectionState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state
}

// GetReconnectAttempts returns the number of reconnection attempts made
func (c *Client) GetReconnectAttempts() int {
	return c.reconnectAttempts
}

// GetLastError returns the last error encountered
func (c *Client) GetLastError() error {
	return c.lastError
}

// connect establishes a websocket connection with timeout
func (c *Client) connect(ctx context.Context) error {
	c.setState(StateConnecting)
	c.reconnectAttempts++

	utils.Infof("Attempting to connect to websocket (attempt %d/%d): %s",
		c.reconnectAttempts, c.config.MaxReconnectAttempts, c.config.URL)

	// Create a context with timeout for the connection
	connCtx, cancel := context.WithTimeout(ctx, c.config.ConnectionTimeout)
	defer cancel()

	// Use a channel to handle the connection attempt
	connChan := make(chan *websocket.Conn, 1)
	errChan := make(chan error, 1)

	go func() {
		conn, err := websocket.Dial(c.config.URL, "", c.config.Origin)
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	select {
	case <-connCtx.Done():
		return fmt.Errorf("connection timeout after %v", c.config.ConnectionTimeout)
	case err := <-errChan:
		c.lastError = err
		return fmt.Errorf("failed to connect to websocket: %w", err)
	case conn := <-connChan:
		c.conn = conn
		c.setState(StateConnected)
		c.reconnectAttempts = 0 // Reset on successful connection
		c.lastError = nil
		utils.Infof("Successfully connected to websocket")
		return nil
	}
}

// processMessages handles incoming websocket messages
func (c *Client) processMessages(ctx context.Context) error {
	// Set read timeout on the connection
	if err := c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read message from websocket
			var message []byte
			err := websocket.Message.Receive(c.conn, &message)
			if err != nil {
				c.lastError = err
				return fmt.Errorf("failed to receive websocket message: %w", err)
			}

			// Update read deadline for next message
			if err := c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
				utils.Warnf("Failed to update read deadline: %v", err)
			}

			// Process the message using the handler
			if err := c.handler(message); err != nil {
				utils.Errorf("Failed to process websocket message: %v", err)
				// Continue processing other messages even if one fails
				continue
			}
		}
	}
}

// closeConnection safely closes the websocket connection
func (c *Client) closeConnection() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.setState(StateDisconnected)
}

// waitForReconnect implements exponential backoff for reconnection attempts
func (c *Client) waitForReconnect(ctx context.Context) error {
	if c.reconnectAttempts >= c.config.MaxReconnectAttempts {
		c.setState(StateFailed)
		return fmt.Errorf("max reconnection attempts (%d) exceeded", c.config.MaxReconnectAttempts)
	}

	c.setState(StateReconnecting)

	// Calculate backoff delay with exponential backoff
	baseDelay := float64(c.config.ReconnectInterval)
	backoffDelay := baseDelay * math.Pow(c.config.BackoffMultiplier, float64(c.reconnectAttempts-1))

	// Cap the delay at max backoff interval
	if backoffDelay > float64(c.config.MaxBackoffInterval) {
		backoffDelay = float64(c.config.MaxBackoffInterval)
	}

	delay := time.Duration(backoffDelay)
	utils.Infof("Waiting %v before reconnection attempt %d/%d (last error: %v)",
		delay, c.reconnectAttempts, c.config.MaxReconnectAttempts, c.lastError)

	// Wait with context cancellation support
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// isUnrecoverableError determines if an error is unrecoverable and should cause client exit
func (c *Client) isUnrecoverableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return true
	}

	// Check for network errors that might be recoverable
	if netErr, ok := err.(net.Error); ok {
		// Temporary network errors are usually recoverable
		if netErr.Temporary() {
			return false
		}
		// Timeout errors might be recoverable
		if netErr.Timeout() {
			return false
		}
	}

	// Check for specific websocket errors
	if err.Error() == "EOF" {
		// EOF usually means connection was closed, which is recoverable
		return false
	}

	// Check for authentication/authorization errors (usually unrecoverable)
	errorStr := err.Error()
	if containsAny(errorStr, []string{"401", "403", "unauthorized", "forbidden"}) {
		return true
	}

	// Check for malformed URL errors (unrecoverable)
	if containsAny(errorStr, []string{"invalid URL", "malformed", "parse"}) {
		return true
	}

	// Most other errors are considered recoverable
	return false
}

// setState safely updates the connection state
func (c *Client) setState(state ConnectionState) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.state = state
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	if len(substrings) == 0 {
		return false
	}

	for _, substr := range substrings {
		if len(substr) == 0 {
			continue // Skip empty substrings
		}
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
