defmodule MetricsAgentTest do
  use ExUnit.Case
  doctest MetricsAgent

  test "greets the world" do
    assert MetricsAgent.hello() == :world
  end
end
