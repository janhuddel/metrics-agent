package utils

import (
	"log/slog"
	"os"
)

func InitLogger(config *AppConfig) *slog.Logger {
	var level slog.Level

	switch config.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	return logger
}
