import Config

# Production configuration
config :metrics_agent,
  log_level: :info

# Demo module - disabled in production
config :metrics_agent, :demo,
  enabled: false

# Tasmota module - configure for production MQTT broker
config :metrics_agent, :tasmota,
  enabled: true,
  broker: "tcp://mqtt.example.com:1883",
  username: System.get_env("MQTT_USERNAME"),
  password: System.get_env("MQTT_PASSWORD")

# OpenDTU module - configure for production
config :metrics_agent, :opendtu,
  enabled: true,
  url: System.get_env("OPENDTU_URL") || "http://opendtu:80",
  username: System.get_env("OPENDTU_USERNAME"),
  password: System.get_env("OPENDTU_PASSWORD")

# WebSocket module - configure for production
config :metrics_agent, :websocket,
  enabled: true,
  url: System.get_env("WEBSOCKET_URL") || "ws://websocket.example.com:8080/ws"
