package metrics_test

import (
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

func TestToLineProtocol_IntAndTags(t *testing.T) {
	m := metrics.Metric{
		Name: "cpu_usage",
		Tags: map[string]string{
			"host":   "my host", // enthält Leerzeichen → muss escaped werden
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

	want := `cpu_usage,host=my\ host,vendor=demo value=42i 1234567890`
	if got != want {
		t.Errorf("wrong line protocol:\n got:  %s\n want: %s", got, want)
	}
}

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
