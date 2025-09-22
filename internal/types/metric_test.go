package types

import (
	"testing"
	"time"
)

func TestNewMetric(t *testing.T) {
	name := "temperature"
	source := "dummy"
	timestamp := time.Now()

	metric := NewMetric(name, source, timestamp)

	if metric.Name != name {
		t.Errorf("Expected name %s, got %s", name, metric.Name)
	}

	if metric.Source != source {
		t.Errorf("Expected source %s, got %s", source, metric.Source)
	}

	if !metric.Timestamp.Equal(timestamp) {
		t.Errorf("Expected timestamp %v, got %v", timestamp, metric.Timestamp)
	}

	if metric.Tags == nil {
		t.Error("Expected Tags map to be initialized")
	}

	if metric.Fields == nil {
		t.Error("Expected Fields map to be initialized")
	}
}

func TestAddTag(t *testing.T) {
	metric := NewMetric("test", "source", time.Now())

	metric.AddTag("key1", "value1")
	metric.AddTag("key2", "value2")

	if metric.Tags["key1"] != "value1" {
		t.Errorf("Expected tag key1=value1, got key1=%s", metric.Tags["key1"])
	}

	if metric.Tags["key2"] != "value2" {
		t.Errorf("Expected tag key2=value2, got key2=%s", metric.Tags["key2"])
	}
}

func TestAddField(t *testing.T) {
	metric := NewMetric("test", "source", time.Now())

	metric.AddField("int_field", 42)
	metric.AddField("float_field", 3.14)
	metric.AddField("string_field", "test")
	metric.AddField("bool_field", true)

	if metric.Fields["int_field"] != 42 {
		t.Errorf("Expected int_field=42, got %v", metric.Fields["int_field"])
	}

	if metric.Fields["float_field"] != 3.14 {
		t.Errorf("Expected float_field=3.14, got %v", metric.Fields["float_field"])
	}

	if metric.Fields["string_field"] != "test" {
		t.Errorf("Expected string_field=test, got %v", metric.Fields["string_field"])
	}

	if metric.Fields["bool_field"] != true {
		t.Errorf("Expected bool_field=true, got %v", metric.Fields["bool_field"])
	}
}
