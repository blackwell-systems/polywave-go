package protocol

import (
	"testing"
)

func TestValidateWorktreeNames_Valid(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task a", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:   "complete",
				Branch:   "wave1-agent-A",
				Worktree: ".claude/worktrees/wave1-agent-A",
				Commit:   "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid worktree names, got %d: %v", len(errs), errs)
	}
}

func TestValidateWorktreeNames_InvalidBranch(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task a", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status: "complete",
				Branch: "feature-branch",
				Commit: "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E5_INVALID_WORKTREE_NAME" {
		t.Errorf("Expected E5_INVALID_WORKTREE_NAME, got %s", errs[0].Code)
	}
}

func TestValidateWorktreeNames_WrongWave(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task a", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status: "complete",
				Branch: "wave2-agent-A",
				Commit: "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for wrong wave number, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E5_INVALID_WORKTREE_NAME" {
		t.Errorf("Expected E5_INVALID_WORKTREE_NAME, got %s", errs[0].Code)
	}
}

func TestValidateWorktreeNames_EmptyBranch(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task a", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status: "complete",
				Branch: "",
				Commit: "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty branch (backward compat), got %d: %v", len(errs), errs)
	}
}

func TestValidateWorktreeNames_ValidPath(t *testing.T) {
	testCases := []struct {
		name     string
		worktree string
	}{
		{
			name:     "relative path",
			worktree: ".claude/worktrees/wave1-agent-A",
		},
		{
			name:     "absolute path",
			worktree: "/home/user/repo/.claude/worktrees/wave1-agent-A",
		},
		{
			name:     "Windows path",
			worktree: "C:\\Users\\user\\repo\\.claude\\worktrees\\wave1-agent-A",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &IMPLManifest{
				Waves: []Wave{
					{
						Number: 1,
						Agents: []Agent{
							{ID: "A", Task: "task a", Files: []string{"file1.go"}},
						},
					},
				},
				CompletionReports: map[string]CompletionReport{
					"A": {
						Status:   "complete",
						Worktree: tc.worktree,
						Commit:   "abc123",
					},
				},
			}

			errs := ValidateWorktreeNames(m)
			if len(errs) != 0 {
				t.Errorf("Expected no errors for valid worktree path %q, got %d: %v", tc.worktree, len(errs), errs)
			}
		})
	}
}

func TestValidateWorktreeNames_InvalidPath(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task a", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:   "complete",
				Worktree: ".claude/worktrees/feature-branch",
				Commit:   "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for invalid worktree path, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E5_INVALID_WORKTREE_PATH" {
		t.Errorf("Expected E5_INVALID_WORKTREE_PATH, got %s", errs[0].Code)
	}
}

func TestValidateWorktreeNames_MultiDigitAgentID(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C2", Task: "task c2", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"C2": {
				Status:   "complete",
				Branch:   "wave2-agent-C2",
				Worktree: ".claude/worktrees/wave2-agent-C2",
				Commit:   "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for multi-digit agent ID, got %d: %v", len(errs), errs)
	}
}

func TestValidateWorktreeNames_WrongAgentID(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task a", Files: []string{"file1.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status: "complete",
				Branch: "wave1-agent-B",
				Commit: "abc123",
			},
		},
	}

	errs := ValidateWorktreeNames(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for wrong agent ID, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E5_INVALID_WORKTREE_NAME" {
		t.Errorf("Expected E5_INVALID_WORKTREE_NAME, got %s", errs[0].Code)
	}
}

func TestValidateVerificationField_Pass(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				Verification: "PASS",
				Commit:       "abc123",
			},
		},
	}

	errs := ValidateVerificationField(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for 'PASS', got %d: %v", len(errs), errs)
	}
}

func TestValidateVerificationField_FailWithDetails(t *testing.T) {
	testCases := []struct {
		name         string
		verification string
	}{
		{
			name:         "simple fail with details",
			verification: "FAIL (go test - 3/5 tests)",
		},
		{
			name:         "fail with parentheses in details",
			verification: "FAIL (build error: missing dependency (pkg/foo))",
		},
		{
			name:         "fail with simple message",
			verification: "FAIL (timeout)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &IMPLManifest{
				CompletionReports: map[string]CompletionReport{
					"A": {
						Status:       "complete",
						Verification: tc.verification,
						Commit:       "abc123",
					},
				},
			}

			errs := ValidateVerificationField(m)
			if len(errs) != 0 {
				t.Errorf("Expected no errors for %q, got %d: %v", tc.verification, len(errs), errs)
			}
		})
	}
}

func TestValidateVerificationField_Invalid(t *testing.T) {
	testCases := []struct {
		name         string
		verification string
	}{
		{
			name:         "lowercase pass",
			verification: "pass",
		},
		{
			name:         "arbitrary text",
			verification: "maybe",
		},
		{
			name:         "incomplete format",
			verification: "FAIL incomplete",
		},
		{
			name:         "wrong case",
			verification: "Pass",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &IMPLManifest{
				CompletionReports: map[string]CompletionReport{
					"A": {
						Status:       "complete",
						Verification: tc.verification,
						Commit:       "abc123",
					},
				},
			}

			errs := ValidateVerificationField(m)
			if len(errs) != 1 {
				t.Fatalf("Expected 1 error for invalid verification %q, got %d: %v", tc.verification, len(errs), errs)
			}
			if errs[0].Code != "E10_INVALID_VERIFICATION" {
				t.Errorf("Expected E10_INVALID_VERIFICATION, got %s", errs[0].Code)
			}
		})
	}
}

func TestValidateVerificationField_Empty(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				Verification: "",
				Commit:       "abc123",
			},
		},
	}

	errs := ValidateVerificationField(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty verification (backward compat), got %d: %v", len(errs), errs)
	}
}

func TestValidateVerificationField_MultipleReports(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				Verification: "PASS",
				Commit:       "abc123",
			},
			"B": {
				Status:       "complete",
				Verification: "FAIL (lint errors)",
				Commit:       "def456",
			},
			"C": {
				Status:       "complete",
				Verification: "invalid",
				Commit:       "ghi789",
			},
		},
	}

	errs := ValidateVerificationField(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for agent C, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E10_INVALID_VERIFICATION" {
		t.Errorf("Expected E10_INVALID_VERIFICATION, got %s", errs[0].Code)
	}
}
