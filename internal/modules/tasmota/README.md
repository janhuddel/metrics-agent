# Tasmota Module

The Tasmota module provides integration with Tasmota devices through MQTT. It automatically discovers Tasmota devices and collects sensor data from them.

## Features

- **Device Discovery**: Automatically discovers Tasmota devices by subscribing to the `tasmota/discovery/+/config` topic
- **Sensor Data Collection**: Subscribes to sensor data topics for each discovered device
- **Metric Generation**: Converts device and sensor data into metrics compatible with the metrics-agent system

## Configuration

The module can be configured using environment variables. You can set these in several ways:

### Using a .env file (Recommended)

Create a `.env` file in your project root:

```bash
# Copy the example file
cp env.example .env

# Edit the .env file with your settings
nano .env
```

Example `.env` file:
```env
TASMOTA_MQTT_BROKER=tcp://localhost:1883
TASMOTA_MQTT_USERNAME=your_username
TASMOTA_MQTT_PASSWORD=your_password
TASMOTA_MQTT_CLIENT_ID=my-tasmota-client
```

### Environment Variables

- `TASMOTA_MQTT_BROKER`: MQTT broker address (default: `tcp://localhost:1883`)
- `TASMOTA_MQTT_USERNAME`: MQTT username (optional)
- `TASMOTA_MQTT_PASSWORD`: MQTT password (optional)
- `TASMOTA_MQTT_CLIENT_ID`: MQTT client ID (default: hostname + "-tasmota")

### Setting Environment Variables Directly

```bash
export TASMOTA_MQTT_BROKER="tcp://your-mqtt-broker:1883"
export TASMOTA_MQTT_USERNAME="your-username"
export TASMOTA_MQTT_PASSWORD="your-password"
```

## Device Discovery

When a Tasmota device publishes its configuration to the discovery topic, the module:

1. Parses the device information from the JSON payload
2. Stores device metadata (IP, hostname, MAC, module type, etc.)
3. Subscribes to the device's sensor topic (`tele/{topic}/SENSOR`)
4. Sends a device discovery metric

## Sensor Data Processing

For each sensor data message received, the module:

1. Parses the JSON sensor data
2. Creates metrics for each sensor type and field
3. Includes device metadata as tags

## Generated Metrics

### Device Metrics (`tasmota_device`)

Tags:
- `device`: Device topic name
- `device_name`: Device friendly name
- `hostname`: Device hostname
- `ip`: Device IP address
- `mac`: Device MAC address
- `module`: Device module type
- `status`: Device status (e.g., "discovered")

Fields:
- `value`: Status value

### Sensor Metrics (`tasmota_sensor`)

Tags:
- `device`: Device topic name
- `device_name`: Device friendly name
- `hostname`: Device hostname
- `ip`: Device IP address
- `mac`: Device MAC address
- `module`: Device module type
- `sensor_type`: Type of sensor (e.g., "DS18B20", "DHT22")

Fields:
- Dynamic fields based on sensor data (e.g., "Temperature", "Humidity")

## Example Usage

### Using the Command Line

```bash
# Create a .env file with your MQTT broker settings
cp env.example .env
# Edit .env with your broker details

# Run the metrics agent with the tasmota module
./metrics-agent --module tasmota

# Or run in supervisor mode (runs all registered modules)
./metrics-agent
```

### Programmatic Usage

```go
// The module is automatically registered as "tasmota"
// and can be run through the module registry:

ctx := context.Background()
ch := make(chan metrics.Metric, 100)

err := modules.Global.Run(ctx, "tasmota", ch)
if err != nil {
    log.Fatal(err)
}
```

## Example Device Discovery Payload

```json
{
  "ip": "172.19.13.2",
  "dn": "plug-geschirrspueler",
  "fn": ["Geschirrspüler", null, null, null, null, null, null, null],
  "hn": "tasmota-17E7AE-1966",
  "mac": "48551917E7AE",
  "md": "Nous A1T",
  "ty": 0,
  "if": 0,
  "ofln": "Offline",
  "onln": "Online",
  "state": ["OFF", "ON", "TOGGLE", "HOLD"],
  "sw": "13.1.0.1",
  "t": "tasmota_17E7AE",
  "ft": "%prefix%/%topic%/",
  "tp": ["cmnd", "stat", "tele"],
  "rl": [1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  "swc": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1],
  "swn": [null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null],
  "btn": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  "so": {"4": 0, "11": 0, "13": 0, "17": 0, "20": 0, "30": 0, "68": 0, "73": 0, "82": 0, "114": 0, "117": 0},
  "lk": 0,
  "lt_st": 0,
  "bat": 0,
  "dslp": 0,
  "sho": [],
  "sht": [],
  "ver": 1
}
```

This device would result in subscription to: `tele/tasmota_17E7AE/SENSOR`
