// Package ingest provides a registry system for metric collection modules.
// It allows dynamic registration and execution of different metric collection
// modules through a unified interface.
//
// The package supports:
// - Module registration and discovery
// - Unified module execution interface
// - Panic recovery for module execution
// - Configurable module support
package ingest

import (
	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// SourceCreatorFunc represents a function that creates a metric collection source.
// It receives a configuration map and returns a Source instance.
type SourceCreatorFunc func(config map[string]any, store *utils.Store) types.Ingester

// Registry holds all available metric collection modules.
// It provides thread-safe access to registered modules and their execution.
type Registry struct {
	modules map[string]SourceCreatorFunc
}

// NewRegistry creates a new module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]SourceCreatorFunc),
	}
}

// Register adds a module to the registry.
// If a module with the same name already exists, it will be overwritten.
func (r *Registry) Register(name string, fn SourceCreatorFunc) {
	r.modules[name] = fn
}

// GetRegisteredSources returns all registered sources from the registry.
func (r *Registry) GetRegisteredSources() map[string]SourceCreatorFunc {
	return r.modules

}
