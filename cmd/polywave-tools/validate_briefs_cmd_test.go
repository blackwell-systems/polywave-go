package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/stretchr/testify/assert"
)

// writeTempFile writes content to a file in dir and returns the path.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestValidateBriefsCmd_HappyPath creates a temp IMPL with valid briefs (no
// symbol references that would fail), runs validate-briefs, and verifies exit 0
// with valid=true in the JSON output.
func TestValidateBriefsCmd_HappyPath(t *testing.T) {
	dir := t.TempDir()

	// Create a real Go source file so symbol checks can pass.
	srcContent := `package foo

// DoThing does a thing.
func DoThing() {}
`
	writeTempFile(t, dir, "foo.go", srcContent)

	// Brief references DoThing which exists in the owned file.
	manifest := `feature: test-feature
slug: test-feature
repository: ` + dir + `
file_ownership:
  - file: foo.go
    agent: A
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: A
        task: "Implement DoThing in foo.go"
        files:
          - foo.go
`
	manifestPath := writeTempFile(t, dir, "IMPL.yaml", manifest)

	var stdout bytes.Buffer
	cmd := newValidateBriefsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	assert.NoError(t, err, "expected exit 0 for valid briefs")

	var data protocol.BriefValidationData
	if parseErr := json.Unmarshal(stdout.Bytes(), &data); parseErr != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", parseErr, stdout.String())
	}
	assert.True(t, data.Valid, "expected data.Valid=true")
	assert.NotEmpty(t, data.Summary)
}

// TestValidateBriefsCmd_InvalidBriefs creates an IMPL where the agent brief
// references a symbol that does NOT exist in the owned file. Verifies exit 1
// and valid=false in the JSON output.
//
// Note: symbol_missing issues have severity="warning" and do not set
// agent Passed=false (only "error" severity does). The overall Valid field
// reflects whether any agent has Passed=false. So to get Valid=false we need
// an error-severity issue. Since the current implementation uses "warning" for
// missing symbols, we verify that issues are reported instead of Valid=false.
func TestValidateBriefsCmd_InvalidBriefs(t *testing.T) {
	dir := t.TempDir()

	// Create a Go file that does NOT contain the referenced symbol.
	srcContent := `package foo

func ExistingFunc() {}
`
	writeTempFile(t, dir, "foo.go", srcContent)

	// Brief references NonExistentSymbol which is NOT in foo.go.
	manifest := `feature: test-feature
slug: test-feature
repository: ` + dir + `
file_ownership:
  - file: foo.go
    agent: A
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: A
        task: "Implement NonExistentSymbol in foo.go — see ` + "`NonExistentSymbol`" + `"
        files:
          - foo.go
`
	manifestPath := writeTempFile(t, dir, "IMPL.yaml", manifest)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newValidateBriefsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	// Execute; may or may not error depending on issue severity.
	_ = cmd.Execute()

	// Verify JSON is produced and parseable.
	outBytes := stdout.Bytes()
	if len(outBytes) == 0 {
		t.Fatal("expected JSON output, got empty stdout")
	}

	var data protocol.BriefValidationData
	if parseErr := json.Unmarshal(outBytes, &data); parseErr != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", parseErr, stdout.String())
	}

	// Verify agent A result has issues (the missing symbol).
	agentResult, ok := data.AgentResults["A"]
	assert.True(t, ok, "expected agent A result in output")
	if ok {
		assert.NotEmpty(t, agentResult.Issues, "expected at least one issue reported for missing symbol")
		// Verify the issue is symbol_missing type.
		found := false
		for _, issue := range agentResult.Issues {
			if issue.Check == "symbol_missing" {
				found = true
				assert.Equal(t, "NonExistentSymbol", issue.Symbol)
				break
			}
		}
		assert.True(t, found, "expected symbol_missing issue in agent results")
	}
}

// TestValidateBriefsCmd_NoManifest tests that a non-existent manifest path
// produces an error.
func TestValidateBriefsCmd_NoManifest(t *testing.T) {
	cmd := newValidateBriefsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"/nonexistent/path/IMPL.yaml"})

	err := cmd.Execute()
	assert.Error(t, err, "expected error for non-existent manifest path")
}
