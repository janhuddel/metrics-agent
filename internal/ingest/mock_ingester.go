package ingest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// MockIngester is a test implementation of the Ingester interface
type MockIngester struct {
	name           string
	shouldError    bool
	errorAfter     time.Duration
	shouldPanic    bool
	panicAfter     time.Duration
	metricsToSend  int
	metricInterval time.Duration
	cleanupTime    time.Duration
	mu             sync.Mutex
	started        bool
	stopped        bool
	metricsSent    int
}

// NewMockIngester creates a new mock ingester for testing
func NewMockIngester(name string) *MockIngester {
	return &MockIngester{
		name:           name,
		metricInterval: 100 * time.Millisecond,
		cleanupTime:    50 * time.Millisecond,
	}
}

// WithError configures the mock to return an error after the specified duration
func (m *MockIngester) WithError(after time.Duration) *MockIngester {
	m.shouldError = true
	m.errorAfter = after
	return m
}

// WithPanic configures the mock to panic after the specified duration
func (m *MockIngester) WithPanic(after time.Duration) *MockIngester {
	m.shouldPanic = true
	m.panicAfter = after
	return m
}

// WithMetrics configures the mock to send a specific number of metrics
func (m *MockIngester) WithMetrics(count int, interval time.Duration) *MockIngester {
	m.metricsToSend = count
	m.metricInterval = interval
	return m
}

// WithCleanupTime configures the cleanup time for graceful shutdown
func (m *MockIngester) WithCleanupTime(duration time.Duration) *MockIngester {
	m.cleanupTime = duration
	return m
}

// Name returns the ingester name
func (m *MockIngester) Name() string {
	return m.name
}

// Start implements the Ingester interface
func (m *MockIngester) Start(ctx context.Context, out chan<- *types.Metric, gracefulShutdown <-chan struct{}, hardShutdown <-chan struct{}) error {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.stopped = true
		m.mu.Unlock()
	}()

	// Set up metric sending
	metricTicker := time.NewTicker(m.metricInterval)
	defer metricTicker.Stop()

	// Set up error and panic channels
	errorChan := make(chan struct{})
	panicChan := make(chan struct{})

	// Start error timer if needed
	if m.shouldError {
		go func() {
			time.Sleep(m.errorAfter)
			close(errorChan)
		}()
	}

	// Start panic timer if needed
	if m.shouldPanic {
		go func() {
			time.Sleep(m.panicAfter)
			close(panicChan)
		}()
	}

	metricsSent := 0

	for {
		select {
		case <-gracefulShutdown:
			// Simulate cleanup time
			time.Sleep(m.cleanupTime)
			return nil
		case <-hardShutdown:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-metricTicker.C:
			if metricsSent < m.metricsToSend || m.metricsToSend == 0 {
				metric := types.NewMetric("test_metric", m.name, time.Now())
				metric.AddField("value", metricsSent)
				select {
				case out <- metric:
					metricsSent++
					m.mu.Lock()
					m.metricsSent = metricsSent
					m.mu.Unlock()
				case <-gracefulShutdown:
					time.Sleep(m.cleanupTime)
					return nil
				case <-hardShutdown:
					return nil
				}
			}
		case <-errorChan:
			if m.shouldError {
				return errors.New("mock ingester error")
			}
		case <-panicChan:
			if m.shouldPanic {
				panic("mock ingester panic")
			}
		}
	}
}

// IsStarted returns whether the ingester has been started
func (m *MockIngester) IsStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// IsStopped returns whether the ingester has been stopped
func (m *MockIngester) IsStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

// MetricsSent returns the number of metrics sent
func (m *MockIngester) MetricsSent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metricsSent
}

// MockIngesterCreator creates a mock ingester for testing
func MockIngesterCreator(config map[string]interface{}, store *utils.Store) types.Ingester {
	name := "mock"
	if nameVal, ok := config["name"].(string); ok {
		name = nameVal
	}
	return NewMockIngester(name)
}
