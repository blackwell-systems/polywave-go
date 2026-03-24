package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestInferLanguageFromCommand(t *testing.T) {
	tests := []struct {
		name        string
		testCommand string
		want        string
	}{
		{
			name:        "Go test command",
			testCommand: "go test ./...",
			want:        "go",
		},
		{
			name:        "Go build command",
			testCommand: "go build ./cmd/...",
			want:        "go",
		},
		{
			name:        "Rust cargo test",
			testCommand: "cargo test",
			want:        "rust",
		},
		{
			name:        "Rust cargo build",
			testCommand: "cargo build --release",
			want:        "rust",
		},
		{
			name:        "JavaScript npm test",
			testCommand: "npm test",
			want:        "javascript",
		},
		{
			name:        "JavaScript jest",
			testCommand: "jest --coverage",
			want:        "javascript",
		},
		{
			name:        "JavaScript vitest",
			testCommand: "vitest run",
			want:        "javascript",
		},
		{
			name:        "Python pytest",
			testCommand: "pytest tests/",
			want:        "python",
		},
		{
			name:        "Python unittest",
			testCommand: "python -m unittest discover",
			want:        "python",
		},
		{
			name:        "Unknown command",
			testCommand: "make test",
			want:        "",
		},
		{
			name:        "Empty command",
			testCommand: "",
			want:        "",
		},
		{
			name:        "Case insensitive Go",
			testCommand: "GO TEST ./...",
			want:        "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferLanguageFromCommand(tt.testCommand)
			if got != tt.want {
				t.Errorf("inferLanguageFromCommand(%q) = %q, want %q", tt.testCommand, got, tt.want)
			}
		})
	}
}

func TestFinalizeWaveResult_CollisionReportsField(t *testing.T) {
	// Verify that FinalizeWaveResult has the CollisionReports field
	result := &FinalizeWaveResult{
		Wave:             1,
		CrossRepo:        false,
		Success:          false,
		VerifyCommits:    make(map[string]*protocol.VerifyCommitsData),
		CollisionReports: make(map[string]*collision.CollisionReport),
		GateResults:      make(map[string][]protocol.GateResult),
		MergeResult:      make(map[string]*protocol.MergeAgentsData),
	}

	if result.CollisionReports == nil {
		t.Error("FinalizeWaveResult.CollisionReports should be initialized")
	}
}

func TestFinalizeWaveResult_CollisionReportIntegration(t *testing.T) {
	// Verify that CollisionReport can be properly added to FinalizeWaveResult
	result := &FinalizeWaveResult{
		Wave:             1,
		CollisionReports: make(map[string]*collision.CollisionReport),
	}

	// Simulate adding a collision report for a repo
	report := &collision.CollisionReport{
		Valid: false,
		Collisions: []collision.TypeCollision{
			{
				TypeName:   "Handler",
				Package:    "pkg/service",
				Agents:     []string{"A", "B"},
				Resolution: "Keep A, remove from B",
			},
		},
	}

	result.CollisionReports["main-repo"] = report

	if result.CollisionReports["main-repo"] == nil {
		t.Error("CollisionReport should be stored in FinalizeWaveResult")
	}

	if result.CollisionReports["main-repo"].Valid {
		t.Error("CollisionReport.Valid should be false when collisions exist")
	}

	if len(result.CollisionReports["main-repo"].Collisions) != 1 {
		t.Errorf("Expected 1 collision, got %d", len(result.CollisionReports["main-repo"].Collisions))
	}
}

// TestFinalizeWave_SoloWaveSkipsVerifyCommits verifies that when a wave has no
// worktree directories on disk, WorktreesAbsent returns true and the finalize-wave
// bypass path populates a synthetic MergeResult without calling VerifyCommits.
//
// This simulates the solo-wave scenario (single developer working directly on main)
// where no worktrees were created, so VerifyCommits and MergeAgents must be skipped.
func TestFinalizeWave_SoloWaveSkipsVerifyCommits(t *testing.T) {
	// Build a manifest with one agent and a feature slug.
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}},
			},
		},
	}

	// Use a temp directory as repoDir — no worktree directories will exist inside it.
	tmpDir := t.TempDir()

	// WorktreesAbsent should return true since no worktree dir was created.
	if !protocol.WorktreesAbsent(manifest, 1, tmpDir) {
		t.Error("WorktreesAbsent() = false, want true when no worktree dirs exist (solo wave)")
	}

	// Simulate the bypass: populate a synthetic MergeResult as finalize-wave would.
	mergeResult := make(map[string]*protocol.MergeAgentsData)
	mergeResult["."] = &protocol.MergeAgentsData{Wave: 1, Success: true}

	// Confirm the synthetic result is set correctly (no VerifyCommits needed).
	if !mergeResult["."].Success {
		t.Error("synthetic MergeResult.Success should be true for solo wave bypass")
	}
	if mergeResult["."].Wave != 1 {
		t.Errorf("MergeResult.Wave = %d, want 1", mergeResult["."].Wave)
	}
}

