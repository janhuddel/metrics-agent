// Package modules provides a registry system for metric collection modules.
// It allows dynamic registration and execution of different metric collection
// modules through a unified interface.
//
// This file handles the initialization and registration of all available modules.
package modules

import (
	"github.com/janhuddel/metrics-agent/internal/modules/netatmo"
	"github.com/janhuddel/metrics-agent/internal/modules/opendtu"
	"github.com/janhuddel/metrics-agent/internal/modules/tasmota"
)

// Global is the global registry instance used throughout the application.
// It contains all registered metric collection modules.
var Global = NewRegistry()

func init() {
	// Register all available modules
	// Note: The demo module is commented out for production use
	//Global.Register("demo", demo.Run)
	Global.Register("tasmota", tasmota.Run)
	Global.Register("netatmo", netatmo.Run)
	Global.Register("opendtu", opendtu.Run)
}
