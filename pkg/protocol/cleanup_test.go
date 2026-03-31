package protocol

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

	// Create worktrees and branches for both agents (slug-scoped)
	sawDir := filepath.Join(worktreesDir, "saw", "test-cleanup")
	if err := os.MkdirAll(sawDir, 0755); err != nil {
		t.Fatalf("failed to create saw dir: %v", err)
	}

	worktreePathA := filepath.Join(sawDir, "wave1-agent-A")
	if err := git.WorktreeAdd(tmpDir, worktreePathA, "saw/test-cleanup/wave1-agent-A"); err != nil {
		t.Fatalf("failed to create worktree for agent A: %v", err)
	}

	worktreePathB := filepath.Join(sawDir, "wave1-agent-B")
	if err := git.WorktreeAdd(tmpDir, worktreePathB, "saw/test-cleanup/wave1-agent-B"); err != nil {
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
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	result := cleanupResult.GetData()

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
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	result := cleanupResult.GetData()

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
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 2, tmpDir, nil)
	if err == nil {
		t.Fatalf("expected error for nonexistent wave, got nil")
	}
	if cleanupResult.IsSuccess() {
		t.Errorf("expected non-success result for nonexistent wave, got success")
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

	// Create worktree and branch for agent D only (agent E doesn't exist) — slug-scoped
	sawDir := filepath.Join(worktreesDir, "saw", "test-cleanup")
	if err := os.MkdirAll(sawDir, 0755); err != nil {
		t.Fatalf("failed to create saw dir: %v", err)
	}
	worktreePathD := filepath.Join(sawDir, "wave1-agent-D")
	if err := git.WorktreeAdd(tmpDir, worktreePathD, "saw/test-cleanup/wave1-agent-D"); err != nil {
		t.Fatalf("failed to create worktree for agent D: %v", err)
	}

	// Run cleanup (should handle both existing and nonexistent gracefully)
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	result := cleanupResult.GetData()

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

// TestCleanup_ForcesDeleteUnmergedBranches verifies that cleanup force-deletes
// branches even when they haven't been fast-forward merged.
func TestCleanup_ForcesDeleteUnmergedBranches(t *testing.T) {
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

	// Create an initial commit on main
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

	// Get current branch name (could be main or master)
	out, err := git.Run(tmpDir, "branch", "--show-current")
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	mainBranch := strings.TrimSpace(out)

	// Create manifest
	manifest := &IMPLManifest{
		Title:       "test-cleanup",
		FeatureSlug: "test-cleanup",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "F", Task: "Task F", Files: []string{"f.go"}},
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

	// Create worktree and make a commit (slug-scoped)
	sawDir := filepath.Join(worktreesDir, "saw", "test-cleanup")
	if err := os.MkdirAll(sawDir, 0755); err != nil {
		t.Fatalf("failed to create saw dir: %v", err)
	}
	branchF := "saw/test-cleanup/wave1-agent-F"
	worktreePathF := filepath.Join(sawDir, "wave1-agent-F")
	if err := git.WorktreeAdd(tmpDir, worktreePathF, branchF); err != nil {
		t.Fatalf("failed to create worktree for agent F: %v", err)
	}

	// Make a commit in the worktree
	testFilePath := filepath.Join(worktreePathF, "f.go")
	if err := os.WriteFile(testFilePath, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if _, err := git.Run(worktreePathF, "add", "f.go"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := git.Run(worktreePathF, "commit", "-m", "Add f.go"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Switch back to main branch and merge with --no-ff
	if _, err := git.Run(tmpDir, "checkout", mainBranch); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := git.Run(tmpDir, "merge", "--no-ff", branchF, "-m", "Merge "+branchF); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	// Run cleanup (should force-delete the branch)
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	result := cleanupResult.GetData()

	// Verify cleanup succeeded
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent status, got %d", len(result.Agents))
	}
	status := result.Agents[0]
	if !status.BranchDeleted {
		t.Errorf("expected BranchDeleted=true, got false")
	}

	// Verify branch is actually gone
	_, err = git.Run(tmpDir, "rev-parse", "--verify", branchF)
	if err == nil {
		t.Errorf("branch %s still exists after cleanup", branchF)
	}
}

// TestCleanup_IdempotentBranchDeletion verifies that cleanup is idempotent
// when branches are already deleted.
func TestCleanup_IdempotentBranchDeletion(t *testing.T) {
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

	// Create manifest
	manifest := &IMPLManifest{
		Title:       "test-cleanup",
		FeatureSlug: "test-cleanup",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "G", Task: "Task G", Files: []string{"g.go"}},
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

	// Create and then manually delete worktree and branch (slug-scoped)
	sawDir := filepath.Join(worktreesDir, "saw", "test-cleanup")
	if err := os.MkdirAll(sawDir, 0755); err != nil {
		t.Fatalf("failed to create saw dir: %v", err)
	}
	branchG := "saw/test-cleanup/wave1-agent-G"
	worktreePathG := filepath.Join(sawDir, "wave1-agent-G")
	if err := git.WorktreeAdd(tmpDir, worktreePathG, branchG); err != nil {
		t.Fatalf("failed to create worktree for agent G: %v", err)
	}

	// Manually clean up (simulating previous cleanup or manual deletion)
	if _, err := git.Run(tmpDir, "worktree", "remove", "--force", worktreePathG); err != nil {
		t.Fatalf("failed to remove worktree: %v", err)
	}
	if _, err := git.Run(tmpDir, "branch", "-D", branchG); err != nil {
		t.Fatalf("failed to delete branch: %v", err)
	}

	// Run cleanup (should be idempotent)
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	result := cleanupResult.GetData()

	// Verify cleanup reports success (idempotent)
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent status, got %d", len(result.Agents))
	}
	status := result.Agents[0]
	if !status.WorktreeRemoved {
		t.Errorf("expected WorktreeRemoved=true (idempotent), got false")
	}
	if !status.BranchDeleted {
		t.Errorf("expected BranchDeleted=true (idempotent), got false")
	}
}

// TestCleanup_PrunesStaleWorktrees verifies that Cleanup calls git worktree prune
// to clean up stale worktree entries after removing individual worktrees.
func TestCleanup_PrunesStaleWorktrees(t *testing.T) {
	// Create a temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "cleanup-prune-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
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

	// Create a worktree, then delete its directory (but not via git worktree remove)
	// to leave a stale entry
	worktreesDir := filepath.Join(tmpDir, ".claude", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}
	sawDir := filepath.Join(worktreesDir, "saw", "test-cleanup-prune")
	if err := os.MkdirAll(sawDir, 0755); err != nil {
		t.Fatalf("failed to create saw dir: %v", err)
	}
	stalePath := filepath.Join(sawDir, "wave1-agent-H")
	if err := git.WorktreeAdd(tmpDir, stalePath, "saw/test-cleanup-prune/wave1-agent-H"); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Forcefully remove the directory without telling git (creates a stale entry)
	if err := os.RemoveAll(stalePath); err != nil {
		t.Fatalf("failed to remove worktree dir: %v", err)
	}

	// Verify the stale entry exists in git worktree list
	worktreesBefore, err := git.Run(tmpDir, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}
	if !strings.Contains(worktreesBefore, "wave1-agent-H") {
		t.Fatalf("expected stale worktree entry for wave1-agent-H, not found in: %s", worktreesBefore)
	}

	// Create manifest with a different agent (so cleanup processes something)
	manifest := &IMPLManifest{
		Title:       "test-cleanup-prune",
		FeatureSlug: "test-cleanup-prune",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "Z", Task: "Task Z", Files: []string{"z.go"}},
				},
			},
		},
	}
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Run cleanup — this should prune stale entries at the end
	_, err = Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify the stale worktree entry was pruned
	worktreesAfter, err := git.Run(tmpDir, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("failed to list worktrees after cleanup: %v", err)
	}
	if strings.Contains(worktreesAfter, "wave1-agent-H") {
		t.Errorf("stale worktree entry for wave1-agent-H should have been pruned, but still found in: %s", worktreesAfter)
	}
}

