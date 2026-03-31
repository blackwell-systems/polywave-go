package orchestrator

import "log/slog"

// setters.go previously contained SetParseIMPLDocFunc which injected the IMPL
// doc parser. Now that orchestrator.New() calls protocol.Load(ctx, ...) directly, this
// setter is no longer needed. The file is retained for backward compatibility
// of the package structure.

// SetLogger sets the logger used by all Orchestrator methods.
// Nil resets to slog.Default() fallback.
func (o *Orchestrator) SetLogger(logger *slog.Logger) {
	o.logger = logger
}
