package protocol

import "fmt"

// PredictConflictsFromReports cross-references completion reports for all agents
// in the given wave to detect files that appear in more than one agent's report.
//
// This implements E11: conflict prediction before merge. Any non-IMPL file
// (files outside .saw-state/ or docs/IMPL/) that appears in multiple agents'
// files_changed or files_created lists is flagged as a conflict risk.
//
// Returns a descriptive error if any conflict is detected, nil otherwise.
func PredictConflictsFromReports(manifest *IMPLManifest, waveNum int) error {
	if manifest == nil {
		return nil
	}

	// Find the target wave.
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}
	if targetWave == nil {
		return nil
	}

	// Build map: file -> list of agent IDs that reported touching it.
	fileAgents := make(map[string][]string)
	for _, agent := range targetWave.Agents {
		report, ok := manifest.CompletionReports[agent.ID]
		if !ok {
			continue
		}
		seen := make(map[string]bool)
		allFiles := append([]string{}, report.FilesChanged...)
		allFiles = append(allFiles, report.FilesCreated...)
		for _, f := range allFiles {
			if f == "" || isIMPLStateFile(f) {
				continue
			}
			if !seen[f] {
				seen[f] = true
				fileAgents[f] = append(fileAgents[f], agent.ID)
			}
		}
	}

	// Collect conflicts: any file touched by 2+ agents.
	var conflicts []string
	for file, agents := range fileAgents {
		if len(agents) > 1 {
			conflicts = append(conflicts, fmt.Sprintf("  %s (agents: %v)", file, agents))
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	return fmt.Errorf("E11 conflict prediction: %d file(s) appear in multiple agent reports (merge conflict risk):\n%s",
		len(conflicts), joinLines(conflicts))
}

// isIMPLStateFile returns true for IMPL doc paths and .saw-state/ files, which
// are expected to be modified by multiple agents and do not cause merge conflicts.
func isIMPLStateFile(path string) bool {
	// Allow multiple agents to touch IMPL docs and state directories.
	return hasPathPrefix(path, "docs/IMPL/") ||
		hasPathPrefix(path, ".saw-state/") ||
		hasPathPrefix(path, "docs/IMPL")
}

// hasPathPrefix returns true if path starts with prefix (allowing for leading slash variation).
func hasPathPrefix(path, prefix string) bool {
	if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
		return true
	}
	// Handle leading slash
	if len(path) > 0 && path[0] == '/' {
		return hasPathPrefix(path[1:], prefix)
	}
	return false
}

// joinLines joins a slice of strings into a newline-separated string.
func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
