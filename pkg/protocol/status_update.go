package protocol

import (
	"fmt"
)

// UpdateStatusResult contains the result of a status update operation.
type UpdateStatusResult struct {
	Wave      int    `json:"wave"`
	Agent     string `json:"agent"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Updated   bool   `json:"updated"`
}

// UpdateStatus loads the manifest, finds the specified agent, updates its completion report status,
// and saves the manifest back to disk. Returns the old and new status values.
// Returns an error if the agent is not found in the specified wave.
func UpdateStatus(manifestPath string, waveNum int, agentID string, status string) (*UpdateStatusResult, error) {
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

	// Find the agent in that wave
	found := false
	for _, agent := range targetWave.Agents {
		if agent.ID == agentID {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("agent %s not found in wave %d", agentID, waveNum)
	}

	// Initialize CompletionReports map if nil
	if manifest.CompletionReports == nil {
		manifest.CompletionReports = make(map[string]CompletionReport)
	}

	// Get old status (empty string if no report exists yet)
	oldStatus := ""
	if existingReport, exists := manifest.CompletionReports[agentID]; exists {
		oldStatus = existingReport.Status
	}

	// Update or create completion report
	report := manifest.CompletionReports[agentID]
	report.Status = status
	manifest.CompletionReports[agentID] = report

	// Save manifest
	if err := Save(manifest, manifestPath); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	return &UpdateStatusResult{
		Wave:      waveNum,
		Agent:     agentID,
		OldStatus: oldStatus,
		NewStatus: status,
		Updated:   true,
	}, nil
}
