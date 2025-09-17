# Example configuration file for metrics agent
# Copy this to config.exs and modify as needed

import Config

# General application configuration
config :metrics_agent,
  log_level: :info,
  max_restarts: 3,
  restart_interval: 1000

# Demo module configuration
config :metrics_agent, :demo,
  enabled: true,
  interval: 5000,
  panic_trigger_file: "/tmp/metrics-agent-panic-demo"

# Tasmota module configuration
config :metrics_agent, :tasmota,
  enabled: true,
  broker: "tcp://localhost:1883",
  client_id: nil,  # Will be auto-generated
  username: nil,
  password: nil,
  keep_alive: 60,
  timeout: 10_000,
  ping_timeout: 10_000,
  discovery_topic: "tele/+/INFO1",
  sensor_topic: "tele/+/SENSOR",
  lwt_topic: "tele/+/LWT"

# OpenDTU module configuration
config :metrics_agent, :opendtu,
  enabled: false,
  url: "http://localhost:80",
  username: nil,
  password: nil,
  interval: 10_000

# WebSocket module configuration
config :metrics_agent, :websocket,
  enabled: false,
  url: "ws://localhost:8080/ws",
  reconnect_interval: 5000,
  max_reconnect_attempts: 10,
  connection_timeout: 10_000,
  read_timeout: 30_000,
  write_timeout: 10_000
