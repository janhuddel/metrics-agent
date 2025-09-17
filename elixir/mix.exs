defmodule MetricsAgent.MixProject do
  use Mix.Project

  def project do
    [
      app: :metrics_agent,
      version: "0.1.0",
      elixir: "~> 1.14",
      start_permanent: Mix.env() == :prod,
      deps: deps(),
      elixirc_paths: elixirc_paths(Mix.env()),
      test_paths: ["test"],
      test_pattern: "*_test.exs"
    ]
  end

  def application do
    [
      extra_applications: [:logger, :crypto],
      mod: {MetricsAgent.Application, []}
    ]
  end

  defp deps do
    [
      # MQTT client
      {:tortoise, "~> 0.12"},
      
      # JSON handling
      {:jason, "~> 1.4"},
      
      # HTTP client for OpenDTU
      {:req, "~> 0.4"},
      
      # WebSocket client
      {:websockex, "~> 0.4"},
      
      # Configuration
      {:config_tuples, "~> 0.5"},
      
      # Development and testing
      {:ex_doc, "~> 0.27", only: :dev, runtime: false},
      {:mix_test_watch, "~> 1.0", only: [:dev, :test], runtime: false}
    ]
  end

  defp elixirc_paths(:test), do: ["lib", "test/support"]
  defp elixirc_paths(_), do: ["lib"]
end
