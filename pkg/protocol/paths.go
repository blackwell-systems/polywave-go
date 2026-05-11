package protocol

import (
	"fmt"
	"path/filepath"
)

// IMPLDir returns the path to the docs/IMPL directory.
func IMPLDir(repoPath string) string {
	return filepath.Join(repoPath, "docs", "IMPL")
}

// IMPLPath returns the canonical path for an active IMPL doc.
// Format: {repoPath}/docs/IMPL/IMPL-{slug}.yaml
func IMPLPath(repoPath, slug string) string {
	return filepath.Join(repoPath, "docs", "IMPL", fmt.Sprintf("IMPL-%s.yaml", slug))
}

// IMPLCompleteDir returns the path to the docs/IMPL/complete directory.
func IMPLCompleteDir(repoPath string) string {
	return filepath.Join(repoPath, "docs", "IMPL", "complete")
}

// IMPLCompletePath returns the canonical path for a completed IMPL doc.
// Format: {repoPath}/docs/IMPL/complete/IMPL-{slug}.yaml
func IMPLCompletePath(repoPath, slug string) string {
	return filepath.Join(repoPath, "docs", "IMPL", "complete", fmt.Sprintf("IMPL-%s.yaml", slug))
}

// IMPLQueueDir returns the path to the docs/IMPL/queue directory.
func IMPLQueueDir(repoPath string) string {
	return filepath.Join(repoPath, "docs", "IMPL", "queue")
}

// ContextMDPath returns the canonical path for docs/CONTEXT.md.
func ContextMDPath(repoPath string) string {
	return filepath.Join(repoPath, "docs", "CONTEXT.md")
}

// PolywaveStateDir returns the path to the .polywave-state directory.
func PolywaveStateDir(repoPath string) string {
	return filepath.Join(repoPath, ".polywave-state")
}

// PolywaveStateArchiveDir returns the path to the .polywave-state/archive directory.
func PolywaveStateArchiveDir(repoPath string) string {
	return filepath.Join(repoPath, ".polywave-state", "archive")
}

// PolywaveStateAgentDir returns the path to the per-agent state directory.
// Format: {repoPath}/.polywave-state/wave{N}/{agentID}
func PolywaveStateAgentDir(repoPath string, waveNum int, agentID string) string {
	return filepath.Join(repoPath, ".polywave-state", fmt.Sprintf("wave%d", waveNum), agentID)
}
