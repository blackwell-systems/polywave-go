package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"gopkg.in/yaml.v3"
)

func TestCleanup_AllRemoved(t *testing.T) {
	// Create a temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user (required for commits)
	if _, err := git.Run(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}

	// Create an initial commit (required for worktree creation)
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	if _, err := git.Run(tmpDir, "add", "README.md"); err != nil {
		t.Fatalf("failed to add README: %v", err)
	}
	if _, err := git.Run(tmpDir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Create manifest with a wave containing two agents
	manifest := &IMPLManifest{
		Title:       "test-cleanup",
		FeatureSlug: "test-cleanup",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Task A", Files: []string{"a.go"}},
					{ID: "B", Task: "Task B", Files: []string{"b.go"}},
				},
			},
		},
	}

	// Write manifest to file
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create worktrees directory
	worktreesDir := filepath.Join(tmpDir, ".claude", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}

	// Create worktrees and branches for both agents
	worktreePathA := filepath.Join(worktreesDir, "wave1-agent-A")
	if err := git.WorktreeAdd(tmpDir, worktreePathA, "wave1-agent-A"); err != nil {
		t.Fatalf("failed to create worktree for agent A: %v", err)
	}

	worktreePathB := filepath.Join(worktreesDir, "wave1-agent-B")
	if err := git.WorktreeAdd(tmpDir, worktreePathB, "wave1-agent-B"); err != nil {
		t.Fatalf("failed to create worktree for agent B: %v", err)
	}

	// Verify worktrees and branches exist before cleanup
	worktrees, err := git.WorktreeList(tmpDir)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees before cleanup, got %d", len(worktrees))
	}

	// Run cleanup
	result, err := Cleanup(manifestPath, 1, tmpDir)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify result structure
	if result.Wave != 1 {
		t.Errorf("expected Wave=1, got %d", result.Wave)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("expected 2 agent statuses, got %d", len(result.Agents))
	}

	// Verify both agents have successful cleanup
	for _, status := range result.Agents {
		if !status.WorktreeRemoved {
			t.Errorf("agent %s: expected WorktreeRemoved=true, got false", status.Agent)
		}
		if !status.BranchDeleted {
			t.Errorf("agent %s: expected BranchDeleted=true, got false", status.Agent)
		}
	}

	// Verify worktrees are actually gone
	worktreesAfter, err := git.WorktreeList(tmpDir)
	if err != nil {
		t.Fatalf("failed to list worktrees after cleanup: %v", err)
	}
	if len(worktreesAfter) != 0 {
		t.Errorf("expected 0 worktrees after cleanup, got %d", len(worktreesAfter))
	}

	// Verify branches are actually gone
	out, err := git.Run(tmpDir, "branch", "--list")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if len(out) > 10 { // Should only contain "* main" or "* master"
		t.Errorf("branches still exist after cleanup: %s", out)
	}
}

func TestCleanup_AlreadyGone(t *testing.T) {
	// Create a temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	if _, err := git.Run(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}

	// Create an initial commit
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	if _, err := git.Run(tmpDir, "add", "README.md"); err != nil {
		t.Fatalf("failed to add README: %v", err)
	}
	if _, err := git.Run(tmpDir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Create manifest with a wave containing one agent
	manifest := &IMPLManifest{
		Title:       "test-cleanup",
		FeatureSlug: "test-cleanup",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "C", Task: "Task C", Files: []string{"c.go"}},
				},
			},
		},
	}

	// Write manifest to file
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create worktrees directory (but don't create actual worktrees)
	worktreesDir := filepath.Join(tmpDir, ".claude", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}

	// Run cleanup on nonexistent worktrees/branches (idempotent test)
	result, err := Cleanup(manifestPath, 1, tmpDir)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify result structure
	if result.Wave != 1 {
		t.Errorf("expected Wave=1, got %d", result.Wave)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent status, got %d", len(result.Agents))
	}

	// Verify cleanup is idempotent (already gone = success)
	status := result.Agents[0]
	if status.Agent != "C" {
		t.Errorf("expected Agent=C, got %s", status.Agent)
	}
	if !status.WorktreeRemoved {
		t.Errorf("expected WorktreeRemoved=true for nonexistent worktree (idempotent), got false")
	}
	if !status.BranchDeleted {
		t.Errorf("expected BranchDeleted=true for nonexistent branch (idempotent), got false")
	}
}

func TestCleanup_WaveNotFound(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create manifest with wave 1 only
	manifest := &IMPLManifest{
		Title:       "test-cleanup",
		FeatureSlug: "test-cleanup",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Task A", Files: []string{"a.go"}},
				},
			},
		},
	}

	// Write manifest to file
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Try to cleanup wave 2 (doesn't exist)
	result, err := Cleanup(manifestPath, 2, tmpDir)
	if err == nil {
		t.Fatalf("expected error for nonexistent wave, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result for nonexistent wave, got %+v", result)
	}

	// Verify error message mentions the wave number
	expectedErrMsg := "wave 2 not found in manifest"
	if err.Error() != expectedErrMsg {
		t.Errorf("expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestCleanup_PartialFailure(t *testing.T) {
	// Create a temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	if _, err := git.Run(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}

	// Create an initial commit
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	if _, err := git.Run(tmpDir, "add", "README.md"); err != nil {
		t.Fatalf("failed to add README: %v", err)
	}
	if _, err := git.Run(tmpDir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Create manifest with a wave containing two agents
	manifest := &IMPLManifest{
		Title:       "test-cleanup",
		FeatureSlug: "test-cleanup",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "D", Task: "Task D", Files: []string{"d.go"}},
					{ID: "E", Task: "Task E", Files: []string{"e.go"}},
				},
			},
		},
	}

	// Write manifest to file
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create worktrees directory
	worktreesDir := filepath.Join(tmpDir, ".claude", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}

	// Create worktree and branch for agent D only (agent E doesn't exist)
	worktreePathD := filepath.Join(worktreesDir, "wave1-agent-D")
	if err := git.WorktreeAdd(tmpDir, worktreePathD, "wave1-agent-D"); err != nil {
		t.Fatalf("failed to create worktree for agent D: %v", err)
	}

	// Run cleanup (should handle both existing and nonexistent gracefully)
	result, err := Cleanup(manifestPath, 1, tmpDir)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify result structure
	if len(result.Agents) != 2 {
		t.Fatalf("expected 2 agent statuses, got %d", len(result.Agents))
	}

	// Find statuses for D and E
	var statusD, statusE *CleanupStatus
	for i := range result.Agents {
		if result.Agents[i].Agent == "D" {
			statusD = &result.Agents[i]
		} else if result.Agents[i].Agent == "E" {
			statusE = &result.Agents[i]
		}
	}

	if statusD == nil {
		t.Fatalf("missing cleanup status for agent D")
	}
	if statusE == nil {
		t.Fatalf("missing cleanup status for agent E")
	}

	// Agent D should be fully cleaned up
	if !statusD.WorktreeRemoved {
		t.Errorf("agent D: expected WorktreeRemoved=true, got false")
	}
	if !statusD.BranchDeleted {
		t.Errorf("agent D: expected BranchDeleted=true, got false")
	}

	// Agent E should report idempotent success (already gone)
	if !statusE.WorktreeRemoved {
		t.Errorf("agent E: expected WorktreeRemoved=true (idempotent), got false")
	}
	if !statusE.BranchDeleted {
		t.Errorf("agent E: expected BranchDeleted=true (idempotent), got false")
	}
}
