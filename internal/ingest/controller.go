package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// IngesterError represents an error from an ingester with context
type IngesterError struct {
	Ingester string
	Error    error
	Attempt  int
}

// Controller manages the lifecycle of metric ingesters and handles graceful/hard shutdowns
type Controller struct {
	config    *utils.AppConfig
	store     *utils.Store
	ingesters []types.Ingester

	// Shutdown state management
	shutdownOnce     sync.Once
	shutdownChan     chan struct{}
	hardShutdown     int32         // atomic flag for hard shutdown
	hardShutdownChan chan struct{} // channel to signal hard shutdown to ingesters
}

// NewController creates a new controller instance with proper initialization
func NewController(config *utils.AppConfig) (*Controller, error) {
	store := utils.NewStore(config.Storage.Path)
	ingesters := getEnabledSources(config, store)
	if len(ingesters) == 0 {
		return nil, errors.New("no sources enabled")
	}

	slog.Debug("controller initialized", "ingester_count", len(ingesters))
	return &Controller{
		config:           config,
		ingesters:        ingesters,
		store:            store,
		shutdownChan:     make(chan struct{}),
		hardShutdownChan: make(chan struct{}),
	}, nil
}

// Start begins the controller operation with proper signal handling and error management
func (c *Controller) Start(ctx context.Context, sigChan <-chan os.Signal) error {
	slog.Info("starting controller")

	// Create context with cancellation for ingesters
	ingesterCtx, ingesterCancel := context.WithCancel(ctx)
	defer ingesterCancel()

	// Create channels for communication
	metricChan := make(chan *types.Metric, 100)
	errorChan := make(chan IngesterError, len(c.ingesters)*2) // Buffer for errors

	// Start metric writer
	metricWriter := utils.NewLineProtocolWriter()
	writerCtx, writerCancel := context.WithCancel(ctx)
	defer writerCancel()

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		metricWriter.Start(writerCtx, metricChan)
	}()

	// Start all ingesters with proper error handling
	var wg sync.WaitGroup
	ingesterDone := make(chan struct{})

	for _, ingester := range c.ingesters {
		wg.Add(1)
		go func(ing types.Ingester) {
			defer wg.Done()
			c.runIngester(ingesterCtx, ing, metricChan, errorChan)
		}(ingester)
		slog.Debug("started ingester", "name", ingester.Name())
	}

	// Monitor ingester completion
	go func() {
		wg.Wait()
		close(ingesterDone)
	}()

	// Main event loop
	return c.eventLoop(ctx, sigChan, metricWriter, metricChan, errorChan, ingesterDone, writerDone)
}

// eventLoop handles signals, errors, and shutdown coordination
func (c *Controller) eventLoop(
	ctx context.Context,
	sigChan <-chan os.Signal,
	metricWriter utils.MetricWriter,
	metricChan chan *types.Metric,
	errorChan chan IngesterError,
	ingesterDone chan struct{},
	writerDone chan struct{},
) error {
	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGTERM:
				slog.Info("received SIGTERM, initiating graceful shutdown")
				return c.handleGracefulShutdown(metricWriter, metricChan, ingesterDone, writerDone)
			case syscall.SIGINT:
				slog.Info("received SIGINT, initiating hard shutdown")
				return c.handleHardShutdown(metricWriter, ingesterDone, writerDone)
			}

		case err := <-errorChan:
			if err := c.handleIngesterError(err); err != nil {
				slog.Error("critical ingester error, initiating shutdown", "error", err)
				return c.handleHardShutdown(metricWriter, ingesterDone, writerDone)
			}

		case <-ingesterDone:
			slog.Debug("all ingesters completed")
			return c.handleGracefulShutdown(metricWriter, metricChan, ingesterDone, writerDone)

		case <-ctx.Done():
			slog.Debug("context cancelled, initiating shutdown")
			return c.handleHardShutdown(metricWriter, ingesterDone, writerDone)
		}
	}
}

// runIngester runs a single ingester with retry logic and error reporting
func (c *Controller) runIngester(
	ctx context.Context,
	ingester types.Ingester,
	metricChan chan<- *types.Metric,
	errorChan chan<- IngesterError,
) {
	retryCfg := types.RetryConfig{
		MaxRetries: c.config.Retry.MaxRetries,
		BaseDelay:  c.config.Retry.BaseDelay,
		MaxDelay:   c.config.Retry.MaxDelay,
	}

	var attempt int

	for {
		// Check for shutdown before starting
		if atomic.LoadInt32(&c.hardShutdown) == 1 {
			slog.Debug("hard shutdown in progress, stopping ingester", "name", ingester.Name())
			return
		}

		// Run ingester with panic protection
		err := c.runIngesterWithPanicProtection(ctx, ingester, metricChan)

		// Handle different exit conditions
		if err == nil {
			slog.Debug("ingester completed successfully", "name", ingester.Name())
			return
		}

		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			slog.Debug("ingester stopped due to context cancellation", "name", ingester.Name())
			return
		}

		// Handle retry logic
		attempt++
		if attempt > retryCfg.MaxRetries {
			slog.Error("ingester retries exhausted", "name", ingester.Name(), "attempts", attempt)
			errorChan <- IngesterError{
				Ingester: ingester.Name(),
				Error:    fmt.Errorf("retries exhausted after %d attempts: %w", attempt, err),
				Attempt:  attempt,
			}
			return
		}

		// Report error and wait for retry
		errorChan <- IngesterError{
			Ingester: ingester.Name(),
			Error:    err,
			Attempt:  attempt,
		}

		// Calculate backoff with jitter
		backoff := c.calculateBackoff(attempt, retryCfg)

		slog.Warn("ingester failed, retrying",
			"name", ingester.Name(),
			"attempt", attempt,
			"error", err,
			"backoff", backoff)

		// Wait for backoff or shutdown
		select {
		case <-time.After(backoff):
			// Continue retry loop
		case <-ctx.Done():
			slog.Debug("context cancelled during backoff", "name", ingester.Name())
			return
		}
	}
}

