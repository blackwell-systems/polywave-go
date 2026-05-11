package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// writeReconcileTestManifest writes a minimal IMPL manifest to a temp dir.
// agents is a map of agentID -> completion report (nil means no report).
func writeReconcileTestManifest(t *testing.T, state protocol.ProtocolState, agents map[string]*protocol.CompletionReport, criticReport *protocol.CriticData) string {
	t.Helper()

	waves := []protocol.Wave{
		{
			Number: 1,
			Agents: []protocol.Agent{},
		},
	}
	fileOwnership := []protocol.FileOwnership{}
	for agentID := range agents {
		waves[0].Agents = append(waves[0].Agents, protocol.Agent{
			ID:   agentID,
			Task: "Implement " + agentID,
		})
		fileOwnership = append(fileOwnership, protocol.FileOwnership{
			File:  "pkg/" + strings.ToLower(agentID) + ".go",
			Agent: agentID,
			Wave:  1,
		})
	}

	completionReports := make(map[string]protocol.CompletionReport)
	for agentID, report := range agents {
		if report != nil {
			completionReports[agentID] = *report
		}
	}

	m := &protocol.IMPLManifest{
		Title:             "Test Feature",
		FeatureSlug:       "test-feature",
		Verdict:           "SUITABLE",
		TestCommand:       "go test ./...",
		LintCommand:       "go vet ./...",
		State:             state,
		FileOwnership:     fileOwnership,
		Waves:             waves,
		CompletionReports: completionReports,
		CriticReport:      criticReport,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "IMPL-test.yaml")
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// initTestRepo creates a real git repo in dir with an initial commit.
// Returns the commit SHA of the initial commit.
func initTestRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, c := range cmds {
		out, err := git.Run(dir, c[2:]...)
		if err != nil && c[1] != "-C" {
			// git init doesn't use -C format; run directly
			_ = out
		}
	}

	// Use git.Run for all git operations.
	if _, err := git.Run(dir, "init"); err != nil {
		// Already initialized from the init above — ignore "reinit" message.
	}
	if _, err := git.Run(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if _, err := git.Run(dir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config name: %v", err)
	}

	// Create an initial commit.
	readmeFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("test repo\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := git.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := git.Run(dir, "commit", "--no-verify", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	sha, err := git.RevParse(dir, "HEAD")
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return dir, sha
}

// TestReconcileState_AllComplete verifies that when all agents have completion
// reports with commits reachable from HEAD, the derived state is WAVE_VERIFIED.
func TestReconcileState_AllComplete(t *testing.T) {
	// Create a real git repo so IsAncestor works.
	gitDir, sha := initTestRepo(t)

	agents := map[string]*protocol.CompletionReport{
		"A": {Status: "complete", Commit: sha, Branch: "polywave/test-feature/wave1-agent-A"},
		"B": {Status: "complete", Commit: sha, Branch: "polywave/test-feature/wave1-agent-B"},
	}
	manifestPath := writeReconcileTestManifest(t, protocol.StateWaveExecuting, agents, nil)

	// Inject repoDir so resolveAgentRepo uses our test git repo.
	origRepoDir := repoDir
	repoDir = gitDir
	defer func() { repoDir = origRepoDir }()

	err := runReconcileState(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("runReconcileState: %v", err)
	}

	// Reload manifest to verify state was updated.
	updated, err := protocol.Load(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.State != protocol.StateWaveVerified {
		t.Errorf("state = %q; want %q", updated.State, protocol.StateWaveVerified)
	}
}

// TestReconcileState_NoProgress verifies that an IMPL with no agents, no
// branches, and no reports stays at SCOUT_PENDING (no state change).
func TestReconcileState_NoProgress(t *testing.T) {
	agents := map[string]*protocol.CompletionReport{
		"A": nil,
	}
	manifestPath := writeReconcileTestManifest(t, protocol.StateScoutPending, agents, nil)

	err := runReconcileState(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("runReconcileState: %v", err)
	}

	updated, err := protocol.Load(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.State != protocol.StateScoutPending {
		t.Errorf("state = %q; want %q (no change expected)", updated.State, protocol.StateScoutPending)
	}
}

// TestReconcileState_PartialProgress verifies that when only 1 of 2 agents has
// a completion report, the derived state is WAVE_EXECUTING.
// The manifest starts in WAVE_EXECUTING (agents are running) to allow the transition.
func TestReconcileState_PartialProgress(t *testing.T) {
	gitDir, sha := initTestRepo(t)

	agents := map[string]*protocol.CompletionReport{
		"A": {Status: "complete", Commit: sha},
		"B": nil, // no report yet
	}
	manifestPath := writeReconcileTestManifest(t, protocol.StateWaveExecuting, agents, nil)

	origRepoDir := repoDir
	repoDir = gitDir
	defer func() { repoDir = origRepoDir }()

	err := runReconcileState(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("runReconcileState: %v", err)
	}

	updated, err := protocol.Load(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.State != protocol.StateWaveExecuting {
		t.Errorf("state = %q; want %q", updated.State, protocol.StateWaveExecuting)
	}
}

// TestReconcileState_CriticReportPresent verifies that when a CriticReport
// with a non-empty Verdict exists and the current state is SCOUT_PENDING,
// the derived state is REVIEWED.
func TestReconcileState_CriticReportPresent(t *testing.T) {
	agents := map[string]*protocol.CompletionReport{
		"A": nil,
	}
	criticReport := &protocol.CriticData{
		Verdict: "PASS",
		Summary: "Looks good",
	}
	manifestPath := writeReconcileTestManifest(t, protocol.StateScoutPending, agents, criticReport)

	err := runReconcileState(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("runReconcileState: %v", err)
	}

	updated, err := protocol.Load(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.State != protocol.StateReviewed {
		t.Errorf("state = %q; want %q", updated.State, protocol.StateReviewed)
	}
}

// TestReconcileState_AlreadyComplete verifies that when the manifest is in
// COMPLETE state, reconcile-state returns early with no state change.
func TestReconcileState_AlreadyComplete(t *testing.T) {
	agents := map[string]*protocol.CompletionReport{
		"A": {Status: "complete"},
	}
	manifestPath := writeReconcileTestManifest(t, protocol.StateComplete, agents, nil)

	err := runReconcileState(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("runReconcileState: %v", err)
	}

	// State must remain COMPLETE.
	updated, err := protocol.Load(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.State != protocol.StateComplete {
		t.Errorf("state = %q; want %q (should be unchanged)", updated.State, protocol.StateComplete)
	}
}
