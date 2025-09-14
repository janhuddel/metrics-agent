package tasmota_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/modules/tasmota"
)

// TestDefaultConfig tests the default configuration creation.
func TestDefaultConfig(t *testing.T) {
	config := tasmota.DefaultConfig()

	if config.Broker == "" {
		t.Error("Expected broker to be set")
	}
	if config.ClientID == "" {
		t.Error("Expected client ID to be set")
	}
	if config.Timeout == 0 {
		t.Error("Expected timeout to be set")
	}
}

// TestDeviceInfoParsing tests parsing of device discovery messages.
func TestDeviceInfoParsing(t *testing.T) {
	// Sample device discovery payload from the user's request
	payload := `{"ip":"172.19.13.2","dn":"plug-geschirrspueler","fn":["Geschirrspüler",null,null,null,null,null,null,null],"hn":"tasmota-17E7AE-1966","mac":"48551917E7AE","md":"Nous A1T","ty":0,"if":0,"ofln":"Offline","onln":"Online","state":["OFF","ON","TOGGLE","HOLD"],"sw":"13.1.0.1","t":"tasmota_17E7AE","ft":"%prefix%/%topic%/","tp":["cmnd","stat","tele"],"rl":[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"swc":[-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1,-1],"swn":[null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null,null],"btn":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"so":{"4":0,"11":0,"13":0,"17":0,"20":0,"30":0,"68":0,"73":0,"82":0,"114":0,"117":0},"lk":0,"lt_st":0,"bat":0,"dslp":0,"sho":[],"sht":[],"ver":1}`

	var device tasmota.DeviceInfo
	err := json.Unmarshal([]byte(payload), &device)
	if err != nil {
		t.Fatalf("Failed to parse device info: %v", err)
	}

	// Verify key fields
	if device.IP != "172.19.13.2" {
		t.Errorf("Expected IP 172.19.13.2, got %s", device.IP)
	}
	if device.DN != "plug-geschirrspueler" {
		t.Errorf("Expected device name 'plug-geschirrspueler', got %s", device.DN)
	}
	if device.T != "tasmota_17E7AE" {
		t.Errorf("Expected topic 'tasmota_17E7AE', got %s", device.T)
	}
	if device.HN != "tasmota-17E7AE-1966" {
		t.Errorf("Expected hostname 'tasmota-17E7AE-1966', got %s", device.HN)
	}
	if device.MAC != "48551917E7AE" {
		t.Errorf("Expected MAC '48551917E7AE', got %s", device.MAC)
	}
	if device.MD != "Nous A1T" {
		t.Errorf("Expected module 'Nous A1T', got %s", device.MD)
	}
}

// TestTasmotaModuleCreation tests creating a new Tasmota module.
func TestTasmotaModuleCreation(t *testing.T) {
	config := tasmota.Config{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Timeout:  5 * time.Second,
	}

	module := tasmota.NewTasmotaModule(config)
	if module == nil {
		t.Fatal("Expected module to be created")
	}
}

// TestSensorTopicGeneration tests the sensor topic generation logic.
func TestSensorTopicGeneration(t *testing.T) {
	device := &tasmota.DeviceInfo{
		T: "tasmota_17E7AE",
	}

	expectedTopic := "tele/tasmota_17E7AE/SENSOR"
	actualTopic := fmt.Sprintf("tele/%s/SENSOR", device.T)

	if actualTopic != expectedTopic {
		t.Errorf("Expected sensor topic '%s', got '%s'", expectedTopic, actualTopic)
	}
}

// TestDeviceManager tests the device manager functionality.
func TestDeviceManager(t *testing.T) {
	deviceMgr := tasmota.NewDeviceManager()

	device := &tasmota.DeviceInfo{
		T:   "tasmota_17E7AE",
		DN:  "plug-geschirrspueler",
		HN:  "tasmota-17E7AE-1966",
		IP:  "172.19.13.2",
		MAC: "48551917E7AE",
		MD:  "Nous A1T",
	}

	// Test storing device
	deviceMgr.StoreDevice(device)

	// Test retrieving device
	retrievedDevice, exists := deviceMgr.GetDevice(device.T)
	if !exists {
		t.Fatal("Expected device to exist")
	}
	if retrievedDevice.DN != device.DN {
		t.Errorf("Expected device name '%s', got '%s'", device.DN, retrievedDevice.DN)
	}

	// Test retrieving non-existent device
	_, exists = deviceMgr.GetDevice("non-existent")
	if exists {
		t.Error("Expected device to not exist")
	}
}

// TestSensorDataProcessing tests processing of sensor data.
func TestSensorDataProcessing(t *testing.T) {
	device := &tasmota.DeviceInfo{
		T:   "tasmota_17E7AE",
		DN:  "plug-geschirrspueler",
		HN:  "tasmota-17E7AE-1966",
		IP:  "172.19.13.2",
		MAC: "48551917E7AE",
		MD:  "Nous A1T",
	}

	// Sample sensor data - only ENERGY sensors are currently processed
	sensorData := map[string]interface{}{
		"ENERGY": map[string]interface{}{
			"Power": 150.5,
		},
	}

	// Create a test channel
	ch := make(chan metrics.Metric, 10)

	// Create module and set metrics channel
	config := tasmota.Config{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Timeout:  5 * time.Second,
	}
	module := tasmota.NewTasmotaModule(config)
	module.SetMetricsChannel(ch)

	// Process sensor data
	module.ProcessSensorData(device, sensorData)

	// Collect metrics
	var metrics []metrics.Metric
	timeout := time.After(2 * time.Second)

	for {
		select {
		case metric := <-ch:
			metrics = append(metrics, metric)
		case <-timeout:
			goto done
		}
	}
done:

	// Verify we got exactly 1 metric for ENERGY sensor
	if len(metrics) != 1 {
		t.Errorf("Expected 1 metric for ENERGY sensor, got %d", len(metrics))
	}

	// Verify metric structure
	if len(metrics) > 0 {
		metric := metrics[0]
		if metric.Name != "electricity" {
			t.Errorf("Expected metric name 'electricity', got '%s'", metric.Name)
		}
		if metric.Tags["device"] != device.T {
			t.Errorf("Expected device tag '%s', got '%s'", device.T, metric.Tags["device"])
		}
		if metric.Tags["vendor"] != "tasmota" {
			t.Errorf("Expected vendor tag 'tasmota', got '%s'", metric.Tags["vendor"])
		}
		if metric.Fields["power"] != 150.5 {
			t.Errorf("Expected power value 150.5, got %v", metric.Fields["power"])
		}
	}
}

