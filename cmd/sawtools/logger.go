package main

import (
	"log/slog"
	"os"
)

// newSawLogger constructs the slog.Logger for CLI commands.
// Level is controlled by SAW_LOG_LEVEL env var (default: WARN).
func newSawLogger() *slog.Logger {
	level := slog.LevelWarn
	if v := os.Getenv("SAW_LOG_LEVEL"); v != "" {
		var l slog.Level
		if err := l.UnmarshalText([]byte(v)); err == nil {
			level = l
		}
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
