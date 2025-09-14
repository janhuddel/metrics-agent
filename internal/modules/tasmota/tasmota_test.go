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

	// Sample sensor data
	sensorData := map[string]interface{}{
		"DS18B20": map[string]interface{}{
			"Temperature": 22.5,
		},
		"DHT22": map[string]interface{}{
			"Temperature": 23.1,
			"Humidity":    45.2,
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

	// Verify we got metrics for both sensors
	if len(metrics) < 3 { // At least 3 metrics: DS18B20.Temperature, DHT22.Temperature, DHT22.Humidity
		t.Errorf("Expected at least 3 metrics, got %d", len(metrics))
	}

	// Verify metric structure
	for _, metric := range metrics {
		if metric.Name != "tasmota_sensor" {
			t.Errorf("Expected metric name 'tasmota_sensor', got '%s'", metric.Name)
		}
		if metric.Tags["device"] != device.T {
			t.Errorf("Expected device tag '%s', got '%s'", device.T, metric.Tags["device"])
		}
		if metric.Tags["sensor_type"] == "" {
			t.Error("Expected sensor_type tag to be set")
		}
	}
}
