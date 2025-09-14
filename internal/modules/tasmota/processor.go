package tasmota

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// SensorProcessor handles sensor data processing and metric creation.
type SensorProcessor struct {
	metricsCh chan<- metrics.Metric
	config    *Config
}

// NewSensorProcessor creates a new sensor processor.
func NewSensorProcessor(metricsCh chan<- metrics.Metric, config *Config) *SensorProcessor {
	return &SensorProcessor{
		metricsCh: metricsCh,
		config:    config,
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

	// Handle Power field - it can be either a single float64 or an array of float64 values
	powerValue, exists := data["Power"]
	if !exists {
		log.Printf("Warning: Power field not found in ENERGY sensor data for device %s", device.T)
		return
	}

	// Check if Power is an array or single value
	switch powerData := powerValue.(type) {
	case float64:
		// Create base tags for this sensor
		tags := map[string]string{
			"vendor":   "tasmota",
			"device":   device.T,
			"friendly": sp.config.GetFriendlyName(device, ""),
		}

		fields := map[string]any{
			"power":   powerValue,
			"voltage": data["Voltage"],
			"current": data["Current"],
		}

		// Single power value
		sp.sendPowerMetric(device, tags, fields, timestamp)
	case []any:
		// Array of power values - send one metric for each element
		for i, powerItem := range powerData {
			suffix := "." + fmt.Sprintf("%d", i)
			// Create base tags for this sensor
			tags := map[string]string{
				"vendor":   "tasmota",
				"device":   device.T + suffix,
				"friendly": sp.config.GetFriendlyName(device, suffix),
			}

			if powerFloat, ok := powerItem.(float64); ok {
				fields := map[string]any{
					"power": powerFloat,
				}

				// Add voltage and current fields using helper function
				sp.addFieldAtIndex(fields, data, "Voltage", i)
				sp.addFieldAtIndex(fields, data, "Current", i)

				sp.sendPowerMetric(device, tags, fields, timestamp)
			} else {
				log.Printf("Warning: invalid power value type at index %d for device %s: %T", i, device.T, powerItem)
			}
		}
	default:
		log.Printf("Warning: unexpected Power field type for device %s: %T", device.T, powerData)
	}
}

// addFieldAtIndex adds a field to the fields map, handling both single values and arrays.
func (sp *SensorProcessor) addFieldAtIndex(fields map[string]any, data map[string]any, fieldName string, index int) {
	fieldKey := fieldName
	if value, exists := data[fieldName]; exists {
		if valueArray, isArray := value.([]any); isArray {
			// Field is an array, get the value at the specified index
			if index < len(valueArray) {
				fields[strings.ToLower(fieldKey)] = valueArray[index]
			}
		} else {
			// Field is a single value, use it for all channels
			fields[strings.ToLower(fieldKey)] = value
		}
	}
}

// sendPowerMetric sends a single power metric to the metrics channel.
func (sp *SensorProcessor) sendPowerMetric(device *DeviceInfo, tags map[string]string, fields map[string]any, timestamp time.Time) {
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
