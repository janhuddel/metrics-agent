defmodule MetricsAgent.TasmotaModule do
  @moduledoc """
  Tasmota module for collecting metrics from Tasmota devices via MQTT.
  
  This replaces the complex Go TasmotaModule with a simple GenServer that
  uses Tortoise for MQTT communication. No manual error handling, mutexes,
  or panic recovery needed.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Logger.info("Starting Tasmota module")
    
    config = Application.get_env(:metrics_agent, :tasmota)
    
    # Generate client ID if not provided
    client_id = config[:client_id] || generate_client_id()
    
    # Start MQTT client
    {:ok, client} = Tortoise.Connection.start_link(
      client_id: client_id,
      server: {Tortoise.Transport.Tcp, host: parse_host(config[:broker]), port: parse_port(config[:broker])},
      user_name: config[:username],
      password: config[:password],
      keep_alive: config[:keep_alive] || 60,
      will: nil,
      subscriptions: [
        {config[:discovery_topic] || "tele/+/INFO1", 1},  # Device discovery
        {config[:lwt_topic] || "tele/+/LWT", 1}           # Device online/offline
      ],
      handler: {__MODULE__, :handle_message, []}
    )

    # Wait for connection
    :ok = Tortoise.Connection.subscribe(client, config[:discovery_topic] || "tele/+/INFO1", qos: 1)
    :ok = Tortoise.Connection.subscribe(client, config[:lwt_topic] || "tele/+/LWT", qos: 1)

    Logger.info("Tasmota module started with MQTT client: #{client_id}")
    
    {:ok, %{
      client: client,
      devices: %{},
      config: config,
      subscribed_topics: MapSet.new()
    }}
  end

  @impl true
  def handle_info({Tortoise, client, :connected}, state) do
    Logger.info("Connected to MQTT broker")
    {:noreply, state}
  end

  @impl true
  def handle_info({Tortoise, client, :disconnected}, state) do
    Logger.warn("Disconnected from MQTT broker")
    {:noreply, state}
  end

  @impl true
  def handle_info({Tortoise, client, {:ok, ref}}, state) do
    Logger.debug("MQTT operation completed: #{inspect(ref)}")
    {:noreply, state}
  end

  @impl true
  def handle_info({Tortoise, client, {:error, reason}}, state) do
    Logger.error("MQTT operation failed: #{inspect(reason)}")
    {:noreply, state}
  end

  @impl true
  def handle_info(msg, state) do
    Logger.warn("Unexpected message in Tasmota module: #{inspect(msg)}")
    {:noreply, state}
  end

  # MQTT message handler
  def handle_message(topic, payload, state) do
    case topic do
      "tele/" <> device_topic <> "/INFO1" ->
        handle_device_discovery(device_topic, payload, state)
      
      "tele/" <> device_topic <> "/SENSOR" ->
        handle_sensor_data(device_topic, payload, state)
      
      "tele/" <> device_topic <> "/LWT" ->
        handle_device_status(device_topic, payload, state)
      
      _ ->
        Logger.debug("Received message on unhandled topic: #{topic}")
        {:noreply, state}
    end
  end

  # Private functions

  defp handle_device_discovery(device_topic, payload, state) do
    case Jason.decode(payload) do
      {:ok, device_info} ->
        Logger.info("Discovered Tasmota device: #{device_info["DN"]} (#{device_topic}) at #{device_info["IP"]}")
        
        # Store device info
        new_devices = Map.put(state.devices, device_topic, device_info)
        
        # Subscribe to sensor data for this device
        sensor_topic = "tele/#{device_topic}/SENSOR"
        if not MapSet.member?(state.subscribed_topics, sensor_topic) do
          :ok = Tortoise.Connection.subscribe(state.client, sensor_topic, qos: 1)
          new_subscribed = MapSet.put(state.subscribed_topics, sensor_topic)
          Logger.debug("Subscribed to sensor topic: #{sensor_topic}")
          {:noreply, %{state | devices: new_devices, subscribed_topics: new_subscribed}}
        else
          {:noreply, %{state | devices: new_devices}}
        end
      
      {:error, reason} ->
        Logger.error("Failed to parse device discovery message: #{reason}")
        {:noreply, state}
    end
  end

  defp handle_sensor_data(device_topic, payload, state) do
    case Jason.decode(payload) do
      {:ok, sensor_data} ->
        # Get device info
        device_info = Map.get(state.devices, device_topic, %{})
        
        # Process sensor data and create metrics
        metrics = process_sensor_data(device_topic, device_info, sensor_data)
        
        # Send metrics to collector
        Enum.each(metrics, &MetricsAgent.MetricsCollector.send_metric/1)
        
        {:noreply, state}
      
      {:error, reason} ->
        Logger.error("Failed to parse sensor data for device #{device_topic}: #{reason}")
        {:noreply, state}
    end
  end

  defp handle_device_status(device_topic, payload, state) do
    status = String.trim(payload)
    Logger.info("Device #{device_topic} status: #{status}")
    
    # Create status metric
    metric = %{
      name: "tasmota_device_status",
      tags: %{device: device_topic},
      fields: %{status: status},
      timestamp: System.system_time(:nanosecond)
    }
    
    MetricsAgent.MetricsCollector.send_metric(metric)
    {:noreply, state}
  end

  defp process_sensor_data(device_topic, device_info, sensor_data) do
    device_name = device_info["DN"] || device_topic
    device_ip = device_info["IP"] || "unknown"
    
    # Process different sensor types
    metrics = []
    
    # Temperature sensors
    metrics = case sensor_data do
      %{"DS18B20" => ds18b20} when is_map(ds18b20) ->
        Enum.reduce(ds18b20, metrics, fn {sensor_id, sensor_data}, acc ->
          case sensor_data do
            %{"Temperature" => temp} ->
              [%{
                name: "tasmota_temperature",
                tags: %{device: device_topic, device_name: device_name, device_ip: device_ip, sensor: sensor_id},
                fields: %{temperature: temp},
                timestamp: System.system_time(:nanosecond)
              } | acc]
            _ -> acc
          end
        end)
      _ -> metrics
    end
    
    # DHT sensors
    metrics = case sensor_data do
      %{"DHT11" => dht_data} when is_map(dht_data) ->
        dht_metrics = []
        dht_metrics = if Map.has_key?(dht_data, "Temperature"), do: [%{
          name: "tasmota_temperature",
          tags: %{device: device_topic, device_name: device_name, device_ip: device_ip, sensor: "DHT11"},
          fields: %{temperature: dht_data["Temperature"]},
          timestamp: System.system_time(:nanosecond)
        } | dht_metrics], else: dht_metrics
        dht_metrics = if Map.has_key?(dht_data, "Humidity"), do: [%{
          name: "tasmota_humidity",
          tags: %{device: device_topic, device_name: device_name, device_ip: device_ip, sensor: "DHT11"},
          fields: %{humidity: dht_data["Humidity"]},
          timestamp: System.system_time(:nanosecond)
        } | dht_metrics], else: dht_metrics
        dht_metrics ++ metrics
      _ -> metrics
    end
    
    # Power sensors (SML, PZEM, etc.)
    metrics = case sensor_data do
      %{"SML" => sml_data} when is_map(sml_data) ->
        sml_metrics = []
        sml_metrics = if Map.has_key?(sml_data, "Total_in"), do: [%{
          name: "tasmota_energy_total_in",
          tags: %{device: device_topic, device_name: device_name, device_ip: device_ip},
          fields: %{total_in: sml_data["Total_in"]},
          timestamp: System.system_time(:nanosecond)
        } | sml_metrics], else: sml_metrics
        sml_metrics = if Map.has_key?(sml_data, "Total_out"), do: [%{
          name: "tasmota_energy_total_out",
          tags: %{device: device_topic, device_name: device_name, device_ip: device_ip},
          fields: %{total_out: sml_data["Total_out"]},
          timestamp: System.system_time(:nanosecond)
        } | sml_metrics], else: sml_metrics
        sml_metrics ++ metrics
      _ -> metrics
    end
    
    # Generic sensor data
    metrics = case sensor_data do
      %{"Time" => time} ->
        [%{
          name: "tasmota_sensor_data",
          tags: %{device: device_topic, device_name: device_name, device_ip: device_ip},
          fields: %{timestamp: time, raw_data: sensor_data},
          timestamp: System.system_time(:nanosecond)
        } | metrics]
      _ -> metrics
    end
    
    metrics
  end

  defp generate_client_id do
    hostname = case :inet.gethostname() do
      {:ok, hostname} -> List.to_string(hostname)
      {:error, _} -> "unknown"
    end
    "#{hostname}-tasmota-#{:rand.uniform(10000)}"
  end

  defp parse_host(broker) when is_binary(broker) do
    case String.split(broker, "://") do
      [_, host_port] ->
        case String.split(host_port, ":") do
          [host, _port] -> host
          [host] -> host
        end
      _ -> "localhost"
    end
  end

  defp parse_port(broker) when is_binary(broker) do
    case String.split(broker, "://") do
      [_, host_port] ->
        case String.split(host_port, ":") do
          [_host, port] -> String.to_integer(port)
          [_host] -> 1883
        end
      _ -> 1883
    end
  end
end
