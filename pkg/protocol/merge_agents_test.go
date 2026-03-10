package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
// It returns the repo path and a cleanup function.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "merge-agents-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "-C", tmpDir, "add", "README.md")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to add README: %v", err)
	}

	cmd = exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create initial commit: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// createAgentBranch creates a branch with a commit for an agent.
func createAgentBranch(t *testing.T, repoDir, branchName, fileName string) {
	t.Helper()

	// Create branch
	cmd := exec.Command("git", "-C", repoDir, "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch %s: %v", branchName, err)
	}

	// Create file
	filePath := filepath.Join(repoDir, fileName)
	if err := os.WriteFile(filePath, []byte("test content\n"), 0644); err != nil {
		t.Fatalf("failed to create file %s: %v", fileName, err)
	}

	// Add and commit
	cmd = exec.Command("git", "-C", repoDir, "add", fileName)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add file %s: %v", fileName, err)
	}

	cmd = exec.Command("git", "-C", repoDir, "commit", "-m", "Add "+fileName)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on branch %s: %v", branchName, err)
	}

	// Return to main branch
	cmd = exec.Command("git", "-C", repoDir, "checkout", "main")
	if err := cmd.Run(); err != nil {
		// Try master as fallback
		cmd = exec.Command("git", "-C", repoDir, "checkout", "master")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to return to main/master branch: %v", err)
		}
	}
}

// createManifest creates a test IMPL manifest file.
func createManifest(t *testing.T, repoDir string, waves []Wave) string {
	t.Helper()

	manifest := &IMPLManifest{
		Title:       "Test Manifest",
		FeatureSlug: "test-feature",
		Waves:       waves,
	}

	manifestPath := filepath.Join(repoDir, "IMPL.yaml")
	if err := Save(manifest, manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	return manifestPath
}

func TestMergeAgents_AllSucceed(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create two agent branches with non-conflicting files
	createAgentBranch(t, repoDir, "wave1-agent-A", "file-a.txt")
	createAgentBranch(t, repoDir, "wave1-agent-B", "file-b.txt")

	// Create manifest
	waves := []Wave{
		{
			Number: 1,
			Agents: []Agent{
				{ID: "A", Task: "Implement feature A"},
				{ID: "B", Task: "Implement feature B"},
			},
		},
	}
	manifestPath := createManifest(t, repoDir, waves)

	// Run merge
	result, err := MergeAgents(manifestPath, 1, repoDir)
	if err != nil {
		t.Fatalf("MergeAgents returned error: %v", err)
	}

	// Verify result
	if !result.Success {
		t.Errorf("expected Success=true, got false")
	}

	if result.Wave != 1 {
		t.Errorf("expected Wave=1, got %d", result.Wave)
	}

	if len(result.Merges) != 2 {
		t.Fatalf("expected 2 merge statuses, got %d", len(result.Merges))
	}

	// Check agent A merge
	if result.Merges[0].Agent != "A" {
		t.Errorf("expected first merge agent=A, got %s", result.Merges[0].Agent)
	}
	if !result.Merges[0].Success {
		t.Errorf("expected agent A merge to succeed, got error: %s", result.Merges[0].Error)
	}
	if result.Merges[0].Branch != "wave1-agent-A" {
		t.Errorf("expected branch=wave1-agent-A, got %s", result.Merges[0].Branch)
	}

	// Check agent B merge
	if result.Merges[1].Agent != "B" {
		t.Errorf("expected second merge agent=B, got %s", result.Merges[1].Agent)
	}
	if !result.Merges[1].Success {
		t.Errorf("expected agent B merge to succeed, got error: %s", result.Merges[1].Error)
	}

	// Verify files exist in main branch
	fileA := filepath.Join(repoDir, "file-a.txt")
	fileB := filepath.Join(repoDir, "file-b.txt")

	if _, err := os.Stat(fileA); os.IsNotExist(err) {
		t.Errorf("file-a.txt does not exist after merge")
	}
	if _, err := os.Stat(fileB); os.IsNotExist(err) {
		t.Errorf("file-b.txt does not exist after merge")
	}
}

func TestMergeAgents_ConflictStops(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create conflicting branches that modify the same line in README.md
	readmePath := filepath.Join(repoDir, "README.md")

	// Agent A modifies README - changes line 1
	cmd := exec.Command("git", "-C", repoDir, "checkout", "-b", "wave1-agent-A")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	if err := os.WriteFile(readmePath, []byte("# Test Repo - Agent A\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}

	cmd = exec.Command("git", "-C", repoDir, "add", "README.md")
	cmd.Run()
	cmd = exec.Command("git", "-C", repoDir, "commit", "-m", "Agent A change")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit agent A: %v", err)
	}

	// Return to main
	cmd = exec.Command("git", "-C", repoDir, "checkout", "main")
	if err := cmd.Run(); err != nil {
		// Try master
		cmd = exec.Command("git", "-C", repoDir, "checkout", "master")
		cmd.Run()
	}

	// Agent B also modifies README - changes the same line (will conflict after A merges)
	cmd = exec.Command("git", "-C", repoDir, "checkout", "-b", "wave1-agent-B")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch B: %v", err)
	}

	if err := os.WriteFile(readmePath, []byte("# Test Repo - Agent B\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}

	cmd = exec.Command("git", "-C", repoDir, "add", "README.md")
	cmd.Run()
	cmd = exec.Command("git", "-C", repoDir, "commit", "-m", "Agent B change")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit agent B: %v", err)
	}

	// Return to main
	cmd = exec.Command("git", "-C", repoDir, "checkout", "main")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "-C", repoDir, "checkout", "master")
		cmd.Run()
	}

	// Create manifest with agent A first (should succeed), then agent B (should conflict)
	waves := []Wave{
		{
			Number: 1,
			Agents: []Agent{
				{ID: "A", Task: "Implement feature A with a longer task description that exceeds fifty characters"},
				{ID: "B", Task: "Implement feature B"},
			},
		},
	}
	manifestPath := createManifest(t, repoDir, waves)

	// Run merge
	result, err := MergeAgents(manifestPath, 1, repoDir)
	if err != nil {
		t.Fatalf("MergeAgents returned error: %v", err)
	}

	// Verify result shows failure
	if result.Success {
		t.Errorf("expected Success=false due to conflict, got true")
	}

	// Should have 2 merge statuses: A succeeded, B failed
	if len(result.Merges) != 2 {
		t.Fatalf("expected 2 merge statuses, got %d", len(result.Merges))
	}

	// Agent A should succeed
	if !result.Merges[0].Success {
		t.Errorf("expected agent A to succeed, got error: %s", result.Merges[0].Error)
	}

	// Agent B should fail
	if result.Merges[1].Success {
		t.Errorf("expected agent B to fail due to conflict, but it succeeded")
	}
	if result.Merges[1].Error == "" {
		t.Errorf("expected error message for agent B conflict, got empty string")
	}

	// Abort merge to clean up (if in conflicted state)
	cmd = exec.Command("git", "-C", repoDir, "merge", "--abort")
	cmd.Run() // Ignore error if no merge in progress
}

