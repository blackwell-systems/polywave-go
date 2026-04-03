package pipeline

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// KeyData carries the result of a required-key lookup.
type KeyData struct {
	Key   string
	Found bool
}

// requiredKey returns result.Result[KeyData] indicating whether key is present
// and non-nil in state.Values.
//
// Returns:
//   - SUCCESS: key is present and non-nil
//   - FATAL (REQUIRED_KEY_MISSING): key is absent, state.Values is nil, or value is nil
func requiredKey(state *State, key string) result.Result[KeyData] {
	v, ok := GetValue[any](state, key)
	if !ok || v == nil {
		return result.NewFailure[KeyData]([]result.SAWError{
			result.NewFatal(result.CodeRequiredKeyMissing,
				fmt.Sprintf("required state key %q not set", key)),
		})
	}
	return result.NewSuccess(KeyData{Key: key, Found: true})
}

// requiredKeyErr is a convenience wrapper that converts requiredKey's Result
// into a plain error for use inside StepFunc bodies (which must return error).
func requiredKeyErr(state *State, key string) error {
	r := requiredKey(state, key)
	if r.IsFatal() {
		return r.Errors[0]
	}
	return nil
}

// StepValidateInvariants returns a Step that checks pre-conditions before a wave run.
// Required state keys: "repo_path", "impl_path".
// Error strategy: fail immediately.
func StepValidateInvariants() Step {
	return Step{
		Name:          "validate_invariants",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			if err := requiredKeyErr(state, "repo_path"); err != nil {
				return fmt.Errorf("validate_invariants: %w", err)
			}
			if err := requiredKeyErr(state, "impl_path"); err != nil {
				return fmt.Errorf("validate_invariants: %w", err)
			}
			return nil
		},
	}
}

// StepCreateWorktrees returns a Step that sets up per-agent worktrees.
// Required state keys: "repo_path".
// Error strategy: fail immediately.
func StepCreateWorktrees() Step {
	return Step{
		Name:          "create_worktrees",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			if err := requiredKeyErr(state, "repo_path"); err != nil {
				return fmt.Errorf("create_worktrees: %w", err)
			}
			// Real implementation wired via registry by the engine layer.
			return nil
		},
	}
}

// StepRunQualityGates returns a Step that runs all configured quality gates.
// Required state keys: "repo_path".
// Error strategy: fail immediately (quality gates are mandatory).
func StepRunQualityGates() Step {
	return Step{
		Name:          "run_quality_gates",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			if err := requiredKeyErr(state, "repo_path"); err != nil {
				return fmt.Errorf("run_quality_gates: %w", err)
			}
			// Real implementation wired via registry by the engine layer.
			return nil
		},
	}
}

// StepMergeAgents returns a Step that merges all agent worktrees into the
// integration branch.
// Required state keys: "repo_path".
// Error strategy: fail immediately.
func StepMergeAgents() Step {
	return Step{
		Name:          "merge_agents",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			if err := requiredKeyErr(state, "repo_path"); err != nil {
				return fmt.Errorf("merge_agents: %w", err)
			}
			// Real implementation wired via registry by the engine layer.
			return nil
		},
	}
}

// StepVerifyBuild returns a Step that verifies the repository builds cleanly
// after all merges.
// Required state keys: "repo_path".
// Error strategy: fail immediately.
func StepVerifyBuild() Step {
	return Step{
		Name:          "verify_build",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			if err := requiredKeyErr(state, "repo_path"); err != nil {
				return fmt.Errorf("verify_build: %w", err)
			}
			// Real implementation wired via registry by the engine layer.
			return nil
		},
	}
}

// StepCleanup returns a Step that removes temporary worktrees and artefacts.
// Required state keys: "repo_path".
// Error strategy: continue (cleanup is best-effort; failure must not abort the run).
func StepCleanup() Step {
	return Step{
		Name:          "cleanup",
		ErrorStrategy: ErrorContinue,
		Func: func(ctx context.Context, state *State) error {
			if err := requiredKeyErr(state, "repo_path"); err != nil {
				return fmt.Errorf("cleanup: %w", err)
			}
			// Real implementation wired via registry by the engine layer.
			return nil
		},
	}
}

// WavePipeline builds the standard wave execution pipeline for the given wave number.
// The pipeline chains all SAW steps in the canonical order:
//
//	validate_invariants → create_worktrees → run_quality_gates →
//	merge_agents → verify_build → cleanup
func WavePipeline(waveNum int) *Pipeline {
	name := fmt.Sprintf("wave-%d", waveNum)
	p := New(name)
	p.AddStep(StepValidateInvariants())
	p.AddStep(StepCreateWorktrees())
	p.AddStep(StepRunQualityGates())
	p.AddStep(StepMergeAgents())
	p.AddStep(StepVerifyBuild())
	p.AddStep(StepCleanup())
	return p
}
