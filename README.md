# Metrics Agent

A lightweight metrics collection agent designed to work with Telegraf's `inputs.execd` plugin. It runs multiple metric collection modules concurrently in a single process, providing a unified interface for collecting metrics from various sources.

## Features

- **Modular Architecture**: Supports multiple metric collection modules
- **Telegraf Integration**: Designed to work seamlessly with Telegraf's `inputs.execd` plugin
- **Graceful Shutdown**: Handles SIGTERM/SIGINT for clean shutdown
- **Module Restart**: Supports SIGHUP for module restart without process termination
- **Panic Recovery**: Built-in panic recovery ensures process stability
- **Flexible Configuration**: JSON-based configuration with module-specific settings
- **Robust Error Handling**: Comprehensive fault tolerance with configurable restart limits

## Installation

### From Source

1. **Prerequisites**:
   - Go 1.25 or later
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
  "module_restart_limit": 3,
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
- `module_restart_limit`: Number of restart attempts before exiting (default: 3)
  - Set to `0` to disable restart limits (unlimited restarts) - **NOT recommended for telegraf/systemd**
  - Set to `1` for immediate exit on first failure
  - Set to `3` (recommended) for telegraf/systemd deployments
  - Higher values allow more restart attempts before giving up
  - Negative values fall back to default (3)

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

### Demo Module

A demonstration module for testing and development purposes. Includes panic simulation capabilities for testing the recovery mechanism.

## Robustness and Fault Tolerance

The metrics-agent is designed with comprehensive fault tolerance:

### Architecture Overview

```
Systemd → Telegraf → metrics-agent (inputs.execd) → Modules
```

- **Systemd**: Manages telegraf service
- **Telegraf**: Manages metrics-agent process lifecycle
- **metrics-agent**: Runs modules with panic recovery
- **Modules**: Collect metrics (demo, tasmota, etc.)

### Key Robustness Features

1. **Panic Recovery**: Comprehensive panic recovery in module execution goroutines prevents crashes from affecting the entire process
2. **Signal Handling**: Proper signal handling for shutdown and restart operations
3. **Resource Management**: Proper resource cleanup and management
4. **Error Handling**: Comprehensive error handling and logging
5. **Module Isolation**: Individual modules can fail without affecting others

### Restart Mechanisms

The metrics-agent supports multiple restart mechanisms:

1. **SIGHUP Restart**: Fast module restart without telegraf delay
   ```bash
   kill -HUP <pid>
   ```

2. **Process Exit**: Telegraf automatically restarts the process
   ```bash
   kill -TERM <pid>
   ```

3. **Telegraf Restart**: Full telegraf service restart
   ```bash
   sudo systemctl restart telegraf
   ```

4. **Systemd Restart**: Ultimate safety net
   ```bash
   sudo systemctl restart telegraf
   ```

### Fault Tolerance Layers

1. **Module Level**: Panic recovery in individual modules
2. **Process Level**: Graceful shutdown and error handling
3. **Telegraf Level**: Process restart on exit
4. **Systemd Level**: Service restart if telegraf fails

## Panic Recovery Testing

The demo module includes a panic simulation feature for testing the recovery mechanism.

### How It Works

- The demo module checks for the existence of `/tmp/metrics-agent-panic-demo` every 5 seconds
- If the file exists, the module will panic with the message: "Demo module panic triggered by /tmp/metrics-agent-panic-demo file"
- The panic recovery mechanism will catch this panic, log it, and restart the module automatically
- **Restart Limit**: After a configurable number of failed restart attempts (default: 3), the program will exit to prevent infinite restart loops

### Testing Steps

#### Option A: Use the Test Script (Recommended)

```bash
./test-panic.sh
```

This interactive script provides a menu to:
- Trigger panics in the demo module
- Remove panic triggers
- Check current status

#### Option B: Manual Commands

**Trigger a panic:**
```bash
touch /tmp/metrics-agent-panic-demo
```

