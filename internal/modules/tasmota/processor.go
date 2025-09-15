package tasmota

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// Constants for field processing and conversions
const (
	// Field names
	fieldPower   = "Power"
	fieldVoltage = "Voltage"
	fieldCurrent = "Current"
	fieldToday   = "Today"
	fieldTotal   = "Total"
	fieldEIn     = "E_in"
	fieldEOut    = "E_out"

	// Sensor types
	sensorTypeEnergy = "ENERGY"
	sensorTypeMT175  = "MT175"

	// Metric names
	metricNameElectricity = "electricity"

	// Conversion factors
	currentToMilliAmps = 1000.0 // Convert A to mAh
	whToKwh            = 1000.0 // Convert Wh to KWh

	// HTTP settings
	httpTimeout       = 5 * time.Second
	metricSendTimeout = 1 * time.Second
)

// EnergyTotalResponse represents the response from the EnergyTotal HTTP endpoint
type EnergyTotalResponse struct {
	EnergyTotal struct {
		Today     []float64 `json:"Today"`
		Total     []float64 `json:"Total"`
		Yesterday []float64 `json:"Yesterday"`
	} `json:"EnergyTotal"`
}

// FieldProcessor handles field extraction and conversion operations
type FieldProcessor struct{}

// NewFieldProcessor creates a new field processor
func NewFieldProcessor() *FieldProcessor {
	return &FieldProcessor{}
}

// convertCurrentToMilliAmps converts current from Amperes to milliAmperes
func (fp *FieldProcessor) convertCurrentToMilliAmps(current any) any {
	if currentFloat, ok := current.(float64); ok {
		return currentFloat * currentToMilliAmps
	}
	return current
}

// convertWhToKwh converts energy from Wh to KWh
func (fp *FieldProcessor) convertWhToKwh(energy any) any {
	if energyFloat, ok := energy.(float64); ok {
		return energyFloat * whToKwh
	}
	return energy
}

// addFieldAtIndex adds a field to the fields map, handling both single values and arrays
func (fp *FieldProcessor) addFieldAtIndex(fields map[string]any, data map[string]any, fieldName string, index int) {
	if value, exists := data[fieldName]; exists {
		if valueArray, isArray := value.([]any); isArray {
			// Field is an array, get the value at the specified index
			if index < len(valueArray) {
				fieldValue := valueArray[index]
				// Convert current from A to mAh if needed
				if fieldName == fieldCurrent {
					fieldValue = fp.convertCurrentToMilliAmps(fieldValue)
				}
				fields[strings.ToLower(fieldName)] = fieldValue
			}
		} else {
			// Field is a single value, use it for all channels
			fieldValue := value
			// Convert current from A to mAh if needed
			if fieldName == fieldCurrent {
				fieldValue = fp.convertCurrentToMilliAmps(fieldValue)
			}
			fields[strings.ToLower(fieldName)] = fieldValue
		}
	}
}

// addEnergyFields adds energy fields (Today/Total) with proper conversion
func (fp *FieldProcessor) addEnergyFields(fields map[string]any, data map[string]any, fieldName string) {
	if value, exists := data[fieldName]; exists {
		convertedValue := fp.convertWhToKwh(value)
		fieldKey := fmt.Sprintf("sum_power_%s", strings.ToLower(fieldName))
		fields[fieldKey] = convertedValue
	}
}

// createBaseTags creates base tags for a device with optional suffix
func (sp *SensorProcessor) createBaseTags(device *DeviceInfo, suffix string) map[string]string {
	return map[string]string{
		"vendor":   "tasmota",
		"device":   device.T + suffix,
		"friendly": sp.config.GetFriendlyName(device, suffix),
	}
}

// SensorProcessor handles sensor data processing and metric creation.
type SensorProcessor struct {
	metricsCh      chan<- metrics.Metric
	config         *Config
	fieldProcessor *FieldProcessor
	httpClient     *http.Client
}

