defmodule MetricsAgent.ModuleSupervisor do
  @moduledoc """
  Supervisor for all metric collection modules.
  
  This replaces the complex Go ModuleManager with a simple supervisor pattern.
  Each module runs as a supervised process with automatic restart on failure.
  """

  use Supervisor
  require Logger

  def start_link(_opts) do
    Supervisor.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Logger.info("Starting module supervisor")
    
    # Get enabled modules from configuration
    enabled_modules = get_enabled_modules()
    
    if Enum.empty?(enabled_modules) do
      Logger.warn("No modules enabled, exiting")
      Supervisor.init([], strategy: :one_for_one)
    else
      Logger.info("Starting enabled modules: #{inspect(enabled_modules)}")
      
      # Create children for enabled modules
      children = Enum.map(enabled_modules, &create_module_child/1)
      
      # One-for-one strategy: if one module crashes, only restart that one
      Supervisor.init(children, strategy: :one_for_one)
    end
  end

  defp get_enabled_modules do
    modules = []
    
    # Check each module configuration
    modules = if Application.get_env(:metrics_agent, :demo)[:enabled], do: [:demo | modules], else: modules
    modules = if Application.get_env(:metrics_agent, :tasmota)[:enabled], do: [:tasmota | modules], else: modules
    modules = if Application.get_env(:metrics_agent, :opendtu)[:enabled], do: [:opendtu | modules], else: modules
    modules = if Application.get_env(:metrics_agent, :websocket)[:enabled], do: [:websocket | modules], else: modules
    
    modules
  end

  defp create_module_child(:demo) do
    {MetricsAgent.DemoModule, []}
  end

  defp create_module_child(:tasmota) do
    {MetricsAgent.TasmotaModule, []}
  end

  defp create_module_child(:opendtu) do
    {MetricsAgent.OpenDTUModule, []}
  end

  defp create_module_child(:websocket) do
    {MetricsAgent.WebSocketModule, []}
  end
end
