package main

import (
	"log/slog"
	"os"
)

// newPolywaveLogger constructs the slog.Logger for CLI commands.
// Level is controlled by POLYWAVE_LOG_LEVEL env var (default: WARN).
func newPolywaveLogger() *slog.Logger {
	level := slog.LevelWarn
	if v := os.Getenv("POLYWAVE_LOG_LEVEL"); v != "" {
		var l slog.Level
		if err := l.UnmarshalText([]byte(v)); err == nil {
			level = l
		}
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
