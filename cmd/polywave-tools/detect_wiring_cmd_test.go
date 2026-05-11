package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDetectWiringCmd_YAMLOutput(t *testing.T) {
	// Create a temporary IMPL doc with cross-agent function calls
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Use string concatenation to include backticks in the YAML
	implContent := "feature: Test feature\n" +
		"repository: .\n" +
		"waves:\n" +
		"  - number: 1\n" +
		"    agents:\n" +
		"      - id: A\n" +
		"        task: \"Implement RunScout function\"\n" +
		"  - number: 2\n" +
		"    agents:\n" +
		"      - id: B\n" +
		"        task: \"CLI that calls `RunScout()` to start scout\"\n" +
		"\n" +
		"file_ownership:\n" +
		"  - agent: A\n" +
		"    wave: 1\n" +
		"    repo: .\n" +
		"    file: pkg/engine/scout.go\n" +
		"  - agent: B\n" +
		"    wave: 2\n" +
		"    repo: .\n" +
		"    file: pkg/cli/main.go\n" +
		"\n" +
		"interface_contracts:\n" +
		"  - name: RunScout\n" +
		"    definition: \"func RunScout() error\"\n" +
		"    location: pkg/engine/scout.go\n" +
		"\n" +
		"scaffolds: []\n"

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command
	cmd := newDetectWiringCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "yaml"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Parse YAML output
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput: %s", err, output)
	}

	// Verify wiring key exists
	wiring, ok := result["wiring"]
	if !ok {
		t.Fatalf("Output missing 'wiring' key: %s", output)
	}

	// Verify we got the wiring declaration
	wiringSlice, ok := wiring.([]interface{})
	if !ok {
		t.Fatalf("wiring is not an array: %T", wiring)
	}

	if len(wiringSlice) != 1 {
		t.Fatalf("Expected 1 wiring declaration, got %d", len(wiringSlice))
	}

	// Verify structure contains expected fields
	firstDecl := wiringSlice[0].(map[string]interface{})
	requiredFields := []string{"symbol", "defined_in", "must_be_called_from", "agent", "wave"}
	for _, field := range requiredFields {
		if _, ok := firstDecl[field]; !ok {
			t.Errorf("Declaration missing required field: %s", field)
		}
	}

	// Verify specific values
	if firstDecl["symbol"] != "RunScout" {
		t.Errorf("Expected symbol 'RunScout', got %v", firstDecl["symbol"])
	}
	if firstDecl["agent"] != "B" {
		t.Errorf("Expected agent 'B', got %v", firstDecl["agent"])
	}
}

func TestDetectWiringCmd_JSONOutput(t *testing.T) {
	// Create a temporary IMPL doc with cross-agent function calls
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Use string concatenation to include backticks in the YAML
	implContent := "feature: Test feature\n" +
		"repository: .\n" +
		"waves:\n" +
		"  - number: 1\n" +
		"    agents:\n" +
		"      - id: A\n" +
		"        task: \"Implement RunScout function\"\n" +
		"  - number: 2\n" +
		"    agents:\n" +
		"      - id: B\n" +
		"        task: \"CLI that calls `RunScout()` to start scout\"\n" +
		"\n" +
		"file_ownership:\n" +
		"  - agent: A\n" +
		"    wave: 1\n" +
		"    repo: .\n" +
		"    file: pkg/engine/scout.go\n" +
		"  - agent: B\n" +
		"    wave: 2\n" +
		"    repo: .\n" +
		"    file: pkg/cli/main.go\n" +
		"\n" +
		"interface_contracts:\n" +
		"  - name: RunScout\n" +
		"    definition: \"func RunScout() error\"\n" +
		"    location: pkg/engine/scout.go\n" +
		"\n" +
		"scaffolds: []\n"

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command with --format json
	cmd := newDetectWiringCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "json"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output is valid JSON
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Fatalf("Output doesn't look like JSON: %s", output)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify wiring key exists
	wiring, ok := result["wiring"]
	if !ok {
		t.Fatalf("Output missing 'wiring' key: %s", output)
	}

	// Verify we got the wiring declaration
	wiringSlice, ok := wiring.([]interface{})
	if !ok {
		t.Fatalf("wiring is not an array: %T", wiring)
	}

	if len(wiringSlice) != 1 {
		t.Fatalf("Expected 1 wiring declaration, got %d", len(wiringSlice))
	}

	// Verify specific values
	firstDecl := wiringSlice[0].(map[string]interface{})
	if firstDecl["symbol"] != "RunScout" {
		t.Errorf("Expected symbol 'RunScout', got %v", firstDecl["symbol"])
	}
}

