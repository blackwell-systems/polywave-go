package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/suitability"
)

func TestAnalyzeSuitabilityCmd_ValidRequirements(t *testing.T) {
	// Create a temporary requirements document
	requirementsContent := `# Test Requirements

## F1: Add authentication handler
This feature adds user authentication.
Location: pkg/auth/handler.go

## F2: Add session timeout
This feature adds session management.
Location: pkg/session/timeout.go
`

	tmpDir := t.TempDir()
	requirementsFile := filepath.Join(tmpDir, "requirements.md")
	if err := os.WriteFile(requirementsFile, []byte(requirementsContent), 0644); err != nil {
		t.Fatalf("failed to write requirements file: %v", err)
	}

	// Create test repository structure
	repoRoot := filepath.Join(tmpDir, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "pkg/auth"), 0755)
	os.MkdirAll(filepath.Join(repoRoot, "pkg/session"), 0755)

	// Write handler.go (simulates DONE status)
	handlerContent := `package auth

func AuthHandler() {
	// Full implementation
}
`
	os.WriteFile(filepath.Join(repoRoot, "pkg/auth/handler.go"), []byte(handlerContent), 0644)

	// Write handler_test.go (test coverage)
	handlerTestContent := strings.Repeat("// Test line\n", 50) // 50 lines to simulate >70% coverage
	os.WriteFile(filepath.Join(repoRoot, "pkg/auth/handler_test.go"), []byte(handlerTestContent), 0644)

	// Execute command
	cmd := newAnalyzeSuitabilityCmd()
	cmd.SetArgs([]string{
		"--requirements", requirementsFile,
		"--repo-root", repoRoot,
		"--output", "json",
	})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Parse output
	var result suitability.SuitabilityResult
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Verify structure
	if result.PreImplementation.TotalItems != 2 {
		t.Errorf("expected 2 total items, got %d", result.PreImplementation.TotalItems)
	}

	if len(result.PreImplementation.ItemStatus) != 2 {
		t.Errorf("expected 2 item status entries, got %d", len(result.PreImplementation.ItemStatus))
	}

	// Verify F1 is DONE (has implementation + tests)
	var f1Status *suitability.ItemStatus
	for i := range result.PreImplementation.ItemStatus {
		if result.PreImplementation.ItemStatus[i].ID == "F1" {
			f1Status = &result.PreImplementation.ItemStatus[i]
			break
		}
	}

	if f1Status == nil {
		t.Fatal("F1 status not found in results")
	}

	if f1Status.Status != "DONE" {
		t.Errorf("expected F1 status DONE, got %s", f1Status.Status)
	}

	// Verify F2 is TODO (no implementation)
	var f2Status *suitability.ItemStatus
	for i := range result.PreImplementation.ItemStatus {
		if result.PreImplementation.ItemStatus[i].ID == "F2" {
			f2Status = &result.PreImplementation.ItemStatus[i]
			break
		}
	}

	if f2Status == nil {
		t.Fatal("F2 status not found in results")
	}

	if f2Status.Status != "TODO" {
		t.Errorf("expected F2 status TODO, got %s", f2Status.Status)
	}
}

func TestAnalyzeSuitabilityCmd_EmptyRequirements(t *testing.T) {
	// Create a requirements document with no valid items
	requirementsContent := `# Empty Requirements

This document has no valid requirements with Location fields.
`

	tmpDir := t.TempDir()
	requirementsFile := filepath.Join(tmpDir, "requirements.md")
	if err := os.WriteFile(requirementsFile, []byte(requirementsContent), 0644); err != nil {
		t.Fatalf("failed to write requirements file: %v", err)
	}

	repoRoot := tmpDir

	// Execute command
	cmd := newAnalyzeSuitabilityCmd()
	cmd.SetArgs([]string{
		"--requirements", requirementsFile,
		"--repo-root", repoRoot,
		"--output", "json",
	})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Should fail with "no valid requirements found"
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected command to fail with no valid requirements")
	} else if !strings.Contains(err.Error(), "no valid requirements found") {
		t.Errorf("expected 'no valid requirements found' error, got: %v", err)
	}
}

