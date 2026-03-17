package resume

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// writeManifest serialises a manifest to a temp directory as a .yaml file and
// returns the path to the file.
func writeManifest(t *testing.T, dir, filename string, manifest *protocol.IMPLManifest) string {
	t.Helper()
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("writeManifest: marshal: %v", err)
	}
	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatalf("writeManifest: write %s: %v", p, err)
	}
	return p
}

// makeRepoWithIMPLDir creates a temporary directory structure with docs/IMPL/
// and returns the repo root.
func makeRepoWithIMPLDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "IMPL", "complete"), 0755); err != nil {
		t.Fatalf("makeRepoWithIMPLDir: %v", err)
	}
	return root
}

// simpleManifest returns a minimal IMPLManifest with one wave and two agents.
func simpleManifest() *protocol.IMPLManifest {
	return &protocol.IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "implement A"},
					{ID: "B", Task: "implement B"},
				},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{},
	}
}

// ---- Tests ----

func TestDetect_NoIMPLDocs(t *testing.T) {
	root := makeRepoWithIMPLDir(t)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestDetect_NoIMPLDir(t *testing.T) {
	root := t.TempDir() // no docs/IMPL at all

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("expected no error for missing IMPL dir, got %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestDetect_CompleteIMPL(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateComplete
	writeManifest(t, implDir, "IMPL-test.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("complete IMPL should be skipped, got %d sessions", len(sessions))
	}
}

func TestDetect_NotSuitableIMPL(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateNotSuitable
	writeManifest(t, implDir, "IMPL-ns.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("NOT_SUITABLE IMPL should be skipped, got %d sessions", len(sessions))
	}
}

func TestDetect_CompleteInSubdir_Skipped(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	completeDir := filepath.Join(root, "docs", "IMPL", "complete")

	// Write a non-complete manifest into the complete/ subdirectory.
	m := simpleManifest()
	writeManifest(t, completeDir, "IMPL-old.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("manifests in complete/ should be ignored, got %d sessions", len(sessions))
	}
}

func TestDetect_InProgressIMPL(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWaveExecuting
	// Agent A is done, B is not.
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "complete"}
	writeManifest(t, implDir, "IMPL-wip.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.IMPLSlug != "test-feature" {
		t.Errorf("IMPLSlug = %q, want %q", s.IMPLSlug, "test-feature")
	}
	if s.CurrentWave != 1 {
		t.Errorf("CurrentWave = %d, want 1", s.CurrentWave)
	}
	if s.TotalWaves != 1 {
		t.Errorf("TotalWaves = %d, want 1", s.TotalWaves)
	}
	if len(s.CompletedAgents) != 1 || s.CompletedAgents[0] != "A" {
		t.Errorf("CompletedAgents = %v, want [A]", s.CompletedAgents)
	}
	if len(s.PendingAgents) != 1 || s.PendingAgents[0] != "B" {
		t.Errorf("PendingAgents = %v, want [B]", s.PendingAgents)
	}
	if len(s.FailedAgents) != 0 {
		t.Errorf("FailedAgents = %v, want []", s.FailedAgents)
	}
}

func TestDetect_FailedAgent(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWaveExecuting
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "complete"}
	m.CompletionReports["B"] = protocol.CompletionReport{Status: "partial"}
	writeManifest(t, implDir, "IMPL-fail.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if len(s.FailedAgents) != 1 || s.FailedAgents[0] != "B" {
		t.Errorf("FailedAgents = %v, want [B]", s.FailedAgents)
	}
	if s.CanAutoResume {
		t.Errorf("CanAutoResume should be false when there are failed agents")
	}
}

func TestDetect_BlockedAgent(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateBlocked
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "blocked"}
	writeManifest(t, implDir, "IMPL-blocked.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if len(s.FailedAgents) != 1 || s.FailedAgents[0] != "A" {
		t.Errorf("FailedAgents = %v, want [A]", s.FailedAgents)
	}
}

func TestDetect_ProgressCalculation(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	// 2-wave manifest: wave1 has A & B, wave2 has C & D.
	m := &protocol.IMPLManifest{
		Title:       "Multi Wave",
		FeatureSlug: "multi-wave",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}}},
			{Number: 2, Agents: []protocol.Agent{{ID: "C"}, {ID: "D"}}},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {Status: "complete"},
			"B": {Status: "complete"},
		},
		State: protocol.StateWaveVerified,
	}
	writeManifest(t, implDir, "IMPL-multi.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]

	// 2 completed out of 4 total = 50%.
	if s.ProgressPct != 50.0 {
		t.Errorf("ProgressPct = %.1f, want 50.0", s.ProgressPct)
	}
	if s.CurrentWave != 1 {
		t.Errorf("CurrentWave = %d, want 1", s.CurrentWave)
	}
	if s.TotalWaves != 2 {
		t.Errorf("TotalWaves = %d, want 2", s.TotalWaves)
	}
}

