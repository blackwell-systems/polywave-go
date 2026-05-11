package protocol

import (
	"context"
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
	if saveRes := Save(context.Background(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("failed to save test manifest: %v", saveRes.Errors)
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
	res := CreateWorktrees(context.Background(), manifestPath, 1, repoDir, nil)
	if !res.IsSuccess() {
		t.Fatalf("CreateWorktrees failed: %v", res.Errors)
	}

	data := res.GetData()
	// Verify we got 3 worktrees
	if len(data.Worktrees) != 3 {
		t.Errorf("expected 3 worktrees, got %d", len(data.Worktrees))
	}

	// Verify each worktree info
	for i, agentID := range agentIDs {
		info := data.Worktrees[i]

		// Check agent ID
		if info.Agent != agentID {
			t.Errorf("worktree %d: expected agent %s, got %s", i, agentID, info.Agent)
		}

		// Check path format (slug-scoped)
		expectedPath := filepath.Join(repoDir, ".claude", "worktrees", "polywave", "test-feature", "wave1-agent-"+agentID)
		if info.Path != expectedPath {
			t.Errorf("worktree %d: expected path %s, got %s", i, expectedPath, info.Path)
		}

		// Check branch name (slug-scoped)
		expectedBranch := "polywave/test-feature/wave1-agent-" + agentID
		if info.Branch != expectedBranch {
			t.Errorf("worktree %d: expected branch %s, got %s", i, expectedBranch, info.Branch)
		}

		// Verify worktree actually exists
		if _, err := os.Stat(info.Path); os.IsNotExist(err) {
			t.Errorf("worktree %d: path %s does not exist", i, info.Path)
		}

		// Verify branch exists (slug-scoped branches contain slashes)
		cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", info.Branch)
		out, err := cmd.Output()
		if err != nil {
			t.Errorf("worktree %d: branch %s not found: %v", i, info.Branch, err)
		}
		if len(strings.TrimSpace(string(out))) == 0 {
			t.Errorf("worktree %d: branch %s not found in git", i, info.Branch)
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
	res := CreateWorktrees(context.Background(), manifestPath, 2, repoDir, nil)
	if res.IsSuccess() {
		t.Fatal("expected error for non-existent wave, got success")
	}

	expectedMsg := "wave 2 not found"
	found := false
	for _, err := range res.Errors {
		if strings.Contains(err.Message, expectedMsg) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error message to contain %q, got: %v", expectedMsg, res.Errors)
	}
}

func TestCreateWorktrees_ManifestLoadFailure(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Use a non-existent manifest path
	manifestPath := filepath.Join(repoDir, "does-not-exist.yaml")

	res := CreateWorktrees(context.Background(), manifestPath, 1, repoDir, nil)
	if res.IsSuccess() {
		t.Fatal("expected error for missing manifest file, got success")
	}

	expectedMsg := "failed to load manifest"
	found := false
	for _, err := range res.Errors {
		if strings.Contains(err.Message, expectedMsg) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error message to contain %q, got: %v", expectedMsg, res.Errors)
	}
}

func TestCreateWorktrees_GitFailure(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create manifest
	agentIDs := []string{"A"}
	manifestPath := createTestManifest(t, repoDir, 1, agentIDs)

	// Create the worktree directory manually to cause a conflict
	conflictPath := filepath.Join(repoDir, ".claude", "worktrees", "polywave", "test-feature", "wave1-agent-A")
	if err := os.MkdirAll(conflictPath, 0755); err != nil {
		t.Fatalf("failed to create conflict directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictPath, "conflict.txt"), []byte("exists"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	// Try to create worktrees (should fail due to directory already existing)
	res := CreateWorktrees(context.Background(), manifestPath, 1, repoDir, nil)
	if res.IsSuccess() {
		t.Fatal("expected error for git worktree add failure, got success")
	}

	expectedMsg := "failed to create worktree"
	found := false
	for _, err := range res.Errors {
		if strings.Contains(err.Message, expectedMsg) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error message to contain %q, got: %v", expectedMsg, res.Errors)
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
	if saveRes := Save(context.Background(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("failed to save test manifest: %v", saveRes.Errors)
	}

	// Create worktrees for empty wave
	res := CreateWorktrees(context.Background(), manifestPath, 1, repoDir, nil)
	if !res.IsSuccess() {
		t.Fatalf("CreateWorktrees failed for empty wave: %v", res.Errors)
	}

	// Should return empty result, not error
	data := res.GetData()
	if len(data.Worktrees) != 0 {
		t.Errorf("expected 0 worktrees for empty wave, got %d", len(data.Worktrees))
	}
}

func TestCreateWorktrees_InstallsHooks(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create pre-commit hook in main repo
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks directory: %v", err)
	}

	hookContent := "#!/bin/sh\n# Polywave isolation enforcement hook\necho 'test hook'\n"
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		t.Fatalf("failed to create pre-commit hook: %v", err)
	}

	// Create manifest with 1 agent
	agentIDs := []string{"A"}
	manifestPath := createTestManifest(t, repoDir, 1, agentIDs)

	// Create worktrees
	res := CreateWorktrees(context.Background(), manifestPath, 1, repoDir, nil)
	if !res.IsSuccess() {
		t.Fatalf("CreateWorktrees failed: %v", res.Errors)
	}

	data := res.GetData()
	// Verify hook exists in worktree (git stores worktree metadata by directory name)
	// For slug-scoped worktrees, git uses the last path component as the worktree name
	worktreeHookPath := filepath.Join(repoDir, ".git", "worktrees", "wave1-agent-A", "hooks", "pre-commit")
	content, err := os.ReadFile(worktreeHookPath)
	if err != nil {
		t.Fatalf("hook not found in worktree: %v", err)
	}

	// Verify hook was generated from embedded template (not copied from test hook)
	if !strings.Contains(string(content), "POLYWAVE_ALLOW_MAIN_COMMIT") {
		t.Error("hook missing POLYWAVE_ALLOW_MAIN_COMMIT marker")
	}
	if !strings.Contains(string(content), "Polywave pre-commit guard") {
		t.Error("hook missing 'Polywave pre-commit guard' comment")
	}

	// Verify hook is executable
	info, err := os.Stat(worktreeHookPath)
	if err != nil {
		t.Fatalf("failed to stat hook: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("hook is not executable, mode: %v", info.Mode())
	}

	// Verify worktree was created successfully
	if len(data.Worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(data.Worktrees))
	}
}

func TestCreateWorktrees_ContinuesOnHookInstallFailure(t *testing.T) {
	repoDir, cleanup := initTestRepo(t)
	defer cleanup()

	// Create manifest with 1 agent but DON'T create hook in main repo
	agentIDs := []string{"A"}
	manifestPath := createTestManifest(t, repoDir, 1, agentIDs)

	// Create worktrees (should succeed despite missing hook)
	res := CreateWorktrees(context.Background(), manifestPath, 1, repoDir, nil)
	if !res.IsSuccess() {
		t.Fatalf("CreateWorktrees failed: %v", res.Errors)
	}

	data := res.GetData()
	// Verify worktree was created
	if len(data.Worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(data.Worktrees))
	}

	// Verify worktree exists
	worktreePath := data.Worktrees[0].Path
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree path %s does not exist", worktreePath)
	}

	// Hook should exist (installed from embedded template, no dependency on main repo hook)
	// Git stores worktree metadata by the directory basename
	worktreeHookPath := filepath.Join(repoDir, ".git", "worktrees", "wave1-agent-A", "hooks", "pre-commit")
	hookContent, err := os.ReadFile(worktreeHookPath)
	if err != nil {
		t.Fatalf("hook not found at %s: %v", worktreeHookPath, err)
	}
	// Verify hook has Polywave isolation markers
	if !strings.Contains(string(hookContent), "POLYWAVE_ALLOW_MAIN_COMMIT") {
		t.Error("hook missing POLYWAVE_ALLOW_MAIN_COMMIT marker")
	}
}
