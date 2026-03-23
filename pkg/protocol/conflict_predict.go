package protocol

import (
	"crypto/sha256"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

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
	// Enhancement: Compare file content hashes - identical edits are allowed.
	var conflicts []string
	for file, agents := range fileAgents {
		if len(agents) > 1 {
			// Check if all agents produced identical content
			if manifest.FeatureSlug != "" && allAgentsHaveSameContent(manifest, file, agents, waveNum) {
				// Identical edits - safe to merge, skip conflict
				continue
			}
			conflicts = append(conflicts, fmt.Sprintf("  %s has differing edits (agents: %v)", file, agents))
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

// allAgentsHaveSameContent checks if all agents produced identical file content.
// Returns true if all hashes match (safe to merge), false if any differ or on error.
func allAgentsHaveSameContent(manifest *IMPLManifest, file string, agents []string, waveNum int) bool {
	if len(agents) <= 1 {
		return true // No conflict with single agent
	}

	var hashes []string
	for _, agentID := range agents {
		branchName := fmt.Sprintf("saw/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, agentID)
		hash, err := computeFileHashInBranch(manifest.Repository, branchName, file)
		if err != nil {
			// Hash computation failed - fall back to blocking (safe default)
			return false
		}
		hashes = append(hashes, hash)
	}

	// Check if all hashes are identical
	firstHash := hashes[0]
	for _, h := range hashes[1:] {
		if h != firstHash {
			return false // Differing content
		}
	}

	return true // All hashes match - identical edits
}

// computeFileHashInBranch reads file content from a git branch and returns SHA256 hash.
// Uses "git show branch:file" to read content without checking out the branch.
func computeFileHashInBranch(repoPath, branchName, relFile string) (string, error) {
	// Use git show to read file content from branch
	content, err := git.Run(repoPath, "show", fmt.Sprintf("%s:%s", branchName, relFile))
	if err != nil {
		return "", fmt.Errorf("failed to read %s from branch %s: %w", relFile, branchName, err)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash), nil
}
