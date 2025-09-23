package types

import (
	"time"
)

// Metric represents a structured metric that can be converted to line protocol.
type Metric struct {
	// Name is the measurement name (e.g., "temperature", "cpu_usage")
	Name string

	// Tags are key-value pairs for metric identification
	Tags map[string]string

	// Fields are the actual metric values
	Fields map[string]any

	// Timestamp is when the metric was collected
	Timestamp time.Time

	// Source identifies which ingester generated this metric
	Source string
}

// NewMetric creates a new Metric with the given parameters.
func NewMetric(name, source string, timestamp time.Time) *Metric {
	return &Metric{
		Name:      name,
		Source:    source,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		Fields:    make(map[string]any),
	}
}

// AddTag adds a tag to the metric.
func (m *Metric) AddTag(key, value string) {
	m.Tags[key] = value
}

// AddField adds a field to the metric.
func (m *Metric) AddField(key string, value any) {
	m.Fields[key] = value
}
