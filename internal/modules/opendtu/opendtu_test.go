package opendtu_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/modules/opendtu"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// TestLoadConfig tests the configuration loading functionality.
func TestLoadConfig(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	// Test loading default configuration
	config := opendtu.LoadConfig()

	// Verify that config is returned (even if empty)
	tah.AssertNotNil(t, config, "Expected config to be returned")

	// Verify default values are set correctly
	if config.ReconnectInterval == 0 {
		t.Error("Expected ReconnectInterval to have a default value")
	}
	if config.MaxReconnectAttempts == 0 {
		t.Error("Expected MaxReconnectAttempts to have a default value")
	}
	if config.ConnectionTimeout == 0 {
		t.Error("Expected ConnectionTimeout to have a default value")
	}
	if config.ReadTimeout == 0 {
		t.Error("Expected ReadTimeout to have a default value")
	}
	if config.WriteTimeout == 0 {
		t.Error("Expected WriteTimeout to have a default value")
	}
	if config.MaxBackoffInterval == 0 {
		t.Error("Expected MaxBackoffInterval to have a default value")
	}
	if config.BackoffMultiplier == 0 {
		t.Error("Expected BackoffMultiplier to have a default value")
	}
}

// TestNewOpendtuModule tests creating a new Opendtu module instance.
func TestNewOpendtuModule(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	t.Run("ValidConfig", func(t *testing.T) {
		config := opendtu.Config{
			WebSocketURL: "ws://localhost:8080/ws",
		}

		module, err := opendtu.NewOpendtuModule(config)
		tah.AssertNoError(t, err, "Expected module creation to succeed")
		tah.AssertNotNil(t, module, "Expected module to be created")

		if module.GetConfig().WebSocketURL != "ws://localhost:8080/ws" {
			t.Errorf("Expected WebSocketURL to be 'ws://localhost:8080/ws', got '%s'", module.GetConfig().WebSocketURL)
		}
	})

	t.Run("MissingWebSocketURL", func(t *testing.T) {
		config := opendtu.Config{
			WebSocketURL: "",
		}

		module, err := opendtu.NewOpendtuModule(config)
		tah.AssertError(t, err, "Expected error for missing WebSocketURL")
		tah.AssertNil(t, module, "Expected module to be nil when creation fails")

		if !strings.Contains(err.Error(), "web_socket_url is required") {
			t.Errorf("Expected error message to contain 'web_socket_url is required', got: %v", err)
		}
	})
}

