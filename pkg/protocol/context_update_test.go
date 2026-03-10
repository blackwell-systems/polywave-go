package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateContext_NewFile(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	projectRoot := tempDir

	// Create a test manifest
	manifestPath := filepath.Join(tempDir, "IMPL-test-feature.yaml")
	manifestContent := `title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: "Test task A"
        files:
          - test.go
  - number: 2
    agents:
      - id: B
        task: "Test task B"
        files:
          - test2.go
      - id: C
        task: "Test task C"
        files:
          - test3.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	// Call UpdateContext
	result, err := UpdateContext(manifestPath, projectRoot)
	if err != nil {
		t.Fatalf("UpdateContext failed: %v", err)
	}

	// Verify result
	if !result.Updated {
		t.Error("expected Updated=true")
	}
	if len(result.NewEntries) != 1 || result.NewEntries[0] != "test-feature" {
		t.Errorf("expected NewEntries=[test-feature], got %v", result.NewEntries)
	}

	expectedContextPath := filepath.Join(projectRoot, "docs", "CONTEXT.md")
	if result.ContextPath != expectedContextPath {
		t.Errorf("expected ContextPath=%s, got %s", expectedContextPath, result.ContextPath)
	}

	// Verify file was created with correct content
	content, err := os.ReadFile(result.ContextPath)
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	contentStr := string(content)

	// Check header
	if !strings.Contains(contentStr, "# Project Context") {
		t.Error("context file missing header")
	}
	if !strings.Contains(contentStr, "## Features Completed") {
		t.Error("context file missing Features Completed section")
	}

	// Check entry format
	if !strings.Contains(contentStr, "- **test-feature**: completed") {
		t.Error("context file missing feature entry")
	}
	if !strings.Contains(contentStr, "2 waves, 3 agents") {
		t.Error("context file has incorrect wave/agent count")
	}
	if !strings.Contains(contentStr, "IMPL doc: IMPL-test-feature.yaml") {
		t.Error("context file missing IMPL doc path")
	}
}

func TestUpdateContext_ExistingFile(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	projectRoot := tempDir

	// Create existing CONTEXT.md
	docsDir := filepath.Join(projectRoot, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("failed to create docs directory: %v", err)
	}

	contextPath := filepath.Join(docsDir, "CONTEXT.md")
	existingContent := `# Project Context

## Features Completed
- **existing-feature**: completed 2025-01-01, 1 waves, 2 agents
  - IMPL doc: IMPL-existing.yaml
`
	if err := os.WriteFile(contextPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing context file: %v", err)
	}

	// Create a test manifest
	manifestPath := filepath.Join(tempDir, "IMPL-new-feature.yaml")
	manifestContent := `title: "New Feature"
feature_slug: new-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: X
        task: "Test task X"
        files:
          - new.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	// Call UpdateContext
	result, err := UpdateContext(manifestPath, projectRoot)
	if err != nil {
		t.Fatalf("UpdateContext failed: %v", err)
	}

	// Verify result
	if !result.Updated {
		t.Error("expected Updated=true")
	}

	// Verify file was appended to
	content, err := os.ReadFile(result.ContextPath)
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	contentStr := string(content)

	// Check both entries exist
	if !strings.Contains(contentStr, "- **existing-feature**: completed 2025-01-01, 1 waves, 2 agents") {
		t.Error("existing entry was lost")
	}
	if !strings.Contains(contentStr, "- **new-feature**: completed") {
		t.Error("new entry was not appended")
	}
	if !strings.Contains(contentStr, "1 waves, 1 agents") {
		t.Error("new entry has incorrect wave/agent count")
	}
}

func TestUpdateContext_InvalidRoot(t *testing.T) {
	// Use a nonexistent manifest path
	manifestPath := "/nonexistent/path/to/manifest.yaml"
	projectRoot := "/tmp"

	// Call UpdateContext - should fail
	_, err := UpdateContext(manifestPath, projectRoot)
	if err == nil {
		t.Error("expected error for nonexistent manifest, got nil")
	}

	// Error should mention loading manifest
	if !strings.Contains(err.Error(), "failed to load manifest") {
		t.Errorf("expected manifest load error, got: %v", err)
	}
}

func TestUpdateContext_ReadOnlyDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping read-only test when running as root")
	}

	// Create temp directory for test
	tempDir := t.TempDir()

	// Create a test manifest
	manifestPath := filepath.Join(tempDir, "IMPL-test.yaml")
	manifestContent := `title: "Test"
feature_slug: test
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: "Test"
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	// Create a read-only project root
	readOnlyRoot := filepath.Join(tempDir, "readonly")
	if err := os.MkdirAll(readOnlyRoot, 0555); err != nil {
		t.Fatalf("failed to create read-only directory: %v", err)
	}
	defer os.Chmod(readOnlyRoot, 0755) // Cleanup

	// Call UpdateContext - should fail
	_, err := UpdateContext(manifestPath, readOnlyRoot)
	if err == nil {
		t.Error("expected error for read-only directory, got nil")
	}
}

func TestUpdateContext_ZeroWaves(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	projectRoot := tempDir

	// Create a test manifest with no waves
	manifestPath := filepath.Join(tempDir, "IMPL-zero-waves.yaml")
	manifestContent := `title: "Zero Waves Feature"
feature_slug: zero-waves
verdict: SUITABLE
waves: []
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	// Call UpdateContext
	result, err := UpdateContext(manifestPath, projectRoot)
	if err != nil {
		t.Fatalf("UpdateContext failed: %v", err)
	}

	// Verify entry shows 0 waves, 0 agents
	content, err := os.ReadFile(result.ContextPath)
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "0 waves, 0 agents") {
		t.Errorf("expected '0 waves, 0 agents', got: %s", contentStr)
	}
}
