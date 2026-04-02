package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// minimalValidIMPL returns a minimal two-agent, single-wave IMPL fixture
// that passes E16 validation.
func minimalValidIMPL() string {
	return `title: Test Implementation
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
}

func TestPreWaveValidateCmd_StepFour_NoProblemsMani(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	if err := os.WriteFile(implPath, []byte(minimalValidIMPL()), 0644); err != nil {
		t.Fatalf("failed to write temp IMPL: %v", err)
	}

	cmd := newPreWaveValidateCmd()
	cmd.SetArgs([]string{"--wave", "1", implPath})

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

	var result PreWaveValidateResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if !result.WaveStructure.Passed {
		t.Errorf("expected wave_structure.passed=true, got false; problems: %v", result.WaveStructure.Problems)
	}

	if len(result.WaveStructure.Problems) != 0 {
		t.Errorf("expected wave_structure.problems to be empty, got: %v", result.WaveStructure.Problems)
	}
}

func TestPreWaveValidateCmd_StepFour_PassedField(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	if err := os.WriteFile(implPath, []byte(minimalValidIMPL()), 0644); err != nil {
		t.Fatalf("failed to write temp IMPL: %v", err)
	}

	cmd := newPreWaveValidateCmd()
	cmd.SetArgs([]string{"--wave", "1", implPath})

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

	// Check that the JSON output contains a "wave_structure" key
	var rawResult map[string]interface{}
	if err := json.Unmarshal([]byte(output), &rawResult); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if _, ok := rawResult["wave_structure"]; !ok {
		t.Errorf("expected JSON output to contain 'wave_structure' key, got keys: %v", rawResult)
	}
}

func TestPreWaveValidateCmd_MissingWaveFlag(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	if err := os.WriteFile(implPath, []byte(minimalValidIMPL()), 0644); err != nil {
		t.Fatalf("failed to write temp IMPL: %v", err)
	}

	cmd := newPreWaveValidateCmd()
	// Intentionally omit --wave flag
	cmd.SetArgs([]string{implPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --wave flag is missing, got nil")
	}
}
