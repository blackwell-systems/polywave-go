package protocol

import "log/slog"

// MergeAgentsOpts holds parameters for MergeAgents.
type MergeAgentsOpts struct {
	ManifestPath string
	WaveNum      int
	RepoDir      string
	MergeTarget  string
	Logger       *slog.Logger
}
