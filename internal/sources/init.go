package sources

import "github.com/janhuddel/metrics-agent/internal/sources/dummy"

// SourceRegistry is the global registry instance used throughout the application.
// It contains all registered metric collection modules.
var SourceRegistry = NewRegistry()

func init() {
	// Register all available modules
	SourceRegistry.Register("dummy", dummy.CreateInstance)
}
