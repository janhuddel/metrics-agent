package netatmo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/janhuddel/metrics-agent/internal/config"
	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// Config represents the configuration for the Netatmo module
type Config struct {
	config.BaseConfig
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Timeout      string `json:"timeout"`
	Interval     string `json:"interval"`
}

// NetatmoModule handles Netatmo API authentication and data collection
type NetatmoModule struct {
	config     Config
	httpClient *http.Client
	baseURL    string
	oauth2     *utils.OAuth2Client
	metricsCh  chan<- metrics.Metric
}

// StationData represents the response from the Netatmo API
type StationData struct {
	Body struct {
		Devices []Device `json:"devices"`
	} `json:"body"`
	Status string `json:"status"`
}

// Device represents a Netatmo device (station or module)
type Device struct {
	ID            string    `json:"_id"`
	StationName   string    `json:"station_name"`
	ModuleName    string    `json:"module_name"`
	Type          string    `json:"type"`
	DashboardData Dashboard `json:"dashboard_data"`
	Modules       []Module  `json:"modules"`
	Place         Place     `json:"place"`
}

// Module represents a Netatmo module (outdoor, rain, wind, etc.)
type Module struct {
	ID            string    `json:"_id"`
	ModuleName    string    `json:"module_name"`
	Type          string    `json:"type"`
	DashboardData Dashboard `json:"dashboard_data"`
	Place         Place     `json:"place"`
}

// Place represents location information
type Place struct {
	Altitude float64   `json:"altitude"`
	City     string    `json:"city"`
	Country  string    `json:"country"`
	Timezone string    `json:"timezone"`
	Location []float64 `json:"location"`
}

// Dashboard represents the sensor data from a device/module
type Dashboard struct {
	TimeUTC          int64   `json:"time_utc"`
	Temperature      float64 `json:"Temperature"`
	Humidity         int     `json:"Humidity"`
	CO2              int     `json:"CO2"`
	Noise            int     `json:"Noise"`
	Pressure         float64 `json:"Pressure"`
	AbsolutePressure float64 `json:"AbsolutePressure"`
	MinTemp          float64 `json:"min_temp"`
	MaxTemp          float64 `json:"max_temp"`
	DateMinTemp      int64   `json:"date_min_temp"`
	DateMaxTemp      int64   `json:"date_max_temp"`
	TempTrend        string  `json:"temp_trend"`
	PressureTrend    string  `json:"pressure_trend"`
	Rain             float64 `json:"Rain"`
	Rain1            float64 `json:"rain_1"`
	Rain24           float64 `json:"rain_24"`
	DateRain         int64   `json:"date_rain"`
	WindStrength     int     `json:"WindStrength"`
	WindAngle        int     `json:"WindAngle"`
	GustStrength     int     `json:"GustStrength"`
	GustAngle        int     `json:"GustAngle"`
	DateWind         int64   `json:"date_wind"`
	MaxWindStr       int     `json:"max_wind_str"`
	MaxWindAngle     int     `json:"max_wind_angle"`
	DateMaxWindStr   int64   `json:"date_max_wind_str"`
}

// NewNetatmoModule creates a new Netatmo module instance
func NewNetatmoModule(config Config) (*NetatmoModule, error) {
	timeout := 30 * time.Second
	if config.Timeout != "" {
		if parsed, err := time.ParseDuration(config.Timeout); err == nil {
			timeout = parsed
		}
	}

	// Create OAuth2 client
	oauth2Config := utils.OAuth2Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		AuthURL:      "https://api.netatmo.com/oauth2/authorize",
		TokenURL:     "https://api.netatmo.com/oauth2/token",
		Scope:        "read_station",
		State:        "netatmo_auth",
	}

	oauth2Client, err := utils.NewOAuth2Client(oauth2Config, "netatmo")
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth2 client: %w", err)
	}

	return &NetatmoModule{
		config: config,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: "https://api.netatmo.com",
		oauth2:  oauth2Client,
	}, nil
}

// Run starts the Netatmo module and begins collecting metrics
func Run(ctx context.Context, ch chan<- metrics.Metric) error {
	config := LoadConfig()
	module, err := NewNetatmoModule(config)
	if err != nil {
		return fmt.Errorf("failed to create Netatmo module: %w", err)
	}
	module.metricsCh = ch

	return module.run(ctx)
}

// run executes the main module loop
func (nm *NetatmoModule) run(ctx context.Context) error {
	return utils.WithPanicRecoveryAndReturnError("Netatmo module", "main", func() error {
		// Authenticate with Netatmo API
		if err := nm.authenticate(ctx); err != nil {
			return fmt.Errorf("failed to authenticate with Netatmo API: %w", err)
		}

		// Set up ticker for data collection
		interval := 5 * time.Minute
		if nm.config.Interval != "" {
			if parsed, err := time.ParseDuration(nm.config.Interval); err == nil {
				interval = parsed
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Collect initial data
		if err := nm.collectData(ctx); err != nil {
			log.Printf("Warning: failed to collect initial data: %v", err)
		}

		// Main collection loop
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if err := nm.collectData(ctx); err != nil {
					log.Printf("Warning: failed to collect data: %v", err)
				}
			}
		}
	})
}

