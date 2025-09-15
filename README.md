# Metrics Agent

A lightweight metrics collection agent designed to work with Telegraf's `inputs.execd` plugin. It runs multiple metric collection modules concurrently in a single process, providing a unified interface for collecting metrics from various sources.

## Features

- **Modular Architecture**: Supports multiple metric collection modules
- **Telegraf Integration**: Designed to work seamlessly with Telegraf's `inputs.execd` plugin
- **Graceful Shutdown**: Handles SIGTERM/SIGINT for clean shutdown
- **Module Restart**: Supports SIGHUP for module restart without process termination
- **Panic Recovery**: Built-in panic recovery ensures process stability
- **Flexible Configuration**: JSON-based configuration with module-specific settings

## Installation

### From Source

1. **Prerequisites**:
   - Go 1.19 or later
   - Git

2. **Clone and Build**:
   ```bash
   git clone https://github.com/janhuddel/metrics-agent.git
   cd metrics-agent
   make build
   ```

3. **Install Binary** (optional):
   ```bash
   sudo cp .build/metrics-agent /usr/local/bin/
   ```

### Using Pre-built Binaries

Download the latest release from the [Releases](https://github.com/janhuddel/metrics-agent/releases) page and extract the appropriate binary for your platform.

## Configuration

### Configuration File Location

The metrics agent looks for configuration files in the following order:

1. Path specified with `-c` flag
2. `metrics-agent.json` in current directory
3. `config/metrics-agent.json`
4. `config.json`
5. `config/config.json`

### Recommended Configuration Locations

For production deployments, we recommend placing the configuration file in one of these locations:

- **Linux/Unix**: `/etc/metrics-agent/metrics-agent.json`
- **macOS**: `/usr/local/etc/metrics-agent/metrics-agent.json`
- **Windows**: `C:\ProgramData\metrics-agent\metrics-agent.json`

### Configuration Format

Create a configuration file based on the example:

```bash
cp metrics-agent.example.json /etc/metrics-agent/metrics-agent.json
```

Edit the configuration file to match your environment:

```json
{
  "log_level": "info",
  "modules": {
    "tasmota": {
      "friendly_name_overrides": {
        "tasmota_6886BC.0": "Filteranlage",
        "tasmota_6886BC.1": "Wärmepumpe"
      },
      "custom": {
        "broker": "tcp://mqtt.example.com:1883",
        "username": "mqtt_user",
        "password": "mqtt_password",
        "timeout": "30s",
        "keep_alive": "60s",
        "ping_timeout": "10s"
      }
    }
  }
}
```

### Configuration Options

#### Global Settings

- `log_level`: Set the logging level (`debug`, `info`, `warn`, `error`)

#### Module Configuration

Each module can have:

- `friendly_name_overrides`: Map device IDs to human-readable names
- `custom`: Module-specific configuration options

## Usage

### Basic Usage

```bash
# Use default configuration file location
./metrics-agent

# Specify custom configuration file
./metrics-agent -c /path/to/config.json

# Show version information
./metrics-agent -version
```

### Integration with Telegraf

Add the following to your Telegraf configuration:

```toml
[[inputs.execd]]
  command = ["/usr/local/bin/metrics-agent", "-c", "/etc/metrics-agent/metrics-agent.json"]
  signal = "STDIN"
  restart_delay = "10s"
```

### Systemd Service (Linux)

Create a systemd service file at `/etc/systemd/system/metrics-agent.service`:

```ini
[Unit]
Description=Metrics Agent
After=network.target

[Service]
Type=simple
User=telegraf
Group=telegraf
ExecStart=/usr/local/bin/metrics-agent -c /etc/metrics-agent/metrics-agent.json
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable metrics-agent
sudo systemctl start metrics-agent
```

## Available Modules

### Tasmota Module

Collects metrics from Tasmota devices via MQTT.

#### Configuration Options

- `broker`: MQTT broker address (default: `tcp://localhost:1883`)
- `username`: MQTT username (optional)
- `password`: MQTT password (optional)
- `client_id`: MQTT client ID (optional, defaults to hostname)
- `timeout`: Connection timeout (default: `30s`)
- `keep_alive`: Keep-alive interval (default: `60s`)
- `ping_timeout`: Ping timeout (default: `10s`)

## Development

### Building

```bash
# Build for current platform
make build

# Build release binaries for multiple platforms
make release

# Run tests
make test

# Clean build artifacts
make clean
```

### Adding New Modules

1. Create a new module package in `internal/modules/`
2. Implement the `ModuleFunc` interface
3. Register the module in `internal/modules/init.go`
4. Add configuration support if needed

## Troubleshooting

### Common Issues

1. **Configuration file not found**: Ensure the configuration file exists and is readable
2. **Permission denied**: Check file permissions and user access
3. **Module errors**: Check the logs for specific error messages

### Logging

The agent logs to stderr with the prefix `[metrics-agent]`. Log levels can be configured in the configuration file.

### Signal Handling

- `SIGTERM`/`SIGINT`: Graceful shutdown
- `SIGHUP`: Restart all modules without terminating the process

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and questions, please use the [GitHub Issues](https://github.com/janhuddel/metrics-agent/issues) page.
