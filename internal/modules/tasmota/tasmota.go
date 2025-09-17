package tasmota

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// TasmotaModule handles MQTT connections and device discovery.
type TasmotaModule struct {
	config           Config
	client           mqtt.Client
	deviceMgr        *DeviceManager
	processor        *SensorProcessor
	metricsCh        chan<- metrics.Metric
	SubscribedTopics map[string]bool // Public for testing
	SubscriptionMux  sync.RWMutex    // Public for testing
}

// NewTasmotaModule creates a new Tasmota module instance.
func NewTasmotaModule(config Config) *TasmotaModule {
	utils.Debugf("Creating new Tasmota module instance")
	utils.Debugf("Loaded Tasmota config: Broker=%s, KeepAlive=%v, PingTimeout=%v, Timeout=%v",
		config.Broker, config.KeepAlive, config.PingTimeout, config.Timeout)

	utils.Debugf("Tasmota module created successfully")
	return &TasmotaModule{
		config:           config,
		deviceMgr:        NewDeviceManager(),
		SubscribedTopics: make(map[string]bool),
	}
}

// Run starts the Tasmota module and begins collecting metrics.
func Run(ctx context.Context, ch chan<- metrics.Metric) error {
	config := LoadConfig()
	module := NewTasmotaModule(config)
	module.metricsCh = ch
	module.processor = NewSensorProcessor(ch, &config)

	return module.run(ctx)
}

// run executes the main module loop.
func (tm *TasmotaModule) run(ctx context.Context) error {
	return utils.WithPanicRecoveryAndReturnError("Tasmota module", "main", func() error {
		// Connect to MQTT broker with context cancellation support
		if err := tm.connectWithContext(ctx); err != nil {
			return fmt.Errorf("failed to connect to MQTT broker: %w", err)
		}
		defer tm.disconnect()

		// Subscribe to discovery topic with context cancellation support
		discoveryTopic := "tasmota/discovery/+/config"
		if err := tm.subscribeWithContext(ctx, discoveryTopic, 1, tm.handleDiscoveryMessage); err != nil {
			return fmt.Errorf("failed to subscribe to discovery topic: %w", err)
		}
		utils.Debugf("Subscribed to discovery topic: %s", discoveryTopic)

		// Wait for context cancellation
		<-ctx.Done()
		return ctx.Err()
	})
}

