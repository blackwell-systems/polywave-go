package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// writeImplStateTestManifest writes a minimal IMPL manifest YAML with the given state.
func writeImplStateTestManifest(t *testing.T, state protocol.ProtocolState) string {
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

// TestSetImplStateCmd_ValidTransition verifies that a valid state transition
// updates the YAML on disk and outputs the correct JSON.
func TestSetImplStateCmd_ValidTransition(t *testing.T) {
	manifestPath := writeImplStateTestManifest(t, protocol.StateScoutPending)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newSetImplStateCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--state", "REVIEWED"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	var result protocol.SetImplStateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	if result.PreviousState != protocol.StateScoutPending {
		t.Errorf("previous_state = %q; want %q", result.PreviousState, protocol.StateScoutPending)
	}
	if result.NewState != protocol.StateReviewed {
		t.Errorf("new_state = %q; want %q", result.NewState, protocol.StateReviewed)
	}
	if result.Committed {
		t.Errorf("committed = true; want false (no --commit flag)")
	}

	// Verify the YAML file was actually updated on disk
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "REVIEWED") {
		t.Errorf("expected REVIEWED in updated manifest, got:\n%s", string(data))
	}
}

// TestSetImplStateCmd_InvalidTransition verifies that COMPLETE -> SCOUT_PENDING
// returns a non-zero exit and an error message containing "not allowed".
func TestSetImplStateCmd_InvalidTransition(t *testing.T) {
	manifestPath := writeImplStateTestManifest(t, protocol.StateComplete)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newSetImplStateCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--state", "SCOUT_PENDING"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for COMPLETE -> SCOUT_PENDING, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "not allowed") {
		t.Errorf("error message %q should contain 'not allowed'", errMsg)
	}
}

// TestSetImplStateCmd_MissingStateFlag verifies that omitting --state produces a cobra error.
func TestSetImplStateCmd_MissingStateFlag(t *testing.T) {
	manifestPath := writeImplStateTestManifest(t, protocol.StateScoutPending)

	cmd := newSetImplStateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --state flag is missing, got nil")
	}
}

// TestSetImplStateCmd_Use verifies the Use field matches the expected string.
func TestSetImplStateCmd_Use(t *testing.T) {
	cmd := newSetImplStateCmd()
	want := "set-impl-state <manifest-path>"
	if cmd.Use != want {
		t.Errorf("Use = %q; want %q", cmd.Use, want)
	}
}