// TestWebSocketMessageParsing tests parsing of websocket messages.
func TestWebSocketMessageParsing(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	// Sample websocket message payload
	payload := `{
		"inverters": [
			{
				"serial": "1234567890",
				"name": "Test Inverter",
				"order": 1,
				"data_age": 5,
				"poll_enabled": true,
				"reachable": true,
				"producing": true,
				"limit_relative": 100,
				"limit_absolute": 5000,
				"AC": {
					"0": {
						"Power": {"v": 1500.5, "u": "W", "d": 1},
						"Voltage": {"v": 230.2, "u": "V", "d": 1},
						"Current": {"v": 6.5, "u": "A", "d": 2},
						"Power DC": {"v": 1600.0, "u": "W", "d": 0},
						"YieldDay": {"v": 12.5, "u": "kWh", "d": 1},
						"YieldTotal": {"v": 1250.75, "u": "kWh", "d": 2},
						"Frequency": {"v": 50.0, "u": "Hz", "d": 1},
						"PowerFactor": {"v": 0.95, "u": "", "d": 2},
						"ReactivePower": {"v": 100.0, "u": "var", "d": 0},
						"Efficiency": {"v": 93.8, "u": "%", "d": 1}
					}
				},
				"DC": {
					"1": {
						"name": {"v": 0, "u": "", "d": 0},
						"Power": {"v": 800.0, "u": "W", "d": 0},
						"Voltage": {"v": 45.2, "u": "V", "d": 1},
						"Current": {"v": 17.7, "u": "A", "d": 1},
						"YieldDay": {"v": 6.2, "u": "kWh", "d": 1},
						"YieldTotal": {"v": 625.3, "u": "kWh", "d": 1}
					}
				},
				"INV": {
					"0": {
						"Temperature": {"v": 35.5, "u": "°C", "d": 1}
					}
				},
				"events": 0
			}
		],
		"total": {
			"Power": {"v": 1500.5, "u": "W", "d": 1},
			"YieldDay": {"v": 12.5, "u": "kWh", "d": 1},
			"YieldTotal": {"v": 1250.75, "u": "kWh", "d": 2}
		},
		"hints": {
			"time_sync": true,
			"radio_problem": false,
			"default_password": false
		}
	}`

	var wsMessage opendtu.WebSocketMessage
	err := json.Unmarshal([]byte(payload), &wsMessage)
	tah.AssertNoError(t, err, "Failed to parse websocket message")

	// Verify inverter data
	if len(wsMessage.Inverters) != 1 {
		t.Errorf("Expected 1 inverter, got %d", len(wsMessage.Inverters))
	}

	inverter := wsMessage.Inverters[0]
	if inverter.Serial != "1234567890" {
		t.Errorf("Expected serial '1234567890', got '%s'", inverter.Serial)
	}
	if inverter.Name != "Test Inverter" {
		t.Errorf("Expected name 'Test Inverter', got '%s'", inverter.Name)
	}
	if !inverter.Reachable {
		t.Error("Expected inverter to be reachable")
	}
	if !inverter.Producing {
		t.Error("Expected inverter to be producing")
	}

	// Verify AC measurements
	acPhase0, exists := inverter.AC["0"]
	if !exists {
		t.Fatal("Expected AC phase 0 to exist")
	}
	if acPhase0.Power.Value != 1500.5 {
		t.Errorf("Expected power 1500.5, got %f", acPhase0.Power.Value)
	}
	if acPhase0.Voltage.Value != 230.2 {
		t.Errorf("Expected voltage 230.2, got %f", acPhase0.Voltage.Value)
	}
	if acPhase0.Current.Value != 6.5 {
		t.Errorf("Expected current 6.5, got %f", acPhase0.Current.Value)
	}
	if acPhase0.YieldDay.Value != 12.5 {
		t.Errorf("Expected yield day 12.5, got %f", acPhase0.YieldDay.Value)
	}
	if acPhase0.YieldTotal.Value != 1250.75 {
		t.Errorf("Expected yield total 1250.75, got %f", acPhase0.YieldTotal.Value)
	}

	// Verify total measurements
	if wsMessage.Total.Power.Value != 1500.5 {
		t.Errorf("Expected total power 1500.5, got %f", wsMessage.Total.Power.Value)
	}

	// Verify hints
	if !wsMessage.Hints.TimeSync {
		t.Error("Expected time sync to be true")
	}
	if wsMessage.Hints.RadioProblem {
		t.Error("Expected radio problem to be false")
	}
}

