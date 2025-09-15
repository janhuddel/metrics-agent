// Package modules provides a registry system for metric collection modules.
// It allows dynamic registration and execution of different metric collection
// modules through a unified interface.
package modules

import (
	"github.com/janhuddel/metrics-agent/internal/modules/demo"
	"github.com/janhuddel/metrics-agent/internal/modules/tasmota"
)

// Global is the global registry instance used throughout the application.
var Global = NewRegistry()

func init() {
	// Register all available modules
	Global.Register("demo", demo.Run)
	Global.Register("tasmota", tasmota.Run)
}
