// Package modules provides a registry system for metric collection modules.
// It allows dynamic registration and execution of different metric collection
// modules through a unified interface.
//
// The package supports:
// - Module registration and discovery
// - Unified module execution interface
// - Panic recovery for module execution
// - Configurable module support
package modules

import (
	"context"
	"fmt"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

// ModuleFunc represents a function that runs a metric collection module.
// It receives a context for cancellation and a channel to send metrics.
// The function should run continuously until the context is cancelled.
type ModuleFunc func(ctx context.Context, ch chan<- metrics.Metric) error

// ConfigurableModule represents a module that can be configured.
// Modules implementing this interface can receive configuration data
// before being started.
type ConfigurableModule interface {
	// Configure is called before Run to set up the module with configuration.
	// It should validate the configuration and prepare the module for execution.
	Configure(config interface{}) error

	// Run starts the module with the provided context and metrics channel.
	// It should run continuously until the context is cancelled.
	Run(ctx context.Context, ch chan<- metrics.Metric) error
}

// Registry holds all available metric collection modules.
// It provides thread-safe access to registered modules and their execution.
type Registry struct {
	modules map[string]ModuleFunc
}

// NewRegistry creates a new module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]ModuleFunc),
	}
}

// Register adds a module to the registry.
// If a module with the same name already exists, it will be overwritten.
func (r *Registry) Register(name string, fn ModuleFunc) {
	r.modules[name] = fn
}

// Get retrieves a module function by name.
// Returns an error if the module is not found.
func (r *Registry) Get(name string) (ModuleFunc, error) {
	fn, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	return fn, nil
}

// List returns all registered module names.
// The order of names is not guaranteed as it depends on map iteration.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	return names
}

// Run executes a module by name.
// It retrieves the module function and runs it with the provided context and channel.
// Panics are recovered and returned as errors to ensure the application remains stable.
func (r *Registry) Run(ctx context.Context, name string, ch chan<- metrics.Metric) error {
	fn, err := r.Get(name)
	if err != nil {
		return err
	}

	// Execute module with panic recovery
	return utils.WithPanicRecoveryAndReturnError("Module execution", name, func() error {
		return fn(ctx, ch)
	})
}
