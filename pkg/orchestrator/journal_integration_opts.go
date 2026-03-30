package orchestrator

import "log/slog"

// PrepareAgentContextOpts holds parameters for PrepareAgentContext.
type PrepareAgentContextOpts struct {
	ProjectRoot string
	WaveNum     int
	AgentID     string
	MaxEntries  int
	Logger      *slog.Logger
}
