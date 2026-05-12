package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildTestManifest creates an in-memory IMPLManifest for solo wave tests.
func buildTestManifest(slug string, waveNum int, agentIDs []string) *IMPLManifest {
	agents := make([]Agent, len(agentIDs))
	for i, id := range agentIDs {
		agents[i] = Agent{ID: id, Task: "Task " + id}
	}
	return &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: slug,
		Waves: []Wave{
			{
				Number: waveNum,
				Agents: agents,
			},
		},
	}
}

// TestIsSoloWave_AllAbsent verifies that WorktreesAbsent returns true when none
// of the expected worktree directories exist on disk.
func TestIsSoloWave_AllAbsent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "solo-wave-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := buildTestManifest("my-feature", 1, []string{"A", "B"})

	// WorktreeDir paths won't exist — expect WorktreesAbsent to return true
	got := WorktreesAbsent(manifest, 1, tmpDir)
	if !got {
		t.Errorf("WorktreesAbsent() = false, want true when no worktree dirs exist")
	}
}

// TestIsSoloWave_OneExists verifies that WorktreesAbsent returns false when at
// least one expected worktree directory exists on disk.
func TestIsSoloWave_OneExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "solo-wave-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := buildTestManifest("my-feature", 1, []string{"A", "B"})

	// Create the worktree directory for agent A
	worktreePath := WorktreeDir(tmpDir, "my-feature", 1, "A")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	got := WorktreesAbsent(manifest, 1, tmpDir)
	if got {
		t.Errorf("WorktreesAbsent() = true, want false when one worktree dir exists at %s", worktreePath)
	}
}

// TestIsSoloWave_WaveNotFound verifies that WorktreesAbsent returns false when
// the requested wave number is not in the manifest.
func TestIsSoloWave_WaveNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "solo-wave-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := buildTestManifest("my-feature", 1, []string{"A"})

	// Request wave 99 which doesn't exist
	got := WorktreesAbsent(manifest, 99, tmpDir)
	if got {
		t.Errorf("WorktreesAbsent() = true, want false for non-existent wave")
	}
}

// setupBranchTestRepo creates a temp git repo for AllBranchesAbsent tests.
func setupBranchTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "all-branches-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create an initial commit so we can create branches
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write README: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "add", "README.md").Run()
	cmd = exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

// TestAllBranchesAbsent_NoBranches verifies that AllBranchesAbsent returns true
// when no polywave/ branches exist in the repository.
func TestAllBranchesAbsent_NoBranches(t *testing.T) {
	repoDir, cleanup := setupBranchTestRepo(t)
	defer cleanup()

	manifest := buildTestManifest("my-feature", 1, []string{"A", "B"})

	// No agent branches created — expect AllBranchesAbsent to return true
	got := AllBranchesAbsent(manifest, 1, repoDir)
	if !got {
		t.Errorf("AllBranchesAbsent() = false, want true when no agent branches exist")
	}
}

// TestAllBranchesAbsent_OneBranchExists verifies that AllBranchesAbsent returns
// false when at least one expected agent branch exists in the repository.
func TestAllBranchesAbsent_OneBranchExists(t *testing.T) {
	repoDir, cleanup := setupBranchTestRepo(t)
	defer cleanup()

	manifest := buildTestManifest("my-feature", 1, []string{"A", "B"})

	// Create the slug-scoped branch for agent A
	branchA := BranchName("my-feature", 1, "A")
	cmd := exec.Command("git", "-C", repoDir, "branch", branchA)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch %s: %v", branchA, err)
	}

	got := AllBranchesAbsent(manifest, 1, repoDir)
	if got {
		t.Errorf("AllBranchesAbsent() = true, want false when branch %s exists", branchA)
	}
}

// TestAllBranchesAbsent_LegacyBranchExists verifies that AllBranchesAbsent
// returns false when a legacy-format branch exists for any agent.
func TestAllBranchesAbsent_LegacyBranchExists(t *testing.T) {
	repoDir, cleanup := setupBranchTestRepo(t)
	defer cleanup()

	manifest := buildTestManifest("my-feature", 1, []string{"A", "B"})

	// Create only the legacy branch for agent A (old format: wave1-agent-A)
	legacyBranchA := LegacyBranchName(1, "A")
	cmd := exec.Command("git", "-C", repoDir, "branch", legacyBranchA)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create legacy branch %s: %v", legacyBranchA, err)
	}

	got := AllBranchesAbsent(manifest, 1, repoDir)
	if got {
		t.Errorf("AllBranchesAbsent() = true, want false when legacy branch %s exists", legacyBranchA)
	}
}

// TestAllBranchesAbsent_WaveNotFound verifies that AllBranchesAbsent returns
// true when the requested wave number is not in the manifest (no wave = no branches).
func TestAllBranchesAbsent_WaveNotFound(t *testing.T) {
	repoDir, cleanup := setupBranchTestRepo(t)
	defer cleanup()

	manifest := buildTestManifest("my-feature", 1, []string{"A"})

	got := AllBranchesAbsent(manifest, 99, repoDir)
	if !got {
		t.Errorf("AllBranchesAbsent() = false, want true for non-existent wave")
	}
}
