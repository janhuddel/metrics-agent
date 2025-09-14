package tasmota

import (
	"fmt"
	"log"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// SensorProcessor handles sensor data processing and metric creation.
type SensorProcessor struct {
	metricsCh chan<- metrics.Metric
}

// NewSensorProcessor creates a new sensor processor.
func NewSensorProcessor(metricsCh chan<- metrics.Metric) *SensorProcessor {
	return &SensorProcessor{
		metricsCh: metricsCh,
	}
}

// ProcessSensorData extracts metrics from sensor data.
func (sp *SensorProcessor) ProcessSensorData(device *DeviceInfo, sensorData map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Sensor processor panic recovered for device %s: %v", device.T, r)
		}
	}()

	timestamp := time.Now()

	// Find and process the ENERGY sensor type only
	for sensorType, data := range sensorData {
		if sensorType == "ENERGY" {
			if energyData, ok := data.(map[string]any); ok {
				sp.processEnergySensor(device, sensorType, energyData, timestamp)
			} else {
				log.Printf("Warning: invalid data format for ENERGY sensor type on device %s", device.T)
			}
		}
	}
}

// processEnergySensor processes the ENERGY sensor type.
func (sp *SensorProcessor) processEnergySensor(device *DeviceInfo, sensorType string, data map[string]any, timestamp time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Sensor type processor panic recovered for device %s, sensor %s: %v", device.T, sensorType, r)
		}
	}()

	// Create base tags for this sensor
	tags := map[string]string{
		"vendor": "tasmota",
		"device": device.T,
		"friendly": func() string {
			if len(device.FN) > 0 && device.FN[0] != "" {
				return device.FN[0]
			}
			return device.DN
		}(),
	}

	// Handle Power field - it can be either a single float64 or an array of float64 values
	powerValue, exists := data["Power"]
	if !exists {
		log.Printf("Warning: Power field not found in ENERGY sensor data for device %s", device.T)
		return
	}

	// Check if Power is an array or single value
	switch powerData := powerValue.(type) {
	case float64:
		// Single power value
		sp.sendPowerMetric(device, tags, powerData, timestamp)
	case []any:
		// Array of power values - send one metric for each element
		for i, powerItem := range powerData {
			if powerFloat, ok := powerItem.(float64); ok {
				// Create tags with index for array elements
				arrayTags := make(map[string]string)
				for k, v := range tags {
					arrayTags[k] = v
				}
				arrayTags["power_index"] = fmt.Sprintf("%d", i)

				sp.sendPowerMetric(device, arrayTags, powerFloat, timestamp)
			} else {
				log.Printf("Warning: invalid power value type at index %d for device %s: %T", i, device.T, powerItem)
			}
		}
	default:
		log.Printf("Warning: unexpected Power field type for device %s: %T", device.T, powerData)
	}
}

// sendPowerMetric sends a single power metric to the metrics channel.
func (sp *SensorProcessor) sendPowerMetric(device *DeviceInfo, tags map[string]string, powerValue float64, timestamp time.Time) {
	fields := map[string]any{
		"power": powerValue,
	}

	metric := metrics.Metric{
		Name:      "electricity",
		Tags:      tags,
		Fields:    fields,
		Timestamp: timestamp,
	}

	// Validate metric before sending to prevent serialization errors
	if err := metric.Validate(); err != nil {
		log.Printf("Warning: invalid metric for device %s: %v", device.T, err)
	} else {
		// Send metric with timeout to prevent blocking
		select {
		case sp.metricsCh <- metric:
			// Metric sent successfully
		case <-time.After(1 * time.Second):
			log.Printf("Warning: metric channel full, dropping metric for device %s", device.T)
		}
	}
}

// SetMetricsChannel sets the metrics channel for testing.
func (sp *SensorProcessor) SetMetricsChannel(ch chan<- metrics.Metric) {
	sp.metricsCh = ch
}
