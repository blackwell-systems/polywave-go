package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
)

// makeTempGitRepo creates a temporary directory with a git repo and an initial
// commit so that cache.BuildKey works correctly.
func makeTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func makePassingManifest() *IMPLManifest {
	return &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{Type: "build", Command: "echo hello", Required: true},
			},
		},
	}
}

func TestRunBaselineGates_AllPass(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{Type: "build", Command: "echo build-ok", Required: true},
				{Type: "lint", Command: "echo lint-ok", Required: true},
			},
		},
	}

	result, err := RunBaselineGates(manifest, 1, repoDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	if result.Reason != "" {
		t.Errorf("expected Reason='', got %q", result.Reason)
	}
	if len(result.GateResults) != 2 {
		t.Errorf("expected 2 gate results, got %d", len(result.GateResults))
	}
}

func TestRunBaselineGates_RequiredFails(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{Type: "build", Command: "false", Required: true},
			},
		},
	}

	result, err := RunBaselineGates(manifest, 1, repoDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected Passed=false, got true")
	}
	if result.Reason != "baseline_verification_failed" {
		t.Errorf("expected Reason='baseline_verification_failed', got %q", result.Reason)
	}
}

func TestRunBaselineGates_OptionalFails(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{Type: "build", Command: "echo ok", Required: true},
				// Required=false means this is optional
				{Type: "lint", Command: "false", Required: false},
			},
		},
	}

	result, err := RunBaselineGates(manifest, 1, repoDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true (optional failure should not block), got false")
	}
	if result.Reason != "" {
		t.Errorf("expected Reason='', got %q", result.Reason)
	}
}

func TestRunBaselineGates_NoGates(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := &IMPLManifest{} // No quality_gates defined

	result, err := RunBaselineGates(manifest, 1, repoDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true for empty manifest, got false")
	}
	if len(result.GateResults) != 0 {
		t.Errorf("expected empty GateResults, got %d", len(result.GateResults))
	}
}

func TestRunBaselineGates_NilCache(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := makePassingManifest()

	// Must not panic with nil cache
	result, err := RunBaselineGates(manifest, 1, repoDir, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil cache: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	// CommitSHA should be empty when cache is nil
	if result.CommitSHA != "" {
		t.Errorf("expected CommitSHA='', got %q (cache is nil)", result.CommitSHA)
	}
}

func TestRunBaselineGates_CommitSHAPopulated(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := makePassingManifest()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	cache := gatecache.New(stateDir, time.Minute)

	result, err := RunBaselineGates(manifest, 1, repoDir, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CommitSHA == "" {
		t.Errorf("expected CommitSHA to be populated when cache is non-nil and repo is valid")
	}
}

// E21B tests

func TestRunCrossRepoBaselineGates_AllPass(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	targetRepos := map[string]string{
		"repo-a": dir1,
		"repo-b": dir2,
	}
	configRepos := []RepoEntry{
		{Name: "repo-a", Path: dir1, BuildCommand: "echo build-a"},
		{Name: "repo-b", Path: dir2, BuildCommand: "echo build-b"},
	}

	result, err := RunCrossRepoBaselineGates(&IMPLManifest{}, 1, targetRepos, configRepos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected all repos to pass")
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 repo results, got %d", len(result.Results))
	}
}

func TestRunCrossRepoBaselineGates_OneFails(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	targetRepos := map[string]string{
		"repo-a": dir1,
		"repo-b": dir2,
	}
	configRepos := []RepoEntry{
		{Name: "repo-a", Path: dir1, BuildCommand: "echo ok"},
		{Name: "repo-b", Path: dir2, BuildCommand: "exit 1"},
	}

	result, err := RunCrossRepoBaselineGates(&IMPLManifest{}, 1, targetRepos, configRepos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected failure when one repo fails")
	}
}

func TestRunCrossRepoBaselineGates_FallbackToManifestGates(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := makePassingManifest()
	targetRepos := map[string]string{
		"repo-a": repoDir,
	}
	// No BuildCommand/TestCommand → falls back to manifest gates
	configRepos := []RepoEntry{
		{Name: "repo-a", Path: repoDir},
	}

	result, err := RunCrossRepoBaselineGates(manifest, 1, targetRepos, configRepos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected pass with manifest gate fallback")
	}
}

func TestRunCrossRepoBaselineGates_WithTestCommand(t *testing.T) {
	dir := t.TempDir()
	targetRepos := map[string]string{"repo-a": dir}
	configRepos := []RepoEntry{
		{Name: "repo-a", Path: dir, BuildCommand: "echo build", TestCommand: "echo test"},
	}

	result, err := RunCrossRepoBaselineGates(&IMPLManifest{}, 1, targetRepos, configRepos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected pass")
	}
	// Should have 2 gate results (build + test)
	if r, ok := result.Results["repo-a"]; ok {
		if len(r.GateResults) != 2 {
			t.Errorf("expected 2 gate results (build+test), got %d", len(r.GateResults))
		}
	}
}

func TestRunSimpleGate_Success(t *testing.T) {
	dir := t.TempDir()
	gr := runSimpleGate("build", "echo hello", dir)
	if !gr.Passed {
		t.Error("expected gate to pass")
	}
	if gr.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", gr.ExitCode)
	}
}

func TestRunSimpleGate_Failure(t *testing.T) {
	dir := t.TempDir()
	gr := runSimpleGate("build", "exit 42", dir)
	if gr.Passed {
		t.Error("expected gate to fail")
	}
	if gr.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", gr.ExitCode)
	}
}