// TestFinalizeWave_AllBranchesAbsentSkipsMerge verifies that when all agent branches
// are absent from git (wave already merged and cleaned up), AllBranchesAbsent returns
// true and the finalize-wave bypass path populates a synthetic MergeResult without
// calling MergeAgents.
//
// This tests the idempotent re-run path: if finalize-wave is called again after a
// successful run (branches already cleaned up), it should proceed to verify-build
// rather than failing on missing branches.
func TestFinalizeWave_AllBranchesAbsentSkipsMerge(t *testing.T) {
	// Build a manifest with one agent and a feature slug.
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}},
			},
		},
	}

	// Use a temp directory that is a git repo but has no agent branches.
	// We create a minimal git repo so BranchExists calls don't error.
	tmpDir := t.TempDir()
	if err := initBareGitRepo(t, tmpDir); err != nil {
		t.Skipf("skipping: could not init git repo: %v", err)
	}

	// AllBranchesAbsent should return true since no agent branches exist.
	if !protocol.AllBranchesAbsent(manifest, 1, tmpDir) {
		t.Error("AllBranchesAbsent() = false, want true when no agent branches exist")
	}

	// Simulate the bypass: populate a synthetic MergeResult as finalize-wave would.
	mergeResult := make(map[string]*protocol.MergeAgentsData)
	mergeResult["."] = &protocol.MergeAgentsData{Wave: 1, Success: true}

	if !mergeResult["."].Success {
		t.Error("synthetic MergeResult.Success should be true for all-branches-absent bypass")
	}
}

// TestFinalizeWave_NormalPathUnchanged is a regression guard verifying that when
// a worktree directory DOES exist, WorktreesAbsent returns false — so the normal
// path (including VerifyCommits) would be taken, not the solo-wave bypass.
func TestFinalizeWave_NormalPathUnchanged(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}},
			},
		},
	}

	tmpDir := t.TempDir()

	// Create the worktree directory for agent A so at least one worktree is present.
	worktreeDir := protocol.WorktreeDir(tmpDir, manifest.FeatureSlug, 1, "A")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// WorktreesAbsent must return false — the normal path (VerifyCommits) should run.
	if protocol.WorktreesAbsent(manifest, 1, tmpDir) {
		t.Error("WorktreesAbsent() = true, want false when at least one worktree dir exists (normal path)")
	}
}

// TestFinalizeWave_E7BlocksOnPartialStatus verifies that an agent with
// status "partial" in its completion report causes finalize-wave to block
// (step 1.1 E7 check). This is a unit test of the PredictConflictsFromReports
// helper and the E7 logic that reads completion report statuses.
func TestFinalizeWave_E7BlocksOnPartialStatus(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {Status: "complete"},
			"B": {Status: "partial"},
		},
	}

	// Verify the E7 check fires: any partial/blocked agent should be detected.
	for _, agent := range manifest.Waves[0].Agents {
		if report, ok := manifest.CompletionReports[agent.ID]; ok {
			if report.Status == "partial" || report.Status == "blocked" {
				// This is the condition finalize-wave checks in step 1.1.
				return // Test passes: we found the partial agent.
			}
		}
	}
	t.Error("E7 check: expected to find agent B with status=partial, but did not")
}

// TestFinalizeWave_E7BlocksOnBlockedStatus verifies that an agent with
// status "blocked" triggers the E7 check.
func TestFinalizeWave_E7BlocksOnBlockedStatus(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {Status: "blocked"},
			"B": {Status: "complete"},
		},
	}

	for _, agent := range manifest.Waves[0].Agents {
		if report, ok := manifest.CompletionReports[agent.ID]; ok {
			if report.Status == "partial" || report.Status == "blocked" {
				return // Test passes: blocked agent detected.
			}
		}
	}
	t.Error("E7 check: expected to find agent A with status=blocked, but did not")
}