func TestDetectWiringCmd_InvalidFormat(t *testing.T) {
	// Create a temporary IMPL doc
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	implContent := `
feature: Test feature
repository: .
waves:
  - number: 1
    agents:
      - id: A
        task: "Implement function"

file_ownership:
  - agent: A
    wave: 1
    repo: .
    file: pkg/test.go

interface_contracts: []
scaffolds: []
`

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run command with invalid format
	cmd := newDetectWiringCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "xml"})

	err := cmd.Execute()

	// Should fail with exit code 1
	if err == nil {
		t.Fatal("Expected command to fail with invalid format, but it succeeded")
	}

	// Verify error message mentions invalid format
	if !strings.Contains(err.Error(), "invalid --format value") {
		t.Errorf("Expected error about invalid format, got: %v", err)
	}
}

func TestDetectWiringCmd_MissingIMPL(t *testing.T) {
	// Run command with non-existent file
	cmd := newDetectWiringCmd()
	cmd.SetArgs([]string{"/nonexistent/path/IMPL-test.yaml", "--format", "yaml"})

	err := cmd.Execute()

	// Should fail with exit code 1
	if err == nil {
		t.Fatal("Expected command to fail with missing file, but it succeeded")
	}

	// Verify error message mentions parsing failure
	if !strings.Contains(err.Error(), "failed to parse IMPL doc") {
		t.Errorf("Expected error about parsing IMPL doc, got: %v", err)
	}
}

func TestDetectWiringCmd_NoWiring(t *testing.T) {
	// Create a temporary IMPL doc with no cross-agent calls
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	implContent := `
feature: Test feature
repository: .
waves:
  - number: 1
    agents:
      - id: A
        task: "Implement internal function"

file_ownership:
  - agent: A
    wave: 1
    repo: .
    file: pkg/test.go

interface_contracts: []
scaffolds: []
`

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command
	cmd := newDetectWiringCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "yaml"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Parse YAML output
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput: %s", err, output)
	}

	// Verify wiring key exists and is empty
	wiring, ok := result["wiring"]
	if !ok {
		t.Fatalf("Output missing 'wiring' key: %s", output)
	}

	wiringSlice, ok := wiring.([]interface{})
	if !ok {
		t.Fatalf("wiring is not an array: %T", wiring)
	}

	if len(wiringSlice) != 0 {
		t.Errorf("Expected 0 wiring declarations, got %d", len(wiringSlice))
	}
}

func TestDetectWiringCmd_MultipleCalls(t *testing.T) {
	// Create a temporary IMPL doc with multiple cross-agent calls
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Use string concatenation to include backticks in the YAML
	implContent := "feature: Test feature\n" +
		"repository: .\n" +
		"waves:\n" +
		"  - number: 1\n" +
		"    agents:\n" +
		"      - id: A\n" +
		"        task: \"Implement engine\"\n" +
		"  - number: 2\n" +
		"    agents:\n" +
		"      - id: B\n" +
		"        task: \"CLI calls `RunScout()` and delegates to `RunWave`\"\n" +
		"\n" +
		"file_ownership:\n" +
		"  - agent: A\n" +
		"    wave: 1\n" +
		"    repo: .\n" +
		"    file: pkg/engine/scout.go\n" +
		"  - agent: A\n" +
		"    wave: 1\n" +
		"    repo: .\n" +
		"    file: pkg/engine/wave.go\n" +
		"  - agent: B\n" +
		"    wave: 2\n" +
		"    repo: .\n" +
		"    file: pkg/cli/main.go\n" +
		"\n" +
		"interface_contracts:\n" +
		"  - name: RunScout\n" +
		"    definition: \"func RunScout() error\"\n" +
		"    location: pkg/engine/scout.go\n" +
		"  - name: RunWave\n" +
		"    definition: \"func RunWave() error\"\n" +
		"    location: pkg/engine/wave.go\n" +
		"\n" +
		"scaffolds: []\n"

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command
	cmd := newDetectWiringCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "yaml"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Parse YAML output
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput: %s", err, output)
	}

	// Verify wiring key exists
	wiring, ok := result["wiring"]
	if !ok {
		t.Fatalf("Output missing 'wiring' key: %s", output)
	}

	// Verify we got multiple wiring declarations
	wiringSlice, ok := wiring.([]interface{})
	if !ok {
		t.Fatalf("wiring is not an array: %T", wiring)
	}

	if len(wiringSlice) != 2 {
		t.Fatalf("Expected 2 wiring declarations, got %d", len(wiringSlice))
	}

	// Verify both symbols are present
	symbols := make(map[string]bool)
	for _, decl := range wiringSlice {
		declMap := decl.(map[string]interface{})
		symbols[declMap["symbol"].(string)] = true
	}

	if !symbols["RunScout"] {
		t.Errorf("Expected to find RunScout in wiring declarations")
	}
	if !symbols["RunWave"] {
		t.Errorf("Expected to find RunWave in wiring declarations")
	}
}