func TestMergeAgents_WaveNotFound(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create manifest with only wave 1
	waves := []Wave{
		{
			Number: 1,
			Agents: []Agent{
				{ID: "A", Task: "Implement feature A"},
			},
		},
	}
	manifestPath := createManifest(t, repoDir, waves)

	// Try to merge wave 2 (does not exist)
	result, err := MergeAgents(manifestPath, 2, repoDir)

	// Should return error
	if err == nil {
		t.Fatalf("expected error for non-existent wave, got nil")
	}

	if result != nil {
		t.Errorf("expected nil result for non-existent wave, got %+v", result)
	}
}

func TestMergeAgents_BranchNotFound(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create manifest but don't create the actual git branches
	waves := []Wave{
		{
			Number: 1,
			Agents: []Agent{
				{ID: "A", Task: "Implement feature A"},
			},
		},
	}
	manifestPath := createManifest(t, repoDir, waves)

	// Run merge (branch wave1-agent-A does not exist)
	result, err := MergeAgents(manifestPath, 1, repoDir)
	if err != nil {
		t.Fatalf("MergeAgents returned error: %v", err)
	}

	// Verify result shows failure
	if result.Success {
		t.Errorf("expected Success=false due to missing branch, got true")
	}

	// Should have 1 merge status showing failure
	if len(result.Merges) != 1 {
		t.Fatalf("expected 1 merge status, got %d", len(result.Merges))
	}

	if result.Merges[0].Success {
		t.Errorf("expected merge to fail due to missing branch, but it succeeded")
	}
	if result.Merges[0].Error == "" {
		t.Errorf("expected error message for missing branch, got empty string")
	}
}

func TestMergeAgents_TaskTruncation(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create agent branch
	createAgentBranch(t, repoDir, "wave1-agent-A", "file-a.txt")

	// Create manifest with long task description
	longTask := "This is a very long task description that exceeds fifty characters and should be truncated in the commit message"
	waves := []Wave{
		{
			Number: 1,
			Agents: []Agent{
				{ID: "A", Task: longTask},
			},
		},
	}
	manifestPath := createManifest(t, repoDir, waves)

	// Run merge
	result, err := MergeAgents(manifestPath, 1, repoDir)
	if err != nil {
		t.Fatalf("MergeAgents returned error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected merge to succeed, got error: %v", result.Merges[0].Error)
	}

	// Verify commit message (check git log)
	cmd := exec.Command("git", "-C", repoDir, "log", "-1", "--pretty=format:%s")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to read git log: %v", err)
	}

	commitMsg := string(output)
	// Task should be truncated to 50 chars
	expectedMsg := "Merge wave1-agent-A: This is a very long task description that exceeds"
	if commitMsg != expectedMsg {
		t.Errorf("commit message not truncated correctly\ngot:  %q (len=%d)\nwant: %q (len=%d)", commitMsg, len(commitMsg), expectedMsg, len(expectedMsg))
	}
}
