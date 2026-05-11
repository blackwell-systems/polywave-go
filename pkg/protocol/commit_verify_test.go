package protocol

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/polywave-go/internal/git"
)

// createTestRepo sets up a temporary git repository for testing.
// Returns the repo path and a cleanup function.
func createTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "commit-verify-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo with main as default branch
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to config git user.email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to config git user.name: %v", err)
	}

	// Create initial commit on main branch
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write README: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to git commit: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// createBranchWithCommits creates a branch with the specified number of commits.
func createBranchWithCommits(t *testing.T, repoDir, branchName string, commitCount int) {
	t.Helper()

	// Create branch
	_, err := git.Run(repoDir, "checkout", "-b", branchName)
	if err != nil {
		t.Fatalf("failed to create branch %s: %v", branchName, err)
	}

	// Create commits - use base name (last path segment) for file names to avoid subdirectories
	baseName := filepath.Base(branchName)
	for i := 0; i < commitCount; i++ {
		filePath := filepath.Join(repoDir, baseName+"-file"+string(rune('a'+i))+".txt")
		content := []byte("content from " + branchName + "\n")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write file for commit %d: %v", i, err)
		}

		cmd := exec.Command("git", "add", ".")
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to git add for commit %d: %v", i, err)
		}

		cmd = exec.Command("git", "commit", "-m", "Commit "+string(rune('1'+i))+" on "+branchName)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to git commit %d: %v", i, err)
		}
	}

	// Return to main branch
	_, err = git.Run(repoDir, "checkout", "main")
	if err != nil {
		t.Fatalf("failed to return to main branch: %v", err)
	}
}