// runIngesterWithPanicProtection runs an ingester with panic recovery
func (c *Controller) runIngesterWithPanicProtection(
	ctx context.Context,
	ingester types.Ingester,
	metricChan chan<- *types.Metric,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in ingester %s: %v", ingester.Name(), r)
		}
	}()

	// Create shutdown channels for this ingester
	gracefulShutdown := make(chan struct{})
	hardShutdown := make(chan struct{})

	// Monitor shutdown state
	go func() {
		select {
		case <-c.shutdownChan:
			close(gracefulShutdown)
		case <-c.hardShutdownChan:
			close(hardShutdown)
		case <-ctx.Done():
			close(hardShutdown)
		}
	}()

	return ingester.Start(ctx, metricChan, gracefulShutdown, hardShutdown)
}

// handleIngesterError processes errors from ingesters
func (c *Controller) handleIngesterError(err IngesterError) error {
	slog.Warn("ingester error",
		"ingester", err.Ingester,
		"attempt", err.Attempt,
		"error", err.Error)

	// For now, we just log errors. In the future, we could implement
	// more sophisticated error handling (e.g., circuit breakers)
	return nil
}

// handleGracefulShutdown performs graceful shutdown with timeout
func (c *Controller) handleGracefulShutdown(
	metricWriter utils.MetricWriter,
	metricChan chan *types.Metric,
	ingesterDone chan struct{},
	writerDone chan struct{},
) error {
	slog.Info("initiating graceful shutdown")

	// Signal graceful shutdown to all ingesters
	c.shutdownOnce.Do(func() {
		close(c.shutdownChan)
	})

	timeout := c.config.Controller.GracefulShutdownTimeout

	// Wait for ingesters to complete or timeout
	if timeout > 0 {
		select {
		case <-ingesterDone:
			slog.Debug("all ingesters completed gracefully")
		case <-time.After(timeout):
			slog.Warn("graceful shutdown timeout exceeded, forcing hard shutdown")
			atomic.StoreInt32(&c.hardShutdown, 1)
		}
	} else {
		<-ingesterDone
	}

	// Drain remaining metrics
	metricWriter.Drain(metricChan)

	// Stop writer and wait for completion
	metricWriter.Stop()
	<-writerDone

	slog.Info("graceful shutdown completed")
	return nil
}

// handleHardShutdown performs immediate shutdown
func (c *Controller) handleHardShutdown(
	metricWriter utils.MetricWriter,
	ingesterDone chan struct{},
	writerDone chan struct{},
) error {
	slog.Info("initiating hard shutdown")

	// Signal hard shutdown immediately
	atomic.StoreInt32(&c.hardShutdown, 1)

	// Signal hard shutdown to all ingesters
	c.shutdownOnce.Do(func() {
		close(c.hardShutdownChan)
	})

	timeout := c.config.Controller.HardShutdownTimeout

	// Wait for ingesters to complete (with a short timeout for hard shutdown)
	select {
	case <-ingesterDone:
		slog.Debug("all ingesters completed during hard shutdown")
	case <-time.After(timeout):
		slog.Warn("hard shutdown timeout exceeded, forcing writer stop")
	}

	// Stop writer after ingesters are done
	metricWriter.Stop()
	<-writerDone

	slog.Info("hard shutdown completed")
	return nil
}

// calculateBackoff calculates exponential backoff with jitter
func (c *Controller) calculateBackoff(attempt int, cfg types.RetryConfig) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	backoff := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))

	// Cap at max delay
	if backoff > cfg.MaxDelay {
		backoff = cfg.MaxDelay
	}

	// Add jitter (±25% of the backoff time)
	jitter := time.Duration(float64(backoff) * 0.25)
	jitterOffset := time.Duration(float64(jitter) * (2*math.Pi*float64(time.Now().UnixNano()%1000)/1000 - 1))

	return backoff + jitterOffset
}

// Initialize creates a list of ingesters from the registered sources
func getEnabledSources(config *utils.AppConfig, store *utils.Store) []types.Ingester {
	ingesters := []types.Ingester{}

	for sourceName, sourceCreator := range SourceRegistry.GetRegisteredSources() {
		slog.Debug("source", "name", sourceName, "enabled", config.IsSourceEnabled(sourceName))
		if config.IsSourceEnabled(sourceName) {
			ingesters = append(ingesters, sourceCreator(config.GetSourceConfig(sourceName), store))
		}
	}

	return ingesters
}
