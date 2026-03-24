package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// writeCloseImplTestManifest writes a minimal IMPL manifest with the given state to a temp dir.
func writeCloseImplTestManifest(t *testing.T, state protocol.ProtocolState) string {
	t.Helper()
	m := &protocol.IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		State:       state,
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "new"},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement foo", Files: []string{"pkg/foo.go"}},
				},
			},
		},
		CompletionReports: make(map[string]protocol.CompletionReport),
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

// TestCloseImplCmd_SetsCompleteState verifies that SetImplState transitions
// a WAVE_VERIFIED manifest to COMPLETE state.
func TestCloseImplCmd_SetsCompleteState(t *testing.T) {
	manifestPath := writeCloseImplTestManifest(t, protocol.StateWaveVerified)

	// Call SetImplState directly (the same code path wired into close-impl)
	res := protocol.SetImplState(manifestPath, protocol.StateComplete, protocol.SetImplStateOpts{})
	if !res.IsSuccess() {
		t.Fatalf("SetImplState WAVE_VERIFIED -> COMPLETE failed: %v", res.Errors)
	}

	data := res.GetData()
	if data.PreviousState != protocol.StateWaveVerified {
		t.Errorf("PreviousState = %q; want %q", data.PreviousState, protocol.StateWaveVerified)
	}
	if data.NewState != protocol.StateComplete {
		t.Errorf("NewState = %q; want %q", data.NewState, protocol.StateComplete)
	}

	// Verify the manifest on disk was updated
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var updated protocol.IMPLManifest
	if err := yaml.Unmarshal(raw, &updated); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if updated.State != protocol.StateComplete {
		t.Errorf("on-disk state = %q; want %q", updated.State, protocol.StateComplete)
	}
}

// TestCloseImplCmd_InvalidState_ContinuesGracefully verifies that when a manifest
// is in SCOUT_PENDING state (invalid transition to COMPLETE), SetImplState returns
// a failure but close-impl continues. We simulate this by verifying the transition
// fails and then confirming WriteCompletionMarker still succeeds independently.
func TestCloseImplCmd_InvalidState_ContinuesGracefully(t *testing.T) {
	manifestPath := writeCloseImplTestManifest(t, protocol.StateScoutPending)

	// Transition SCOUT_PENDING -> COMPLETE is not allowed.
	res := protocol.SetImplState(manifestPath, protocol.StateComplete, protocol.SetImplStateOpts{})
	if res.IsSuccess() {
		t.Fatal("expected SetImplState SCOUT_PENDING -> COMPLETE to fail, but it succeeded")
	}

	// Verify manifest state is unchanged on disk (still SCOUT_PENDING)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var updated protocol.IMPLManifest
	if err := yaml.Unmarshal(raw, &updated); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if updated.State != protocol.StateScoutPending {
		t.Errorf("on-disk state = %q; want %q (should be unchanged)", updated.State, protocol.StateScoutPending)
	}

	// Simulate the close-impl graceful-continue behavior:
	// WriteCompletionMarker must still succeed even if state transition failed.
	if err := protocol.WriteCompletionMarker(manifestPath, "2026-03-24"); err != nil {
		t.Errorf("WriteCompletionMarker should succeed even after failed state transition, got: %v", err)
	}
}