// authenticate performs OAuth2 authentication with Netatmo using the centralized OAuth2 client
func (nm *NetatmoModule) authenticate(ctx context.Context) error {
	return utils.WithPanicRecoveryAndReturnError("Netatmo authentication", "oauth", func() error {
		// Validate required configuration
		if nm.config.ClientID == "" {
			return fmt.Errorf("client_id is required but not configured")
		}
		if nm.config.ClientSecret == "" {
			return fmt.Errorf("client_secret is required but not configured")
		}

		// Use centralized OAuth2 authentication
		_, err := nm.oauth2.Authenticate(ctx)
		if err != nil {
			return fmt.Errorf("OAuth2 authentication failed: %w", err)
		}

		log.Printf("Successfully authenticated with Netatmo API")
		return nil
	})
}

// collectData fetches data from Netatmo API and sends metrics
func (nm *NetatmoModule) collectData(ctx context.Context) error {
	return utils.WithPanicRecoveryAndReturnError("Netatmo data collection", "api", func() error {
		// Create request
		req, err := http.NewRequest("GET", nm.baseURL+"/api/getstationsdata", nil)
		if err != nil {
			return err
		}

		// Use OAuth2Client's authenticated request method (handles retries automatically)
		resp, err := nm.oauth2.AuthenticatedRequest(ctx, nm.httpClient, req)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		defer resp.Body.Close()

		// Handle non-200 responses (after retries)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Parse response
		var stationData StationData
		if err := json.NewDecoder(resp.Body).Decode(&stationData); err != nil {
			return fmt.Errorf("failed to parse API response: %w", err)
		}

		if stationData.Status != "ok" {
			return fmt.Errorf("API returned non-ok status: %s", stationData.Status)
		}

		// Process the data and send metrics
		nm.processStationData(&stationData)

		return nil
	})
}

// processStationData processes the station data and sends metrics
func (nm *NetatmoModule) processStationData(data *StationData) {
	timestamp := time.Unix(data.Body.Devices[0].DashboardData.TimeUTC, 0)

	for _, device := range data.Body.Devices {
		// Get friendly name for the device
		friendlyName := nm.config.GetFriendlyName(device.ID, device.StationName, device.StationName)

		// Process main station data
		nm.sendDeviceMetrics(device.ID, friendlyName, &device.DashboardData, timestamp)

		// Process module data
		for _, module := range device.Modules {
			moduleFriendlyName := nm.config.GetFriendlyName(module.ID, module.ModuleName, module.ModuleName)
			nm.sendDeviceMetrics(module.ID, moduleFriendlyName, &module.DashboardData, timestamp)
		}
	}
}

// sendDeviceMetrics sends metrics for a specific device/module
func (nm *NetatmoModule) sendDeviceMetrics(deviceID string, friendlyName string, data *Dashboard, timestamp time.Time) {
	// Create base tags
	tags := map[string]string{
		"vendor":   "netatmo",
		"device":   deviceID,
		"friendly": friendlyName,
	}

	// Create fields map
	fields := make(map[string]interface{})

	// Add temperature if available
	if data.Temperature != 0 {
		fields["temperature"] = data.Temperature
	}

	// Add humidity if available
	if data.Humidity != 0 {
		fields["humidity"] = data.Humidity
	}

	// Add CO2 if available
	if data.CO2 != 0 {
		fields["co2"] = data.CO2
	}

	// Add noise if available
	if data.Noise != 0 {
		fields["noise"] = data.Noise
	}

	// Add pressure if available
	if data.Pressure != 0 {
		fields["pressure"] = data.Pressure
	}

	// Only send metrics if we have data
	if len(fields) > 0 {
		metric := metrics.Metric{
			Name:      "climate",
			Tags:      tags,
			Fields:    fields,
			Timestamp: timestamp,
		}

		select {
		case nm.metricsCh <- metric:
		default:
			log.Printf("Warning: metrics channel is full, dropping metric for device %s", deviceID)
		}
	}
}

// LoadConfig loads the Netatmo module configuration
func LoadConfig() Config {
	defaultConfig := Config{
		Timeout:  "30s",
		Interval: "5m",
	}

	loader := config.NewLoader("netatmo")
	if config.GlobalConfigPath != "" {
		loader.SetConfigPath(config.GlobalConfigPath)
	}

	loadedConfig, err := loader.LoadConfig(&defaultConfig)
	if err != nil {
		log.Printf("Warning: failed to load Netatmo configuration: %v", err)
		return defaultConfig
	}

	return *loadedConfig.(*Config)
}
