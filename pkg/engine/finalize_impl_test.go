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
  - file: pkg/auth/handler.go
    wave: 1
    agent: A
  - file: pkg/auth/handler_test.go
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
	res := FinalizeIMPLEngine(ctx, FinalizeIMPLEngineOpts{IMPLPath: implPath, RepoRoot: repoRoot})

	// Verify: result matches protocol.FinalizeIMPL output
	if !res.IsSuccess() {
		t.Fatalf("expected success, got failure. Errors: %+v", res.Errors)
	}
	data := res.GetData()

	// Should have H2 data available
	if !data.GatePopulation.H2DataAvailable {
		t.Error("expected H2DataAvailable=true")
	}

	// Should detect Make toolchain (Makefile takes precedence)
	if data.GatePopulation.Toolchain != "make" {
		t.Errorf("expected toolchain=make, got %s", data.GatePopulation.Toolchain)
	}

	// Should update 1 agent (added verification gate)
	if data.GatePopulation.AgentsUpdated != 1 {
		t.Errorf("expected 1 agent updated, got %d", data.GatePopulation.AgentsUpdated)
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

	res := FinalizeIMPLEngine(ctx, FinalizeIMPLEngineOpts{IMPLPath: implPath, RepoRoot: repoRoot})

	// Verify: returns fatal result with cancellation code
	if !res.IsFatal() {
		t.Error("expected fatal result when context is cancelled, got non-fatal")
	}

	// Should not be success
	if res.IsSuccess() {
		t.Errorf("expected non-success result when context cancelled, got: %+v", res)
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

	res := FinalizeIMPLEngine(ctx, FinalizeIMPLEngineOpts{IMPLPath: implPath, RepoRoot: repoRoot})

	// Verify: should return fatal result or success (timing-dependent)
	// Note: timing-dependent - might complete before timeout, so we allow both outcomes
	if res.IsSuccess() {
		// Operation completed before timeout - this is OK
		t.Logf("operation completed before context timeout (timing-dependent)")
	} else if !res.IsFatal() {
		t.Errorf("expected fatal result on context cancellation, got partial: %+v", res)
	}
}

// TestFinalizeIMPLEngine_ValidationParameters tests parameter validation.
func TestFinalizeIMPLEngine_ValidationParameters(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		implPath string
		repoRoot string
		wantMsg  string
	}{
		{
			name:     "missing implPath",
			implPath: "",
			repoRoot: "/tmp/repo",
			wantMsg:  "implPath is required",
		},
		{
			name:     "missing repoRoot",
			implPath: "/tmp/IMPL.yaml",
			repoRoot: "",
			wantMsg:  "repoRoot is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := FinalizeIMPLEngine(ctx, FinalizeIMPLEngineOpts{IMPLPath: tt.implPath, RepoRoot: tt.repoRoot})

			if !res.IsFatal() {
				t.Errorf("expected fatal result for %q, got: %+v", tt.wantMsg, res)
			} else if len(res.Errors) == 0 || !containsString(res.Errors[0].Message, tt.wantMsg) {
				t.Errorf("expected error containing %q, got errors: %+v", tt.wantMsg, res.Errors)
			}

			if res.IsSuccess() {
				t.Errorf("expected non-success result on parameter validation error, got: %+v", res)
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
