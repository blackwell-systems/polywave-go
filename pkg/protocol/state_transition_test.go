package protocol

import (
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"os"
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

	result, err := SetImplState(path, StateReviewed, SetImplStateOpts{})
	if err != nil {
		t.Fatalf("SetImplState: unexpected error: %v", err)
	}

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
	updated, err := Load(path)
	if err != nil {
		t.Fatalf("Load after SetImplState: %v", err)
	}
	if updated.State != StateReviewed {
		t.Errorf("persisted state = %q; want %q", updated.State, StateReviewed)
	}
}

func TestSetImplState_InvalidTransition(t *testing.T) {
	path := writeStateTestManifest(t, StateComplete)

	_, err := SetImplState(path, StateScoutPending, SetImplStateOpts{})
	if err == nil {
		t.Fatal("SetImplState: expected error for COMPLETE -> SCOUT_PENDING, got nil")
	}

	// Error message should mention the states
	errMsg := err.Error()
	if !containsAll(errMsg, "COMPLETE", "SCOUT_PENDING") {
		t.Errorf("error message %q should mention both states", errMsg)
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

	result, err := SetImplState(path, StateReviewed, SetImplStateOpts{})
	if err != nil {
		t.Fatalf("SetImplState BLOCKED -> REVIEWED: unexpected error: %v", err)
	}
	if result.PreviousState != StateBlocked {
		t.Errorf("PreviousState = %q; want %q", result.PreviousState, StateBlocked)
	}
	if result.NewState != StateReviewed {
		t.Errorf("NewState = %q; want %q", result.NewState, StateReviewed)
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
