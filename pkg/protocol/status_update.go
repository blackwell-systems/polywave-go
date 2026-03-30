package protocol

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// UpdateStatusData contains the data of a status update operation.
type UpdateStatusData struct {
	Wave      int              `json:"wave"`
	Agent     string           `json:"agent"`
	OldStatus CompletionStatus `json:"old_status"`
	NewStatus CompletionStatus `json:"new_status"`
	Updated   bool             `json:"updated"`
}

// UpdateStatus loads the manifest, finds the specified agent, updates its completion report status,
// and saves the manifest back to disk. Returns the old and new status values.
// Returns a failure if the agent is not found in the specified wave.
func UpdateStatus(manifestPath string, waveNum int, agentID string, status CompletionStatus) result.Result[*UpdateStatusData] {
	// Load manifest
	manifest, err := Load(manifestPath)
	if err != nil {
		return result.NewFailure[*UpdateStatusData]([]result.SAWError{{
			Code:     result.CodeStatusUpdateFailed,
			Message:  fmt.Sprintf("failed to load manifest: %v", err),
			Severity: "fatal",
		}})
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
		return result.NewFailure[*UpdateStatusData]([]result.SAWError{{
			Code:     result.CodeStatusUpdateFailed,
			Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
			Severity: "fatal",
		}})
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
		return result.NewFailure[*UpdateStatusData]([]result.SAWError{{
			Code:     result.CodeStatusUpdateFailed,
			Message:  fmt.Sprintf("agent %s not found in wave %d", agentID, waveNum),
			Severity: "fatal",
		}})
	}

	// Initialize CompletionReports map if nil
	if manifest.CompletionReports == nil {
		manifest.CompletionReports = make(map[string]CompletionReport)
	}

	// Get old status (empty string if no report exists yet)
	oldStatus := CompletionStatus("")
	if existingReport, exists := manifest.CompletionReports[agentID]; exists {
		oldStatus = existingReport.Status
	}

	// Update or create completion report
	report := manifest.CompletionReports[agentID]
	report.Status = status
	manifest.CompletionReports[agentID] = report

	// Save manifest
	if err := Save(manifest, manifestPath); err != nil {
		return result.NewFailure[*UpdateStatusData]([]result.SAWError{{
			Code:     result.CodeStatusUpdateFailed,
			Message:  fmt.Sprintf("failed to save manifest: %v", err),
			Severity: "fatal",
		}})
	}

	return result.NewSuccess(&UpdateStatusData{
		Wave:      waveNum,
		Agent:     agentID,
		OldStatus: oldStatus,
		NewStatus: status,
		Updated:   true,
	})
}
