package protocol

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// SetImplStateOpts contains options for the SetImplState operation.
type SetImplStateOpts struct {
	Commit    bool   // if true, git commit after state write
	CommitMsg string // optional commit message; default generated if empty
}

// SetImplStateResult contains the outcome of a SetImplState call.
type SetImplStateResult struct {
	PreviousState ProtocolState `json:"previous_state"`
	NewState      ProtocolState `json:"new_state"`
	Committed     bool          `json:"committed"`
	CommitSHA     string        `json:"commit_sha,omitempty"`
}

// allowedTransitions maps each state to the set of valid next states.
var allowedTransitions = map[ProtocolState][]ProtocolState{
	StateScoutPending:    {StateReviewed, StateNotSuitable},
	StateScoutValidating: {StateReviewed, StateNotSuitable},
	StateReviewed:        {StateScaffoldPending, StateWavePending, StateWaveExecuting, StateBlocked},
	StateScaffoldPending: {StateWavePending, StateWaveExecuting, StateBlocked},
	StateWavePending:     {StateWaveExecuting, StateBlocked},
	StateWaveExecuting:   {StateWaveMerging, StateWaveVerified, StateBlocked, StateComplete},
	StateWaveMerging:     {StateWaveVerified, StateBlocked},
	StateWaveVerified:    {StateWaveExecuting, StateComplete, StateBlocked},
	StateBlocked:         {StateReviewed, StateWaveExecuting, StateWavePending},
	StateComplete:        {},
	StateNotSuitable:     {},
}

// SetImplState atomically reads the manifest, validates the state transition,
// writes the new state, and optionally commits to git.
func SetImplState(manifestPath string, newState ProtocolState, opts SetImplStateOpts) (*SetImplStateResult, error) {
	manifest, err := Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	previousState := manifest.State

	// Validate transition
	allowed, ok := allowedTransitions[previousState]
	if !ok {
		return nil, fmt.Errorf("unknown state %q; cannot determine valid transitions", previousState)
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
		return nil, fmt.Errorf("transition from %s to %s is not allowed; valid targets: [%s]",
			previousState, newState, strings.Join(validTargets, ", "))
	}

	// Write the new state atomically
	if err := updateManifestState(manifestPath, newState); err != nil {
		return nil, fmt.Errorf("failed to update manifest state: %w", err)
	}

	result := &SetImplStateResult{
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

		addCmd := exec.Command("git", "-C", manifestDir, "add", manifestPath)
		if out, err := addCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git add failed: %w\n%s", err, out)
		}

		commitCmd := exec.Command("git", "-C", manifestDir, "commit", "-m", commitMsg)
		if out, err := commitCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git commit failed: %w\n%s", err, out)
		}

		shaCmd := exec.Command("git", "-C", manifestDir, "rev-parse", "HEAD")
		shaOut, err := shaCmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git rev-parse HEAD failed: %w", err)
		}

		result.Committed = true
		result.CommitSHA = strings.TrimSpace(string(shaOut))
	}

	return result, nil
}