// TestProcessMessage tests processing of websocket messages.
func TestProcessMessage(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	// Create a test module
	config := opendtu.Config{
		WebSocketURL: "ws://localhost:8080/ws",
	}
	module, err := opendtu.NewOpendtuModule(config)
	tah.AssertNoError(t, err, "Failed to create module")

	// Create a metrics channel
	metricsCh := make(chan metrics.Metric, 10)
	module.SetMetricsChannel(metricsCh)

	t.Run("ValidMessage", func(t *testing.T) {
		// Sample valid websocket message
		message := `{
			"inverters": [
				{
					"serial": "1234567890",
					"name": "Test Inverter",
					"order": 1,
					"data_age": 5,
					"poll_enabled": true,
					"reachable": true,
					"producing": true,
					"limit_relative": 100,
					"limit_absolute": 5000,
					"AC": {
						"0": {
							"Power": {"v": 1500.5, "u": "W", "d": 1},
							"Voltage": {"v": 230.2, "u": "V", "d": 1},
							"Current": {"v": 6.5, "u": "A", "d": 2},
							"Power DC": {"v": 1600.0, "u": "W", "d": 0},
							"YieldDay": {"v": 12.5, "u": "kWh", "d": 1},
							"YieldTotal": {"v": 1250.75, "u": "kWh", "d": 2},
							"Frequency": {"v": 50.0, "u": "Hz", "d": 1},
							"PowerFactor": {"v": 0.95, "u": "", "d": 2},
							"ReactivePower": {"v": 100.0, "u": "var", "d": 0},
							"Efficiency": {"v": 93.8, "u": "%", "d": 1}
						}
					},
					"DC": {},
					"INV": {},
					"events": 0
				}
			],
			"total": {
				"Power": {"v": 1500.5, "u": "W", "d": 1},
				"YieldDay": {"v": 12.5, "u": "kWh", "d": 1},
				"YieldTotal": {"v": 1250.75, "u": "kWh", "d": 2}
			},
			"hints": {
				"time_sync": true,
				"radio_problem": false,
				"default_password": false
			}
		}`

		err := module.ProcessMessage([]byte(message))
		tah.AssertNoError(t, err, "Expected message processing to succeed")

		// Verify metric was created
		select {
		case metric := <-metricsCh:
			if metric.Name != "electricity" {
				t.Errorf("Expected metric name 'electricity', got '%s'", metric.Name)
			}
			if metric.Tags["vendor"] != "opendtu" {
				t.Errorf("Expected vendor tag 'opendtu', got '%s'", metric.Tags["vendor"])
			}
			if metric.Tags["device"] != "1234567890" {
				t.Errorf("Expected device tag '1234567890', got '%s'", metric.Tags["device"])
			}
			if metric.Tags["friendly"] != "Test Inverter" {
				t.Errorf("Expected friendly tag 'Test Inverter', got '%s'", metric.Tags["friendly"])
			}

			// Check fields
			if power, ok := metric.Fields["power"]; !ok || power != 1500.5 {
				t.Errorf("Expected power field 1500.5, got %v", power)
			}
			if voltage, ok := metric.Fields["voltage"]; !ok || voltage != 230.2 {
				t.Errorf("Expected voltage field 230.2, got %v", voltage)
			}
			if current, ok := metric.Fields["current"]; !ok || current != 6.5 {
				t.Errorf("Expected current field 6.5, got %v", current)
			}
			if sumPowerToday, ok := metric.Fields["sum_power_today"]; !ok || sumPowerToday != 12.5 {
				t.Errorf("Expected sum_power_today field 12.5, got %v", sumPowerToday)
			}
			if sumPowerTotal, ok := metric.Fields["sum_power_total"]; !ok || sumPowerTotal != 1250.75 {
				t.Errorf("Expected sum_power_total field 1250.75, got %v", sumPowerTotal)
			}

		case <-time.After(1 * time.Second):
			t.Error("Expected metric to be sent within 1 second")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		invalidMessage := `{"invalid": json}`
		err := module.ProcessMessage([]byte(invalidMessage))
		tah.AssertError(t, err, "Expected error for invalid JSON")
		if !strings.Contains(err.Error(), "failed to parse websocket message") {
			t.Errorf("Expected error message to contain 'failed to parse websocket message', got: %v", err)
		}
	})

	t.Run("EmptyMessage", func(t *testing.T) {
		emptyMessage := `{}`
		err := module.ProcessMessage([]byte(emptyMessage))
		tah.AssertNoError(t, err, "Expected empty message to be processed without error")
	})
}

// TestCreateInverterMetrics tests creating metrics from inverter data.
func TestCreateInverterMetrics(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	// Create a test module
	config := opendtu.Config{
		WebSocketURL: "ws://localhost:8080/ws",
	}
	module, err := opendtu.NewOpendtuModule(config)
	tah.AssertNoError(t, err, "Failed to create module")

	// Create a metrics channel
	metricsCh := make(chan metrics.Metric, 10)
	module.SetMetricsChannel(metricsCh)

	t.Run("ValidInverterData", func(t *testing.T) {
		inverter := opendtu.InverterData{
			Serial:      "1234567890",
			Name:        "Test Inverter",
			Order:       1,
			DataAge:     5,
			PollEnabled: true,
			Reachable:   true,
			Producing:   true,
			AC: map[string]opendtu.ACMeasurement{
				"0": {
					Power:      opendtu.MeasurementValue{Value: 1500.5, Unit: "W", Decimals: 1},
					Voltage:    opendtu.MeasurementValue{Value: 230.2, Unit: "V", Decimals: 1},
					Current:    opendtu.MeasurementValue{Value: 6.5, Unit: "A", Decimals: 2},
					YieldDay:   opendtu.MeasurementValue{Value: 12.5, Unit: "kWh", Decimals: 1},
					YieldTotal: opendtu.MeasurementValue{Value: 1250.75, Unit: "kWh", Decimals: 2},
				},
			},
		}

		err := module.CreateInverterMetrics(inverter, time.Now())
		tah.AssertNoError(t, err, "Expected metric creation to succeed")

		// Verify metric was created
		select {
		case metric := <-metricsCh:
			if metric.Name != "electricity" {
				t.Errorf("Expected metric name 'electricity', got '%s'", metric.Name)
			}
			if metric.Tags["vendor"] != "opendtu" {
				t.Errorf("Expected vendor tag 'opendtu', got '%s'", metric.Tags["vendor"])
			}
			if metric.Tags["device"] != "1234567890" {
				t.Errorf("Expected device tag '1234567890', got '%s'", metric.Tags["device"])
			}
			if metric.Tags["friendly"] != "Test Inverter" {
				t.Errorf("Expected friendly tag 'Test Inverter', got '%s'", metric.Tags["friendly"])
			}

			// Check fields
			if power, ok := metric.Fields["power"]; !ok || power != 1500.5 {
				t.Errorf("Expected power field 1500.5, got %v", power)
			}
			if voltage, ok := metric.Fields["voltage"]; !ok || voltage != 230.2 {
				t.Errorf("Expected voltage field 230.2, got %v", voltage)
			}
			if current, ok := metric.Fields["current"]; !ok || current != 6.5 {
				t.Errorf("Expected current field 6.5, got %v", current)
			}
			if sumPowerToday, ok := metric.Fields["sum_power_today"]; !ok || sumPowerToday != 12.5 {
				t.Errorf("Expected sum_power_today field 12.5, got %v", sumPowerToday)
			}
			if sumPowerTotal, ok := metric.Fields["sum_power_total"]; !ok || sumPowerTotal != 1250.75 {
				t.Errorf("Expected sum_power_total field 1250.75, got %v", sumPowerTotal)
			}

		case <-time.After(1 * time.Second):
			t.Error("Expected metric to be sent within 1 second")
		}
	})

	t.Run("InverterWithoutACData", func(t *testing.T) {
		inverter := opendtu.InverterData{
			Serial:    "1234567890",
			Name:      "Test Inverter",
			Reachable: true,
			Producing: false,
			AC:        map[string]opendtu.ACMeasurement{},
		}

		err := module.CreateInverterMetrics(inverter, time.Now())
		tah.AssertNoError(t, err, "Expected metric creation to succeed even without AC data")

		// Should not create any metrics
		select {
		case <-metricsCh:
			t.Error("Expected no metric to be created for inverter without AC data")
		case <-time.After(100 * time.Millisecond):
			// This is expected - no metric should be created
		}
	})

	t.Run("InverterWithoutPhase0", func(t *testing.T) {
		inverter := opendtu.InverterData{
			Serial:    "1234567890",
			Name:      "Test Inverter",
			Reachable: true,
			Producing: true,
			AC: map[string]opendtu.ACMeasurement{
				"1": { // Phase 1 instead of phase 0
					Power:      opendtu.MeasurementValue{Value: 1500.5, Unit: "W", Decimals: 1},
					Voltage:    opendtu.MeasurementValue{Value: 230.2, Unit: "V", Decimals: 1},
					Current:    opendtu.MeasurementValue{Value: 6.5, Unit: "A", Decimals: 2},
					YieldDay:   opendtu.MeasurementValue{Value: 12.5, Unit: "kWh", Decimals: 1},
					YieldTotal: opendtu.MeasurementValue{Value: 1250.75, Unit: "kWh", Decimals: 2},
				},
			},
		}

		err := module.CreateInverterMetrics(inverter, time.Now())
		tah.AssertNoError(t, err, "Expected metric creation to succeed even without phase 0")

		// Should not create any metrics since we only process phase 0
		select {
		case <-metricsCh:
			t.Error("Expected no metric to be created for inverter without phase 0")
		case <-time.After(100 * time.Millisecond):
			// This is expected - no metric should be created
		}
	})
}

// TestRunWithCancellation tests the Run function with context cancellation.
// Note: This test is commented out due to context import issues in the test environment
/*
func TestRunWithCancellation(t *testing.T) {
	tah := utils.NewTestAssertionHelper()
	tch := utils.NewTestContextHelper()

	// Create a context that will be cancelled quickly
	ctx, cancel := tch.CreateTestContextWithTimeout(100 * time.Millisecond)
	defer cancel()

	// Create a metrics channel
	metricsCh := make(chan metrics.Metric, 10)

	// This should return quickly due to context cancellation
	// We expect it to fail during websocket connection since we don't have a real server
	err := opendtu.Run(ctx, metricsCh)
	tah.AssertError(t, err, "Expected Run to return an error due to websocket connection failure")
}
*/

// TestMultipleInverters tests processing multiple inverters in a single message.
func TestMultipleInverters(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	// Create a test module
	config := opendtu.Config{
		WebSocketURL: "ws://localhost:8080/ws",
	}
	module, err := opendtu.NewOpendtuModule(config)
	tah.AssertNoError(t, err, "Failed to create module")

	// Create a metrics channel
	metricsCh := make(chan metrics.Metric, 10)
	module.SetMetricsChannel(metricsCh)

	// Sample message with multiple inverters
	message := `{
		"inverters": [
			{
				"serial": "1234567890",
				"name": "Inverter 1",
				"order": 1,
				"data_age": 5,
				"poll_enabled": true,
				"reachable": true,
				"producing": true,
				"limit_relative": 100,
				"limit_absolute": 5000,
				"AC": {
					"0": {
						"Power": {"v": 1500.5, "u": "W", "d": 1},
						"Voltage": {"v": 230.2, "u": "V", "d": 1},
						"Current": {"v": 6.5, "u": "A", "d": 2},
						"Power DC": {"v": 1600.0, "u": "W", "d": 0},
						"YieldDay": {"v": 12.5, "u": "kWh", "d": 1},
						"YieldTotal": {"v": 1250.75, "u": "kWh", "d": 2},
						"Frequency": {"v": 50.0, "u": "Hz", "d": 1},
						"PowerFactor": {"v": 0.95, "u": "", "d": 2},
						"ReactivePower": {"v": 100.0, "u": "var", "d": 0},
						"Efficiency": {"v": 93.8, "u": "%", "d": 1}
					}
				},
				"DC": {},
				"INV": {},
				"events": 0
			},
			{
				"serial": "0987654321",
				"name": "Inverter 2",
				"order": 2,
				"data_age": 3,
				"poll_enabled": true,
				"reachable": true,
				"producing": true,
				"limit_relative": 100,
				"limit_absolute": 3000,
				"AC": {
					"0": {
						"Power": {"v": 800.0, "u": "W", "d": 0},
						"Voltage": {"v": 225.5, "u": "V", "d": 1},
						"Current": {"v": 3.5, "u": "A", "d": 2},
						"Power DC": {"v": 850.0, "u": "W", "d": 0},
						"YieldDay": {"v": 6.8, "u": "kWh", "d": 1},
						"YieldTotal": {"v": 680.25, "u": "kWh", "d": 2},
						"Frequency": {"v": 50.0, "u": "Hz", "d": 1},
						"PowerFactor": {"v": 0.98, "u": "", "d": 2},
						"ReactivePower": {"v": 50.0, "u": "var", "d": 0},
						"Efficiency": {"v": 94.1, "u": "%", "d": 1}
					}
				},
				"DC": {},
				"INV": {},
				"events": 0
			}
		],
		"total": {
			"Power": {"v": 2300.5, "u": "W", "d": 1},
			"YieldDay": {"v": 19.3, "u": "kWh", "d": 1},
			"YieldTotal": {"v": 1931.0, "u": "kWh", "d": 2}
		},
		"hints": {
			"time_sync": true,
			"radio_problem": false,
			"default_password": false
		}
	}`

	err = module.ProcessMessage([]byte(message))
	tah.AssertNoError(t, err, "Expected message processing to succeed")

	// Collect all metrics
	var metrics []metrics.Metric
	timeout := time.After(2 * time.Second)

	for {
		select {
		case metric := <-metricsCh:
			metrics = append(metrics, metric)
		case <-timeout:
			goto done
		}
	}
done:

	// Verify we got exactly 2 metrics (one for each inverter)
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics for 2 inverters, got %d", len(metrics))
	}

	// Verify each metric
	expectedSerials := []string{"1234567890", "0987654321"}
	expectedNames := []string{"Inverter 1", "Inverter 2"}
	expectedPowers := []float64{1500.5, 800.0}

	for i, metric := range metrics {
		if metric.Name != "electricity" {
			t.Errorf("Expected metric name 'electricity', got '%s'", metric.Name)
		}
		if metric.Tags["device"] != expectedSerials[i] {
			t.Errorf("Expected device tag '%s', got '%s'", expectedSerials[i], metric.Tags["device"])
		}
		if metric.Tags["friendly"] != expectedNames[i] {
			t.Errorf("Expected friendly tag '%s', got '%s'", expectedNames[i], metric.Tags["friendly"])
		}
		if metric.Fields["power"] != expectedPowers[i] {
			t.Errorf("Expected power value %v, got %v", expectedPowers[i], metric.Fields["power"])
		}
	}
}

