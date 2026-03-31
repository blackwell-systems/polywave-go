package protocol

import (
	"context"
	"log/slog"
)

// MergeAgentsOpts holds parameters for MergeAgents.
type MergeAgentsOpts struct {
	Ctx          context.Context // required; pass cmd.Context() or parent ctx
	ManifestPath string
	WaveNum      int
	RepoDir      string
	MergeTarget  string
	Logger       *slog.Logger
}
