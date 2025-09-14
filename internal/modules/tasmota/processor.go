package tasmota

import (
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
func (sp *SensorProcessor) ProcessSensorData(device *DeviceInfo, sensorData map[string]interface{}) {
	timestamp := time.Now()

	// Process each sensor type
	for sensorType, data := range sensorData {
		if dataMap, ok := data.(map[string]interface{}); ok {
			sp.processSensorType(device, sensorType, dataMap, timestamp)
		}
	}
}

// processSensorType processes a specific sensor type (e.g., "DS18B20", "DHT22", etc.).
func (sp *SensorProcessor) processSensorType(device *DeviceInfo, sensorType string, data map[string]interface{}, timestamp time.Time) {
	// Create base tags for this sensor
	tags := map[string]string{
		"device":      device.T,
		"device_name": device.DN,
		"hostname":    device.HN,
		"ip":          device.IP,
		"mac":         device.MAC,
		"module":      device.MD,
		"sensor_type": sensorType,
	}

	// Process each field in the sensor data
	for field, value := range data {
		if value != nil {
			metric := metrics.Metric{
				Name:      "tasmota_sensor",
				Tags:      tags,
				Fields:    map[string]interface{}{field: value},
				Timestamp: timestamp,
			}
			sp.metricsCh <- metric
		}
	}
}

// SetMetricsChannel sets the metrics channel for testing.
func (sp *SensorProcessor) SetMetricsChannel(ch chan<- metrics.Metric) {
	sp.metricsCh = ch
}