// TestVerifyCommits_AllValid verifies that VerifyCommits returns AllValid=true
// when all agent branches have commits.
func TestVerifyCommits_AllValid(t *testing.T) {
	repoDir, cleanup := createTestRepo(t)
	defer cleanup()

	// Create a manifest with Wave 1 containing 2 agents
	manifestPath := filepath.Join(repoDir, "manifest.yaml")
	manifestContent := `title: Test Feature
feature_slug: test-feature
waves:
  - number: 1
    agents:
      - id: A
        task: Implement feature A
        files:
          - pkg/feature_a.go
      - id: B
        task: Implement feature B
        files:
          - pkg/feature_b.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Commit manifest to main branch
	cmd := exec.Command("git", "add", "manifest.yaml")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add manifest: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add manifest")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git commit manifest: %v", err)
	}

	// Create branches with commits
	createBranchWithCommits(t, repoDir, "polywave/test-feature/wave1-agent-A", 2)
	createBranchWithCommits(t, repoDir, "polywave/test-feature/wave1-agent-B", 1)

	// Verify commits
	res := VerifyCommits(context.Background(), manifestPath, 1, repoDir)

	// Check result is successful
	if !res.IsSuccess() {
		t.Fatalf("expected IsSuccess()=true, got false. Errors: %v", res.Errors)
	}

	// Get data
	data := res.GetData()

	if len(data.Agents) != 2 {
		t.Fatalf("expected 2 agent statuses, got %d", len(data.Agents))
	}

	// Check agent A
	agentA := data.Agents[0]
	if agentA.Agent != "A" {
		t.Errorf("expected agent A, got %s", agentA.Agent)
	}
	if agentA.Branch != "polywave/test-feature/wave1-agent-A" {
		t.Errorf("expected branch polywave/test-feature/wave1-agent-A, got %s", agentA.Branch)
	}
	if agentA.CommitCount != 2 {
		t.Errorf("expected 2 commits for agent A, got %d", agentA.CommitCount)
	}
	if !agentA.HasCommits {
		t.Errorf("expected HasCommits=true for agent A")
	}

	// Check agent B
	agentB := data.Agents[1]
	if agentB.Agent != "B" {
		t.Errorf("expected agent B, got %s", agentB.Agent)
	}
	if agentB.Branch != "polywave/test-feature/wave1-agent-B" {
		t.Errorf("expected branch polywave/test-feature/wave1-agent-B, got %s", agentB.Branch)
	}
	if agentB.CommitCount != 1 {
		t.Errorf("expected 1 commit for agent B, got %d", agentB.CommitCount)
	}
	if !agentB.HasCommits {
		t.Errorf("expected HasCommits=true for agent B")
	}
}

// TestVerifyCommits_MissingCommits verifies that VerifyCommits returns AllValid=false
// when one or more agent branches have no commits.
func TestVerifyCommits_MissingCommits(t *testing.T) {
	repoDir, cleanup := createTestRepo(t)
	defer cleanup()

	// Create a manifest with Wave 1 containing 2 agents
	manifestPath := filepath.Join(repoDir, "manifest.yaml")
	manifestContent := `title: Test Feature
feature_slug: test-feature
waves:
  - number: 1
    agents:
      - id: A
        task: Implement feature A
        files:
          - pkg/feature_a.go
      - id: B
        task: Implement feature B
        files:
          - pkg/feature_b.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Commit manifest to main branch
	cmd := exec.Command("git", "add", "manifest.yaml")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add manifest: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add manifest")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git commit manifest: %v", err)
	}

	// Create only one branch with commits
	createBranchWithCommits(t, repoDir, "polywave/test-feature/wave1-agent-A", 1)

	// Create the other branch but with no commits
	_, err := git.Run(repoDir, "checkout", "-b", "polywave/test-feature/wave1-agent-B")
	if err != nil {
		t.Fatalf("failed to create branch wave1-agent-B: %v", err)
	}
	_, err = git.Run(repoDir, "checkout", "main")
	if err != nil {
		t.Fatalf("failed to return to main: %v", err)
	}

	// Verify commits
	res := VerifyCommits(context.Background(), manifestPath, 1, repoDir)

	// Check result is partial (success with warnings)
	if !res.IsPartial() {
		t.Fatalf("expected IsPartial()=true when some agents have no commits, got Code=%s", res.Code)
	}

	if len(res.Errors) == 0 {
		t.Errorf("expected warnings in Errors for missing commits")
	}

	// Get data
	data := res.GetData()

	if len(data.Agents) != 2 {
		t.Fatalf("expected 2 agent statuses, got %d", len(data.Agents))
	}

	// Check agent A (has commits)
	agentA := data.Agents[0]
	if !agentA.HasCommits {
		t.Errorf("expected HasCommits=true for agent A")
	}

	// Check agent B (no commits)
	agentB := data.Agents[1]
	if agentB.HasCommits {
		t.Errorf("expected HasCommits=false for agent B")
	}
	if agentB.CommitCount != 0 {
		t.Errorf("expected 0 commits for agent B, got %d", agentB.CommitCount)
	}
}

// TestVerifyCommits_BranchNotExist verifies that VerifyCommits treats
// nonexistent branches as having 0 commits.
func TestVerifyCommits_BranchNotExist(t *testing.T) {
	repoDir, cleanup := createTestRepo(t)
	defer cleanup()

	// Create a manifest with Wave 1 containing 1 agent
	manifestPath := filepath.Join(repoDir, "manifest.yaml")
	manifestContent := `title: Test Feature
feature_slug: test-feature
waves:
  - number: 1
    agents:
      - id: A
        task: Implement feature A
        files:
          - pkg/feature_a.go
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Don't create the branch at all

	// Verify commits
	res := VerifyCommits(context.Background(), manifestPath, 1, repoDir)

	// Check result is partial (success with warnings)
	if !res.IsPartial() {
		t.Fatalf("expected IsPartial()=true when branch doesn't exist, got Code=%s", res.Code)
	}

	if len(res.Errors) == 0 {
		t.Errorf("expected warnings in Errors for missing branch")
	}

	// Get data
	data := res.GetData()

	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent status, got %d", len(data.Agents))
	}

	// Check agent A (branch doesn't exist)
	agentA := data.Agents[0]
	if agentA.HasCommits {
		t.Errorf("expected HasCommits=false for nonexistent branch")
	}
	if agentA.CommitCount != 0 {
		t.Errorf("expected 0 commits for nonexistent branch, got %d", agentA.CommitCount)
	}
}