**Remove panic trigger:**
```bash
rm /tmp/metrics-agent-panic-demo
```

### Expected Behavior

#### When you trigger a panic:

1. **Before panic**: Demo module sends metrics every 5 seconds
2. **Panic occurs**: You'll see logs like:
   ```
   [demo] starting module (attempt 1/4)
   Module execution panic recovered for device demo: Demo module panic triggered by /tmp/metrics-agent-panic-demo file
   [demo] module error: panic in Module execution: Demo module panic triggered by /tmp/metrics-agent-panic-demo file
   [demo] module stopped
   [demo] restarting module after completion/panic (restart 1/3)
   ```
3. **After restart**: Demo module restarts and continues sending metrics
4. **Tasmota module**: Continues running unaffected

#### When restart limit is reached:

After 3 failed restart attempts, you'll see:
```
[demo] starting module (attempt 4/4)
Module execution panic recovered for device demo: Demo module panic triggered by /tmp/metrics-agent-panic-demo file
[demo] module error: panic in Module execution: Demo module panic triggered by /tmp/metrics-agent-panic-demo file
[demo] module stopped
[demo] module failed 4 times, exiting program
```
The program will then exit gracefully.

### Cleanup

After testing, remove the panic trigger file:

```bash
rm -f /tmp/metrics-agent-panic-demo
```

## Telegraf/Systemd Deployment Considerations

When running metrics-agent under telegraf (inputs.execd) with systemd management:

### ✅ **Recommended Configuration:**
```json
{
    "module_restart_limit": 3
}
```

**Why this works:**
- **Temporary issues**: Module restarts automatically (resilient)
- **Persistent issues**: Process exits, telegraf restarts it
- **Proper failure propagation**: Systemd can monitor telegraf health
- **No infinite loops**: Prevents resource exhaustion

### ❌ **Avoid This Configuration:**
```json
{
    "module_restart_limit": 0
}
```

**Why this is problematic:**
- Process never exits on persistent module failures
- Telegraf thinks process is healthy but no metrics are collected
- Systemd can't detect the underlying issue
- No external restart mechanism triggered

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

## Monitoring and Alerting

### Health Checks

1. **Telegraf Health**: Monitor telegraf service status
2. **Metrics Flow**: Monitor metrics-agent output
3. **Process Restarts**: Alert on frequent telegraf restarts

### Log Monitoring

```bash
# Monitor metrics-agent logs
journalctl -u telegraf -f | grep metrics-agent

# Monitor restart events
journalctl -u telegraf | grep "module failed.*times, exiting program"
```

## Troubleshooting

### Common Issues

1. **Configuration file not found**: Ensure the configuration file exists and is readable
2. **Permission denied**: Check file permissions and user access
3. **Module errors**: Check the logs for specific error messages
4. **No Metrics**: Check if metrics-agent is running
5. **Frequent Restarts**: Check module configuration
6. **High CPU**: Check for infinite restart loops (shouldn't happen with limit=3)

### Debug Commands

```bash
# Check telegraf status
systemctl status telegraf

# Check metrics-agent process
ps aux | grep metrics-agent

# Test metrics-agent directly
/usr/local/bin/metrics-agent -c /path/to/config.json
```

### Logging

The agent logs to stderr with the prefix `[metrics-agent]`. Log levels can be configured in the configuration file.

### Signal Handling

- `SIGTERM`/`SIGINT`: Graceful shutdown
- `SIGHUP`: Restart all modules without terminating the process

## Best Practices

1. **Use restart limit 3**: Balances resilience with proper failure handling
2. **Monitor logs**: Watch for restart patterns
3. **Test configurations**: Validate before deployment
4. **Resource limits**: Set appropriate memory/CPU limits in systemd
5. **Backup configs**: Keep configuration files in version control

## Performance Benefits

The simplified architecture provides:

- Lower resource usage (single process vs multiple)
- Reduced complexity (no supervisor overhead)
- Better performance (no subprocess overhead)
- Easier debugging (single process, unified logs)

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