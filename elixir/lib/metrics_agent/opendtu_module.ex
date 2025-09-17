defmodule MetricsAgent.OpenDTUModule do
  @moduledoc """
  OpenDTU module for collecting metrics from OpenDTU devices.
  
  This replaces the Go OpenDTU module with a simple GenServer that polls
  OpenDTU devices via HTTP and sends metrics.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Logger.info("Starting OpenDTU module")
    
    config = Application.get_env(:metrics_agent, :opendtu)
    interval = config[:interval] || 10_000
    
    # Schedule first data collection
    Process.send_after(self(), :collect_data, interval)
    
    {:ok, %{
      config: config,
      interval: interval
    }}
  end

  @impl true
  def handle_info(:collect_data, state) do
    # Collect data from OpenDTU
    case collect_opendtu_data(state.config) do
      {:ok, data} ->
        # Process data and create metrics
        metrics = process_opendtu_data(data)
        
        # Send metrics to collector
        Enum.each(metrics, &MetricsAgent.MetricsCollector.send_metric/1)
        
        Logger.debug("OpenDTU module sent #{length(metrics)} metrics")
      
      {:error, reason} ->
        Logger.error("Failed to collect OpenDTU data: #{reason}")
    end
    
    # Schedule next collection
    Process.send_after(self(), :collect_data, state.interval)
    
    {:noreply, state}
  end

  @impl true
  def handle_info(msg, state) do
    Logger.warn("Unexpected message in OpenDTU module: #{inspect(msg)}")
    {:noreply, state}
  end

  # Private functions

  defp collect_opendtu_data(config) do
    url = config[:url] || "http://localhost:80"
    username = config[:username]
    password = config[:password]
    
    # Build request options
    opts = [url: "#{url}/api/livedata/status"]
    opts = if username && password do
      [auth: {username, password} | opts]
    else
      opts
    end
    
    case Req.get(opts) do
      {:ok, %{status: 200, body: body}} ->
        {:ok, body}
      
      {:ok, %{status: status}} ->
        {:error, "HTTP #{status}"}
      
      {:error, reason} ->
        {:error, inspect(reason)}
    end
  end

  defp process_opendtu_data(data) do
    timestamp = System.system_time(:nanosecond)
    
    case data do
      %{"inverters" => inverters} when is_list(inverters) ->
        Enum.flat_map(inverters, fn inverter ->
          process_inverter_data(inverter, timestamp)
        end)
      
      _ ->
        Logger.warn("Unexpected OpenDTU data format")
        []
    end
  end

  defp process_inverter_data(inverter, timestamp) do
    serial = inverter["serial"] || "unknown"
    name = inverter["name"] || serial
    
    metrics = []
    
    # AC power data
    metrics = case inverter["AC"] do
      %{"0" => ac_data} when is_map(ac_data) ->
        ac_metrics = []
        ac_metrics = if Map.has_key?(ac_data, "Power"), do: [%{
          name: "opendtu_ac_power",
          tags: %{inverter: serial, inverter_name: name},
          fields: %{power: ac_data["Power"]},
          timestamp: timestamp
        } | ac_metrics], else: ac_metrics
        ac_metrics = if Map.has_key?(ac_data, "Voltage"), do: [%{
          name: "opendtu_ac_voltage",
          tags: %{inverter: serial, inverter_name: name},
          fields: %{voltage: ac_data["Voltage"]},
          timestamp: timestamp
        } | ac_metrics], else: ac_metrics
        ac_metrics = if Map.has_key?(ac_data, "Current"), do: [%{
          name: "opendtu_ac_current",
          tags: %{inverter: serial, inverter_name: name},
          fields: %{current: ac_data["Current"]},
          timestamp: timestamp
        } | ac_metrics], else: ac_metrics
        ac_metrics = if Map.has_key?(ac_data, "Frequency"), do: [%{
          name: "opendtu_ac_frequency",
          tags: %{inverter: serial, inverter_name: name},
          fields: %{frequency: ac_data["Frequency"]},
          timestamp: timestamp
        } | ac_metrics], else: ac_metrics
        ac_metrics ++ metrics
      _ -> metrics
    end
    
    # DC power data
    metrics = case inverter["DC"] do
      dc_data when is_map(dc_data) ->
        Enum.reduce(dc_data, metrics, fn {string_id, string_data}, acc ->
          case string_data do
            %{"Power" => power} ->
              [%{
                name: "opendtu_dc_power",
                tags: %{inverter: serial, inverter_name: name, string: string_id},
                fields: %{power: power},
                timestamp: timestamp
              } | acc]
            _ -> acc
          end
        end)
      _ -> metrics
    end
    
    # Total energy data
    metrics = case inverter["TOTAL_ENERGY"] do
      %{"v" => total_energy} ->
        [%{
          name: "opendtu_total_energy",
          tags: %{inverter: serial, inverter_name: name},
          fields: %{total_energy: total_energy},
          timestamp: timestamp
        } | metrics]
      _ -> metrics
    end
    
    # Yield today
    metrics = case inverter["TOTAL_YIELD"] do
      %{"v" => yield_today} ->
        [%{
          name: "opendtu_yield_today",
          tags: %{inverter: serial, inverter_name: name},
          fields: %{yield_today: yield_today},
          timestamp: timestamp
        } | metrics]
      _ -> metrics
    end
    
    metrics
  end
end
