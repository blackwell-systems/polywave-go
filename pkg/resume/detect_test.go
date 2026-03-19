package resume

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestDetect_SkipsNoWorkStarted(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	// IMPL with no completion reports and no orphaned worktrees
	// should NOT be detected as interrupted — it hasn't started yet.
	m := simpleManifest()
	m.State = protocol.StateWavePending
	writeManifest(t, implDir, "IMPL-new.yaml", m)

	sessions, err := Detect(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions for not-yet-started IMPL, got %d", len(sessions))
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
	m.State = protocol.StateWaveExecuting
	// Need at least one completion report so the IMPL is detected as interrupted
	m.CompletionReports["A"] = protocol.CompletionReport{Status: "complete"}
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
	orphaned, err := detectOrphanedWorktrees([]string{t.TempDir()}, manifest)
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

func TestDetectOrphanedWorktrees_ScopedBranch(t *testing.T) {
	// Verify that slug-scoped branch names are detected in worktree list output.
	fake := `worktree /repo
HEAD abc123
branch refs/heads/main

worktree /repo/.claude/worktrees/saw/my-feature/wave1-agent-A
HEAD def456
branch refs/heads/saw/my-feature/wave1-agent-A

`
	entries := parseWorktreePorcelain([]byte(fake))
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].branch != "refs/heads/saw/my-feature/wave1-agent-A" {
		t.Errorf("entries[1].branch = %q", entries[1].branch)
	}

	// Now test that detectOrphanedWorktrees matches the scoped branch name.
	manifest := simpleManifest()
	// Agent A not complete -> worktree is orphaned.

	// We can't call detectOrphanedWorktrees directly with fake git output,
	// but we can verify the pattern matches the scoped format.
	candidate := "refs/heads/saw/my-feature/wave1-agent-A"
	candidate = strings.TrimPrefix(candidate, "refs/heads/")

	m := worktreePattern.FindStringSubmatch(candidate)
	if m == nil {
		t.Fatalf("worktreePattern did not match scoped branch %q", candidate)
	}
	if m[1] != "1" {
		t.Errorf("wave number = %q, want %q", m[1], "1")
	}
	if m[2] != "A" {
		t.Errorf("agent ID = %q, want %q", m[2], "A")
	}

	_ = manifest // used for context only
}

func TestDetectOrphanedWorktrees_LegacyAndScopedMixed(t *testing.T) {
	// Verify the pattern matches both legacy and scoped formats.
	tests := []struct {
		input   string
		wantW   string
		wantA   string
		wantOK  bool
	}{
		{"wave1-agent-A", "1", "A", true},
		{"saw/slug/wave2-agent-B3", "2", "B3", true},
		{"saw/my-feature/wave3-agent-C", "3", "C", true},
		{"feature-branch", "", "", false},
		{"main", "", "", false},
	}
	for _, tc := range tests {
		m := worktreePattern.FindStringSubmatch(tc.input)
		if tc.wantOK {
			if m == nil {
				t.Errorf("worktreePattern should match %q", tc.input)
				continue
			}
			if m[1] != tc.wantW || m[2] != tc.wantA {
				t.Errorf("for %q: wave=%q agent=%q, want wave=%q agent=%q", tc.input, m[1], m[2], tc.wantW, tc.wantA)
			}
		} else {
			if m != nil {
				t.Errorf("worktreePattern should NOT match %q", tc.input)
			}
		}
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
	m1.State = protocol.StateWaveExecuting
	m1.CompletionReports["A"] = protocol.CompletionReport{Status: "partial"}
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

// ---- New tests for DetectWithConfig, wave inference, and dirty worktree action ----

// TestDetectWithConfig_EmptyRepoList verifies that an empty repo list returns
// empty results with no error.
func TestDetectWithConfig_EmptyRepoList(t *testing.T) {
	sessions, err := DetectWithConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

// TestDetectWithConfig_MultipleRepos verifies that DetectWithConfig scans IMPL docs
// across multiple repo paths. Each repo contributes its own sessions independently.
func TestDetectWithConfig_MultipleRepos(t *testing.T) {
	// Create two independent repos, each with an in-progress IMPL.
	repo1 := makeRepoWithIMPLDir(t)
	repo2 := makeRepoWithIMPLDir(t)

	m1 := simpleManifest()
	m1.FeatureSlug = "repo1-feature"
	m1.State = protocol.StateWaveExecuting
	m1.CompletionReports["A"] = protocol.CompletionReport{Status: "complete"}
	writeManifest(t, filepath.Join(repo1, "docs", "IMPL"), "IMPL-r1.yaml", m1)

	m2 := simpleManifest()
	m2.FeatureSlug = "repo2-feature"
	m2.State = protocol.StateWaveExecuting
	m2.CompletionReports["A"] = protocol.CompletionReport{Status: "partial"}
	writeManifest(t, filepath.Join(repo2, "docs", "IMPL"), "IMPL-r2.yaml", m2)

	sessions, err := DetectWithConfig([]string{repo1, repo2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (one per repo), got %d", len(sessions))
	}

	slugs := map[string]bool{}
	for _, s := range sessions {
		slugs[s.IMPLSlug] = true
	}
	if !slugs["repo1-feature"] {
		t.Errorf("expected session for repo1-feature, got slugs: %v", slugs)
	}
	if !slugs["repo2-feature"] {
		t.Errorf("expected session for repo2-feature, got slugs: %v", slugs)
	}
}

// TestDetermineCurrentWave_FallbackToWorktrees verifies that when no completion
// reports exist, determineCurrentWave falls back to orphaned worktree branch paths
// and returns the lowest wave number found.
func TestDetermineCurrentWave_FallbackToWorktrees(t *testing.T) {
	manifest := simpleManifest()
	// No completion reports at all.

	// Simulate orphaned worktree paths that embed wave numbers in their path.
	// SAW worktrees are stored at paths like .claude/worktrees/saw/{slug}/wave{N}-agent-{ID}.
	orphaned := []string{
		"/repo/.claude/worktrees/saw/test-feature/wave2-agent-A",
		"/repo/.claude/worktrees/saw/test-feature/wave1-agent-B",
	}

	wave := determineCurrentWave(manifest, orphaned)

	// Should return the lowest wave (1), not the highest (2).
	if wave != 1 {
		t.Errorf("determineCurrentWave fallback = %d, want 1 (lowest wave with orphaned worktrees)", wave)
	}
}

// TestBuildAction_DirtyWorktrees verifies that buildActionAndCommand distinguishes
// between dirty and clean orphaned worktrees.
func TestBuildAction_DirtyWorktrees(t *testing.T) {
	root := makeRepoWithIMPLDir(t)
	implDir := filepath.Join(root, "docs", "IMPL")

	manifest := simpleManifest()
	manifest.State = protocol.StateWaveExecuting
	implPath := writeManifest(t, implDir, "IMPL-dirty.yaml", manifest)

	orphaned := []string{"/some/worktree/path/wave1-agent-A"}

	// Case 1: dirty worktree (HasChanges: true) -> "Resume wave N (agents have uncommitted work)"
	dirtyWorktrees := []DirtyWorktree{
		{Path: "/some/worktree/path/wave1-agent-A", Branch: "saw/test-feature/wave1-agent-A", AgentID: "A", WaveNum: 1, HasChanges: true},
	}
	action, _ := buildActionAndCommandInternal(implPath, manifest, orphaned, dirtyWorktrees, nil)
	wantDirty := "Resume wave 1 (agents have uncommitted work)"
	if action != wantDirty {
		t.Errorf("dirty action = %q, want %q", action, wantDirty)
	}

	// Case 2: clean worktree (HasChanges: false) -> "Clean up orphaned worktrees, then resume wave N"
	cleanWorktrees := []DirtyWorktree{
		{Path: "/some/worktree/path/wave1-agent-A", Branch: "saw/test-feature/wave1-agent-A", AgentID: "A", WaveNum: 1, HasChanges: false},
	}
	action, _ = buildActionAndCommandInternal(implPath, manifest, orphaned, cleanWorktrees, nil)
	wantClean := "Clean up orphaned worktrees, then resume wave 1"
	if action != wantClean {
		t.Errorf("clean action = %q, want %q", action, wantClean)
	}

	// Case 3: no dirty worktrees slice at all -> still produces cleanup action
	action, _ = buildActionAndCommandInternal(implPath, manifest, orphaned, nil, nil)
	if action != wantClean {
		t.Errorf("nil dirtyWorktrees action = %q, want %q", action, wantClean)
	}

	_ = implPath // referenced above
}
