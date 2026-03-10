package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary git repository for testing.
// It returns the path to the repo and a cleanup function.
func initTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user (required for commits)
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()

	// Create initial commit (required for worktree creation)
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cmd = exec.Command("git", "-C", tmpDir, "add", "README.md")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add README: %v", err)
	}

	cmd = exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	cleanup := func() {
		// Cleanup is automatic with t.TempDir()
	}

	return tmpDir, cleanup
}

// createTestManifest creates a manifest file with a test wave.
func createTestManifest(t *testing.T, repoDir string, waveNum int, agentIDs []string) string {
	t.Helper()

	manifest := &IMPLManifest{
		Title:       "Test Manifest",
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: waveNum,
				Agents: make([]Agent, len(agentIDs)),
			},
		},
	}

	for i, id := range agentIDs {
		manifest.Waves[0].Agents[i] = Agent{
			ID:   id,
			Task: "Test task for " + id,
		}
	}

	manifestPath := filepath.Join(repoDir, "IMPL.yaml")
	if err := Save(manifest, manifestPath); err != nil {
		t.Fatalf("failed to save test manifest: %v", err)
	}

	return manifestPath
}

func TestCreateWorktrees_HappyPath(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create manifest with 3 agents
	agentIDs := []string{"A", "B", "C"}
	manifestPath := createTestManifest(t, repoDir, 1, agentIDs)

	// Create worktrees
	result, err := CreateWorktrees(manifestPath, 1, repoDir)
	if err != nil {
		t.Fatalf("CreateWorktrees failed: %v", err)
	}

	// Verify we got 3 worktrees
	if len(result.Worktrees) != 3 {
		t.Errorf("expected 3 worktrees, got %d", len(result.Worktrees))
	}

	// Verify each worktree info
	for i, agentID := range agentIDs {
		info := result.Worktrees[i]

		// Check agent ID
		if info.Agent != agentID {
			t.Errorf("worktree %d: expected agent %s, got %s", i, agentID, info.Agent)
		}

		// Check path format
		expectedPath := filepath.Join(repoDir, ".claude", "worktrees", "wave1-agent-"+agentID)
		if info.Path != expectedPath {
			t.Errorf("worktree %d: expected path %s, got %s", i, expectedPath, info.Path)
		}

		// Check branch name
		expectedBranch := "wave1-agent-" + agentID
		if info.Branch != expectedBranch {
			t.Errorf("worktree %d: expected branch %s, got %s", i, expectedBranch, info.Branch)
		}

		// Verify worktree actually exists
		if _, err := os.Stat(info.Path); os.IsNotExist(err) {
			t.Errorf("worktree %d: path %s does not exist", i, info.Path)
		}

		// Verify branch exists
		cmd := exec.Command("git", "-C", repoDir, "branch", "--list", info.Branch)
		out, err := cmd.Output()
		if err != nil {
			t.Errorf("worktree %d: failed to check branch existence: %v", i, err)
		}
		if !strings.Contains(string(out), info.Branch) {
			t.Errorf("worktree %d: branch %s not found in git branch list", i, info.Branch)
		}
	}
}

func TestCreateWorktrees_WaveNotFound(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create manifest with wave 1
	agentIDs := []string{"A"}
	manifestPath := createTestManifest(t, repoDir, 1, agentIDs)

	// Try to create worktrees for wave 2 (doesn't exist)
	_, err := CreateWorktrees(manifestPath, 2, repoDir)
	if err == nil {
		t.Fatal("expected error for non-existent wave, got nil")
	}

	expectedMsg := "wave 2 not found"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

func TestCreateWorktrees_ManifestLoadFailure(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Use a non-existent manifest path
	manifestPath := filepath.Join(repoDir, "does-not-exist.yaml")

	_, err := CreateWorktrees(manifestPath, 1, repoDir)
	if err == nil {
		t.Fatal("expected error for missing manifest file, got nil")
	}

	expectedMsg := "failed to load manifest"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

func TestCreateWorktrees_GitFailure(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create manifest
	agentIDs := []string{"A"}
	manifestPath := createTestManifest(t, repoDir, 1, agentIDs)

	// Create the worktree directory manually to cause a conflict
	conflictPath := filepath.Join(repoDir, ".claude", "worktrees", "wave1-agent-A")
	if err := os.MkdirAll(conflictPath, 0755); err != nil {
		t.Fatalf("failed to create conflict directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictPath, "conflict.txt"), []byte("exists"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	// Try to create worktrees (should fail due to directory already existing)
	_, err := CreateWorktrees(manifestPath, 1, repoDir)
	if err == nil {
		t.Fatal("expected error for git worktree add failure, got nil")
	}

	expectedMsg := "failed to create worktree"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

func TestCreateWorktrees_EmptyWave(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create manifest with empty wave (no agents)
	manifest := &IMPLManifest{
		Title:       "Test Manifest",
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{}, // Empty agents list
			},
		},
	}

	manifestPath := filepath.Join(repoDir, "IMPL.yaml")
	if err := Save(manifest, manifestPath); err != nil {
		t.Fatalf("failed to save test manifest: %v", err)
	}

	// Create worktrees for empty wave
	result, err := CreateWorktrees(manifestPath, 1, repoDir)
	if err != nil {
		t.Fatalf("CreateWorktrees failed for empty wave: %v", err)
	}

	// Should return empty result, not error
	if len(result.Worktrees) != 0 {
		t.Errorf("expected 0 worktrees for empty wave, got %d", len(result.Worktrees))
	}
}
