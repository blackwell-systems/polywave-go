package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateAgentIDs_ValidIDs tests that valid agent IDs pass validation.
func TestValidateAgentIDs_ValidIDs(t *testing.T) {
	implDoc := `# IMPL: test-feature
title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...

` + "```yaml type=impl-file-ownership\n" + `| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/foo.go | A | 1 | - |
| pkg/bar.go | B | 1 | - |
| pkg/baz.go | C2 | 2 | A, B |
` + "```\n\n" +
		"```yaml type=impl-dep-graph\n" + `Wave 1:
    [A] pkg/foo.go
        ✓ root
    [B] pkg/bar.go
        ✓ root

Wave 2:
    [C2] pkg/baz.go
        depends on: [A], [B]
` + "```\n\n" +
		"```yaml type=impl-wave-structure\n" + `Wave 1: [A] [B]
Wave 2: [C2]
` + "```\n"

	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implDoc), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL doc: %v", err)
	}

	errs, err := ValidateIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ValidateIMPLDoc failed: %v", err)
	}

	if len(errs) > 0 {
		t.Errorf("Expected no validation errors, got %d:", len(errs))
		for _, e := range errs {
			t.Logf("  Line %d [%s]: %s", e.LineNumber, e.BlockType, e.Message)
		}
	}
}

// TestValidateAgentIDs_InvalidGeneration1 tests that [A1] is rejected (generation 1 must be bare letter).
func TestValidateAgentIDs_InvalidGeneration1(t *testing.T) {
	implDoc := `# IMPL: test-feature
title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...

` + "```yaml type=impl-file-ownership\n" + `| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/foo.go | A1 | 1 | - |
` + "```\n\n" +
		"```yaml type=impl-dep-graph\n" + `Wave 1:
    [A1] pkg/foo.go
        ✓ root
` + "```\n\n" +
		"```yaml type=impl-wave-structure\n" + `Wave 1: [A1]
` + "```\n"

	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implDoc), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL doc: %v", err)
	}

	errs, err := ValidateIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ValidateIMPLDoc failed: %v", err)
	}

	if len(errs) == 0 {
		t.Fatal("Expected validation errors for [A1], got none")
	}

	// Should have at least 2 agent-id errors (dep-graph, wave-structure) plus suggestion
	// Note: file-ownership may or may not report an error depending on table parsing
	agentIDErrors := 0
	hasSuggestion := false
	for _, e := range errs {
		if e.BlockType == "agent-id" {
			agentIDErrors++
			if strings.Contains(e.Message, "assign-agent-ids --count") {
				hasSuggestion = true
			}
		}
	}

	if agentIDErrors < 2 {
		t.Errorf("Expected at least 2 agent-id errors, got %d", agentIDErrors)
		for _, e := range errs {
			t.Logf("  Line %d [%s]: %s", e.LineNumber, e.BlockType, e.Message)
		}
	}

	if !hasSuggestion {
		t.Error("Expected validation to include 'assign-agent-ids --count' suggestion")
	}
}

// TestValidateAgentIDs_MixedValidInvalid tests that validation catches invalid IDs among valid ones.
func TestValidateAgentIDs_MixedValidInvalid(t *testing.T) {
	implDoc := `# IMPL: test-feature
title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...

` + "```yaml type=impl-file-ownership\n" + `| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/foo.go | A | 1 | - |
| pkg/bar.go | B1 | 1 | - |
| pkg/baz.go | C2 | 2 | A, B1 |
` + "```\n\n" +
		"```yaml type=impl-dep-graph\n" + `Wave 1:
    [A] pkg/foo.go
        ✓ root
    [B1] pkg/bar.go
        ✓ root

Wave 2:
    [C2] pkg/baz.go
        depends on: [A], [B1]
` + "```\n\n" +
		"```yaml type=impl-wave-structure\n" + `Wave 1: [A] [B1]
Wave 2: [C2]
` + "```\n"

	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implDoc), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL doc: %v", err)
	}

	errs, err := ValidateIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ValidateIMPLDoc failed: %v", err)
	}

	if len(errs) == 0 {
		t.Fatal("Expected validation errors for [B1], got none")
	}

	// Check that [B1] errors are reported
	foundB1Error := false
	for _, e := range errs {
		if e.BlockType == "agent-id" && strings.Contains(e.Message, "B1") {
			foundB1Error = true
			break
		}
	}

	if !foundB1Error {
		t.Error("Expected agent-id error for [B1], but none found")
		for _, e := range errs {
			t.Logf("  Line %d [%s]: %s", e.LineNumber, e.BlockType, e.Message)
		}
	}
}

// TestValidateAgentIDs_CountSuggestion tests that the validator suggests correct agent count.
func TestValidateAgentIDs_CountSuggestion(t *testing.T) {
	implDoc := `# IMPL: test-feature
title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...

` + "```yaml type=impl-file-ownership\n" + `| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/a.go | A1 | 1 | - |
| pkg/b.go | B1 | 1 | - |
| pkg/c.go | C1 | 1 | - |
` + "```\n\n" +
		"```yaml type=impl-dep-graph\n" + `Wave 1:
    [A1] pkg/a.go
        ✓ root
    [B1] pkg/b.go
        ✓ root
    [C1] pkg/c.go
        ✓ root
` + "```\n\n" +
		"```yaml type=impl-wave-structure\n" + `Wave 1: [A1] [B1] [C1]
` + "```\n"

	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implDoc), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL doc: %v", err)
	}

	errs, err := ValidateIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ValidateIMPLDoc failed: %v", err)
	}

	// Find the suggestion error
	var suggestionMsg string
	for _, e := range errs {
		if e.BlockType == "agent-id" && e.LineNumber == 0 {
			suggestionMsg = e.Message
			break
		}
	}

	if suggestionMsg == "" {
		t.Fatal("Expected agent-id suggestion with count, got none")
	}

	// Should suggest count of 3 (distinct agent IDs: A1, B1, C1)
	expectedSubstr := "assign-agent-ids --count 3"
	if !strings.Contains(suggestionMsg, expectedSubstr) {
		t.Errorf("Expected suggestion to contain %q, got: %s", expectedSubstr, suggestionMsg)
	}
}

// TestValidateAgentIDs_NoErrors tests that no suggestion is appended when IDs are valid.
func TestValidateAgentIDs_NoErrors(t *testing.T) {
	implDoc := `# IMPL: test-feature
title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...

` + "```yaml type=impl-file-ownership\n" + `| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/foo.go | A | 1 | - |
| pkg/bar.go | B | 1 | - |
` + "```\n\n" +
		"```yaml type=impl-dep-graph\n" + `Wave 1:
    [A] pkg/foo.go
        ✓ root
    [B] pkg/bar.go
        ✓ root
` + "```\n\n" +
		"```yaml type=impl-wave-structure\n" + `Wave 1: [A] [B]
` + "```\n"

	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implDoc), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL doc: %v", err)
	}

	errs, err := ValidateIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ValidateIMPLDoc failed: %v", err)
	}

	// No agent-id errors should be present
	for _, e := range errs {
		if e.BlockType == "agent-id" {
			t.Errorf("Unexpected agent-id error: %s", e.Message)
		}
	}
}
