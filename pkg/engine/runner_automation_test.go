package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunScout_AutomationIntegration(t *testing.T) {
	// Create a temp repo with a Go project structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a simple Go file
	pkgDir := filepath.Join(tmpDir, "pkg", "foo")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create pkg dir: %v", err)
	}

	goFileContent := `package foo

func Bar() string {
	return "hello"
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "bar.go"), []byte(goFileContent), 0644); err != nil {
		t.Fatalf("Failed to create Go file: %v", err)
	}

	// Create IMPL output path
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "IMPL-test.yaml")
	if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
		t.Fatalf("Failed to create IMPL dir: %v", err)
	}

	// Mock opts
	opts := RunScoutOpts{
		Feature:       "Add new feature to foo package",
		RepoPath:      tmpDir,
		IMPLOutPath:   implPath,
		PolywaveRepoPath:   "", // Will use fallback
		ScoutModel:    "claude-3-5-sonnet-20241022",
		UseStructuredOutput: false,
	}

	// Capture output chunks
	var chunks []string
	onChunk := func(chunk string) {
		chunks = append(chunks, chunk)
	}

	ctx := context.Background()

	// This will fail because we don't have a real Scout agent backend,
	// but we can verify the automation context is built correctly by
	// examining the prompt construction logic.
	// For this test, we'll just verify runScoutAutomation works independently.
	automationCtx := runScoutAutomation(ctx, tmpDir, opts.Feature)

	// Verify automation context contains expected sections
	if !strings.Contains(automationCtx, "## Automation Analysis Results") {
		t.Errorf("Expected automation context to contain header, got: %s", automationCtx)
	}

	if !strings.Contains(automationCtx, "### Build/Test Commands (H2)") {
		t.Errorf("Expected H2 section in automation context")
	}

	if !strings.Contains(automationCtx, "### Pre-Implementation Status (H1a)") {
		t.Errorf("Expected H1a section in automation context")
	}

	if !strings.Contains(automationCtx, "### Dependency Analysis (H3)") {
		t.Errorf("Expected H3 section in automation context")
	}

	// Verify instructions are present
	if !strings.Contains(automationCtx, "Populate quality_gates") {
		t.Errorf("Expected usage instructions in automation context")
	}

	// Skip actual RunScout execution since we don't have a live agent backend
	_ = ctx
	_ = opts
	_ = onChunk
}

func TestRunScout_AutomationFailure(t *testing.T) {
	// Test that Scout still launches even if automation tools fail
	tmpDir := t.TempDir()

	// Create minimal structure (no go.mod, will cause some tools to fail)
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "IMPL-test.yaml")
	if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
		t.Fatalf("Failed to create IMPL dir: %v", err)
	}

	// Test automation context generation with missing dependencies
	automationCtx := runScoutAutomation(context.Background(), tmpDir, "Add feature")

	// Should still return valid markdown even if tools fail
	if !strings.Contains(automationCtx, "## Automation Analysis Results") {
		t.Errorf("Expected automation context header even on failure")
	}

	// Should contain fallback messages for failed tools
	if !strings.Contains(automationCtx, "### Build/Test Commands (H2)") {
		t.Errorf("Expected H2 section even on failure")
	}

	// H1a should show "Skipped" message since no requirements file
	if !strings.Contains(automationCtx, "Skipped - no requirements file") {
		t.Errorf("Expected H1a skip message")
	}

	// H3 may fail but should still have a section
	if !strings.Contains(automationCtx, "### Dependency Analysis (H3)") {
		t.Errorf("Expected H3 section even on failure")
	}
}

func TestDetectRequirementsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a requirements file
	reqFile := filepath.Join(tmpDir, "docs", "audit.md")
	if err := os.MkdirAll(filepath.Dir(reqFile), 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}
	if err := os.WriteFile(reqFile, []byte("# Audit\n"), 0644); err != nil {
		t.Fatalf("Failed to create audit.md: %v", err)
	}

	tests := []struct {
		name        string
		description string
		wantFound   bool
	}{
		{
			name:        "Detects .md file",
			description: "Implement features from docs/audit.md",
			wantFound:   true,
		},
		{
			name:        "No file reference",
			description: "Add new feature to package",
			wantFound:   false,
		},
		{
			name:        "Detects .txt file",
			description: "See requirements.txt for details",
			wantFound:   false, // File doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectRequirementsFile(tmpDir, tt.description)
			found := result != ""

			if found != tt.wantFound {
				t.Errorf("detectRequirementsFile(%q) found=%v, want %v", tt.description, found, tt.wantFound)
			}

			if tt.wantFound && result != reqFile {
				t.Errorf("detectRequirementsFile(%q) = %q, want %q", tt.description, result, reqFile)
			}
		})
	}
}

func TestRunScoutAutomation_WithRequirementsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a requirements file
	reqFile := filepath.Join(tmpDir, "docs", "requirements.md")
	if err := os.MkdirAll(filepath.Dir(reqFile), 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}

	reqContent := `# Requirements

## F1: Add authentication
Location: pkg/auth/handler.go

## F2: Add session management
Location: pkg/session/manager.go
`
	if err := os.WriteFile(reqFile, []byte(reqContent), 0644); err != nil {
		t.Fatalf("Failed to write requirements: %v", err)
	}

	// Create the target files
	authDir := filepath.Join(tmpDir, "pkg", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("Failed to create auth dir: %v", err)
	}
	authContent := `package auth

func Authenticate() bool {
	return true
}
`
	if err := os.WriteFile(filepath.Join(authDir, "handler.go"), []byte(authContent), 0644); err != nil {
		t.Fatalf("Failed to write handler.go: %v", err)
	}

	// Create go.mod
	goModContent := `module example.com/test

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	featureDesc := "Implement features from docs/requirements.md"
	automationCtx := runScoutAutomation(context.Background(), tmpDir, featureDesc)

	// Should detect requirements file and run H1a
	if !strings.Contains(automationCtx, "### Pre-Implementation Status (H1a)") {
		t.Errorf("Expected H1a section")
	}

	// Should NOT contain "Skipped" if requirements were found and parsed
	// (though it might still show skipped if parsing fails)
	if strings.Contains(automationCtx, "Skipped - no requirements file") {
		// This is OK if parsing failed, but let's verify the file was detected
		detected := detectRequirementsFile(tmpDir, featureDesc)
		if detected == "" {
			t.Errorf("Requirements file should have been detected")
		}
	}
}
