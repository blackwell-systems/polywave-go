// Package config provides unified configuration loading and wave state queries
// for the scout-and-wave engine.
package config

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// WaveState consolidates all state for a wave from the IMPL manifest.
type WaveState struct {
	WaveNum         int      `json:"wave_num"`
	TotalAgents     int      `json:"total_agents"`
	CompletedAgents []string `json:"completed_agents"`
	FailedAgents    []string `json:"failed_agents"`
	PendingAgents   []string `json:"pending_agents"`
	IsComplete      bool     `json:"is_complete"`
	MergeState      string   `json:"merge_state,omitempty"`
}

// getWaveState derives the complete state of a wave from the IMPL manifest.
// This is the single query function -- no .saw-state/ directory needed.
func getWaveState(manifest *protocol.IMPLManifest, waveNum int) result.Result[*WaveState] {
	if manifest == nil {
		return result.NewFailure[*WaveState]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "manifest is nil"),
		})
	}
	// Find the wave by number.
	var wave *protocol.Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			wave = &manifest.Waves[i]
			break
		}
	}
	if wave == nil {
		return result.NewFailure[*WaveState]([]result.SAWError{
			result.NewError(result.CodeWaveNotReady,
				fmt.Sprintf("wave %d not found in manifest", waveNum)),
		})
	}

	// Build agent ID list from wave definition.
	agentIDs := make([]string, len(wave.Agents))
	for i, a := range wave.Agents {
		agentIDs[i] = a.ID
	}

	// Classify agents based on completion reports.
	var completed, failed, pending []string
	for _, id := range agentIDs {
		reportKey := fmt.Sprintf("wave%d-%s", waveNum, id)
		report, ok := manifest.CompletionReports[reportKey]
		if !ok {
			pending = append(pending, id)
			continue
		}
		switch report.Status {
		case protocol.StatusBlocked:
			failed = append(failed, id)
		case protocol.StatusPartial:
			failed = append(failed, id)
		case protocol.StatusComplete:
			completed = append(completed, id)
		default:
			// Unknown status treated as pending.
			pending = append(pending, id)
		}
	}

	// Ensure slices are non-nil for consistent JSON output.
	if completed == nil {
		completed = []string{}
	}
	if failed == nil {
		failed = []string{}
	}
	if pending == nil {
		pending = []string{}
	}

	isComplete := len(completed) == len(agentIDs) && len(failed) == 0

	ws := &WaveState{
		WaveNum:         waveNum,
		TotalAgents:     len(agentIDs),
		CompletedAgents: completed,
		FailedAgents:    failed,
		PendingAgents:   pending,
		IsComplete:      isComplete,
		MergeState:      string(manifest.MergeState),
	}

	return result.NewSuccess(ws)
}

// getAllWaveStates derives the state of all waves from the IMPL manifest.
func getAllWaveStates(manifest *protocol.IMPLManifest) result.Result[[]WaveState] {
	if manifest == nil {
		return result.NewFailure[[]WaveState]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "manifest is nil"),
		})
	}
	states := make([]WaveState, 0, len(manifest.Waves))
	for _, w := range manifest.Waves {
		r := getWaveState(manifest, w.Number)
		if r.IsFatal() {
			wrapped := make([]result.SAWError, len(r.Errors))
			for i, e := range r.Errors {
				wrapped[i] = result.NewError(e.Code,
					fmt.Sprintf("wave %d: %s", w.Number, e.Message))
			}
			return result.NewFailure[[]WaveState](wrapped)
		}
		states = append(states, *r.GetData())
	}
	return result.NewSuccess(states)
}