func TestAnalyzeSuitabilityCmd_InvalidRepoRoot(t *testing.T) {
	requirementsContent := `## F1: Test feature
Location: test.go
`

	tmpDir := t.TempDir()
	requirementsFile := filepath.Join(tmpDir, "requirements.md")
	if err := os.WriteFile(requirementsFile, []byte(requirementsContent), 0644); err != nil {
		t.Fatalf("failed to write requirements file: %v", err)
	}

	// Use non-existent directory as repo root
	nonExistentRoot := filepath.Join(tmpDir, "does-not-exist")

	cmd := newAnalyzeSuitabilityCmd()
	cmd.SetArgs([]string{
		"--requirements", requirementsFile,
		"--repo-root", nonExistentRoot,
		"--output", "json",
	})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Should fail (exact error depends on suitability.ScanPreImplementation behavior)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected command to fail with invalid repo root")
	}
}

func TestParseRequirements_Markdown(t *testing.T) {
	content := `# Requirements Document

## F1: Add authentication handler
This is a description.
Location: pkg/auth/handler.go

## SEC-01: Add session timeout
Multi-line description
can span multiple lines.
Location: pkg/session/timeout.go
Location: pkg/session/manager.go

## F3: Invalid requirement without location
This should be skipped.
`

	reqResult := suitability.ParseRequirements(content)
	if reqResult.IsFatal() {
		t.Fatalf("parse requirements: %v", reqResult.Errors)
	}
	requirements := reqResult.GetData()

	if len(requirements) != 2 {
		t.Fatalf("expected 2 requirements, got %d", len(requirements))
	}

	// Verify F1
	if requirements[0].ID != "F1" {
		t.Errorf("expected ID F1, got %s", requirements[0].ID)
	}
	if requirements[0].Description != "Add authentication handler" {
		t.Errorf("expected description 'Add authentication handler', got '%s'", requirements[0].Description)
	}
	if len(requirements[0].Files) != 1 || requirements[0].Files[0] != "pkg/auth/handler.go" {
		t.Errorf("expected file pkg/auth/handler.go, got %v", requirements[0].Files)
	}

	// Verify SEC-01 (multiple files)
	if requirements[1].ID != "SEC-01" {
		t.Errorf("expected ID SEC-01, got %s", requirements[1].ID)
	}
	if len(requirements[1].Files) != 2 {
		t.Errorf("expected 2 files for SEC-01, got %d", len(requirements[1].Files))
	}
}

func TestParseRequirements_PlainText(t *testing.T) {
	content := `F1: Add authentication | pkg/auth/handler.go
F2: Add session timeout | pkg/session/timeout.go
INVALID: Missing pipe delimiter
F3: Another feature | pkg/feature/feature.go
`

	reqResult := suitability.ParseRequirements(content)
	if reqResult.IsFatal() {
		t.Fatalf("parse requirements: %v", reqResult.Errors)
	}
	requirements := reqResult.GetData()

	if len(requirements) != 3 {
		t.Fatalf("expected 3 requirements, got %d", len(requirements))
	}

	// Verify F1
	if requirements[0].ID != "F1" {
		t.Errorf("expected ID F1, got %s", requirements[0].ID)
	}
	if requirements[0].Files[0] != "pkg/auth/handler.go" {
		t.Errorf("expected file pkg/auth/handler.go, got %v", requirements[0].Files)
	}

	// Verify F2
	if requirements[1].ID != "F2" {
		t.Errorf("expected ID F2, got %s", requirements[1].ID)
	}

	// Verify F3
	if requirements[2].ID != "F3" {
		t.Errorf("expected ID F3, got %s", requirements[2].ID)
	}
}

func TestParseRequirements_EmptyDocument(t *testing.T) {
	content := ""

	reqResult := suitability.ParseRequirements(content)
	if reqResult.IsFatal() {
		t.Fatalf("parse requirements: %v", reqResult.Errors)
	}
	requirements := reqResult.GetData()

	if len(requirements) != 0 {
		t.Errorf("expected 0 requirements from empty document, got %d", len(requirements))
	}
}

func TestParseRequirements_NoLocations(t *testing.T) {
	content := `## F1: Feature without location
This should be skipped.

## F2: Another feature without location
Also skipped.
`

	reqResult := suitability.ParseRequirements(content)
	if reqResult.IsFatal() {
		t.Fatalf("parse requirements: %v", reqResult.Errors)
	}
	requirements := reqResult.GetData()

	if len(requirements) != 0 {
		t.Errorf("expected 0 requirements (no locations), got %d", len(requirements))
	}
}
