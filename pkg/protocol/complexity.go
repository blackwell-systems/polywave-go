package protocol

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// CheckAgentComplexity returns W001_AGENT_SCOPE_LARGE warnings for any agent
// that owns more than 8 files total, or creates more than 5 new files.
// Warnings are advisory — they do not block execution.
func CheckAgentComplexity(m *IMPLManifest) []result.SAWError {
	var warnings []result.SAWError

	// Build per-agent file count maps from file_ownership
	totalFiles := make(map[string]int)
	newFiles := make(map[string]int)

	for _, fo := range m.FileOwnership {
		totalFiles[fo.Agent]++
		if fo.Action == "new" {
			newFiles[fo.Agent]++
		}
	}

	// Warn for any agent with total files > 8
	for agentID, count := range totalFiles {
		if count > 8 {
			warnings = append(warnings, result.SAWError{
				// TODO: replace with result.CodeAgentScopeLarge after wave merge
				Code:     "W001_AGENT_SCOPE_LARGE",
				Message:  fmt.Sprintf("agent %s owns %d files (threshold: 8) — consider splitting into two agents", agentID, count),
				Severity: "warning",
				Field:    "file_ownership",
			})
		}
	}

	// Warn for any agent with new files > 5
	for agentID, newCount := range newFiles {
		if newCount > 5 {
			warnings = append(warnings, result.SAWError{
				// TODO: replace with result.CodeAgentScopeLarge after wave merge
				Code:     "W001_AGENT_SCOPE_LARGE",
				Message:  fmt.Sprintf("agent %s creates %d new files (threshold: 5) — consider splitting into two agents", agentID, newCount),
				Severity: "warning",
				Field:    "file_ownership",
			})
		}
	}

	return warnings
}