// TestCleanup_LegacyBranchFallback verifies that cleanup can remove worktrees
// and branches that were created with the legacy (pre-slug) naming format.
func TestCleanup_LegacyBranchFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-legacy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}

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

	// Create manifest
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
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create worktree with LEGACY naming (no slug)
	worktreesDir := filepath.Join(tmpDir, ".claude", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}
	legacyPath := filepath.Join(worktreesDir, "wave1-agent-A")
	if err := git.WorktreeAdd(tmpDir, legacyPath, "wave1-agent-A"); err != nil {
		t.Fatalf("failed to create legacy worktree: %v", err)
	}

	// Run cleanup — should find and clean up the legacy branch
	cleanupResult, err := Cleanup(context.Background(), manifestPath, 1, tmpDir, nil)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	result := cleanupResult.GetData()

	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent status, got %d", len(result.Agents))
	}
	status := result.Agents[0]
	if !status.BranchDeleted {
		t.Errorf("expected BranchDeleted=true for legacy branch, got false")
	}

	// Verify legacy branch is gone
	_, err = git.Run(tmpDir, "rev-parse", "--verify", "wave1-agent-A")
	if err == nil {
		t.Errorf("legacy branch wave1-agent-A still exists after cleanup")
	}
}

// TestCleanupBySlug_EmptyRepo verifies that CleanupBySlug on an empty repo
// (no worktrees) returns an empty result without error.
func TestCleanupBySlug_EmptyRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-by-slug-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}

	// Create an initial commit so the repo is valid
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

	res := CleanupBySlug(tmpDir, "some-slug", false)
	if res.IsFatal() {
		t.Fatalf("CleanupBySlug on empty repo failed: %+v", res.Errors)
	}
	result := res.GetData()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Cleaned) != 0 {
		t.Errorf("expected 0 cleaned, got %d", len(result.Cleaned))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped, got %d", len(result.Skipped))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

// TestCleanupAllStale_EmptyRepo verifies that CleanupAllStale on an empty repo
// (no worktrees) returns an empty result without error.
func TestCleanupAllStale_EmptyRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-all-stale-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}
	if _, err := git.Run(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}

	// Create an initial commit so the repo is valid
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

	result, err := CleanupAllStale(context.Background(), tmpDir, false)
	if err != nil {
		t.Fatalf("CleanupAllStale on empty repo failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Cleaned) != 0 {
		t.Errorf("expected 0 cleaned, got %d", len(result.Cleaned))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped, got %d", len(result.Skipped))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

// TestCleanupBySlug_NoMatch verifies that CleanupBySlug returns an empty result
// when the specified slug doesn't match any stale worktrees.
func TestCleanupBySlug_NoMatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-by-slug-nomatch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := git.Run(tmpDir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
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

	// Create a stale worktree with a completed IMPL for slug "slug-alpha"
	worktreesDir := filepath.Join(tmpDir, ".claude", "worktrees", "saw", "slug-alpha")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}
	worktreePath := filepath.Join(worktreesDir, "wave1-agent-A")
	if err := git.WorktreeAdd(tmpDir, worktreePath, "saw/slug-alpha/wave1-agent-A"); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Mark the IMPL as completed so DetectStaleWorktrees finds it as stale
	completeDir := filepath.Join(tmpDir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0755); err != nil {
		t.Fatalf("failed to create complete dir: %v", err)
	}
	implPath := filepath.Join(completeDir, "IMPL-slug-alpha.yaml")
	if err := os.WriteFile(implPath, []byte("title: slug-alpha\n"), 0644); err != nil {
		t.Fatalf("failed to create IMPL file: %v", err)
	}

	// Request cleanup for a DIFFERENT slug — should return empty result
	res := CleanupBySlug(tmpDir, "slug-beta", false)
	if res.IsFatal() {
		t.Fatalf("CleanupBySlug with non-matching slug failed: %+v", res.Errors)
	}
	result := res.GetData()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Cleaned) != 0 {
		t.Errorf("expected 0 cleaned (no match), got %d", len(result.Cleaned))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped (no match), got %d", len(result.Skipped))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors (no match), got %d", len(result.Errors))
	}

	// Verify the non-matching worktree is still present
	worktrees, err := git.WorktreeList(tmpDir)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}
	// The main worktree + agent-A worktree = 2 total; WorktreeList returns non-main ones
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree still present (slug-alpha untouched), got %d", len(worktrees))
	}
}
