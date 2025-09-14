package modules

import (
	"context"
	"fmt"

	"github.com/janhuddel/metrics-agent/internal/metrics"
)

// ModuleFunc represents a function that runs a metric collection module.
// It receives a context for cancellation and a channel to send metrics.
type ModuleFunc func(ctx context.Context, ch chan<- metrics.Metric) error

// ConfigurableModule represents a module that can be configured.
// Modules implementing this interface can receive configuration data.
type ConfigurableModule interface {
	// Configure is called before Run to set up the module with configuration.
	Configure(config interface{}) error
	// Run starts the module with the provided context and metrics channel.
	Run(ctx context.Context, ch chan<- metrics.Metric) error
}

// Registry holds all available metric collection modules.
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
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	return names
}

// Run executes a module by name.
// It retrieves the module function and runs it with the provided context and channel.
// Panics are recovered and returned as errors.
func (r *Registry) Run(ctx context.Context, name string, ch chan<- metrics.Metric) error {
	fn, err := r.Get(name)
	if err != nil {
		return err
	}

	// Execute module with panic recovery
	defer func() {
		if r := recover(); r != nil {
			// Panic is handled by the caller (supervisor or worker)
			panic(r)
		}
	}()

	return fn(ctx, ch)
}
