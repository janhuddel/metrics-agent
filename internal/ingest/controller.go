package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

type Controller struct {
	config    *utils.AppConfig
	store     *utils.Store
	ingesters []types.Ingester
}

func NewController(config *utils.AppConfig) (*Controller, error) {
	store := utils.NewStore("./.data")
	ingesters := getEnabledSources(config, store)
	if len(ingesters) == 0 {
		return nil, errors.New("no sources enabled")
	}

	return &Controller{
		config:    config,
		ingesters: ingesters,
		store:     store,
	}, nil
}

func (c *Controller) Start(sigChan chan os.Signal) {
	// Shared channels
	metricChan := make(chan *types.Metric, 100) // Channel for structured metrics
	errs := make(chan error, 10)
	gracefulShutdown := make(chan struct{}) // Graceful shutdown signal
	hardShutdown := make(chan struct{})     // Hard shutdown signal

	// Get retry configuration from config
	retryCfg := types.RetryConfig{
		MaxRetries: c.config.Retry.MaxRetries,
		BaseDelay:  c.config.Retry.BaseDelay,
		MaxDelay:   c.config.Retry.MaxDelay,
	}

	// Start the metric writer
	metricWriter := utils.NewLineProtocolWriter()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		metricWriter.Start(context.Background(), metricChan)
	}()

	var wg sync.WaitGroup
	completionChan := make(chan struct{})

	// Start all sources with shutdown channels
	for _, ingester := range c.ingesters {
		wg.Add(1)
		go func(source types.Ingester) {
			defer wg.Done()
			c.startIngester(context.Background(), ingester, metricChan, gracefulShutdown, hardShutdown, retryCfg)
		}(ingester)
		slog.Info("started ingester", "name", ingester.Name())
	}

	// Goroutine to signal completion when all sources finish
	go func() {
		wg.Wait()
		close(completionChan)
	}()

	// Main loop: signal handling and error processing
	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGTERM:
				slog.Info("received SIGTERM, initiating graceful shutdown")
				close(gracefulShutdown)

				// Wait for graceful shutdown with timeout
				gracefulTimeout := 30 * time.Second
				select {
				case <-time.After(gracefulTimeout):
					slog.Warn("graceful shutdown timeout exceeded, forcing hard shutdown")
					close(hardShutdown)
					// Stop the writer immediately for hard shutdown
					metricWriter.Stop()
					// Wait for writer to complete
					<-writerDone
					return
				case <-completionChan:
					slog.Info("all sources completed gracefully")
					// Drain remaining metrics before stopping the writer
					metricWriter.Drain(metricChan)
					metricWriter.Stop()
					// Wait for writer to complete
					<-writerDone
					return
				}
			case syscall.SIGINT:
				slog.Info("received SIGINT, initiating hard shutdown")
				close(hardShutdown)
				// Stop the writer immediately for hard shutdown
				metricWriter.Stop()
				// Wait for writer to complete
				<-writerDone
				return
			}
		case err := <-errs:
			slog.Error("source error", "err", err)
		}
	}
}

func (c *Controller) startIngester(
	ctx context.Context,
	ingester types.Ingester,
	out chan<- *types.Metric,
	gracefulShutdown <-chan struct{},
	hardShutdown <-chan struct{},
	cfg types.RetryConfig) {

	var attempt int

	for {
		// Panic protection
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in %s: %v", ingester.Name(), r)
				}
			}()
			return ingester.Start(ctx, out, gracefulShutdown, hardShutdown)
		}()

		// Normal exit due to context cancellation
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			slog.Info("stopping source (context canceled)", "name", ingester.Name())
			return
		}

		// Successful graceful shutdown (source returned nil)
		if err == nil {
			slog.Info("source completed gracefully", "name", ingester.Name())
			return
		}

		// Error case → Retry
		attempt++
		if attempt > cfg.MaxRetries {
			slog.Error("source retries exhausted, crashing",
				"name", ingester.Name(), "err", err)
			os.Exit(1) // Crash → Control back to Telegraf
		}

		// Calculate backoff
		backoff := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if backoff > cfg.MaxDelay {
			backoff = cfg.MaxDelay
		}

		slog.Warn("source failed, will retry",
			"name", ingester.Name(), "attempt", attempt, "err", err, "backoff", backoff)

		select {
		case <-time.After(backoff):
			// Continue retry loop
		case <-gracefulShutdown:
			slog.Info("stopping source during backoff (graceful shutdown)", "name", ingester.Name())
			return
		case <-hardShutdown:
			slog.Info("stopping source during backoff (hard shutdown)", "name", ingester.Name())
			return
		}
	}
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
