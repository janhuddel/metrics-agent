defmodule MetricsAgent.MetricsCollector do
  @moduledoc """
  Collects metrics from all modules and serializes them to stdout.
  
  This replaces the Go metricchannel with a simple GenServer that receives
  metrics and outputs them in Line Protocol format.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Logger.info("Starting metrics collector")
    {:ok, %{}}
  end

  @impl true
  def handle_cast({:metric, metric}, state) do
    # Serialize metric to Line Protocol format and output to stdout
    line = serialize_metric(metric)
    IO.puts(line)
    {:noreply, state}
  end

  @impl true
  def handle_info(msg, state) do
    Logger.warn("Unexpected message in metrics collector: #{inspect(msg)}")
    {:noreply, state}
  end

  # Public API

  @doc """
  Send a metric to the collector for serialization and output.
  """
  def send_metric(metric) do
    GenServer.cast(__MODULE__, {:metric, metric})
  end

  # Private functions

  defp serialize_metric(%{name: name, tags: tags, fields: fields, timestamp: timestamp}) do
    # Convert tags to string format
    tag_string = tags
    |> Enum.map(fn {k, v} -> "#{k}=#{v}" end)
    |> Enum.join(",")
    
    # Convert fields to string format
    field_string = fields
    |> Enum.map(fn {k, v} -> "#{k}=#{format_field_value(v)}" end)
    |> Enum.join(",")
    
    # Build Line Protocol string
    case tag_string do
      "" -> "#{name} #{field_string} #{timestamp}"
      _ -> "#{name},#{tag_string} #{field_string} #{timestamp}"
    end
  end

  defp format_field_value(value) when is_binary(value), do: "\"#{value}\""
  defp format_field_value(value) when is_float(value), do: Float.to_string(value)
  defp format_field_value(value) when is_integer(value), do: Integer.to_string(value)
  defp format_field_value(value) when is_boolean(value), do: to_string(value)
  defp format_field_value(value), do: "\"#{inspect(value)}\""
end
