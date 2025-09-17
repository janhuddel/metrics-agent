import Config

# Development configuration
config :metrics_agent,
  log_level: :debug

# Demo module - enabled for testing
config :metrics_agent, :demo,
  enabled: true,
  interval: 5000

# Tasmota module - configure for your local MQTT broker
config :metrics_agent, :tasmota,
  enabled: true,
  broker: "tcp://localhost:1883",
  username: nil,
  password: nil

# OpenDTU module - disabled by default in dev
config :metrics_agent, :opendtu,
  enabled: false

# WebSocket module - disabled by default in dev
config :metrics_agent, :websocket,
  enabled: false