func TestDetect_ProgressZeroAgents(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := &protocol.IMPLManifest{
		Title:             "Empty Wave",
		FeatureSlug:       "empty-wave",
		Verdict:           "SUITABLE",
		TestCommand:       "go test ./...",
		LintCommand:       "go vet ./...",
		Waves:             []protocol.Wave{{Number: 1, Agents: []protocol.Agent{}}},
		CompletionReports: map[string]protocol.CompletionReport{},
		State:             protocol.StateWavePending,
	}
	writeManifest(t, implDir, "IMPL-empty.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) == 1 && sessions[0].ProgressPct != 0.0 {
		t.Errorf("ProgressPct = %.1f for 0 agents, want 0.0", sessions[0].ProgressPct)
	}
}

func TestDetect_SuggestedAction_NoAgentsStarted(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWavePending
	writeManifest(t, implDir, "IMPL-new.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.SuggestedAction != "Start wave 1" {
		t.Errorf("SuggestedAction = %q, want %q", s.SuggestedAction, "Start wave 1")
	}
}

func TestDetect_SuggestedAction_FailedAgents(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWaveExecuting
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "partial"}
	writeManifest(t, implDir, "IMPL-failed.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	want := "Rerun failed agent(s): A"
	if s.SuggestedAction != want {
		t.Errorf("SuggestedAction = %q, want %q", s.SuggestedAction, want)
	}
}

func TestDetect_SuggestedAction_StartNextWave(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := &protocol.IMPLManifest{
		Title:       "Two Waves",
		FeatureSlug: "two-waves",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}}},
			{Number: 2, Agents: []protocol.Agent{{ID: "C"}}},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {Status: "complete"},
			"B": {Status: "complete"},
		},
		State: protocol.StateWaveVerified,
	}
	writeManifest(t, implDir, "IMPL-tw.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	want := "Start wave 2"
	if s.SuggestedAction != want {
		t.Errorf("SuggestedAction = %q, want %q", s.SuggestedAction, want)
	}
	if !s.CanAutoResume {
		t.Errorf("CanAutoResume should be true when wave 1 is fully complete with no failures")
	}
}

func TestDetect_SuggestedAction_ResumeCurrentWave(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWaveExecuting
	// Only A has a report; B is still pending.
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "complete"}
	writeManifest(t, implDir, "IMPL-resume.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	want := "Resume wave 1"
	if s.SuggestedAction != want {
		t.Errorf("SuggestedAction = %q, want %q", s.SuggestedAction, want)
	}
}

func TestDetect_ResumeCommand(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWavePending
	implPath := writeManifest(t, implDir, "IMPL-cmd.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	want := fmt.Sprintf("sawtools run-wave %s --wave 1", implPath)
	if s.ResumeCommand != want {
		t.Errorf("ResumeCommand = %q, want %q", s.ResumeCommand, want)
	}
}

func TestDetect_ResumeCommand_FailedAgent(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m := simpleManifest()
	m.State = protocol.StateWaveExecuting
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "partial"}
	implPath := writeManifest(t, implDir, "IMPL-fcmd.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	want := fmt.Sprintf("sawtools retry %s --wave 1", implPath)
	if s.ResumeCommand != want {
		t.Errorf("ResumeCommand = %q, want %q", s.ResumeCommand, want)
	}
}

func TestDetectOrphanedWorktrees_NoGit(t *testing.T) {
	// Use a directory that is not a git repo so git fails gracefully.
	manifest := simpleManifest()
	orphaned, err := detectOrphanedWorktrees(t.TempDir(), manifest)
	if err != nil {
		t.Fatalf("expected no error when git fails, got %v", err)
	}
	if len(orphaned) != 0 {
		t.Errorf("expected no orphaned worktrees, got %v", orphaned)
	}
}

func TestDetectOrphanedWorktrees_ParsePorcelain(t *testing.T) {
	// Build fake porcelain output.
	fake := `worktree /repo
HEAD abc123
branch refs/heads/main

worktree /repo/.worktrees/wave1-agent-A
HEAD def456
branch refs/heads/wave1-agent-A

`
	entries := parseWorktreePorcelain([]byte(fake))
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].path != "/repo" {
		t.Errorf("entries[0].path = %q, want /repo", entries[0].path)
	}
	if entries[1].path != "/repo/.worktrees/wave1-agent-A" {
		t.Errorf("entries[1].path = %q", entries[1].path)
	}
	if entries[1].branch != "refs/heads/wave1-agent-A" {
		t.Errorf("entries[1].branch = %q", entries[1].branch)
	}
}

func TestDetect_OutputIsArray(t *testing.T) {
	// Even for an empty result, JSON marshalling should produce an array.
	sessions := []SessionState{}
	data, err := json.Marshal(sessions)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("expected [], got %s", string(data))
	}
}

func TestDetect_MultipleIMPLs(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	m1 := simpleManifest()
	m1.FeatureSlug = "feature-one"
	m1.State = protocol.StateWavePending
	writeManifest(t, implDir, "IMPL-one.yaml", m1)

	m2 := simpleManifest()
	m2.FeatureSlug = "feature-two"
	m2.State = protocol.StateWaveExecuting
	m2.CompletionReports["A"] = protocol.CompletionReport{Status: "complete"}
	writeManifest(t, implDir, "IMPL-two.yaml", m2)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}
