# Metrics Agent

A flexible metrics collection agent that can run various metric collection modules and output data in InfluxDB Line Protocol format.

## Features

- **Modular Architecture**: Pluggable metric collection modules
- **Supervisor Mode**: Manages multiple modules as subprocesses
- **Worker Mode**: Runs individual modules directly
- **InfluxDB Compatible**: Outputs metrics in Line Protocol format
- **Environment Configuration**: Supports `.env` files for easy configuration

## Available Modules

- **tasmota**: MQTT-based integration with Tasmota devices
- **demo**: Demonstration module for testing

## Quick Start

### 1. Build the Application

```bash
go build -o metrics-agent cmd/metrics-agent/main.go
```

### 2. Configure Environment Variables

Create a `.env` file in the project root:

```bash
# Copy the example file
cp env.example .env

# Edit with your settings
nano .env
```

Example `.env` file:
```env
# MQTT Configuration for Tasmota module
TASMOTA_MQTT_BROKER=tcp://localhost:1883
TASMOTA_MQTT_USERNAME=your_username
TASMOTA_MQTT_PASSWORD=your_password
```

### 3. Run the Application

```bash
# Run in supervisor mode (all modules)
./metrics-agent

# Run a specific module
./metrics-agent --module tasmota

# Run in worker mode (for debugging)
./metrics-agent --worker --module tasmota
```

## Configuration

The application supports configuration through:

1. **`.env` file**: Recommended for development and deployment
2. **Environment variables**: Set directly in your shell
3. **Command line flags**: For runtime options

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TASMOTA_MQTT_BROKER` | MQTT broker address | `tcp://localhost:1883` |
| `TASMOTA_MQTT_USERNAME` | MQTT username | (empty) |
| `TASMOTA_MQTT_PASSWORD` | MQTT password | (empty) |
| `TASMOTA_MQTT_CLIENT_ID` | MQTT client ID | `{hostname}-tasmota` |

### Command Line Flags

| Flag | Description |
|------|-------------|
| `--module` | Run specific module in worker mode |
| `--worker` | Run as worker subprocess |
| `--inprocess` | Run modules in-process (for debugging) |
| `--version` | Print version and exit |

## Output Format

The application outputs metrics in InfluxDB Line Protocol format to stdout:

```
tasmota_device,device=tasmota_17E7AE,device_name=plug-geschirrspueler,hostname=tasmota-17E7AE-1966,ip=172.19.13.2,mac=48551917E7AE,module=Nous\ A1T,status=discovered value=1i 1634234234000000000
tasmota_sensor,device=tasmota_17E7AE,device_name=plug-geschirrspueler,hostname=tasmota-17E7AE-1966,ip=172.19.13.2,mac=48551917E7AE,module=Nous\ A1T,sensor_type=DS18B20 Temperature=22.5 1634234234000000000
```

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build ./...
```

### Adding New Modules

1. Create a new module in `internal/modules/{module-name}/`
2. Implement the `ModuleFunc` signature: `func(ctx context.Context, ch chan<- metrics.Metric) error`
3. Register the module in `internal/modules/init.go`

## License

[Add your license information here]