// connectWithContext establishes connection to the MQTT broker with context cancellation support.
func (tm *TasmotaModule) connectWithContext(ctx context.Context) error {
	return utils.WithPanicRecoveryAndReturnError("MQTT connect", "broker", func() error {
		// Set default client ID if not provided
		clientID := tm.config.ClientID
		if clientID == "" {
			hostname, _ := os.Hostname()
			clientID = hostname + "-tasmota"
		}

		opts := mqtt.NewClientOptions()
		opts.AddBroker(tm.config.Broker)
		opts.SetClientID(clientID)
		opts.SetUsername(tm.config.Username)
		opts.SetPassword(tm.config.Password)
		opts.SetConnectTimeout(tm.config.Timeout)
		opts.SetAutoReconnect(true)
		opts.SetResumeSubs(true)    // Resume subscriptions after reconnection
		opts.SetCleanSession(false) // Use persistent session to maintain subscriptions
		opts.SetKeepAlive(tm.config.KeepAlive)
		opts.SetPingTimeout(tm.config.PingTimeout)
		opts.SetMaxReconnectInterval(5 * time.Minute)  // Limit max reconnect interval
		opts.SetConnectRetryInterval(10 * time.Second) // Retry connection every 10 seconds
		opts.SetOrderMatters(false)                    // Allow out-of-order message processing
		opts.SetProtocolVersion(4)                     // Use MQTT 3.1.1 protocol

		// Set connection lost handler with panic recovery
		opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
			utils.WithPanicRecoveryAndContinue("MQTT connection lost handler", "broker", func() {
				utils.Errorf("MQTT connection lost: %v", err)
				// Note: AutoReconnect is enabled, so the client will automatically attempt to reconnect
				// Subscriptions will be restored due to SetResumeSubs(true) and SetCleanSession(false)
			})
		})

		// Set reconnect handler with panic recovery
		opts.SetOnConnectHandler(func(client mqtt.Client) {
			utils.WithPanicRecoveryAndContinue("MQTT reconnect handler", "broker", func() {
				utils.Infof("Connected to MQTT broker: %s", tm.config.Broker)
				// Note: Subscriptions will be automatically restored due to SetResumeSubs(true)
			})
		})

		tm.client = mqtt.NewClient(opts)

		// Use context-aware connection with timeout
		connChan := make(chan error, 1)
		go func() {
			token := tm.client.Connect()
			connChan <- token.Error()
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-connChan:
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// connect establishes connection to the MQTT broker.
func (tm *TasmotaModule) connect() error {
	return utils.WithPanicRecoveryAndReturnError("MQTT connect", "broker", func() error {
		// Set default client ID if not provided
		clientID := tm.config.ClientID
		if clientID == "" {
			hostname, _ := os.Hostname()
			clientID = hostname + "-tasmota"
		}

		opts := mqtt.NewClientOptions()
		opts.AddBroker(tm.config.Broker)
		opts.SetClientID(clientID)
		opts.SetUsername(tm.config.Username)
		opts.SetPassword(tm.config.Password)
		opts.SetConnectTimeout(tm.config.Timeout)
		opts.SetAutoReconnect(true)
		opts.SetResumeSubs(true)    // Resume subscriptions after reconnection
		opts.SetCleanSession(false) // Use persistent session to maintain subscriptions
		opts.SetKeepAlive(tm.config.KeepAlive)
		opts.SetPingTimeout(tm.config.PingTimeout)
		opts.SetMaxReconnectInterval(5 * time.Minute)  // Limit max reconnect interval
		opts.SetConnectRetryInterval(10 * time.Second) // Retry connection every 10 seconds
		opts.SetOrderMatters(false)                    // Allow out-of-order message processing
		opts.SetProtocolVersion(4)                     // Use MQTT 3.1.1 protocol

		// Set connection lost handler with panic recovery
		opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
			utils.WithPanicRecoveryAndContinue("MQTT connection lost handler", "broker", func() {
				utils.Errorf("MQTT connection lost: %v", err)
				// Note: AutoReconnect is enabled, so the client will automatically attempt to reconnect
				// Subscriptions will be restored due to SetResumeSubs(true) and SetCleanSession(false)
			})
		})

		// Set reconnect handler with panic recovery
		opts.SetOnConnectHandler(func(client mqtt.Client) {
			utils.WithPanicRecoveryAndContinue("MQTT reconnect handler", "broker", func() {
				utils.Infof("Connected to MQTT broker: %s", tm.config.Broker)
				// Note: Subscriptions will be automatically restored due to SetResumeSubs(true)
			})
		})

		tm.client = mqtt.NewClient(opts)
		if token := tm.client.Connect(); token.Wait() && token.Error() != nil {
			return token.Error()
		}

		return nil
	})
}

// subscribeWithContext subscribes to an MQTT topic with context cancellation support.
func (tm *TasmotaModule) subscribeWithContext(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	return utils.WithPanicRecoveryAndReturnError("MQTT subscribe", "broker", func() error {
		// Use context-aware subscription with timeout
		subChan := make(chan error, 1)
		go func() {
			token := tm.client.Subscribe(topic, qos, callback)
			subChan <- token.Error()
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-subChan:
			return err
		}
	})
}

// disconnect closes the MQTT connection.
func (tm *TasmotaModule) disconnect() {
	utils.WithPanicRecoveryAndContinue("MQTT disconnect", "broker", func() {
		if tm.client != nil && tm.client.IsConnected() {
			tm.client.Disconnect(250)
		}
	})
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
		tm.processor = NewSensorProcessor(ch, &tm.config)
	} else {
		tm.processor.SetMetricsChannel(ch)
	}
}