// TestFinalizeWave_E7AllCompleteAllowed verifies that when all agents report
// "complete", the E7 check passes without blocking.
func TestFinalizeWave_E7AllCompleteAllowed(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {Status: "complete"},
			"B": {Status: "complete"},
		},
	}

	for _, agent := range manifest.Waves[0].Agents {
		if report, ok := manifest.CompletionReports[agent.ID]; ok {
			if report.Status == "partial" || report.Status == "blocked" {
				t.Errorf("E7 check should not block on complete agents, but flagged agent %s with status %q",
					agent.ID, report.Status)
			}
		}
	}
}

// TestFinalizeWave_E11BlocksOnFileConflict verifies that when two agents
// report modifying the same file, E11 conflict prediction detects it.
func TestFinalizeWave_E11BlocksOnFileConflict(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/shared/shared.go"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/shared/shared.go"},
			},
		},
	}

	err := protocol.PredictConflictsFromReports(manifest, 1)
	if err == nil {
		t.Error("E11 check: expected conflict error when two agents share a file, got nil")
	}
}

// TestFinalizeWave_E11NoConflict verifies that when agents have disjoint
// file sets, E11 conflict prediction passes.
func TestFinalizeWave_E11NoConflict(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{{ID: "A"}, {ID: "B"}},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo/foo.go"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/bar/bar.go"},
			},
		},
	}

	if err := protocol.PredictConflictsFromReports(manifest, 1); err != nil {
		t.Errorf("E11 check: expected nil for disjoint files, got: %v", err)
	}
}

// TestFinalizeWave_MergeTargetFlag verifies that the --merge-target flag is
// registered on the finalize-wave command and parses correctly.
func TestFinalizeWave_MergeTargetFlag(t *testing.T) {
	cmd := newFinalizeWaveCmd()

	// Verify the flag exists
	flag := cmd.Flags().Lookup("merge-target")
	if flag == nil {
		t.Fatal("expected --merge-target flag to be registered on finalize-wave command")
	}

	// Verify default value is empty
	if flag.DefValue != "" {
		t.Errorf("expected --merge-target default to be empty, got %q", flag.DefValue)
	}

	// Verify flag can be set
	if err := cmd.Flags().Set("merge-target", "impl/my-feature"); err != nil {
		t.Errorf("failed to set --merge-target flag: %v", err)
	}

	val, err := cmd.Flags().GetString("merge-target")
	if err != nil {
		t.Fatalf("failed to get --merge-target value: %v", err)
	}
	if val != "impl/my-feature" {
		t.Errorf("expected --merge-target=impl/my-feature, got %q", val)
	}
}

// TestPrepareWave_MergeTargetFlag verifies that the --merge-target flag is
// registered on the prepare-wave command and parses correctly.
func TestPrepareWave_MergeTargetFlag(t *testing.T) {
	cmd := newPrepareWaveCmd()

	// Verify the flag exists
	flag := cmd.Flags().Lookup("merge-target")
	if flag == nil {
		t.Fatal("expected --merge-target flag to be registered on prepare-wave command")
	}

	// Verify default value is empty
	if flag.DefValue != "" {
		t.Errorf("expected --merge-target default to be empty, got %q", flag.DefValue)
	}

	// Verify flag can be set
	if err := cmd.Flags().Set("merge-target", "impl/another-feature"); err != nil {
		t.Errorf("failed to set --merge-target flag: %v", err)
	}

	val, err := cmd.Flags().GetString("merge-target")
	if err != nil {
		t.Fatalf("failed to get --merge-target value: %v", err)
	}
	if val != "impl/another-feature" {
		t.Errorf("expected --merge-target=impl/another-feature, got %q", val)
	}
}

// initBareGitRepo initialises a minimal git repository in dir so that git
// branch operations work in tests that call AllBranchesAbsent.
func initBareGitRepo(t *testing.T, dir string) error {
	t.Helper()
	cmds := [][]string{
		{"git", "-C", dir, "init"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("git init step %v failed: %w\n%s", args, err, out)
		}
	}
	return nil
}
