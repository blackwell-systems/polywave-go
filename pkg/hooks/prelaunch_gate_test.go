package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// validManifest returns a minimal valid manifest for testing.
func validManifest() *protocol.IMPLManifest {
	return &protocol.IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Wave: 1},
			{File: "pkg/foo/baz.go", Agent: "B", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "implement foo", Files: []string{"pkg/foo/bar.go"}},
					{ID: "B", Task: "implement baz", Files: []string{"pkg/foo/baz.go"}},
				},
			},
		},
	}
}

func TestPreLaunchGate_AllPass(t *testing.T) {
	m := validManifest()
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if !result.Ready {
		t.Errorf("expected Ready=true, got false")
		for _, c := range result.Checks {
			if c.Status == "fail" {
				t.Logf("  FAIL: %s — %s", c.Name, c.Message)
			}
		}
	}

	// Verify we got all 7 checks
	if len(result.Checks) != 7 {
		t.Errorf("expected 7 checks, got %d", len(result.Checks))
	}
}

func TestPreLaunchGate_MissingWave(t *testing.T) {
	m := validManifest()
	res := PreLaunchGate(m, 99, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if result.Ready {
		t.Errorf("expected Ready=false for missing wave")
	}

	found := false
	for _, c := range result.Checks {
		if c.Name == "wave_exists" {
			found = true
			if c.Status != "fail" {
				t.Errorf("wave_exists: expected fail, got %s", c.Status)
			}
		}
	}
	if !found {
		t.Errorf("wave_exists check not found")
	}
}

func TestPreLaunchGate_MissingAgent(t *testing.T) {
	m := validManifest()
	res := PreLaunchGate(m, 1, "Z", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if result.Ready {
		t.Errorf("expected Ready=false for missing agent")
	}

	for _, c := range result.Checks {
		if c.Name == "agent_exists" {
			if c.Status != "fail" {
				t.Errorf("agent_exists: expected fail, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("agent_exists check not found")
}

func TestPreLaunchGate_UncommittedScaffolds(t *testing.T) {
	m := validManifest()
	m.Scaffolds = []protocol.ScaffoldFile{
		{FilePath: "pkg/types.go", Status: "pending"},
	}
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if result.Ready {
		t.Errorf("expected Ready=false for uncommitted scaffolds")
	}

	for _, c := range result.Checks {
		if c.Name == "scaffolds_committed" {
			if c.Status != "fail" {
				t.Errorf("scaffolds_committed: expected fail, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("scaffolds_committed check not found")
}

func TestPreLaunchGate_AllAgentsCompleted(t *testing.T) {
	m := validManifest()
	m.CompletionReports = map[string]protocol.CompletionReport{
		"A": {Status: "complete", Commit: "abc1234", Branch: "saw/test-feature/wave1-agent-A"},
		"B": {Status: "complete", Commit: "def5678", Branch: "saw/test-feature/wave1-agent-B"},
	}
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	// Should still be "ready" (warn, not fail) but wave_exists should warn
	for _, c := range result.Checks {
		if c.Name == "wave_exists" {
			if c.Status != "warn" {
				t.Errorf("wave_exists: expected warn for completed wave, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("wave_exists check not found")
}

func TestPreLaunchGate_I1Violation(t *testing.T) {
	m := validManifest()
	// Create an I1 violation: same file owned by two agents in the same wave
	m.FileOwnership = []protocol.FileOwnership{
		{File: "pkg/foo/bar.go", Agent: "A", Wave: 1},
		{File: "pkg/foo/bar.go", Agent: "B", Wave: 1},
	}
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if result.Ready {
		t.Errorf("expected Ready=false for I1 violation")
	}

	for _, c := range result.Checks {
		if c.Name == "ownership_disjoint" {
			if c.Status != "fail" {
				t.Errorf("ownership_disjoint: expected fail, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("ownership_disjoint check not found")
}

func TestPreLaunchGate_CriticRequired(t *testing.T) {
	m := validManifest()
	// Add a third agent to trigger critic requirement
	m.Waves[0].Agents = append(m.Waves[0].Agents, protocol.Agent{
		ID: "C", Task: "implement qux", Files: []string{"pkg/foo/qux.go"},
	})
	m.FileOwnership = append(m.FileOwnership, protocol.FileOwnership{
		File: "pkg/foo/qux.go", Agent: "C", Wave: 1,
	})

	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if result.Ready {
		t.Errorf("expected Ready=false when critic review required but missing")
	}

	for _, c := range result.Checks {
		if c.Name == "critic_review" {
			if c.Status != "fail" {
				t.Errorf("critic_review: expected fail, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("critic_review check not found")
}

func TestPreLaunchGate_CriticPassed(t *testing.T) {
	m := validManifest()
	m.Waves[0].Agents = append(m.Waves[0].Agents, protocol.Agent{
		ID: "C", Task: "implement qux", Files: []string{"pkg/foo/qux.go"},
	})
	m.FileOwnership = append(m.FileOwnership, protocol.FileOwnership{
		File: "pkg/foo/qux.go", Agent: "C", Wave: 1,
	})
	m.CriticReport = &protocol.CriticData{
		Verdict: protocol.CriticVerdictPass,
	}

	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "critic_review" {
			if c.Status != "pass" {
				t.Errorf("critic_review: expected pass, got %s: %s", c.Status, c.Message)
			}
			return
		}
	}
	t.Errorf("critic_review check not found")
}

func TestPreLaunchGate_CriticFailed(t *testing.T) {
	m := validManifest()
	// Add a third agent to trigger critic requirement
	m.Waves[0].Agents = append(m.Waves[0].Agents, protocol.Agent{
		ID: "C", Task: "implement qux", Files: []string{"pkg/foo/qux.go"},
	})
	m.FileOwnership = append(m.FileOwnership, protocol.FileOwnership{
		File: "pkg/foo/qux.go", Agent: "C", Wave: 1,
	})
	m.CriticReport = &protocol.CriticData{
		Verdict:    protocol.CriticVerdictIssues,
		IssueCount: 3,
	}

	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")
	if !res.IsSuccess() && !res.IsPartial() {
		t.Fatalf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	if result.Ready {
		t.Errorf("expected Ready=false when critic fails")
	}

	for _, c := range result.Checks {
		if c.Name == "critic_review" {
			if c.Status != "fail" {
				t.Errorf("critic_review: expected fail, got %s: %s", c.Status, c.Message)
			}
			if !strings.Contains(c.Message, "3 issue") {
				t.Errorf("expected issue count in message, got: %s", c.Message)
			}
			return
		}
	}
	t.Errorf("critic_review check not found")
}

func TestPreLaunchGate_WorktreeBranch(t *testing.T) {
	// Create a temp git repo to test branch checking
	dir := t.TempDir()
	gitInit := exec.Command("git", "init", dir)
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Configure git user for the test repo
	for _, args := range [][]string{
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config failed: %v\n%s", err, out)
		}
	}

	// Create an initial commit so we can create branches
	emptyFile := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	for _, args := range [][]string{
		{"-C", dir, "add", "."},
		{"-C", dir, "commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Create and checkout the expected branch
	branchName := "saw/test-feature/wave1-agent-A"
	checkout := exec.Command("git", "-C", dir, "checkout", "-b", branchName)
	if out, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout failed: %v\n%s", err, out)
	}

	m := validManifest()
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", dir)

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "worktree_branch" {
			if c.Status != "pass" {
				t.Errorf("worktree_branch: expected pass, got %s: %s", c.Status, c.Message)
			}
			return
		}
	}
	t.Errorf("worktree_branch check not found")
}

func TestPreLaunchGate_WorktreeBranchMismatch(t *testing.T) {
	dir := t.TempDir()
	gitInit := exec.Command("git", "init", dir)
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Configure git user
	for _, args := range [][]string{
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config failed: %v\n%s", err, out)
		}
	}

	emptyFile := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	for _, args := range [][]string{
		{"-C", dir, "add", "."},
		{"-C", dir, "commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Stay on default branch (wrong branch)
	m := validManifest()
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", dir)

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "worktree_branch" {
			if c.Status != "fail" {
				t.Errorf("worktree_branch: expected fail for wrong branch, got %s: %s", c.Status, c.Message)
			}
			return
		}
	}
	t.Errorf("worktree_branch check not found")
}

func TestPreLaunchGate_WorktreeNonexistent(t *testing.T) {
	m := validManifest()
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "/nonexistent/path/that/does/not/exist")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "worktree_branch" {
			if c.Status != "fail" {
				t.Errorf("worktree_branch: expected fail for nonexistent path, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("worktree_branch check not found")
}

func TestPreLaunchGate_ReadyOnlyFalseOnFail(t *testing.T) {
	// Verify that "warn" status doesn't make Ready=false
	m := validManifest()
	m.CompletionReports = map[string]protocol.CompletionReport{
		"A": {Status: "complete", Commit: "abc1234", Branch: "saw/test-feature/wave1-agent-A"},
		"B": {Status: "complete", Commit: "def5678", Branch: "saw/test-feature/wave1-agent-B"},
	}
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	// wave_exists should be "warn" but Ready should still be true
	hasWarn := false
	for _, c := range result.Checks {
		if c.Status == "warn" {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Errorf("expected at least one warn check")
	}
	if !result.Ready {
		t.Errorf("expected Ready=true when only warns (no fails)")
		for _, c := range result.Checks {
			t.Logf("  %s: %s — %s", c.Name, c.Status, c.Message)
		}
	}
}

func TestPreLaunchGate_CheckNames(t *testing.T) {
	// Verify all expected check names are present
	m := validManifest()
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	expectedNames := []string{
		"validation",
		"wave_exists",
		"agent_exists",
		"worktree_branch",
		"scaffolds_committed",
		"ownership_disjoint",
		"critic_review",
	}

	nameSet := make(map[string]bool)
	for _, c := range result.Checks {
		nameSet[c.Name] = true
	}

	for _, name := range expectedNames {
		if !nameSet[name] {
			t.Errorf("missing check: %s", name)
		}
	}
}

func TestPreLaunchGate_CommittedScaffoldsPass(t *testing.T) {
	m := validManifest()
	m.Scaffolds = []protocol.ScaffoldFile{
		{FilePath: "pkg/types.go", Status: "committed"},
	}
	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "scaffolds_committed" {
			if c.Status != "pass" {
				t.Errorf("scaffolds_committed: expected pass for committed scaffolds, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("scaffolds_committed check not found")
}

func TestPreLaunchGate_MultiRepo_CriticRequired(t *testing.T) {
	m := validManifest()
	m.Repositories = []string{"/repo1", "/repo2"}

	res := PreLaunchGate(m, 1, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "critic_review" {
			if c.Status != "fail" {
				t.Errorf("critic_review: expected fail for multi-repo without critic, got %s", c.Status)
			}
			return
		}
	}
	t.Errorf("critic_review check not found")
}

func TestPreLaunchGate_AgentExistsWhenWaveMissing(t *testing.T) {
	m := validManifest()
	res := PreLaunchGate(m, 99, "A", "/tmp/repo", "")

	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("PreLaunchGate failed: %v", res.Errors)
	}
	result := res.GetData()

	for _, c := range result.Checks {
		if c.Name == "agent_exists" {
			if c.Status != "fail" {
				t.Errorf("agent_exists: expected fail when wave is missing, got %s", c.Status)
			}
			if c.Message == "" {
				t.Errorf("agent_exists: message should not be empty")
			}
			return
		}
	}
	t.Errorf("agent_exists check not found")
}
