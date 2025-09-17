defmodule MetricsAgent.DemoModule do
  @moduledoc """
  Demo module for testing the metrics agent.
  
  This replaces the Go demo module with a simple GenServer that sends
  periodic metrics. Includes panic simulation for testing fault tolerance.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Logger.info("Starting demo module")
    
    config = Application.get_env(:metrics_agent, :demo)
    interval = config[:interval] || 5000
    
    # Schedule first metric collection
    Process.send_after(self(), :collect_metrics, interval)
    
    {:ok, %{
      config: config,
      interval: interval,
      counter: 0
    }}
  end

  @impl true
  def handle_info(:collect_metrics, state) do
    # Check for panic trigger file
    if File.exists?(state.config[:panic_trigger_file] || "/tmp/metrics-agent-panic-demo") do
      Logger.error("Demo module panic triggered by panic trigger file")
      # This will cause the process to crash and be restarted by the supervisor
      raise "Demo module panic triggered by /tmp/metrics-agent-panic-demo file"
    end
    
    # Generate demo metrics
    metrics = generate_demo_metrics(state.counter)
    
    # Send metrics to collector
    Enum.each(metrics, &MetricsAgent.MetricsCollector.send_metric/1)
    
    Logger.debug("Demo module sent #{length(metrics)} metrics (counter: #{state.counter})")
    
    # Schedule next collection
    Process.send_after(self(), :collect_metrics, state.interval)
    
    {:noreply, %{state | counter: state.counter + 1}}
  end

  @impl true
  def handle_info(msg, state) do
    Logger.warn("Unexpected message in demo module: #{inspect(msg)}")
    {:noreply, state}
  end

  # Private functions

  defp generate_demo_metrics(counter) do
    timestamp = System.system_time(:nanosecond)
    
    [
      %{
        name: "demo_counter",
        tags: %{module: "demo"},
        fields: %{value: counter},
        timestamp: timestamp
      },
      %{
        name: "demo_temperature",
        tags: %{module: "demo", sensor: "sensor1"},
        fields: %{temperature: 20.0 + :rand.uniform() * 10.0},
        timestamp: timestamp
      },
      %{
        name: "demo_humidity",
        tags: %{module: "demo", sensor: "sensor1"},
        fields: %{humidity: 40.0 + :rand.uniform() * 20.0},
        timestamp: timestamp
      },
      %{
        name: "demo_pressure",
        tags: %{module: "demo", sensor: "sensor1"},
        fields: %{pressure: 1013.25 + :rand.uniform() * 10.0},
        timestamp: timestamp
      }
    ]
  end
end
