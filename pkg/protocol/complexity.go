package protocol

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// CheckAgentComplexity returns complexity errors and warnings for agent scope.
//
// Hard errors (block execution):
//   - V047_TRIVIAL_SCOPE: IMPL is SUITABLE but has only 1 agent owning 1 file.
//     SAW adds no parallelization value; the change should be made directly.
//
// Warnings (advisory):
//   - W001_AGENT_SCOPE_LARGE: agent owns >8 files or creates >5 new files.
func CheckAgentComplexity(m *IMPLManifest) []result.SAWError {
	var warnings []result.SAWError

	// V047: Reject trivial single-agent, single-file IMPLs declared SUITABLE.
	// The suitability gate is LLM-driven; this catch ensures small-scope work
	// doesn't incur full SAW orchestration overhead for zero parallelism benefit.
	if m.Verdict == "SUITABLE" || m.Verdict == "SUITABLE_WITH_CAVEATS" {
		totalAgents := 0
		for _, wave := range m.Waves {
			totalAgents += len(wave.Agents)
		}
		if totalAgents == 1 && len(m.FileOwnership) == 1 {
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeTrivialScope,
				Message:  "IMPL has 1 agent owning 1 file — SAW adds no parallelization value at this scope; make the change directly instead",
				Severity: "error",
				Field:    "file_ownership",
			})
		}
	}

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
				Code:     result.CodeAgentScopeLarge,
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
				Code:     result.CodeAgentScopeLarge,
				Message:  fmt.Sprintf("agent %s creates %d new files (threshold: 5) — consider splitting into two agents", agentID, newCount),
				Severity: "warning",
				Field:    "file_ownership",
			})
		}
	}

	return warnings
}
