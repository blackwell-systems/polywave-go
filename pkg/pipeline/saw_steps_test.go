package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// stateWithKeys builds a State that has the given key→value pairs set.
func stateWithKeys(pairs ...string) *State {
	s := makeState()
	for i := 0; i+1 < len(pairs); i += 2 {
		s.Values[pairs[i]] = pairs[i+1]
	}
	return s
}

// TestRequiredKey_Missing verifies that requiredKey returns a FATAL result
// when the key is absent from state.Values.
func TestRequiredKey_Missing(t *testing.T) {
	r := requiredKey(makeState(), "missing_key")
	if !r.IsFatal() {
		t.Fatal("expected FATAL result for missing key")
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error in FATAL result")
	}
	if r.Errors[0].Code != result.CodeRequiredKeyMissing {
		t.Errorf("expected code %q, got %q", result.CodeRequiredKeyMissing, r.Errors[0].Code)
	}
}

// TestRequiredKey_Present verifies that requiredKey returns a SUCCESS result
// when the key exists and is non-nil.
func TestRequiredKey_Present(t *testing.T) {
	s := stateWithKeys("my_key", "some_value")
	r := requiredKey(s, "my_key")
	if !r.IsSuccess() {
		t.Fatalf("expected SUCCESS result, got code=%s errors=%v", r.Code, r.Errors)
	}
	data := r.GetData()
	if data.Key != "my_key" {
		t.Errorf("expected Key=%q, got %q", "my_key", data.Key)
	}
	if !data.Found {
		t.Error("expected Found=true")
	}
}

// TestRequiredKey_NilValues verifies that requiredKey returns FATAL when
// state.Values is nil.
func TestRequiredKey_NilValues(t *testing.T) {
	s := &State{} // Values is nil
	r := requiredKey(s, "any_key")
	if !r.IsFatal() {
		t.Fatal("expected FATAL result when state.Values is nil")
	}
	if r.Errors[0].Code != result.CodeRequiredKeyMissing {
		t.Errorf("expected code %q, got %q", result.CodeRequiredKeyMissing, r.Errors[0].Code)
	}
}

// TestRequiredKeyErr_ReturnsSAWError verifies that requiredKeyErr returns
// a SAWError (not a plain fmt.Errorf string) so the Code field is preserved.
func TestRequiredKeyErr_ReturnsSAWError(t *testing.T) {
	err := requiredKeyErr(makeState(), "nonexistent_key")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	var sawErr result.SAWError
	if !errors.As(err, &sawErr) {
		t.Fatalf("expected SAWError, got %T: %v", err, err)
	}
	if sawErr.Code != result.CodeRequiredKeyMissing {
		t.Errorf("expected code %q, got %q", result.CodeRequiredKeyMissing, sawErr.Code)
	}
}

// TestStepValidateInvariants_MissingRepoPath verifies an error when repo_path is absent.
func TestStepValidateInvariants_MissingRepoPath(t *testing.T) {
	step := StepValidateInvariants()
	err := step.Func(context.Background(), makeState())
	if err == nil {
		t.Fatal("expected error for missing repo_path")
	}
}

// TestStepValidateInvariants_MissingImplPath verifies an error when impl_path is absent.
func TestStepValidateInvariants_MissingImplPath(t *testing.T) {
	step := StepValidateInvariants()
	s := stateWithKeys("repo_path", "/tmp/repo")
	err := step.Func(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for missing impl_path")
	}
}

// TestStepValidateInvariants_OK verifies success when both required keys are present.
func TestStepValidateInvariants_OK(t *testing.T) {
	step := StepValidateInvariants()
	s := stateWithKeys("repo_path", "/tmp/repo", "impl_path", "/tmp/impl.yaml")
	if err := step.Func(context.Background(), s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestStepCreateWorktrees_ErrorStrategy verifies the step uses ErrorFail.
func TestStepCreateWorktrees_ErrorStrategy(t *testing.T) {
	step := StepCreateWorktrees()
	if step.ErrorStrategy != ErrorFail {
		t.Errorf("expected ErrorFail, got %q", step.ErrorStrategy)
	}
}

// TestStepRunQualityGates_ErrorStrategy verifies the step uses ErrorFail.
func TestStepRunQualityGates_ErrorStrategy(t *testing.T) {
	step := StepRunQualityGates()
	if step.ErrorStrategy != ErrorFail {
		t.Errorf("expected ErrorFail, got %q", step.ErrorStrategy)
	}
}

// TestStepMergeAgents_ErrorStrategy verifies the step uses ErrorFail.
func TestStepMergeAgents_ErrorStrategy(t *testing.T) {
	step := StepMergeAgents()
	if step.ErrorStrategy != ErrorFail {
		t.Errorf("expected ErrorFail, got %q", step.ErrorStrategy)
	}
}

// TestStepVerifyBuild_ErrorStrategy verifies the step uses ErrorFail.
func TestStepVerifyBuild_ErrorStrategy(t *testing.T) {
	step := StepVerifyBuild()
	if step.ErrorStrategy != ErrorFail {
		t.Errorf("expected ErrorFail, got %q", step.ErrorStrategy)
	}
}

// TestStepCleanup_ErrorStrategy verifies cleanup uses ErrorContinue.
func TestStepCleanup_ErrorStrategy(t *testing.T) {
	step := StepCleanup()
	if step.ErrorStrategy != ErrorContinue {
		t.Errorf("expected ErrorContinue, got %q", step.ErrorStrategy)
	}
}

// TestStepCleanup_MissingRepoPath verifies cleanup returns an error for missing repo_path.
func TestStepCleanup_MissingRepoPath(t *testing.T) {
	step := StepCleanup()
	if err := step.Func(context.Background(), makeState()); err == nil {
		t.Fatal("expected error for missing repo_path")
	}
}

// TestWavePipeline_Structure verifies the pipeline has exactly the expected
// steps in canonical order.
func TestWavePipeline_Structure(t *testing.T) {
	p := WavePipeline(3)

	wantName := "wave-3"
	if p.name != wantName {
		t.Errorf("expected pipeline name %q, got %q", wantName, p.name)
	}

	wantSteps := []string{
		"validate_invariants",
		"create_worktrees",
		"run_quality_gates",
		"merge_agents",
		"verify_build",
		"cleanup",
	}
	if len(p.steps) != len(wantSteps) {
		t.Fatalf("expected %d steps, got %d", len(wantSteps), len(p.steps))
	}
	for i, want := range wantSteps {
		if p.steps[i].Name != want {
			t.Errorf("step[%d]: expected %q, got %q", i, want, p.steps[i].Name)
		}
	}
}

// TestWavePipeline_RunOK verifies a full wave pipeline succeeds when all
// required state keys are present.
func TestWavePipeline_RunOK(t *testing.T) {
	p := WavePipeline(1)
	s := stateWithKeys("repo_path", "/tmp/repo", "impl_path", "/tmp/impl.yaml")
	r := p.Run(context.Background(), s)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
}
