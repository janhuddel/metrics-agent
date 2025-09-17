package opendtu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/janhuddel/metrics-agent/internal/config"
	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/utils"
	"github.com/janhuddel/metrics-agent/internal/websocket"
)

// Config represents the configuration for the Opendtu module
type Config struct {
	config.BaseConfig
	WebSocketURL         string        `json:"web_socket_url"`
	ReconnectInterval    time.Duration `json:"reconnect_interval,omitempty"`
	MaxReconnectAttempts int           `json:"max_reconnect_attempts,omitempty"`
	ConnectionTimeout    time.Duration `json:"connection_timeout,omitempty"`
	ReadTimeout          time.Duration `json:"read_timeout,omitempty"`
	WriteTimeout         time.Duration `json:"write_timeout,omitempty"`
	MaxBackoffInterval   time.Duration `json:"max_backoff_interval,omitempty"`
	BackoffMultiplier    float64       `json:"backoff_multiplier,omitempty"`
}

// MeasurementValue represents a single measurement with value, unit, and decimal places
type MeasurementValue struct {
	Value    float64 `json:"v"`
	Unit     string  `json:"u"`
	Decimals int     `json:"d"`
}

// ACMeasurement represents AC electrical measurements for a single phase
type ACMeasurement struct {
	Power         MeasurementValue `json:"Power"`
	Voltage       MeasurementValue `json:"Voltage"`
	Current       MeasurementValue `json:"Current"`
	PowerDC       MeasurementValue `json:"Power DC"`
	YieldDay      MeasurementValue `json:"YieldDay"`
	YieldTotal    MeasurementValue `json:"YieldTotal"`
	Frequency     MeasurementValue `json:"Frequency"`
	PowerFactor   MeasurementValue `json:"PowerFactor"`
	ReactivePower MeasurementValue `json:"ReactivePower"`
	Efficiency    MeasurementValue `json:"Efficiency"`
}

// DCMeasurement represents DC electrical measurements for a single channel
type DCMeasurement struct {
	Name       MeasurementValue `json:"name"`
	Power      MeasurementValue `json:"Power"`
	Voltage    MeasurementValue `json:"Voltage"`
	Current    MeasurementValue `json:"Current"`
	YieldDay   MeasurementValue `json:"YieldDay"`
	YieldTotal MeasurementValue `json:"YieldTotal"`
}

// INVMeasurement represents inverter-specific measurements
type INVMeasurement struct {
	Temperature MeasurementValue `json:"Temperature"`
}

// TotalMeasurement represents total/summary measurements across all inverters
type TotalMeasurement struct {
	Power      MeasurementValue `json:"Power"`
	YieldDay   MeasurementValue `json:"YieldDay"`
	YieldTotal MeasurementValue `json:"YieldTotal"`
}

// Hints represents system status hints
type Hints struct {
	TimeSync        bool `json:"time_sync"`
	RadioProblem    bool `json:"radio_problem"`
	DefaultPassword bool `json:"default_password"`
}

// WebSocketMessage represents a message received from the OpenDTU websocket
type WebSocketMessage struct {
	Inverters []InverterData   `json:"inverters"`
	Total     TotalMeasurement `json:"total"`
	Hints     Hints            `json:"hints"`
}

// InverterData represents data for a single inverter
type InverterData struct {
	Serial        string                    `json:"serial"`
	Name          string                    `json:"name"`
	Order         int                       `json:"order"`
	DataAge       int                       `json:"data_age"`
	PollEnabled   bool                      `json:"poll_enabled"`
	Reachable     bool                      `json:"reachable"`
	Producing     bool                      `json:"producing"`
	LimitRelative int                       `json:"limit_relative"`
	LimitAbsolute int                       `json:"limit_absolute"`
	AC            map[string]ACMeasurement  `json:"AC"`
	DC            map[string]DCMeasurement  `json:"DC"`
	INV           map[string]INVMeasurement `json:"INV"`
	Events        int                       `json:"events"`
}

// OpendtuModule handles Opendtu API authentication and data collection
type OpendtuModule struct {
	config    Config
	wsClient  *websocket.Client
	metricsCh chan<- metrics.Metric
}

func Run(ctx context.Context, ch chan<- metrics.Metric) error {
	config := LoadConfig()
	module, err := NewOpendtuModule(config)
	if err != nil {
		return fmt.Errorf("failed to create Opendtu module: %w", err)
	}
	module.metricsCh = ch

	return module.run(ctx)
}

// NewOpendtuModule creates a new Opendtu module instance
func NewOpendtuModule(config Config) (*OpendtuModule, error) {
	utils.Debugf("Creating new Opendtu module instance")

	websocketURL := config.WebSocketURL
	if websocketURL == "" {
		return nil, fmt.Errorf("web_socket_url is required but not configured")
	}

	utils.Debugf("Opendtu module created successfully")
	return &OpendtuModule{
		config: config,
	}, nil
}

