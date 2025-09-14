package metrics_test

import (
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// TestToLineProtocol_IntAndTags tests Line Protocol conversion with integer fields and tags.
func TestToLineProtocol_IntAndTags(t *testing.T) {
	m := metrics.Metric{
		Name: "cpu_usage",
		Tags: map[string]string{
			"host":   "my host", // contains spaces → must be escaped
			"vendor": "demo",
		},
		Fields: map[string]interface{}{
			"value": 42,
		},
		Timestamp: time.Unix(0, 1234567890),
	}

	got, err := m.ToLineProtocol()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the result contains the expected components (order may vary)
	expectedComponents := []string{
		"cpu_usage",
		"host=my\\ host",
		"vendor=demo",
		"value=42i",
		"1234567890",
	}

	for _, component := range expectedComponents {
		if !containsString(got, component) {
			t.Errorf("wrong line protocol: missing component '%s' in result: %s", component, got)
		}
	}
}

// TestToLineProtocol_StringField tests Line Protocol conversion with string fields.
func TestToLineProtocol_StringField(t *testing.T) {
	m := metrics.Metric{
		Name:   "demo_metric",
		Fields: map[string]interface{}{"status": "ok"},
	}

	got, err := m.ToLineProtocol()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := `demo_metric status="ok"`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Helper function to check if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
