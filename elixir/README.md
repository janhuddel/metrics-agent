# Metrics Agent - Elixir Implementation

This is an Elixir implementation of the metrics agent, designed to be much simpler and more robust than the Go version.

## Key Advantages Over Go Implementation

### **Eliminated Complexity**

- ❌ **No panic recovery** - Supervisor handles all crashes automatically
- ❌ **No mutex management** - Each process has isolated state
- ❌ **No manual signal handling** - BEAM VM handles process lifecycle
- ❌ **No complex error handling** - Let it crash philosophy
- ❌ **No manual reconnection logic** - Libraries handle reconnection

### **Built-in Fault Tolerance**

- ✅ **Supervisor trees** - Automatic restart of crashed processes
- ✅ **Process isolation** - One module crash doesn't affect others
- ✅ **Hot code reloading** - Update modules without restart
- ✅ **Millions of concurrent connections** - BEAM VM is designed for this

## Architecture

```
Application
├── MetricsCollector (GenServer)
└── ModuleSupervisor
    ├── DemoModule (GenServer)
    ├── TasmotaModule (GenServer + Tortoise MQTT)
    ├── OpenDTUModule (GenServer + HTTP)
    └── WebSocketModule (GenServer + WebSockex)
```

## Installation

1. Install Elixir (1.14+)
2. Install dependencies:
   ```bash
   cd elixir
   mix deps.get
   ```

## Configuration

Edit `config/config.exs` to enable/disable modules and configure settings:

```elixir
# Enable/disable modules
config :metrics_agent, :demo, enabled: true
config :metrics_agent, :tasmota, enabled: true
config :metrics_agent, :opendtu, enabled: false
config :metrics_agent, :websocket, enabled: false

# Configure MQTT broker
config :metrics_agent, :tasmota,
  broker: "tcp://localhost:1883",
  username: nil,
  password: nil
```

## Running

### Development

```bash
mix run --no-halt
```

### Production

```bash
MIX_ENV=prod mix run --no-halt
```

### Interactive Shell

```bash
iex -S mix
```

## Testing Panic Recovery

The demo module includes panic simulation for testing fault tolerance:

```bash
# Trigger a panic
touch /tmp/metrics-agent-panic-demo

# Remove panic trigger
rm /tmp/metrics-agent-panic-demo
```

When a panic occurs, you'll see:

1. Module crashes and logs the panic
2. Supervisor automatically restarts the module
3. Module continues working normally
4. Other modules are unaffected

## Module Details

### Demo Module

- Sends periodic demo metrics (temperature, humidity, pressure, counter)
- Includes panic simulation for testing
- Configurable interval (default: 5 seconds)

### Tasmota Module

- Connects to MQTT broker using Tortoise
- Automatically discovers Tasmota devices
- Subscribes to sensor data topics
- Processes temperature, humidity, power, and other sensor data
- Automatic reconnection on connection loss

### OpenDTU Module

- Polls OpenDTU devices via HTTP API
- Collects inverter power, voltage, current, and energy data
- Configurable polling interval (default: 10 seconds)

### WebSocket Module

- Connects to WebSocket endpoints
- Processes incoming messages and creates metrics
- Automatic reconnection on connection loss

## Metrics Output

All metrics are output to stdout in InfluxDB Line Protocol format:

```
demo_counter,module=demo value=1 1640995200000000000
demo_temperature,module=demo,sensor=sensor1 temperature=25.3 1640995200000000000
tasmota_temperature,device=sonoff-1,device_name=Living Room,sensor=DHT11 temperature=22.1 1640995200000000000
```

## Comparison with Go Implementation

| Feature              | Go Implementation          | Elixir Implementation     |
| -------------------- | -------------------------- | ------------------------- |
| **Lines of Code**    | ~2000+ lines               | ~800 lines                |
| **Panic Recovery**   | Manual wrappers everywhere | Automatic via supervisor  |
| **Mutex Management** | Manual sync.RWMutex        | No mutexes needed         |
| **Signal Handling**  | Complex channel-based      | Automatic via BEAM        |
| **Error Handling**   | Manual error propagation   | Let it crash philosophy   |
| **Reconnection**     | Manual retry logic         | Automatic via libraries   |
| **Fault Tolerance**  | Manual restart logic       | Built-in supervisor trees |
| **Concurrency**      | Goroutines + channels      | Lightweight processes     |
| **Memory Usage**     | ~8KB per goroutine         | ~2KB per process          |
| **Hot Reloading**    | Not supported              | Built-in support          |

## Benefits

1. **Simpler Code** - 60% less code, much easier to understand
2. **Better Reliability** - Built-in fault tolerance and supervision
3. **Higher Performance** - Millions of concurrent connections
4. **Easier Maintenance** - Hot code reloading and better debugging
5. **No Race Conditions** - Process isolation prevents data races
6. **Automatic Recovery** - No manual error handling needed

## Dependencies

- **Tortoise** - MQTT client library
- **Jason** - JSON parsing
- **Req** - HTTP client for OpenDTU
- **WebSockex** - WebSocket client
- **ConfigTuples** - Configuration management

## License

Same as the original Go implementation.
