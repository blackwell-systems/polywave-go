package protocol

import (
	"context"
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

	res := RunBaselineGates(manifest, 1, repoDir, nil)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %+v", res.Errors)
	}
	data := res.GetData()
	if !data.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	if data.Reason != "" {
		t.Errorf("expected Reason='', got %q", data.Reason)
	}
	if len(data.GateResults) != 2 {
		t.Errorf("expected 2 gate results, got %d", len(data.GateResults))
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

	res := RunBaselineGates(manifest, 1, repoDir, nil)
	data := res.GetData()
	if data.Passed {
		t.Errorf("expected Passed=false, got true")
	}
	if data.Reason != "baseline_verification_failed" {
		t.Errorf("expected Reason='baseline_verification_failed', got %q", data.Reason)
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

	res := RunBaselineGates(manifest, 1, repoDir, nil)
	data := res.GetData()
	if !data.Passed {
		t.Errorf("expected Passed=true (optional failure should not block), got false")
	}
	if data.Reason != "" {
		t.Errorf("expected Reason='', got %q", data.Reason)
	}
}

func TestRunBaselineGates_NoGates(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := &IMPLManifest{} // No quality_gates defined

	res := RunBaselineGates(manifest, 1, repoDir, nil)
	data := res.GetData()
	if !data.Passed {
		t.Errorf("expected Passed=true for empty manifest, got false")
	}
	if len(data.GateResults) != 0 {
		t.Errorf("expected empty GateResults, got %d", len(data.GateResults))
	}
}

func TestRunBaselineGates_NilCache(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := makePassingManifest()

	// Must not panic with nil cache
	res := RunBaselineGates(manifest, 1, repoDir, nil)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure with nil cache: %+v", res.Errors)
	}
	data := res.GetData()
	if !data.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	// CommitSHA should be empty when cache is nil
	if data.CommitSHA != "" {
		t.Errorf("expected CommitSHA='', got %q (cache is nil)", data.CommitSHA)
	}
}

func TestRunBaselineGates_CommitSHAPopulated(t *testing.T) {
	repoDir := makeTempGitRepo(t)
	manifest := makePassingManifest()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	cache := gatecache.New(context.Background(), stateDir, time.Minute)

	res := RunBaselineGates(manifest, 1, repoDir, cache)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %+v", res.Errors)
	}
	data := res.GetData()
	if data.CommitSHA == "" {
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

	res := RunCrossRepoBaselineGates(&IMPLManifest{}, 1, targetRepos, configRepos)
	data := res.GetData()
	if !data.Passed {
		t.Error("expected all repos to pass")
	}
	if len(data.Results) != 2 {
		t.Errorf("expected 2 repo results, got %d", len(data.Results))
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

	res := RunCrossRepoBaselineGates(&IMPLManifest{}, 1, targetRepos, configRepos)
	data := res.GetData()
	if data.Passed {
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

	res := RunCrossRepoBaselineGates(manifest, 1, targetRepos, configRepos)
	data := res.GetData()
	if !data.Passed {
		t.Error("expected pass with manifest gate fallback")
	}
}

func TestRunCrossRepoBaselineGates_WithTestCommand(t *testing.T) {
	dir := t.TempDir()
	targetRepos := map[string]string{"repo-a": dir}
	configRepos := []RepoEntry{
		{Name: "repo-a", Path: dir, BuildCommand: "echo build", TestCommand: "echo test"},
	}

	res := RunCrossRepoBaselineGates(&IMPLManifest{}, 1, targetRepos, configRepos)
	data := res.GetData()
	if !data.Passed {
		t.Error("expected pass")
	}
	// Should have 2 gate results (build + test)
	if r, ok := data.Results["repo-a"]; ok {
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
