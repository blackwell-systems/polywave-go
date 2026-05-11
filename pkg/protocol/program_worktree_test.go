package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProgramBranchName(t *testing.T) {
	tests := []struct {
		programSlug string
		tierNumber  int
		implSlug    string
		want        string
	}{
		{
			programSlug: "my-program",
			tierNumber:  1,
			implSlug:    "auth-service",
			want:        "saw/program/my-program/tier1-impl-auth-service",
		},
		{
			programSlug: "big-refactor",
			tierNumber:  2,
			implSlug:    "db-migration",
			want:        "saw/program/big-refactor/tier2-impl-db-migration",
		},
		{
			programSlug: "single",
			tierNumber:  10,
			implSlug:    "final",
			want:        "saw/program/single/tier10-impl-final",
		},
	}

	for _, tc := range tests {
		got := ProgramBranchName(tc.programSlug, tc.tierNumber, tc.implSlug)
		if got != tc.want {
			t.Errorf("ProgramBranchName(%q, %d, %q) = %q; want %q",
				tc.programSlug, tc.tierNumber, tc.implSlug, got, tc.want)
		}
	}
}

func TestProgramWorktreeDir(t *testing.T) {
	tests := []struct {
		repoDir     string
		programSlug string
		tierNumber  int
		implSlug    string
		want        string
	}{
		{
			repoDir:     "/home/user/myrepo",
			programSlug: "my-program",
			tierNumber:  1,
			implSlug:    "auth-service",
			want:        "/home/user/myrepo/.claude/worktrees/polywave/program/my-program/tier1-impl-auth-service",
		},
		{
			repoDir:     "/repos/project",
			programSlug: "big-refactor",
			tierNumber:  3,
			implSlug:    "api-gateway",
			want:        "/repos/project/.claude/worktrees/polywave/program/big-refactor/tier3-impl-api-gateway",
		},
	}

	for _, tc := range tests {
		got := ProgramWorktreeDir(tc.repoDir, tc.programSlug, tc.tierNumber, tc.implSlug)
		if got != tc.want {
			t.Errorf("ProgramWorktreeDir(%q, %q, %d, %q) = %q; want %q",
				tc.repoDir, tc.programSlug, tc.tierNumber, tc.implSlug, got, tc.want)
		}
	}
}

func TestCreateProgramWorktrees_TierNotFound(t *testing.T) {
	// Write a minimal PROGRAM manifest with only tier 1
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "PROGRAM-test.yaml")
	content := `title: Test Program
program_slug: test-program
state: TIER_EXECUTING
impls:
  - slug: impl-a
    title: Impl A
    tier: 1
    status: pending
tiers:
  - number: 1
    impls:
      - impl-a
tier_gates: []
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Request tier 99 which doesn't exist
	res := CreateProgramWorktrees(manifestPath, 99, dir, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal result for missing tier")
	}
}

func TestCreateProgramWorktrees_EmptyTier(t *testing.T) {
	// Write a PROGRAM manifest with a tier that has no IMPLs
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "PROGRAM-empty.yaml")
	content := `title: Empty Tier Program
program_slug: empty-program
state: TIER_EXECUTING
impls: []
tiers:
  - number: 2
    impls: []
tier_gates: []
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 0
  total_agents: 0
  total_waves: 0
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Tier 2 exists but has no IMPLs — should return empty result, no error
	res := CreateProgramWorktrees(manifestPath, 2, dir, nil)
	if res.IsFatal() {
		t.Fatalf("unexpected error for empty tier: %+v", res.Errors)
	}
	result := res.GetData()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.TierNumber != 2 {
		t.Errorf("TierNumber = %d; want 2", result.TierNumber)
	}
	if len(result.Worktrees) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(result.Worktrees))
	}
}
