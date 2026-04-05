package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeStateTestManifest writes a minimal IMPL manifest with the given state to a temp dir.
func writeStateTestManifest(t *testing.T, state ProtocolState) string {
	t.Helper()
	m := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		State:       state,
		FileOwnership: []FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "new"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement foo", Files: []string{"pkg/foo.go"}},
				},
			},
		},
		CompletionReports: make(map[string]CompletionReport),
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "IMPL-test.yaml")
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestSetImplState_ValidTransition(t *testing.T) {
	path := writeStateTestManifest(t, StateScoutPending)

	res := SetImplState(context.Background(), path, StateReviewed, SetImplStateOpts{})
	if res.IsFatal() {
		t.Fatalf("SetImplState: unexpected error: %+v", res.Errors)
	}
	result := res.GetData()

	if result.PreviousState != StateScoutPending {
		t.Errorf("PreviousState = %q; want %q", result.PreviousState, StateScoutPending)
	}
	if result.NewState != StateReviewed {
		t.Errorf("NewState = %q; want %q", result.NewState, StateReviewed)
	}
	if result.Committed {
		t.Errorf("Committed = true; want false (no commit requested)")
	}

	// Verify the file was actually updated
	updated, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load after SetImplState: %v", err)
	}
	if updated.State != StateReviewed {
		t.Errorf("persisted state = %q; want %q", updated.State, StateReviewed)
	}
}

func TestSetImplState_InvalidTransition(t *testing.T) {
	path := writeStateTestManifest(t, StateComplete)

	res := SetImplState(context.Background(), path, StateScoutPending, SetImplStateOpts{})
	if !res.IsFatal() {
		t.Fatal("SetImplState: expected fatal result for COMPLETE -> SCOUT_PENDING")
	}
}

func TestSetImplState_AllowedTransitionsComplete(t *testing.T) {
	targets, ok := allowedTransitions[StateComplete]
	if !ok {
		t.Fatal("StateComplete not found in allowedTransitions")
	}
	if len(targets) != 0 {
		t.Errorf("COMPLETE should have no allowed targets; got %v", targets)
	}
}

func TestSetImplState_BlockedCanGoBack(t *testing.T) {
	path := writeStateTestManifest(t, StateBlocked)

	res := SetImplState(context.Background(), path, StateReviewed, SetImplStateOpts{})
	if res.IsFatal() {
		t.Fatalf("SetImplState BLOCKED -> REVIEWED: unexpected error: %+v", res.Errors)
	}
	result := res.GetData()
	if result.PreviousState != StateBlocked {
		t.Errorf("PreviousState = %q; want %q", result.PreviousState, StateBlocked)
	}
	if result.NewState != StateReviewed {
		t.Errorf("NewState = %q; want %q", result.NewState, StateReviewed)
	}
}

func TestSetImplState_ReviewedToComplete(t *testing.T) {
	path := writeStateTestManifest(t, StateReviewed)

	res := SetImplState(context.Background(), path, StateComplete, SetImplStateOpts{})
	if res.IsFatal() {
		t.Fatalf("SetImplState REVIEWED -> COMPLETE: unexpected error: %+v", res.Errors)
	}
	result := res.GetData()
	if result.PreviousState != StateReviewed {
		t.Errorf("PreviousState = %q; want %q", result.PreviousState, StateReviewed)
	}
	if result.NewState != StateComplete {
		t.Errorf("NewState = %q; want %q", result.NewState, StateComplete)
	}

	// Verify the file was actually updated
	updated, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load after SetImplState: %v", err)
	}
	if updated.State != StateComplete {
		t.Errorf("persisted state = %q; want %q", updated.State, StateComplete)
	}
}

func TestIsValidTransition(t *testing.T) {
	valid := []struct {
		from, to ProtocolState
	}{
		// Core happy-path transitions
		{StateScoutPending, StateReviewed},
		{StateScoutPending, StateScoutValidating},
		{StateScoutPending, StateNotSuitable},
		{StateScoutValidating, StateReviewed},
		{StateReviewed, StateWavePending},
		// close-impl without wave execution
		{StateReviewed, StateComplete},
		{StateWavePending, StateWaveExecuting},
		{StateWaveExecuting, StateWaveMerging},
		{StateWaveMerging, StateWaveVerified},
		{StateWaveVerified, StateComplete},
		{StateWaveVerified, StateWavePending},
		// Blocked recovery paths
		{StateBlocked, StateReviewed},
		{StateBlocked, StateWaveExecuting},
		{StateBlocked, StateWavePending},
	}
	for _, tc := range valid {
		if !IsValidTransition(tc.from, tc.to) {
			t.Errorf("IsValidTransition(%s, %s) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct {
		from, to ProtocolState
	}{
		// Terminal states cannot be exited
		{StateComplete, StateWavePending},
		{StateComplete, StateReviewed},
		{StateNotSuitable, StateReviewed},
		// Backwards jumps not permitted
		{StateScoutPending, StateWaveExecuting},
		{StateScoutPending, StateComplete},
		{StateWaveExecuting, StateWavePending},
	}
	for _, tc := range invalid {
		if IsValidTransition(tc.from, tc.to) {
			t.Errorf("IsValidTransition(%s, %s) = true, want false", tc.from, tc.to)
		}
	}
}

func TestValidateStateTransitionContext_SoloWaveNoWarning(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "solo"}}},
		},
	}
	warnings := ValidateStateTransitionContext(m, StateWaveExecuting, StateComplete)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for solo-wave, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateStateTransitionContext_MultiAgentWarning(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "first"},
				{ID: "B", Task: "second"},
			}},
		},
	}
	warnings := ValidateStateTransitionContext(m, StateWaveExecuting, StateComplete)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for multi-agent wave, got %d", len(warnings))
	}
	if warnings[0].Severity != "warning" {
		t.Errorf("expected severity=warning, got %q", warnings[0].Severity)
	}
}

func TestValidateStateTransitionContext_NormalTransitionNoWarning(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "first"},
				{ID: "B", Task: "second"},
			}},
		},
	}
	warnings := ValidateStateTransitionContext(m, StateWaveExecuting, StateWaveMerging)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for normal transition, got %d", len(warnings))
	}
}

// containsAll returns true if s contains all of the given substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
