package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFinalizeIMPLEngine_Success tests the engine integration with a valid IMPL doc.
func TestFinalizeIMPLEngine_Success(t *testing.T) {
	// Setup: Create temp directory
	tmpDir := t.TempDir()

	// Create repo root with go.mod and build system files
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatalf("failed to create repo root: %v", err)
	}

	// Create go.mod for H2 toolchain detection
	goModContent := `module example.com/test
go 1.22
`
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create Makefile with build/test/lint commands
	makefileContent := `build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...
`
	if err := os.WriteFile(filepath.Join(repoRoot, "Makefile"), []byte(makefileContent), 0644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}

	// Create pkg directory structure
	pkgDir := filepath.Join(repoRoot, "pkg", "auth")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg directory: %v", err)
	}

	// Create a valid IMPL doc without verification gates
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	implContent := `title: "Test Feature"
description: "Test description"
feature_slug: test-feature
verdict: SUITABLE
h1_analysis:
  feature_type: enhancement
  complexity: standard
  dependencies: []
file_ownership:
  - repo: .
    file: pkg/auth/handler.go
    wave: 1
    agent: A
  - repo: .
    file: pkg/auth/handler_test.go
    wave: 1
    agent: A
waves:
  - number: 1
    agents:
      - id: A
        files:
          - pkg/auth/handler.go
          - pkg/auth/handler_test.go
        task: "Implement auth handler."
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("failed to write IMPL file: %v", err)
	}

	// Run: FinalizeIMPLEngine with context
	ctx := context.Background()
	result, err := FinalizeIMPLEngine(ctx, implPath, repoRoot)

	// Verify: no error
	if err != nil {
		t.Errorf("FinalizeIMPLEngine returned error: %v", err)
	}

	// Verify: result matches protocol.FinalizeIMPL output
	if result == nil {
		t.Fatal("result is nil")
	}

	// Should succeed (validation passes, gates populated)
	if !result.Success {
		t.Errorf("expected Success=true, got false. Validation errors: %+v, Final validation errors: %+v",
			result.Validation.Errors, result.FinalValidation.Errors)
	}

	// Should have H2 data available
	if !result.GatePopulation.H2DataAvailable {
		t.Error("expected H2DataAvailable=true")
	}

	// Should detect Make toolchain (Makefile takes precedence)
	if result.GatePopulation.Toolchain != "make" {
		t.Errorf("expected toolchain=make, got %s", result.GatePopulation.Toolchain)
	}

	// Should update 1 agent (added verification gate)
	if result.GatePopulation.AgentsUpdated != 1 {
		t.Errorf("expected 1 agent updated, got %d", result.GatePopulation.AgentsUpdated)
	}

	// Verify: IMPL file was updated on disk
	updatedContent, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("failed to read updated IMPL file: %v", err)
	}

	// Should contain verification gate
	updatedStr := string(updatedContent)
	if !containsString(updatedStr, "## Verification Gate") {
		t.Error("updated IMPL should contain '## Verification Gate'")
	}
	if !containsString(updatedStr, "make build") {
		t.Error("updated IMPL should contain 'make build' command")
	}
	if !containsString(updatedStr, "make test") {
		t.Error("updated IMPL should contain 'make test' command")
	}
}

// TestFinalizeIMPLEngine_ContextCancellation tests that the engine respects context cancellation.
func TestFinalizeIMPLEngine_ContextCancellation(t *testing.T) {
	// Setup: Create temp IMPL file (minimal, just needs to exist)
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	implContent := `title: "Test"
description: "Test"
feature_slug: test
verdict: SUITABLE
h1_analysis:
  feature_type: enhancement
  complexity: standard
  dependencies: []
file_ownership:
  - repo: .
    file: test.go
    wave: 1
    agent: A
waves:
  - number: 1
    agents:
      - id: A
        files: ["test.go"]
        task: "Test task"
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("failed to write IMPL file: %v", err)
	}

	repoRoot := tmpDir

	// Run: FinalizeIMPLEngine with pre-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := FinalizeIMPLEngine(ctx, implPath, repoRoot)

	// Verify: returns context.Canceled error
	if err == nil {
		t.Error("expected error when context is cancelled, got nil")
	}

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}

	// Result should be nil when context cancelled before execution
	if result != nil {
		t.Errorf("expected nil result when context cancelled, got: %+v", result)
	}
}

// TestFinalizeIMPLEngine_ContextCancellationDuringExecution tests cancellation during operation.
func TestFinalizeIMPLEngine_ContextCancellationDuringExecution(t *testing.T) {
	// Setup: Create temp IMPL file
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	implContent := `title: "Test"
description: "Test"
feature_slug: test
verdict: SUITABLE
h1_analysis:
  feature_type: enhancement
  complexity: standard
  dependencies: []
file_ownership:
  - repo: .
    file: test.go
    wave: 1
    agent: A
waves:
  - number: 1
    agents:
      - id: A
        files: ["test.go"]
        task: "Test task"
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("failed to write IMPL file: %v", err)
	}

	repoRoot := tmpDir

	// Run: FinalizeIMPLEngine with context that cancels during execution
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Add small delay to increase chance of catching cancellation during execution
	time.Sleep(2 * time.Millisecond)

	result, err := FinalizeIMPLEngine(ctx, implPath, repoRoot)

	// Verify: should return error (either context.Canceled or context.DeadlineExceeded)
	// Note: timing-dependent - might complete before timeout, so we allow both outcomes
	if err == nil && result != nil {
		// Operation completed before timeout - this is OK
		t.Logf("operation completed before context timeout (timing-dependent)")
	} else if err != nil {
		// Should be a context error
		if err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("expected context error, got: %v", err)
		}
	}
}

// TestFinalizeIMPLEngine_ValidationParameters tests parameter validation.
func TestFinalizeIMPLEngine_ValidationParameters(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		implPath string
		repoRoot string
		wantErr  string
	}{
		{
			name:     "missing implPath",
			implPath: "",
			repoRoot: "/tmp/repo",
			wantErr:  "implPath is required",
		},
		{
			name:     "missing repoRoot",
			implPath: "/tmp/IMPL.yaml",
			repoRoot: "",
			wantErr:  "repoRoot is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FinalizeIMPLEngine(ctx, tt.implPath, tt.repoRoot)

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
			} else if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}

			if result != nil {
				t.Errorf("expected nil result on parameter validation error, got: %+v", result)
			}
		})
	}
}

// containsString checks if haystack contains needle (case-sensitive substring match).
func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		findSubstring(haystack, needle)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