// TestEnergySensorPowerArrayHandling tests processing of ENERGY sensor data with Power as array.
func TestEnergySensorPowerArrayHandling(t *testing.T) {
	device := &tasmota.DeviceInfo{
		T:   "tasmota_6886BC",
		DN:  "test-device",
		HN:  "tasmota-6886BC-1234",
		IP:  "192.168.1.100",
		MAC: "1234566886BC",
		MD:  "Test Module",
	}

	// Test case 1: Power as single float64 value
	t.Run("SinglePowerValue", func(t *testing.T) {
		sensorData := map[string]interface{}{
			"ENERGY": map[string]interface{}{
				"Power": 150.5,
			},
		}

		ch := make(chan metrics.Metric, 10)
		config := tasmota.Config{
			Broker:   "tcp://localhost:1883",
			ClientID: "test-client",
			Timeout:  5 * time.Second,
		}
		module := tasmota.NewTasmotaModule(config)
		module.SetMetricsChannel(ch)

		module.ProcessSensorData(device, sensorData)

		// Collect metrics
		var metrics []metrics.Metric
		timeout := time.After(1 * time.Second)

		for {
			select {
			case metric := <-ch:
				metrics = append(metrics, metric)
			case <-timeout:
				goto done1
			}
		}
	done1:

		// Should have exactly 1 metric
		if len(metrics) != 1 {
			t.Errorf("Expected 1 metric for single power value, got %d", len(metrics))
		}

		if len(metrics) > 0 {
			metric := metrics[0]
			if metric.Name != "electricity" {
				t.Errorf("Expected metric name 'electricity', got '%s'", metric.Name)
			}
			if metric.Fields["power"] != 150.5 {
				t.Errorf("Expected power value 150.5, got %v", metric.Fields["power"])
			}
			if metric.Tags["power_index"] != "" {
				t.Error("Expected no power_index tag for single value")
			}
		}
	})

	// Test case 2: Power as array of float64 values
	t.Run("PowerArray", func(t *testing.T) {
		sensorData := map[string]interface{}{
			"ENERGY": map[string]interface{}{
				"Power": []interface{}{100.0, 200.5, 75.3},
			},
		}

		ch := make(chan metrics.Metric, 10)
		config := tasmota.Config{
			Broker:   "tcp://localhost:1883",
			ClientID: "test-client",
			Timeout:  5 * time.Second,
		}
		module := tasmota.NewTasmotaModule(config)
		module.SetMetricsChannel(ch)

		module.ProcessSensorData(device, sensorData)

		// Collect metrics
		var metrics []metrics.Metric
		timeout := time.After(1 * time.Second)

		for {
			select {
			case metric := <-ch:
				metrics = append(metrics, metric)
			case <-timeout:
				goto done2
			}
		}
	done2:

		// Should have exactly 3 metrics (one for each array element)
		if len(metrics) != 3 {
			t.Errorf("Expected 3 metrics for power array, got %d", len(metrics))
		}

		// Verify each metric
		expectedValues := []float64{100.0, 200.5, 75.3}
		for i, metric := range metrics {
			if metric.Name != "electricity" {
				t.Errorf("Expected metric name 'electricity', got '%s'", metric.Name)
			}
			if metric.Fields["power"] != expectedValues[i] {
				t.Errorf("Expected power value %v at index %d, got %v", expectedValues[i], i, metric.Fields["power"])
			}
			if metric.Tags["power_index"] != fmt.Sprintf("%d", i) {
				t.Errorf("Expected power_index tag '%d', got '%s'", i, metric.Tags["power_index"])
			}
		}
	})

	// Test case 3: Power field missing
	t.Run("MissingPowerField", func(t *testing.T) {
		sensorData := map[string]interface{}{
			"ENERGY": map[string]interface{}{
				"Voltage": 230.0,
			},
		}

		ch := make(chan metrics.Metric, 10)
		config := tasmota.Config{
			Broker:   "tcp://localhost:1883",
			ClientID: "test-client",
			Timeout:  5 * time.Second,
		}
		module := tasmota.NewTasmotaModule(config)
		module.SetMetricsChannel(ch)

		module.ProcessSensorData(device, sensorData)

		// Collect metrics
		var metrics []metrics.Metric
		timeout := time.After(1 * time.Second)

		for {
			select {
			case metric := <-ch:
				metrics = append(metrics, metric)
			case <-timeout:
				goto done3
			}
		}
	done3:

		// Should have no metrics when Power field is missing
		if len(metrics) != 0 {
			t.Errorf("Expected 0 metrics when Power field is missing, got %d", len(metrics))
		}
	})
}
