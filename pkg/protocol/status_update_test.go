package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUpdateStatus_NewReport tests creating a new completion report for an agent with no existing report.
func TestUpdateStatus_NewReport(t *testing.T) {
	// Create a temporary directory and manifest file
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	// Write a minimal valid manifest with one wave and one agent
	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// Update status for agent A (provide commit so the complete guard passes)
	res := UpdateStatus(manifestPath, 1, "A", "complete", UpdateStatusOpts{Commit: "abc123"})
	if res.IsFatal() {
		t.Fatalf("UpdateStatus failed: %+v", res.Errors)
	}
	result := res.GetData()

	// Verify result
	if result.Wave != 1 {
		t.Errorf("Expected wave=1, got %d", result.Wave)
	}
	if result.Agent != "A" {
		t.Errorf("Expected agent=A, got %s", result.Agent)
	}
	if result.OldStatus != "" {
		t.Errorf("Expected old status to be empty, got %q", result.OldStatus)
	}
	if result.NewStatus != "complete" {
		t.Errorf("Expected new status=complete, got %q", result.NewStatus)
	}
	if !result.Updated {
		t.Errorf("Expected Updated=true, got false")
	}

	// Verify manifest was saved correctly
	manifest, err := Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	report, exists := manifest.CompletionReports["A"]
	if !exists {
		t.Fatalf("Expected completion report for agent A to exist")
	}
	if report.Status != "complete" {
		t.Errorf("Expected status=complete, got %q", report.Status)
	}
}

// TestUpdateStatus_ExistingReport tests updating an existing completion report.
func TestUpdateStatus_ExistingReport(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	// Write manifest with existing completion report
	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: B
        task: Test task
        files:
          - test.go
completion_reports:
  B:
    status: partial
    worktree: .claude/worktrees/wave1-agent-B
    branch: wave1-agent-B
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// Update status from partial to complete (provide commit so the complete guard passes)
	res := UpdateStatus(manifestPath, 1, "B", "complete", UpdateStatusOpts{Commit: "def456"})
	if res.IsFatal() {
		t.Fatalf("UpdateStatus failed: %+v", res.Errors)
	}
	result := res.GetData()

	// Verify result
	if result.OldStatus != "partial" {
		t.Errorf("Expected old status=partial, got %q", result.OldStatus)
	}
	if result.NewStatus != "complete" {
		t.Errorf("Expected new status=complete, got %q", result.NewStatus)
	}

	// Verify manifest was saved correctly
	manifest, err := Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	report, exists := manifest.CompletionReports["B"]
	if !exists {
		t.Fatalf("Expected completion report for agent B to exist")
	}
	if report.Status != "complete" {
		t.Errorf("Expected status=complete, got %q", report.Status)
	}

	// Verify other fields were preserved
	if report.Worktree != ".claude/worktrees/wave1-agent-B" {
		t.Errorf("Expected worktree to be preserved, got %q", report.Worktree)
	}
	if report.Branch != "wave1-agent-B" {
		t.Errorf("Expected branch to be preserved, got %q", report.Branch)
	}
}

// TestUpdateStatus_AgentNotFound tests error handling when agent ID is invalid.
func TestUpdateStatus_AgentNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	// Write manifest with one agent
	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// Try to update status for non-existent agent
	res := UpdateStatus(manifestPath, 1, "Z", "complete", UpdateStatusOpts{})
	if !res.IsFatal() {
		t.Fatal("Expected fatal result for non-existent agent")
	}
}

// TestUpdateStatus_WaveNotFound tests error handling when wave number is invalid.
func TestUpdateStatus_WaveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	// Write manifest with wave 1 only
	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// Try to update status for wave 99
	res := UpdateStatus(manifestPath, 99, "A", "complete", UpdateStatusOpts{})
	if !res.IsFatal() {
		t.Fatal("Expected fatal result for non-existent wave")
	}
}

// TestUpdateStatus_CompleteRequiresCommit tests that setting status=complete without a commit fails.
func TestUpdateStatus_CompleteRequiresCommit(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// No prior completion report, no commit in opts — should fail
	res := UpdateStatus(manifestPath, 1, "A", StatusComplete, UpdateStatusOpts{})
	if !res.IsFatal() {
		t.Fatal("Expected fatal result when setting complete without a commit")
	}

	// Verify error code
	if len(res.Errors) == 0 {
		t.Fatal("Expected at least one error")
	}
	if res.Errors[0].Code != "G003_COMMIT_MISSING" {
		t.Errorf("Expected code G003_COMMIT_MISSING, got %q", res.Errors[0].Code)
	}
}

// TestUpdateStatus_CompleteWithCommitInOpts tests that providing a commit in opts succeeds.
func TestUpdateStatus_CompleteWithCommitInOpts(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// Provide commit in opts — should succeed
	res := UpdateStatus(manifestPath, 1, "A", StatusComplete, UpdateStatusOpts{Commit: "abc123"})
	if res.IsFatal() {
		t.Fatalf("UpdateStatus failed: %+v", res.Errors)
	}

	// Verify saved report has the commit
	manifest, err := Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	report, exists := manifest.CompletionReports["A"]
	if !exists {
		t.Fatalf("Expected completion report for agent A to exist")
	}
	if report.Commit != "abc123" {
		t.Errorf("Expected commit=abc123, got %q", report.Commit)
	}
	if report.Status != StatusComplete {
		t.Errorf("Expected status=complete, got %q", report.Status)
	}
}

// TestUpdateStatus_CompleteWithExistingCommit tests that an existing commit in the report satisfies the guard.
func TestUpdateStatus_CompleteWithExistingCommit(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	// Manifest with existing completion report containing a commit SHA
	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
completion_reports:
  A:
    status: partial
    commit: existing-sha
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// No commit in opts but existing report has one — should succeed
	res := UpdateStatus(manifestPath, 1, "A", StatusComplete, UpdateStatusOpts{})
	if res.IsFatal() {
		t.Fatalf("UpdateStatus failed: %+v", res.Errors)
	}

	// Verify saved report still has the original commit
	manifest, err := Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	report, exists := manifest.CompletionReports["A"]
	if !exists {
		t.Fatalf("Expected completion report for agent A to exist")
	}
	if report.Commit != "existing-sha" {
		t.Errorf("Expected commit=existing-sha, got %q", report.Commit)
	}
	if report.Status != StatusComplete {
		t.Errorf("Expected status=complete, got %q", report.Status)
	}
}

// TestUpdateStatus_PartialNoCommitRequired tests that partial status does not require a commit.
func TestUpdateStatus_PartialNoCommitRequired(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")

	manifestYAML := `title: Test Manifest
feature_slug: test-feature
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test task
        files:
          - test.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("Failed to write test manifest: %v", err)
	}

	// partial status with no commit — should succeed
	res := UpdateStatus(manifestPath, 1, "A", StatusPartial, UpdateStatusOpts{})
	if res.IsFatal() {
		t.Fatalf("UpdateStatus failed unexpectedly: %+v", res.Errors)
	}

	data := res.GetData()
	if data.NewStatus != StatusPartial {
		t.Errorf("Expected new status=partial, got %q", data.NewStatus)
	}
}