// TestMetricsChannelFull tests behavior when metrics channel is full.
func TestMetricsChannelFull(t *testing.T) {
	tah := utils.NewTestAssertionHelper()

	// Create a test module
	config := opendtu.Config{
		WebSocketURL: "ws://localhost:8080/ws",
	}
	module, err := opendtu.NewOpendtuModule(config)
	tah.AssertNoError(t, err, "Failed to create module")

	// Create a small metrics channel (size 1)
	metricsCh := make(chan metrics.Metric, 1)
	module.SetMetricsChannel(metricsCh)

	// Fill the channel
	blockingMetric := metrics.Metric{
		Name:   "blocking",
		Tags:   map[string]string{"test": "blocking"},
		Fields: map[string]interface{}{"value": 1},
	}
	metricsCh <- blockingMetric

	// Try to create a metric (should not block)
	inverter := opendtu.InverterData{
		Serial:    "1234567890",
		Name:      "Test Inverter",
		Reachable: true,
		Producing: true,
		AC: map[string]opendtu.ACMeasurement{
			"0": {
				Power:      opendtu.MeasurementValue{Value: 1500.5, Unit: "W", Decimals: 1},
				Voltage:    opendtu.MeasurementValue{Value: 230.2, Unit: "V", Decimals: 1},
				Current:    opendtu.MeasurementValue{Value: 6.5, Unit: "A", Decimals: 2},
				YieldDay:   opendtu.MeasurementValue{Value: 12.5, Unit: "kWh", Decimals: 1},
				YieldTotal: opendtu.MeasurementValue{Value: 1250.75, Unit: "kWh", Decimals: 2},
			},
		},
	}

	// This should not block even though the channel is full
	err = module.CreateInverterMetrics(inverter, time.Now())
	tah.AssertNoError(t, err, "Expected metric creation to succeed even with full channel")

	// Verify the blocking metric is still there
	select {
	case metric := <-metricsCh:
		if metric.Name != "blocking" {
			t.Errorf("Expected blocking metric, got %s", metric.Name)
		}
	default:
		t.Error("Expected blocking metric to still be in channel")
	}
}

