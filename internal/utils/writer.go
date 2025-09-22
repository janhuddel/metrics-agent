package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/janhuddel/metrics-agent/internal/types"
)

// MetricWriter defines the interface for writing metrics.
type MetricWriter interface {
	// Start begins the writer goroutine that processes metrics from the input channel
	// and writes them directly to stdout in line protocol format.
	Start(ctx context.Context, metricChan <-chan *types.Metric)

	// Drain processes any remaining metrics in the channel and writes them to stdout.
	// This should be called during graceful shutdown.
	Drain(metricChan <-chan *types.Metric)

	// Stop signals the writer to stop processing new metrics.
	// The writer should finish processing any current metric and then exit.
	Stop()
}

// LineProtocolWriter implements MetricWriter for InfluxDB line protocol format.
type LineProtocolWriter struct {
	stopChan chan struct{}
	stopped  bool
}

// NewLineProtocolWriter creates a new LineProtocolWriter.
func NewLineProtocolWriter() *LineProtocolWriter {
	return &LineProtocolWriter{
		stopChan: make(chan struct{}),
	}
}

// Start begins processing metrics and converting them to line protocol format.
func (w *LineProtocolWriter) Start(ctx context.Context, metricChan <-chan *types.Metric) {
	slog.Info("metric writer started")
	defer slog.Info("metric writer stopped")

	for {
		select {
		case <-w.stopChan:
			slog.Info("metric writer: received stop signal")
			return
		case <-ctx.Done():
			slog.Info("metric writer: context canceled")
			return
		case metric := <-metricChan:
			if metric != nil {
				line := w.ConvertToLineProtocol(metric)
				if line != "" {
					fmt.Fprintln(os.Stdout, line)
				}
			}
		}
	}
}

// Drain processes any remaining metrics in the channel during graceful shutdown.
func (w *LineProtocolWriter) Drain(metricChan <-chan *types.Metric) {
	slog.Info("metric writer: draining remaining metrics...")
	for {
		select {
		case metric := <-metricChan:
			if metric != nil {
				line := w.ConvertToLineProtocol(metric)
				if line != "" {
					fmt.Fprintln(os.Stdout, line)
				}
			}
		default:
			// No more metrics to process
			slog.Info("metric writer: finished draining metrics")
			return
		}
	}
}

// Stop signals the writer to stop processing new metrics.
func (w *LineProtocolWriter) Stop() {
	if !w.stopped {
		w.stopped = true
		close(w.stopChan)
	}
}

// ConvertToLineProtocol converts a Metric to InfluxDB line protocol format.
// This method is exposed for testing purposes.
func (w *LineProtocolWriter) ConvertToLineProtocol(metric *types.Metric) string {
	if metric == nil || metric.Name == "" || len(metric.Fields) == 0 {
		return ""
	}

	var parts []string

	// Measurement name
	parts = append(parts, escapeMeasurement(metric.Name))

	// Tags (sorted for consistency)
	if len(metric.Tags) > 0 {
		tagPairs := make([]string, 0, len(metric.Tags))
		for k, v := range metric.Tags {
			if k != "" && v != "" {
				tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", escapeTagKey(k), escapeTagValue(v)))
			}
		}
		sort.Strings(tagPairs)
		if len(tagPairs) > 0 {
			parts = append(parts, strings.Join(tagPairs, ","))
		}
	}

	// Fields (sorted for consistency)
	fieldPairs := make([]string, 0, len(metric.Fields))
	for k, v := range metric.Fields {
		if k != "" && v != nil {
			fieldStr := formatFieldValue(v)
			if fieldStr != "" {
				fieldPairs = append(fieldPairs, fmt.Sprintf("%s=%s", escapeFieldKey(k), fieldStr))
			}
		}
	}
	sort.Strings(fieldPairs)
	if len(fieldPairs) == 0 {
		return "" // No valid fields
	}
	parts = append(parts, strings.Join(fieldPairs, ","))

	// Timestamp
	timestamp := metric.Timestamp.UnixNano()
	parts = append(parts, strconv.FormatInt(timestamp, 10))

	return strings.Join(parts, " ")
}

// escapeMeasurement escapes special characters in measurement names.
func escapeMeasurement(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, " ", "\\ "), ",", "\\,")
}

// escapeTagKey escapes special characters in tag keys.
func escapeTagKey(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, " ", "\\ "), ",", "\\,"), "=", "\\=")
}

// escapeTagValue escapes special characters in tag values.
func escapeTagValue(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(value, " ", "\\ "), ",", "\\,"), "=", "\\=")
}

// escapeFieldKey escapes special characters in field keys.
func escapeFieldKey(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, " ", "\\ "), ",", "\\,"), "=", "\\=")
}

// formatFieldValue formats a field value according to InfluxDB line protocol rules.
func formatFieldValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(v, "\"", "\\\""))
	case int:
		return strconv.FormatInt(int64(v), 10) + "i"
	case int8:
		return strconv.FormatInt(int64(v), 10) + "i"
	case int16:
		return strconv.FormatInt(int64(v), 10) + "i"
	case int32:
		return strconv.FormatInt(int64(v), 10) + "i"
	case int64:
		return strconv.FormatInt(v, 10) + "i"
	case uint:
		return strconv.FormatUint(uint64(v), 10) + "i"
	case uint8:
		return strconv.FormatUint(uint64(v), 10) + "i"
	case uint16:
		return strconv.FormatUint(uint64(v), 10) + "i"
	case uint32:
		return strconv.FormatUint(uint64(v), 10) + "i"
	case uint64:
		return strconv.FormatUint(v, 10) + "i"
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		// For unknown types, try to convert to string
		return fmt.Sprintf("\"%v\"", v)
	}
}
