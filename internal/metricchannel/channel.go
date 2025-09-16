// Package metricchannel provides utilities for managing metric channels and serialization.
package metricchannel

import (
	"context"
	"fmt"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// Channel manages a buffered channel for metrics and handles serialization.
type Channel struct {
	metricCh chan metrics.Metric
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new metric channel with the specified buffer size.
func New(bufferSize int) *Channel {
	metricCh := make(chan metrics.Metric, bufferSize)
	ctx, cancel := context.WithCancel(context.Background())

	return &Channel{
		metricCh: metricCh,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Get returns the underlying metric channel.
func (c *Channel) Get() chan metrics.Metric {
	return c.metricCh
}

// StartSerializer starts a goroutine that serializes metrics from the channel
// and writes them to stdout in Line Protocol format.
func (c *Channel) StartSerializer() {
	go func() {
		utils.WithPanicRecoveryAndContinue("Metric serializer", "worker", func() {
			for {
				select {
				case m, ok := <-c.metricCh:
					if !ok {
						// Channel closed, exit
						return
					}
					line, err := m.ToLineProtocolSafe()
					if err != nil {
						utils.Errorf("[worker] serialization error: %v", err)
						continue
					}
					fmt.Println(line) // Write directly to stdout
				case <-c.ctx.Done():
					// Context cancelled, exit
					return
				}
			}
		})
	}()
}

// Close closes the metric channel and cancels the context.
func (c *Channel) Close() {
	c.cancel()
	close(c.metricCh)
}

// Context returns the context associated with this channel.
func (c *Channel) Context() context.Context {
	return c.ctx
}