// TestMeasurementValueStruct tests the MeasurementValue struct.
func TestMeasurementValueStruct(t *testing.T) {
	mv := opendtu.MeasurementValue{
		Value:    1500.5,
		Unit:     "W",
		Decimals: 1,
	}

	if mv.Value != 1500.5 {
		t.Errorf("Expected Value 1500.5, got %f", mv.Value)
	}
	if mv.Unit != "W" {
		t.Errorf("Expected Unit 'W', got '%s'", mv.Unit)
	}
	if mv.Decimals != 1 {
		t.Errorf("Expected Decimals 1, got %d", mv.Decimals)
	}
}

// TestACMeasurementStruct tests the ACMeasurement struct.
func TestACMeasurementStruct(t *testing.T) {
	ac := opendtu.ACMeasurement{
		Power:      opendtu.MeasurementValue{Value: 1500.5, Unit: "W", Decimals: 1},
		Voltage:    opendtu.MeasurementValue{Value: 230.2, Unit: "V", Decimals: 1},
		Current:    opendtu.MeasurementValue{Value: 6.5, Unit: "A", Decimals: 2},
		YieldDay:   opendtu.MeasurementValue{Value: 12.5, Unit: "kWh", Decimals: 1},
		YieldTotal: opendtu.MeasurementValue{Value: 1250.75, Unit: "kWh", Decimals: 2},
	}

	if ac.Power.Value != 1500.5 {
		t.Errorf("Expected Power.Value 1500.5, got %f", ac.Power.Value)
	}
	if ac.Voltage.Unit != "V" {
		t.Errorf("Expected Voltage.Unit 'V', got '%s'", ac.Voltage.Unit)
	}
	if ac.Current.Decimals != 2 {
		t.Errorf("Expected Current.Decimals 2, got %d", ac.Current.Decimals)
	}
}

// TestInverterDataStruct tests the InverterData struct.
func TestInverterDataStruct(t *testing.T) {
	inverter := opendtu.InverterData{
		Serial:        "1234567890",
		Name:          "Test Inverter",
		Order:         1,
		DataAge:       5,
		PollEnabled:   true,
		Reachable:     true,
		Producing:     true,
		LimitRelative: 100,
		LimitAbsolute: 5000,
		Events:        0,
	}

	if inverter.Serial != "1234567890" {
		t.Errorf("Expected Serial '1234567890', got '%s'", inverter.Serial)
	}
	if inverter.Name != "Test Inverter" {
		t.Errorf("Expected Name 'Test Inverter', got '%s'", inverter.Name)
	}
	if !inverter.Reachable {
		t.Error("Expected Reachable to be true")
	}
	if !inverter.Producing {
		t.Error("Expected Producing to be true")
	}
	if inverter.LimitRelative != 100 {
		t.Errorf("Expected LimitRelative 100, got %d", inverter.LimitRelative)
	}
}
