// Package metrics provides data structures and utilities for handling metrics
// in InfluxDB Line Protocol format.
//
// The package supports:
// - Metric creation and validation
// - Line Protocol serialization
// - Field type conversion and validation
// - Safe metric handling with error recovery
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janhuddel/metrics-agent/internal/utils"
)

// Metric represents a single metric measurement in InfluxDB Line Protocol format.
// It contains all the necessary information to serialize a metric measurement.
type Metric struct {
	// Name is the measurement name (e.g., "cpu_usage", "temperature", "electricity").
	// It must not be empty and will be escaped for Line Protocol format.
	Name string

	// Tags are key-value pairs that identify the metric (e.g., {"host":"server1", "vendor":"tasmota"}).
	// Tags are indexed in InfluxDB and should be used for filtering and grouping.
	// Keys and values will be escaped for Line Protocol format.
	Tags map[string]string

	// Fields are key-value pairs containing the actual metric data (e.g., {"value": 42, "temp": 21.5}).
	// Fields are not indexed and should contain the actual measurement values.
	// Supported types: int, int32, int64, float32, float64, bool, string.
	Fields map[string]interface{}

	// Timestamp is the time when the measurement was taken.
	// If zero, the current time will be used during serialization.
	Timestamp time.Time
}

// ToLineProtocol converts a Metric to InfluxDB Line Protocol format.
// It returns a string in the format: measurement,tag1=value1,tag2=value2 field1=value1i,field2=value2f timestamp
// Example: cpu_usage,vendor=demo,host=foo value=42i,temp=21.5 1634234234000000000
//
// The function performs the following operations:
// - Validates that the metric name is not empty
// - Escapes special characters in names, tags, and fields
// - Sorts tags and fields alphabetically for consistent output
// - Converts field values to appropriate Line Protocol types
// - Uses the metric's timestamp or current time if timestamp is zero
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
// It processes all fields in the input map and returns a new map with converted values.
// Unsupported types are logged as warnings and excluded from the result.
//
// Supported field types:
// - Numeric: int, int32, int64, float32, float64
// - Boolean: bool
// - String: string
// - Collections: []interface{}, map[string]interface{} (converted to strings)
// - Nil values: converted to empty strings
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
// It performs the following validations:
// - Ensures the metric name is not empty
// - Validates and converts field values to supported types
// - Ensures at least one valid field exists after conversion
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
// It validates and converts fields before serialization, ensuring that the output is always valid.
// This is the recommended method for serializing metrics as it handles type conversion gracefully.
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
// It escapes commas, spaces, and equals signs that have special meaning in Line Protocol.
func escape(s string) string {
	r := strings.NewReplacer(",", "\\,", " ", "\\ ", "=", "\\=")
	return r.Replace(s)
}
