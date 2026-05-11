package engine

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// initTestRepo creates a git repo in a temp dir with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	cmd := exec.Command("git", "-C", repoDir, "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "init").Run()
	return repoDir
}

func TestStepCommitState_CleanWorkDir(t *testing.T) {
	repoDir := initTestRepo(t)
	cb, events := collectStepEvents()

	opts := FinalizeWaveOpts{
		RepoPath: repoDir,
		IMPLPath: "/nonexistent/IMPL.yaml",
	}
	manifest := &protocol.IMPLManifest{}

	result, err := StepCommitState(context.Background(), opts, manifest, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status success, got %q", result.Status)
	}
	if result.Detail == "" || !strings.Contains(result.Detail, "clean") {
		t.Errorf("expected detail to contain 'clean', got %q", result.Detail)
	}
	if len(*events) == 0 {
		t.Fatal("expected at least one event")
	}
}

func TestStepCommitState_NilOnEvent(t *testing.T) {
	repoDir := initTestRepo(t)

	opts := FinalizeWaveOpts{
		RepoPath: repoDir,
		IMPLPath: "/nonexistent/IMPL.yaml",
	}
	manifest := &protocol.IMPLManifest{}

	// Should not panic with nil callback
	result, err := StepCommitState(context.Background(), opts, manifest, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status success, got %q", result.Status)
	}
}

func TestStepCommitState_EmitsRunningEvent(t *testing.T) {
	repoDir := initTestRepo(t)
	cb, events := collectStepEvents()

	opts := FinalizeWaveOpts{
		RepoPath: repoDir,
		IMPLPath: "/nonexistent/IMPL.yaml",
	}
	manifest := &protocol.IMPLManifest{}

	_, err := StepCommitState(context.Background(), opts, manifest, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*events) == 0 {
		t.Fatal("expected at least one event")
	}
	first := (*events)[0]
	if first.Step != "commit-state" {
		t.Errorf("expected first event step 'commit-state', got %q", first.Step)
	}
	if first.Status != "running" {
		t.Errorf("expected first event status 'running', got %q", first.Status)
	}
	if first.Detail != "" {
		t.Errorf("expected first event detail empty, got %q", first.Detail)
	}
}