// LoadConfig loads the Opendtu module configuration
func LoadConfig() Config {
	defaultConfig := Config{
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 10,
		ConnectionTimeout:    10 * time.Second,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         10 * time.Second,
		MaxBackoffInterval:   60 * time.Second,
		BackoffMultiplier:    2.0,
	}

	loader := config.NewLoader("opendtu")
	if config.GlobalConfigPath != "" {
		loader.SetConfigPath(config.GlobalConfigPath)
	}

	loadedConfig, err := loader.LoadConfig(&defaultConfig)
	if err != nil {
		utils.Warnf("Failed to load Opendtu configuration: %v", err)
		return defaultConfig
	}

	return *loadedConfig.(*Config)
}

// run executes the main module loop with robust reconnection handling
func (om *OpendtuModule) run(ctx context.Context) error {
	// Create websocket client configuration
	wsConfig := websocket.Config{
		URL:                  om.config.WebSocketURL,
		ReconnectInterval:    om.config.ReconnectInterval,
		MaxReconnectAttempts: om.config.MaxReconnectAttempts,
		ConnectionTimeout:    om.config.ConnectionTimeout,
		ReadTimeout:          om.config.ReadTimeout,
		WriteTimeout:         om.config.WriteTimeout,
		MaxBackoffInterval:   om.config.MaxBackoffInterval,
		BackoffMultiplier:    om.config.BackoffMultiplier,
	}

	// Create websocket client with message handler
	wsClient, err := websocket.NewClient(wsConfig, om.processMessage)
	if err != nil {
		return fmt.Errorf("failed to create websocket client: %w", err)
	}

	// Run the websocket client
	return wsClient.Run(ctx)
}

// processMessage parses a websocket message and creates metrics from the payload
func (om *OpendtuModule) processMessage(message []byte) error {
	// Parse the JSON message
	var wsMessage WebSocketMessage
	if err := json.Unmarshal(message, &wsMessage); err != nil {
		return fmt.Errorf("failed to parse websocket message: %w", err)
	}

	// Create metrics from the message payload
	return om.createMetricsFromPayload(wsMessage)
}

// createMetricsFromPayload creates metrics from the websocket message payload
func (om *OpendtuModule) createMetricsFromPayload(wsMessage WebSocketMessage) error {
	timestamp := time.Now()

	// Process inverter-specific metrics
	for _, inverter := range wsMessage.Inverters {
		if err := om.createInverterMetrics(inverter, timestamp); err != nil {
			utils.Errorf("Failed to create metrics for inverter %s: %v", inverter.Serial, err)
		}
	}

	return nil
}

// SetMetricsChannel sets the metrics channel for the module
func (om *OpendtuModule) SetMetricsChannel(ch chan<- metrics.Metric) {
	om.metricsCh = ch
}

// GetConfig returns the module configuration (for testing)
func (om *OpendtuModule) GetConfig() Config {
	return om.config
}

// ProcessMessage processes a websocket message (public method for testing)
func (om *OpendtuModule) ProcessMessage(message []byte) error {
	return om.processMessage(message)
}

// CreateInverterMetrics creates metrics for a specific inverter (public method for testing)
func (om *OpendtuModule) CreateInverterMetrics(inverter InverterData, timestamp time.Time) error {
	return om.createInverterMetrics(inverter, timestamp)
}

// createInverterMetrics creates metrics for a specific inverter
func (om *OpendtuModule) createInverterMetrics(inverter InverterData, timestamp time.Time) error {
	// Create base tags for inverter metrics
	tags := map[string]string{
		"vendor":   "opendtu",
		"friendly": inverter.Name,
		"device":   inverter.Serial,
	}

	// Create fields from inverter data
	fields := make(map[string]interface{})

	// we are only interested in phase 0
	phase0, exists := inverter.AC["0"]
	if !exists {
		// No phase 0 data available, return without creating metrics
		return nil
	}

	fields["power"] = phase0.Power.Value
	fields["voltage"] = phase0.Voltage.Value
	fields["current"] = phase0.Current.Value
	fields["sum_power_today"] = phase0.YieldDay.Value
	fields["sum_power_total"] = phase0.YieldTotal.Value

	// Only create metric if we have valid fields
	if len(fields) == 0 {
		return nil
	}

	// Create and send the metric
	metric := metrics.Metric{
		Name:      "electricity",
		Tags:      tags,
		Fields:    fields,
		Timestamp: timestamp,
	}

	// Validate the metric before sending
	if err := metric.Validate(); err != nil {
		return fmt.Errorf("invalid inverter metric: %w", err)
	}

	// Send metric to channel
	select {
	case om.metricsCh <- metric:
		// Metric sent successfully
	default:
		utils.Warnf("Metrics channel is full, dropping inverter metric")
	}

	return nil
}
