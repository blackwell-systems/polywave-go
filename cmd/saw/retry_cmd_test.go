package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
	"gopkg.in/yaml.v3"
)

// minimalIMPL creates a minimal IMPL manifest YAML on disk and returns its path.
func minimalIMPL(t *testing.T, dir string) string {
	t.Helper()

	implDir := filepath.Join(dir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create IMPL dir: %v", err)
	}

	m := &protocol.IMPLManifest{
		Title:       "Test feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/bar/bar.go", Agent: "B", Wave: 1, Action: "new"},
			{File: "pkg/baz/baz.go", Agent: "C", Wave: 2, Action: "new"},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "implement foo", Files: []string{"pkg/foo/foo.go"}},
					{ID: "B", Task: "implement bar", Files: []string{"pkg/bar/bar.go"}},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "C", Task: "implement baz", Files: []string{"pkg/baz/baz.go"}},
				},
			},
		},
		State: protocol.StateWavePending,
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	implPath := filepath.Join(implDir, "IMPL-test-feature.yaml")
	if err := os.WriteFile(implPath, data, 0644); err != nil {
		t.Fatalf("failed to write IMPL file: %v", err)
	}

	return implPath
}

// TestRetryCmdArgs verifies that exactly 1 positional argument is required.
func TestRetryCmdArgs(t *testing.T) {
	t.Run("NoArgs", func(t *testing.T) {
		cmd := newRetryCmd()
		cmd.SetArgs([]string{"--wave", "1", "--gate-type", "build"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for missing IMPL path argument, got nil")
		}
		if !strings.Contains(err.Error(), "accepts 1 arg") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("TooManyArgs", func(t *testing.T) {
		cmd := newRetryCmd()
		cmd.SetArgs([]string{"path/one.yaml", "path/two.yaml", "--wave", "1", "--gate-type", "build"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for too many args, got nil")
		}
	})
}

// TestRetryCmdFlags verifies that required flags are enforced.
func TestRetryCmdFlags(t *testing.T) {
	dir := t.TempDir()
	implPath := minimalIMPL(t, dir)

	t.Run("MissingWave", func(t *testing.T) {
		cmd := newRetryCmd()
		cmd.SetArgs([]string{implPath, "--gate-type", "build"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for missing --wave flag, got nil")
		}
		if !strings.Contains(err.Error(), "wave") {
			t.Errorf("error should mention 'wave', got: %v", err)
		}
	})

	t.Run("MissingGateType", func(t *testing.T) {
		cmd := newRetryCmd()
		cmd.SetArgs([]string{implPath, "--wave", "1"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for missing --gate-type flag, got nil")
		}
		if !strings.Contains(err.Error(), "gate-type") {
			t.Errorf("error should mention 'gate-type', got: %v", err)
		}
	})

	t.Run("ValidFlagsDefaultMaxRetries", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := newRetryCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{implPath, "--wave", "1", "--gate-type", "build", "--repo-root", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestRetryCmd_GeneratesIMPL verifies that the retry command writes a YAML file
// under docs/IMPL/ in the repo root and that the file is a valid manifest.
func TestRetryCmd_GeneratesIMPL(t *testing.T) {
	dir := t.TempDir()
	implPath := minimalIMPL(t, dir)

	var stdout bytes.Buffer
	cmd := newRetryCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		implPath,
		"--wave", "1",
		"--gate-type", "test",
		"--max-retries", "3",
		"--repo-root", dir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("retry command failed: %v", err)
	}

	// Parse output as RetryResult
	var result retry.RetryResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}

	if result.RetryIMPL == "" {
		t.Fatal("RetryIMPL path is empty; expected a file to be generated")
	}

	// Verify the file was actually written to disk.
	retryFilePath := filepath.Join(dir, result.RetryIMPL)
	if _, err := os.Stat(retryFilePath); os.IsNotExist(err) {
		t.Fatalf("retry IMPL file not found at %s", retryFilePath)
	}

	// Load and validate it's a valid manifest.
	m, err := protocol.Load(retryFilePath)
	if err != nil {
		t.Fatalf("generated retry IMPL is not a valid manifest: %v", err)
	}

	if m.FeatureSlug == "" {
		t.Error("generated manifest has empty feature_slug")
	}
	if len(m.Waves) == 0 {
		t.Error("generated manifest has no waves")
	}
	if len(m.Waves[0].Agents) == 0 {
		t.Error("generated manifest wave has no agents")
	}
	if m.Waves[0].Agents[0].ID != "R" {
		t.Errorf("retry agent ID = %q, want %q", m.Waves[0].Agents[0].ID, "R")
	}
}

// TestRetryCmd_GeneratesIMPL_Wave2 verifies that only files from the specified
// wave are targeted when --wave is set.
func TestRetryCmd_GeneratesIMPL_Wave2(t *testing.T) {
	dir := t.TempDir()
	implPath := minimalIMPL(t, dir)

	var stdout bytes.Buffer
	cmd := newRetryCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		implPath,
		"--wave", "2",
		"--gate-type", "build",
		"--repo-root", dir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("retry command failed: %v", err)
	}

	var result retry.RetryResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	retryFilePath := filepath.Join(dir, result.RetryIMPL)
	m, err := protocol.Load(retryFilePath)
	if err != nil {
		t.Fatalf("generated retry IMPL invalid: %v", err)
	}

	// Should only target wave 2's file (pkg/baz/baz.go)
	for _, fo := range m.FileOwnership {
		if fo.File == "pkg/foo/foo.go" || fo.File == "pkg/bar/bar.go" {
			t.Errorf("wave 1 file %q should not be in wave 2 retry manifest", fo.File)
		}
	}
	found := false
	for _, fo := range m.FileOwnership {
		if fo.File == "pkg/baz/baz.go" {
			found = true
		}
	}
	if !found {
		t.Error("wave 2 file pkg/baz/baz.go should be in retry manifest")
	}
}

// TestRetryCmd_OutputJSON verifies the JSON output fields match expected types.
func TestRetryCmd_OutputJSON(t *testing.T) {
	dir := t.TempDir()
	implPath := minimalIMPL(t, dir)

	var stdout bytes.Buffer
	cmd := newRetryCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		implPath,
		"--wave", "1",
		"--gate-type", "lint",
		"--repo-root", dir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("retry command failed: %v", err)
	}

	out := strings.TrimSpace(stdout.String())

	// Must be valid JSON.
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	// Check required fields.
	for _, field := range []string{"attempt", "agent_id", "gate_passed", "gate_output", "final_state"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("output JSON missing field %q", field)
		}
	}

	// attempt should be a number
	attempt, ok := raw["attempt"].(float64)
	if !ok || attempt < 1 {
		t.Errorf("attempt field should be a positive number, got %v", raw["attempt"])
	}

	// agent_id should be "R"
	if agentID, ok := raw["agent_id"].(string); !ok || agentID != "R" {
		t.Errorf("agent_id should be %q, got %v", "R", raw["agent_id"])
	}

	// final_state must be one of the allowed values.
	finalState, _ := raw["final_state"].(string)
	validStates := map[string]bool{"passed": true, "retrying": true, "blocked": true}
	if !validStates[finalState] {
		t.Errorf("final_state = %q, want one of [passed, retrying, blocked]", finalState)
	}
}

// TestRetryCmd_Blocked verifies that setting --max-retries 1 and calling
// the command twice (via the loop directly) triggers "blocked".
func TestRetryCmd_Blocked(t *testing.T) {
	dir := t.TempDir()
	implPath := minimalIMPL(t, dir)

	// Set max-retries=1: first attempt retries, second is blocked.
	// We use the loop directly to avoid re-running the command binary.
	cfg := retry.RetryConfig{
		MaxRetries: 1,
		IMPLPath:   implPath,
		RepoPath:   dir,
	}
	loop := retry.NewRetryLoop(cfg)
	gate := retry.QualityGateFailure{
		GateType:    "build",
		FailedFiles: []string{"pkg/foo/foo.go"},
	}

	r1, err := loop.Run(nil, gate, nil) //nolint:staticcheck // context unused in stub
	if err != nil {
		t.Fatalf("attempt 1 error: %v", err)
	}
	if r1.FinalState != "retrying" {
		t.Errorf("attempt 1 final_state = %q, want retrying", r1.FinalState)
	}

	r2, err := loop.Run(nil, gate, nil) //nolint:staticcheck
	if err != nil {
		t.Fatalf("attempt 2 error: %v", err)
	}
	if r2.FinalState != "blocked" {
		t.Errorf("attempt 2 final_state = %q, want blocked", r2.FinalState)
	}
	if r2.RetryIMPL != "" {
		t.Errorf("blocked result should have empty RetryIMPL, got %q", r2.RetryIMPL)
	}
}
