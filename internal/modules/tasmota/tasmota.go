package tasmota

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// TasmotaModule handles MQTT connections and device discovery.
type TasmotaModule struct {
	config    Config
	client    mqtt.Client
	deviceMgr *DeviceManager
	processor *SensorProcessor
	metricsCh chan<- metrics.Metric
}

// NewTasmotaModule creates a new Tasmota module instance.
func NewTasmotaModule(config Config) *TasmotaModule {
	return &TasmotaModule{
		config:    config,
		deviceMgr: NewDeviceManager(),
	}
}

// Run starts the Tasmota module and begins collecting metrics.
func Run(ctx context.Context, ch chan<- metrics.Metric) error {
	config := DefaultConfig()
	module := NewTasmotaModule(config)
	module.metricsCh = ch
	module.processor = NewSensorProcessor(ch)

	return module.run(ctx)
}

// run executes the main module loop.
func (tm *TasmotaModule) run(ctx context.Context) error {
	// Connect to MQTT broker
	if err := tm.connect(); err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}
	defer tm.disconnect()

	// Subscribe to discovery topic
	discoveryTopic := "tasmota/discovery/+/config"
	if token := tm.client.Subscribe(discoveryTopic, 1, tm.handleDiscoveryMessage); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to discovery topic: %w", token.Error())
	}
	log.Printf("Subscribed to discovery topic: %s", discoveryTopic)

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// connect establishes connection to the MQTT broker.
func (tm *TasmotaModule) connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(tm.config.Broker)
	opts.SetClientID(tm.config.ClientID)
	opts.SetUsername(tm.config.Username)
	opts.SetPassword(tm.config.Password)
	opts.SetConnectTimeout(tm.config.Timeout)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	// Set connection lost handler
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})

	// Set reconnect handler
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Printf("Connected to MQTT broker: %s", tm.config.Broker)
	})

	tm.client = mqtt.NewClient(opts)
	if token := tm.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// disconnect closes the MQTT connection.
func (tm *TasmotaModule) disconnect() {
	if tm.client != nil && tm.client.IsConnected() {
		tm.client.Disconnect(250)
	}
}

// Public methods for testing

// ProcessSensorData is a public method for testing sensor data processing.
func (tm *TasmotaModule) ProcessSensorData(device *DeviceInfo, sensorData map[string]interface{}) {
	tm.processor.ProcessSensorData(device, sensorData)
}

// SetMetricsChannel sets the metrics channel for testing.
func (tm *TasmotaModule) SetMetricsChannel(ch chan<- metrics.Metric) {
	tm.metricsCh = ch
	if tm.processor == nil {
		tm.processor = NewSensorProcessor(ch)
	} else {
		tm.processor.SetMetricsChannel(ch)
	}
}
