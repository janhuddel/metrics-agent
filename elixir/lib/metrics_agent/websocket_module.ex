defmodule MetricsAgent.WebSocketModule do
  @moduledoc """
  WebSocket module for collecting metrics via WebSocket connections.
  
  This replaces the Go WebSocket module with a simple GenServer that uses
  WebSockex for WebSocket communication.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Logger.info("Starting WebSocket module")
    
    config = Application.get_env(:metrics_agent, :websocket)
    
    # Start WebSocket connection
    case WebSockex.start_link(config[:url], __MODULE__, config, name: :websocket_client) do
      {:ok, pid} ->
        Logger.info("WebSocket client started")
        {:ok, %{config: config, client: pid}}
      
      {:error, reason} ->
        Logger.error("Failed to start WebSocket client: #{inspect(reason)}")
        # Schedule retry
        Process.send_after(self(), :retry_connection, 5000)
        {:ok, %{config: config, client: nil}}
    end
  end

  @impl true
  def handle_info(:retry_connection, state) do
    case WebSockex.start_link(state.config[:url], __MODULE__, state.config, name: :websocket_client) do
      {:ok, pid} ->
        Logger.info("WebSocket client reconnected")
        {:noreply, %{state | client: pid}}
      
      {:error, reason} ->
        Logger.error("Failed to reconnect WebSocket client: #{inspect(reason)}")
        # Schedule another retry
        Process.send_after(self(), :retry_connection, 5000)
        {:noreply, state}
    end
  end

  @impl true
  def handle_info(msg, state) do
    Logger.warn("Unexpected message in WebSocket module: #{inspect(msg)}")
    {:noreply, state}
  end

  # WebSockex callbacks

  def handle_connect(_conn, state) do
    Logger.info("WebSocket connected")
    {:ok, state}
  end

  def handle_disconnect(disconnect_map, state) do
    Logger.warn("WebSocket disconnected: #{inspect(disconnect_map)}")
    # WebSockex will automatically attempt to reconnect
    {:reconnect, state}
  end

  def handle_frame({:text, message}, state) do
    case Jason.decode(message) do
      {:ok, data} ->
        # Process WebSocket message and create metrics
        metrics = process_websocket_data(data)
        
        # Send metrics to collector
        Enum.each(metrics, &MetricsAgent.MetricsCollector.send_metric/1)
        
        Logger.debug("WebSocket module processed message and sent #{length(metrics)} metrics")
        {:ok, state}
      
      {:error, reason} ->
        Logger.error("Failed to parse WebSocket message: #{reason}")
        {:ok, state}
    end
  end

  def handle_frame({:binary, data}, state) do
    Logger.debug("Received binary WebSocket message: #{byte_size(data)} bytes")
    {:ok, state}
  end

  def handle_frame(frame, state) do
    Logger.debug("Received WebSocket frame: #{inspect(frame)}")
    {:ok, state}
  end

  def handle_ping(ping_frame, state) do
    Logger.debug("Received WebSocket ping")
    {:reply, :pong, state}
  end

  def handle_pong(pong_frame, state) do
    Logger.debug("Received WebSocket pong")
    {:ok, state}
  end

  # Private functions

  defp process_websocket_data(data) do
    timestamp = System.system_time(:nanosecond)
    
    # Generic WebSocket data processing
    # This can be customized based on your WebSocket data format
    case data do
      %{"type" => "metric", "name" => name, "value" => value, "tags" => tags} ->
        [%{
          name: "websocket_#{name}",
          tags: Map.new(tags || []),
          fields: %{value: value},
          timestamp: timestamp
        }]
      
      %{"metrics" => metrics} when is_list(metrics) ->
        Enum.map(metrics, fn metric ->
          %{
            name: "websocket_#{metric["name"]}",
            tags: Map.new(metric["tags"] || []),
            fields: Map.new(metric["fields"] || []),
            timestamp: timestamp
          }
        end)
      
      _ ->
        # Generic metric for any WebSocket data
        [%{
          name: "websocket_data",
          tags: %{},
          fields: %{raw_data: data},
          timestamp: timestamp
        }]
    end
  end
end
