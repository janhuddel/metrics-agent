// Package metrics provides data structures and utilities for handling metrics
// in InfluxDB Line Protocol format.
package metrics

import (
	"fmt"
	"strings"
	"time"
)

// Metric represents a single metric measurement.
type Metric struct {
	Name      string                 // Measurement name, e.g., "cpu_usage"
	Tags      map[string]string      // Key-value tags, e.g., {"host":"foo", "vendor":"demo"}
	Fields    map[string]interface{} // Key-value fields, e.g., {"value": 42, "temp": 21.5}
	Timestamp time.Time              // Timestamp for the measurement
}

// ToLineProtocol converts a Metric to InfluxDB Line Protocol format.
// Example: cpu_usage,vendor=demo,host=foo value=42i,temp=21.5 1634234234000000000
func (m Metric) ToLineProtocol() (string, error) {
	if m.Name == "" {
		return "", fmt.Errorf("metric name is required")
	}
	var sb strings.Builder

	// Write measurement name
	sb.WriteString(escape(m.Name))

	// Write tags
	for k, v := range m.Tags {
		sb.WriteByte(',')
		sb.WriteString(escape(k))
		sb.WriteByte('=')
		sb.WriteString(escape(v))
	}

	// Write fields
	first := true
	sb.WriteByte(' ')
	for k, v := range m.Fields {
		if !first {
			sb.WriteByte(',')
		}
		sb.WriteString(escape(k))
		sb.WriteByte('=')
		switch val := v.(type) {
		case int, int32, int64:
			sb.WriteString(fmt.Sprintf("%di", val))
		case float32, float64:
			sb.WriteString(fmt.Sprintf("%f", val))
		case bool:
			if val {
				sb.WriteString("t")
			} else {
				sb.WriteString("f")
			}
		case string:
			// Strings must be quoted
			sb.WriteString(fmt.Sprintf("\"%s\"", strings.ReplaceAll(val, "\"", "\\\"")))
		default:
			return "", fmt.Errorf("unsupported field type %T", val)
		}
		first = false
	}

	// Write timestamp in nanoseconds
	if !m.Timestamp.IsZero() {
		sb.WriteByte(' ')
		sb.WriteString(fmt.Sprintf("%d", m.Timestamp.UnixNano()))
	}

	return sb.String(), nil
}

// escape escapes special characters in strings for Line Protocol format.
func escape(s string) string {
	r := strings.NewReplacer(",", "\\,", " ", "\\ ", "=", "\\=")
	return r.Replace(s)
}
