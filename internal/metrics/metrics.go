// Package metrics provides data structures and utilities for handling metrics
// in InfluxDB Line Protocol format.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janhuddel/metrics-agent/internal/utils"
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

	// Write tags in alphabetical order
	if len(m.Tags) > 0 {
		tagKeys := make([]string, 0, len(m.Tags))
		for k := range m.Tags {
			tagKeys = append(tagKeys, k)
		}
		sort.Strings(tagKeys)

		for _, k := range tagKeys {
			sb.WriteByte(',')
			sb.WriteString(escape(k))
			sb.WriteByte('=')
			sb.WriteString(escape(m.Tags[k]))
		}
	}

	// Write fields in alphabetical order
	sb.WriteByte(' ')
	fieldKeys := make([]string, 0, len(m.Fields))
	for k := range m.Fields {
		fieldKeys = append(fieldKeys, k)
	}
	sort.Strings(fieldKeys)

	for i, k := range fieldKeys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(escape(k))
		sb.WriteByte('=')
		switch val := m.Fields[k].(type) {
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
	}

	// Write timestamp in nanoseconds
	if !m.Timestamp.IsZero() {
		sb.WriteByte(' ')
		sb.WriteString(fmt.Sprintf("%d", m.Timestamp.UnixNano()))
	}

	return sb.String(), nil
}

// ValidateAndConvertFields validates and converts field values to supported types.
// Returns a new map with converted values and logs warnings for unsupported types.
func ValidateAndConvertFields(fields map[string]interface{}) map[string]interface{} {
	converted := make(map[string]interface{})

	for key, value := range fields {
		if convertedValue, err := convertToSupportedType(value); err == nil {
			converted[key] = convertedValue
		} else {
			utils.Warnf("Skipping unsupported field type %T for key '%s': %v", value, key, err)
		}
	}

	return converted
}

// convertToSupportedType converts a value to a type supported by Line Protocol.
func convertToSupportedType(value interface{}) (interface{}, error) {
	switch val := value.(type) {
	case int, int32, int64, float32, float64, bool, string:
		// Already supported types
		return val, nil
	case []interface{}:
		// Convert slice to string representation
		if len(val) == 0 {
			return "", nil
		}
		// Convert first element if it's a simple type, otherwise use string representation
		if len(val) == 1 {
			return convertToSupportedType(val[0])
		}
		// For multiple elements, create a comma-separated string
		strs := make([]string, 0, len(val))
		for _, item := range val {
			if converted, err := convertToSupportedType(item); err == nil {
				strs = append(strs, fmt.Sprintf("%v", converted))
			}
		}
		return strings.Join(strs, ","), nil
	case map[string]interface{}:
		// Convert map to string representation
		if len(val) == 0 {
			return "", nil
		}
		// Create key=value pairs
		pairs := make([]string, 0, len(val))
		for k, v := range val {
			if converted, err := convertToSupportedType(v); err == nil {
				pairs = append(pairs, fmt.Sprintf("%s=%v", k, converted))
			}
		}
		return strings.Join(pairs, ","), nil
	case nil:
		return "", nil
	default:
		// Try to convert to string as last resort
		return fmt.Sprintf("%v", val), nil
	}
}

// Validate checks if the metric can be serialized and returns an error if not.
func (m Metric) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("metric name is required")
	}

	// Validate and convert fields
	convertedFields := ValidateAndConvertFields(m.Fields)
	if len(convertedFields) == 0 {
		return fmt.Errorf("metric has no valid fields after conversion")
	}

	return nil
}

// ToLineProtocolSafe converts a Metric to InfluxDB Line Protocol format with robust error handling.
// It validates and converts fields before serialization.
func (m Metric) ToLineProtocolSafe() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}

	// Create a copy with converted fields
	safeMetric := Metric{
		Name:      m.Name,
		Tags:      m.Tags,
		Fields:    ValidateAndConvertFields(m.Fields),
		Timestamp: m.Timestamp,
	}

	return safeMetric.ToLineProtocol()
}

// escape escapes special characters in strings for Line Protocol format.
func escape(s string) string {
	r := strings.NewReplacer(",", "\\,", " ", "\\ ", "=", "\\=")
	return r.Replace(s)
}
