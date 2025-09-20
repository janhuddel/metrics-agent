package sources

import (
	"log/slog"

	"github.com/janhuddel/metrics-agent/internal/sources/dummy"
	"github.com/janhuddel/metrics-agent/internal/types"
	"github.com/janhuddel/metrics-agent/internal/utils"
)

var sourceRegistry = map[string]func(map[string]interface{}) types.Source{
	"dummy": func(config map[string]interface{}) types.Source { return dummy.New(config) },
}

func CreateSources(config *utils.AppConfig) []types.Source {
	var sources []types.Source
	for sourceName := range sourceRegistry {
		slog.Debug("source", "name", sourceName, "enabled", config.IsSourceEnabled(sourceName))
		if config.IsSourceEnabled(sourceName) {
			sourceConfig := config.GetSourceConfig(sourceName)
			source := sourceRegistry[sourceName](sourceConfig)
			slog.Debug("created source", "name", sourceName, "config", sourceConfig)
			sources = append(sources, source)
		}
	}
	return sources
}
