package protocol

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// MergeStatus represents the outcome of merging a single agent branch.
type MergeStatus struct {
	Agent         string `json:"agent"`
	Branch        string `json:"branch"`
	Success       bool   `json:"success"`
	StatusUpdated bool   `json:"status_updated,omitempty"`
	Error         string `json:"error,omitempty"`
}

// MergeAgentsResult represents the outcome of merging all agents in a wave.
type MergeAgentsResult struct {
	Wave      int           `json:"wave"`
	Merges    []MergeStatus `json:"merges"`
	Success   bool          `json:"success"`
	NextState ProtocolState `json:"next_state,omitempty"` // State after successful merge
}

// MergeAgents merges all agent branches from a specified wave into the main branch.
// It loads the manifest, finds the wave, and performs non-fast-forward merges for each agent.
//
// The function stops on the first merge conflict and returns a partial result with Success=false.
// All merges are recorded in the result, including both successful and failed attempts.
//
// Parameters:
//   - manifestPath: path to the IMPL manifest file
//   - waveNum: wave number to merge
//   - repoDir: repository directory where merges will be performed
//
// Returns:
//   - MergeAgentsResult with wave number, merge statuses, and overall success flag
//   - error if manifest cannot be loaded or wave is not found (not returned for merge conflicts)
func MergeAgents(manifestPath string, waveNum int, repoDir string) (*MergeAgentsResult, error) {
	// Load manifest
	manifest, err := Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find the specified wave
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}

	if targetWave == nil {
		return nil, fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	// Initialize result
	result := &MergeAgentsResult{
		Wave:    waveNum,
		Merges:  make([]MergeStatus, 0, len(targetWave.Agents)),
		Success: true,
	}

	// Merge each agent branch
	for _, agent := range targetWave.Agents {
		branch := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)

		// Build commit message prefix
		prefix := fmt.Sprintf("Merge wave%d-agent-%s: ", waveNum, agent.ID)

		// Truncate task to ensure total message fits in reasonable length
		// Limit task portion to 50 chars
		taskSummary := agent.Task
		if len(taskSummary) > 50 {
			taskSummary = taskSummary[:50]
		}
		message := prefix + taskSummary

		// Perform merge
		status := MergeStatus{
			Agent:  agent.ID,
			Branch: branch,
		}

		err := git.MergeNoFF(repoDir, branch, message)
		if err != nil {
			// Merge failed - record error and stop
			status.Success = false
			status.Error = err.Error()
			result.Merges = append(result.Merges, status)
			result.Success = false
			return result, nil
		}

		// Merge succeeded — auto-update completion status (best-effort)
		status.Success = true
		if _, err := UpdateStatus(manifestPath, waveNum, agent.ID, "complete"); err == nil {
			status.StatusUpdated = true
		}
		result.Merges = append(result.Merges, status)
	}

	// All merges succeeded - update state to next wave or COMPLETE
	nextState := determineNextState(manifest, waveNum)
	if err := updateManifestState(manifestPath, nextState); err == nil {
		result.NextState = nextState
	}

	return result, nil
}

// determineNextState calculates the next protocol state after a successful wave merge.
func determineNextState(manifest *IMPLManifest, completedWave int) ProtocolState {
	// If this was the final wave, mark complete
	if completedWave >= len(manifest.Waves) {
		return StateComplete
	}

	// Otherwise, next wave is pending
	return StateWavePending // Orchestrator will update to WAVE{N}_PENDING format
}

// updateManifestState updates the state field in the IMPL manifest file.
// Uses line-based editing to preserve formatting (same pattern as mark-complete).
func updateManifestState(manifestPath string, newState ProtocolState) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	stateUpdated := false

	for i, line := range lines {
		if strings.HasPrefix(line, "state:") {
			lines[i] = fmt.Sprintf("state: \"%s\"", newState)
			stateUpdated = true
			break
		}
	}

	if !stateUpdated {
		return fmt.Errorf("state field not found in manifest")
	}

	// Write back
	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(manifestPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}