// NewSensorProcessor creates a new sensor processor.
func NewSensorProcessor(metricsCh chan<- metrics.Metric, config *Config) *SensorProcessor {
	return &SensorProcessor{
		metricsCh:      metricsCh,
		config:         config,
		fieldProcessor: NewFieldProcessor(),
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// ProcessSensorData extracts metrics from sensor data.
func (sp *SensorProcessor) ProcessSensorData(device *DeviceInfo, sensorData map[string]any) {
	utils.WithPanicRecoveryAndContinue("Sensor processor", device.T, func() {
		timestamp := time.Now()

		// Find and process the sensor types
		for sensorType, data := range sensorData {
			switch sensorType {
			case sensorTypeEnergy:
				if energyData, ok := data.(map[string]any); ok {
					sp.processEnergySensor(device, sensorType, energyData, timestamp)
				} else {
					log.Printf("Warning: invalid data format for %s sensor type on device %s", sensorTypeEnergy, device.T)
				}
			case sensorTypeMT175:
				if mt175Data, ok := data.(map[string]any); ok {
					sp.processMT175Sensor(device, sensorType, mt175Data, timestamp)
				} else {
					log.Printf("Warning: invalid data format for %s sensor type on device %s", sensorTypeMT175, device.T)
				}
			}
		}
	})
}

// processEnergySensor processes the ENERGY sensor type.
func (sp *SensorProcessor) processEnergySensor(device *DeviceInfo, sensorType string, data map[string]any, timestamp time.Time) {
	utils.WithPanicRecoveryAndContinue("Sensor type processor", device.T, func() {
		// Handle Power field - it can be either a single float64 or an array of float64 values
		powerValue, exists := data[fieldPower]
		if !exists {
			log.Printf("Warning: %s field not found in %s sensor data for device %s", fieldPower, sensorTypeEnergy, device.T)
			return
		}

		// Check if Power is an array or single value
		switch powerData := powerValue.(type) {
		case float64:
			sp.processSingleChannelEnergy(device, data, powerData, timestamp)
		case []any:
			sp.processMultiChannelEnergy(device, data, powerData, timestamp)
		default:
			log.Printf("Warning: unexpected %s field type for device %s: %T", fieldPower, device.T, powerData)
		}
	})
}

// processMT175Sensor processes the MT175 sensor type.
func (sp *SensorProcessor) processMT175Sensor(device *DeviceInfo, sensorType string, mt175Data map[string]any, timestamp time.Time) {
	utils.WithPanicRecoveryAndContinue("Sensor type processor", device.T, func() {
		tags := sp.createBaseTags(device, "")

		powerValue, exists := mt175Data[fieldPower]
		if !exists {
			log.Printf("Warning: %s field not found in %s sensor data for device %s", fieldPower, sensorTypeMT175, device.T)
			return
		}

		fields := map[string]any{
			"power": powerValue,
		}

		if e_in, exists := mt175Data[fieldEIn]; exists {
			fields["sum_power_total"] = sp.fieldProcessor.convertWhToKwh(e_in)
		}

		if e_out, exists := mt175Data[fieldEOut]; exists {
			fields["sum_power_total_out"] = sp.fieldProcessor.convertWhToKwh(e_out)
		}

		sp.sendPowerMetric(device, tags, fields, timestamp)
	})
}

// processSingleChannelEnergy processes energy data for single-channel devices.
func (sp *SensorProcessor) processSingleChannelEnergy(device *DeviceInfo, data map[string]any, powerValue float64, timestamp time.Time) {
	// Create base tags for this sensor
	tags := sp.createBaseTags(device, "")

	fields := map[string]any{
		"power": powerValue,
	}

	// Add voltage field
	if voltage, exists := data[fieldVoltage]; exists {
		fields["voltage"] = voltage
	}

	// Add current field with conversion
	if current, exists := data[fieldCurrent]; exists {
		fields["current"] = sp.fieldProcessor.convertCurrentToMilliAmps(current)
	}

	// Add energy fields with conversion
	sp.fieldProcessor.addEnergyFields(fields, data, fieldToday)
	sp.fieldProcessor.addEnergyFields(fields, data, fieldTotal)

	sp.sendPowerMetric(device, tags, fields, timestamp)
}

// processMultiChannelEnergy processes energy data for multi-channel devices.
func (sp *SensorProcessor) processMultiChannelEnergy(device *DeviceInfo, data map[string]any, powerData []any, timestamp time.Time) {
	// Fetch energy totals via HTTP for multi-channel devices
	energyTotals, err := sp.fetchEnergyTotals(device)
	if err != nil {
		log.Printf("Warning: failed to fetch energy totals for device %s: %v", device.T, err)
	}

	// Send one metric for each element
	for i, powerItem := range powerData {
		if powerFloat, ok := powerItem.(float64); ok {
			sp.processMultiChannelElement(device, data, powerFloat, i, energyTotals, timestamp)
		} else {
			log.Printf("Warning: invalid power value type at index %d for device %s: %T", i, device.T, powerItem)
		}
	}
}

// processMultiChannelElement processes a single channel element for multi-channel devices.
func (sp *SensorProcessor) processMultiChannelElement(device *DeviceInfo, data map[string]any, powerFloat float64, index int, energyTotals *EnergyTotalResponse, timestamp time.Time) {
	suffix := "." + fmt.Sprintf("%d", index)

	// Create base tags for this sensor
	tags := sp.createBaseTags(device, suffix)

	fields := map[string]any{
		"power": powerFloat,
	}

	// Add voltage and current fields using field processor
	sp.fieldProcessor.addFieldAtIndex(fields, data, fieldVoltage, index)
	sp.fieldProcessor.addFieldAtIndex(fields, data, fieldCurrent, index)

	// Add energy totals from HTTP response if available, converting Wh to KWh
	if energyTotals != nil {
		if index < len(energyTotals.EnergyTotal.Today) {
			fields["sum_power_today"] = energyTotals.EnergyTotal.Today[index] * whToKwh
		}
		if index < len(energyTotals.EnergyTotal.Total) {
			fields["sum_power_total"] = energyTotals.EnergyTotal.Total[index] * whToKwh
		}
	}

	sp.sendPowerMetric(device, tags, fields, timestamp)
}

// sendPowerMetric sends a single power metric to the metrics channel.
func (sp *SensorProcessor) sendPowerMetric(device *DeviceInfo, tags map[string]string, fields map[string]any, timestamp time.Time) {
	metric := metrics.Metric{
		Name:      metricNameElectricity,
		Tags:      tags,
		Fields:    fields,
		Timestamp: timestamp,
	}

	// Validate metric before sending to prevent serialization errors
	if err := metric.Validate(); err != nil {
		log.Printf("Warning: invalid metric for device %s: %v", device.T, err)
		return
	}

	// Send metric with timeout to prevent blocking
	select {
	case sp.metricsCh <- metric:
		// Metric sent successfully
	case <-time.After(metricSendTimeout):
		log.Printf("Warning: metric channel full, dropping metric for device %s", device.T)
	}
}

// fetchEnergyTotals fetches energy totals from a multi-channel device via HTTP
func (sp *SensorProcessor) fetchEnergyTotals(device *DeviceInfo) (*EnergyTotalResponse, error) {
	url := fmt.Sprintf("http://%s/cm?cmnd=EnergyTotal", device.IP)

	resp, err := sp.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch energy totals: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var energyTotal EnergyTotalResponse
	if err := json.Unmarshal(body, &energyTotal); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &energyTotal, nil
}

// SetMetricsChannel sets the metrics channel for testing.
func (sp *SensorProcessor) SetMetricsChannel(ch chan<- metrics.Metric) {
	sp.metricsCh = ch
}
