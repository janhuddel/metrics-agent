package tasmota

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// handleDiscoveryMessage processes incoming device discovery messages.
func (tm *TasmotaModule) handleDiscoveryMessage(client mqtt.Client, msg mqtt.Message) {
	var device DeviceInfo
	if err := json.Unmarshal(msg.Payload(), &device); err != nil {
		log.Printf("Failed to parse device discovery message: %v", err)
		return
	}

	// Store device info
	tm.deviceMgr.StoreDevice(&device)

	log.Printf("Discovered Tasmota device: %s (%s) at %s", device.DN, device.T, device.IP)

	// Subscribe to sensor data for this device (non-blocking)
	tm.subscribeToSensorData(device.T)
}

// subscribeToSensorData subscribes to sensor data for a specific device.
func (tm *TasmotaModule) subscribeToSensorData(deviceTopic string) {
	sensorTopic := fmt.Sprintf("tele/%s/SENSOR", deviceTopic)
	token := tm.client.Subscribe(sensorTopic, 1, tm.createSensorHandler(deviceTopic))

	// Handle subscription result asynchronously to avoid blocking the message handler
	go func() {
		if token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to sensor topic %s: %v", sensorTopic, token.Error())
		} else {
			log.Printf("Subscribed to sensor topic: %s", sensorTopic)
		}
	}()
}

// createSensorHandler creates a message handler for a specific device's sensor data.
func (tm *TasmotaModule) createSensorHandler(deviceTopic string) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		tm.handleSensorMessage(deviceTopic, msg)
	}
}

// handleSensorMessage processes incoming sensor data messages.
func (tm *TasmotaModule) handleSensorMessage(deviceTopic string, msg mqtt.Message) {
	// Get device info
	device, exists := tm.deviceMgr.GetDevice(deviceTopic)

	if !exists {
		log.Printf("Received sensor data for unknown device: %s", deviceTopic)
		return
	}

	// Parse sensor data (this is a generic JSON object)
	var sensorData map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &sensorData); err != nil {
		log.Printf("Failed to parse sensor data for device %s: %v", deviceTopic, err)
		return
	}

	// Process sensor data and create metrics
	tm.processor.ProcessSensorData(device, sensorData)
}

// DeviceManager handles device storage and retrieval.
type DeviceManager struct {
	devices    map[string]*DeviceInfo
	devicesMux sync.RWMutex
}

// NewDeviceManager creates a new device manager.
func NewDeviceManager() *DeviceManager {
	return &DeviceManager{
		devices: make(map[string]*DeviceInfo),
	}
}

// StoreDevice stores device information.
func (dm *DeviceManager) StoreDevice(device *DeviceInfo) {
	dm.devicesMux.Lock()
	defer dm.devicesMux.Unlock()
	dm.devices[device.T] = device
}

// GetDevice retrieves device information by topic.
func (dm *DeviceManager) GetDevice(topic string) (*DeviceInfo, bool) {
	dm.devicesMux.RLock()
	defer dm.devicesMux.RUnlock()
	device, exists := dm.devices[topic]
	return device, exists
}

// GetAllDevices returns a copy of all stored devices.
func (dm *DeviceManager) GetAllDevices() map[string]*DeviceInfo {
	dm.devicesMux.RLock()
	defer dm.devicesMux.RUnlock()

	devices := make(map[string]*DeviceInfo)
	for k, v := range dm.devices {
		devices[k] = v
	}
	return devices
}
