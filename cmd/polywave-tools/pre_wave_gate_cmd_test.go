package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

func TestPreWaveGateCmd_ValidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// A minimal valid IMPL with two agents (V047 requires >1 agent for SUITABLE), no scaffolds
	implContent := `title: Test Implementation
feature_slug: test-implementation
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: implement test feature
      - id: B
        task: implement tests
file_ownership:
  - file: pkg/test/file.go
    agent: A
    wave: 1
    action: new
  - file: pkg/test/file_test.go
    agent: B
    wave: 1
    action: new
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("failed to write temp IMPL: %v", err)
	}

	cmd := newPreWaveGateCmd()
	cmd.SetArgs([]string{implPath})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("expected no error for valid manifest, got: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	var result protocol.PreWaveGateResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if !result.Ready {
		t.Error("expected ready=true for valid manifest")
	}

	if len(result.Checks) != 4 {
		t.Errorf("expected 4 checks, got %d", len(result.Checks))
	}
}

func TestPreWaveGateCmd_InvalidPath(t *testing.T) {
	cmd := newPreWaveGateCmd()
	cmd.SetArgs([]string{"/nonexistent/path/IMPL-test.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent manifest path")
	}
}

func TestPreWaveGateCmd_StateComplete_ExitsOne(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-complete.yaml")

	// A manifest in COMPLETE state should fail pre-wave-gate
	implContent := `title: Test Implementation
feature_slug: test-implementation
state: COMPLETE
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: implement test feature
file_ownership:
  - file: pkg/test/file.go
    agent: A
    wave: 1
    action: new
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("failed to write temp IMPL: %v", err)
	}

	cmd := newPreWaveGateCmd()
	cmd.SetArgs([]string{implPath})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ready=false, got nil")
	}
}

func TestPreWaveGateCmd_Structure(t *testing.T) {
	cmd := newPreWaveGateCmd()

	if cmd.Use != "pre-wave-gate <manifest-path>" {
		t.Errorf("expected Use = %q, got %q", "pre-wave-gate <manifest-path>", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}
