package metrics

import (
	"fmt"
	"strings"
	"time"
)

// Metric repräsentiert eine einzelne Metrik.
type Metric struct {
	Name      string                 // z.B. "cpu_usage"
	Tags      map[string]string      // z.B. {"host":"foo", "vendor":"demo"}
	Fields    map[string]interface{} // z.B. {"value": 42, "temp": 21.5}
	Timestamp time.Time              // Zeitstempel
}

// ToLineProtocol wandelt eine Metric ins Influx Line Protocol um.
// Beispiel: cpu_usage,vendor=demo,host=foo value=42i,temp=21.5 1634234234000000000
func (m Metric) ToLineProtocol() (string, error) {
	if m.Name == "" {
		return "", fmt.Errorf("metric name is required")
	}
	var sb strings.Builder

	// Messungsname
	sb.WriteString(escape(m.Name))

	// Tags
	for k, v := range m.Tags {
		sb.WriteByte(',')
		sb.WriteString(escape(k))
		sb.WriteByte('=')
		sb.WriteString(escape(v))
	}

	// Felder
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
			// Strings müssen in Quotes
			sb.WriteString(fmt.Sprintf("\"%s\"", strings.ReplaceAll(val, "\"", "\\\"")))
		default:
			return "", fmt.Errorf("unsupported field type %T", val)
		}
		first = false
	}

	// Zeitstempel in Nanosekunden
	if !m.Timestamp.IsZero() {
		sb.WriteByte(' ')
		sb.WriteString(fmt.Sprintf("%d", m.Timestamp.UnixNano()))
	}

	return sb.String(), nil
}

func escape(s string) string {
	r := strings.NewReplacer(",", "\\,", " ", "\\ ", "=", "\\=")
	return r.Replace(s)
}
