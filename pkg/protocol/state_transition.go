package protocol

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SetImplStateOpts contains options for the SetImplState operation.
type SetImplStateOpts struct {
	Commit    bool   // if true, git commit after state write
	CommitMsg string // optional commit message; default generated if empty
}

// SetImplStateData contains the outcome of a SetImplState call.
type SetImplStateData struct {
	PreviousState ProtocolState `json:"previous_state"`
	NewState      ProtocolState `json:"new_state"`
	Committed     bool          `json:"committed"`
	CommitSHA     string        `json:"commit_sha,omitempty"`
}

// allowedTransitions maps each state to the set of valid next states.
var allowedTransitions = map[ProtocolState][]ProtocolState{
	StateInterviewing:    {StateScoutPending},
	StateScoutPending:    {StateScoutValidating, StateReviewed, StateNotSuitable, StateBlocked},
	StateScoutValidating: {StateScoutValidating, StateReviewed, StateNotSuitable, StateBlocked},
	// StateReviewed allows direct transition to COMPLETE for close-impl
	// without wave execution (e.g., NOT_SUITABLE after review, or manual closure)
	StateReviewed:        {StateScaffoldPending, StateWavePending, StateWaveExecuting, StateBlocked, StateComplete},
	StateScaffoldPending: {StateWavePending, StateWaveExecuting, StateBlocked},
	StateWavePending:     {StateWaveExecuting, StateBlocked},
	StateWaveExecuting:   {StateWaveMerging, StateWaveVerified, StateBlocked, StateComplete},
	StateWaveMerging:     {StateWaveVerified, StateBlocked},
	StateWaveVerified:    {StateWavePending, StateWaveExecuting, StateComplete, StateBlocked},
	// StateBlocked can recover to any non-terminal state, or to terminal states
	// when the operator manually closes a blocked IMPL.
	StateBlocked: {
		StateScoutPending, StateScoutValidating, StateReviewed,
		StateScaffoldPending, StateWavePending, StateWaveExecuting,
		StateWaveMerging, StateWaveVerified, StateComplete, StateNotSuitable,
	},
	StateComplete:    {},
	StateNotSuitable: {},
}

// IsValidTransition reports whether transitioning from -> to is a valid
// protocol state machine transition per the canonical allowedTransitions map.
func IsValidTransition(from, to ProtocolState) bool {
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// SetImplState atomically reads the manifest, validates the state transition,
// writes the new state, and optionally commits to git.
func SetImplState(ctx context.Context, manifestPath string, newState ProtocolState, opts SetImplStateOpts) result.Result[*SetImplStateData] {
	manifest, err := Load(ctx, manifestPath)
	if err != nil {
		return result.NewFailure[*SetImplStateData]([]result.SAWError{{
			Code:     result.CodeStateTransition,
			Message:  fmt.Sprintf("failed to load manifest: %v", err),
			Severity: "fatal",
		}})
	}

	previousState := manifest.State

	// Validate transition
	allowed, ok := allowedTransitions[previousState]
	if !ok {
		return result.NewFailure[*SetImplStateData]([]result.SAWError{{
			Code:     result.CodeStateTransition,
			Message:  fmt.Sprintf("unknown state %q; cannot determine valid transitions", previousState),
			Severity: "fatal",
		}})
	}

	found := false
	for _, s := range allowed {
		if s == newState {
			found = true
			break
		}
	}

	if !found {
		validTargets := make([]string, len(allowed))
		for i, s := range allowed {
			validTargets[i] = string(s)
		}
		return result.NewFailure[*SetImplStateData]([]result.SAWError{{
			Code:     result.CodeStateTransition,
			Message:  fmt.Sprintf("transition from %s to %s is not allowed; valid targets: [%s]", previousState, newState, strings.Join(validTargets, ", ")),
			Severity: "fatal",
		}})
	}

	// Advisory check: warn about potentially dangerous transitions.
	_ = ValidateStateTransitionContext(manifest, previousState, newState)

	// Write the new state atomically
	if err := updateManifestState(manifestPath, newState); err != nil {
		return result.NewFailure[*SetImplStateData]([]result.SAWError{{
			Code:     result.CodeStateTransition,
			Message:  fmt.Sprintf("failed to update manifest state: %v", err),
			Severity: "fatal",
		}})
	}

	data := &SetImplStateData{
		PreviousState: previousState,
		NewState:      newState,
	}

	// Optionally commit to git
	if opts.Commit {
		manifestDir := filepath.Dir(manifestPath)

		commitMsg := opts.CommitMsg
		if commitMsg == "" {
			commitMsg = fmt.Sprintf("chore: set IMPL state to %s", newState)
		}

		if err := git.Add(manifestDir, filepath.Base(manifestPath)); err != nil {
			return result.NewFailure[*SetImplStateData]([]result.SAWError{{
				Code:     result.CodeStateTransition,
				Message:  fmt.Sprintf("git add failed: %v", err),
				Severity: "fatal",
			}})
		}

		sha, err := git.CommitWithMessage(manifestDir, commitMsg)
		if err != nil {
			return result.NewFailure[*SetImplStateData]([]result.SAWError{{
				Code:     result.CodeStateTransition,
				Message:  fmt.Sprintf("git commit failed: %v", err),
				Severity: "fatal",
			}})
		}

		data.Committed = true
		data.CommitSHA = sha
	}

	return result.NewSuccess(data)
}

// ValidateStateTransitionContext checks whether a state transition is
// semantically appropriate given the manifest context. Returns warning-level
// SAWErrors for transitions that are technically allowed but potentially
// dangerous (e.g., WAVE_EXECUTING -> COMPLETE with multiple agents).
func ValidateStateTransitionContext(manifest *IMPLManifest, from, to ProtocolState) []result.SAWError {
	var warnings []result.SAWError

	if from == StateWaveExecuting && to == StateComplete {
		var currentWave *Wave
		for i := range manifest.Waves {
			w := &manifest.Waves[i]
			if len(w.Agents) > 0 {
				if currentWave == nil || w.Number > currentWave.Number {
					currentWave = w
				}
			}
		}

		if currentWave != nil && !IsSoloWave(currentWave) {
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeStateTransition,
				Message:  fmt.Sprintf("WAVE_EXECUTING -> COMPLETE bypasses merge/verify for wave %d with %d agents; consider WAVE_MERGING -> WAVE_VERIFIED -> COMPLETE path", currentWave.Number, len(currentWave.Agents)),
				Severity: "warning",
			})
		}
	}

	return warnings
}
