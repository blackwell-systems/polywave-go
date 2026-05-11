package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// minimalIMPLContent is a minimal valid IMPL manifest for use in tests.
// It includes the required waves/agents structure so protocol.Load does not
// reject completion report keys referencing unknown agents.
const minimalIMPLContent = `title: Test IMPL
feature_slug: test-impl
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
waves:
  - number: 1
    agents:
      - id: A
        task: "Implement foo"
        files:
          - pkg/foo/foo.go
`

// writeTestIMPL creates a temporary IMPL doc in a temp directory and returns
// the absolute path to the file.
func writeTestIMPL(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("failed to create docs/IMPL dir: %v", err)
	}
	implPath := filepath.Join(docsDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test IMPL: %v", err)
	}
	return implPath
}

// TestRunCriticCmd_BackendFlagRegistered verifies the --backend flag exists on
// the run-critic command and defaults to "cli".
func TestRunCriticCmd_BackendFlagRegistered(t *testing.T) {
	cmd := newRunCriticCmd()
	flag := cmd.Flags().Lookup("backend")
	if flag == nil {
		t.Fatal("expected --backend flag to be registered on run-critic command")
	}
	if flag.DefValue != "cli" {
		t.Errorf("expected --backend default to be %q, got %q", "cli", flag.DefValue)
	}
}

// TestRunCriticCmd_SkipFlagStillWorks verifies the --skip flag is still
// registered on the run-critic command after adding --backend.
func TestRunCriticCmd_SkipFlagStillWorks(t *testing.T) {
	cmd := newRunCriticCmd()
	flag := cmd.Flags().Lookup("skip")
	if flag == nil {
		t.Fatal("expected --skip flag to be registered on run-critic command")
	}
}

// TestRunCriticCmd_SkipFlag verifies that --skip writes a PASS result with
// "Skipped by operator" summary and exits 0 without launching an agent.
func TestRunCriticCmd_SkipFlag(t *testing.T) {
	implPath := writeTestIMPL(t, minimalIMPLContent)

	var stdout bytes.Buffer

	cmd := newRunCriticCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{implPath, "--skip"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run-critic --skip failed: %v", err)
	}

	// Load the IMPL doc and verify critic_report was written
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to load IMPL after run-critic --skip: %v", err)
	}

	result := protocol.GetCriticReview(context.Background(), manifest)
	if result == nil {
		t.Fatal("expected critic_report to be written, got nil")
	}

	if result.Verdict != "PASS" {
		t.Errorf("expected verdict PASS, got %q", result.Verdict)
	}

	if result.Summary != "Skipped by operator" {
		t.Errorf("expected summary %q, got %q", "Skipped by operator", result.Summary)
	}

	if result.IssueCount != 0 {
		t.Errorf("expected issue_count 0, got %d", result.IssueCount)
	}

	if result.ReviewedAt == "" {
		t.Error("expected reviewed_at to be set")
	}
}

// TestRunCriticCmd_NoReviewFlag verifies that --no-review behaves identically to --skip.
func TestRunCriticCmd_NoReviewFlag(t *testing.T) {
	implPath := writeTestIMPL(t, minimalIMPLContent)

	var stdout bytes.Buffer

	cmd := newRunCriticCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{implPath, "--no-review"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run-critic --no-review failed: %v", err)
	}

	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to load IMPL after run-critic --no-review: %v", err)
	}

	result := protocol.GetCriticReview(context.Background(), manifest)
	if result == nil {
		t.Fatal("expected critic_report to be written, got nil")
	}

	if result.Verdict != "PASS" {
		t.Errorf("expected verdict PASS, got %q", result.Verdict)
	}

	if result.Summary != "Skipped by operator" {
		t.Errorf("expected summary %q, got %q", "Skipped by operator", result.Summary)
	}
}

// TestRunCriticCmd_MissingImplPath verifies that run-critic errors when given
// a non-existent IMPL path.
func TestRunCriticCmd_MissingImplPath(t *testing.T) {
	cmd := newRunCriticCmd()
	cmd.SetArgs([]string{"/nonexistent/path/IMPL-missing.yaml", "--skip"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing IMPL path, got nil")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

// TestSetCriticReviewCmd_ValidInput verifies that set-critic-review with valid
// JSON input writes the correct CriticData to the IMPL doc.
func TestSetCriticReviewCmd_ValidInput(t *testing.T) {
	implPath := writeTestIMPL(t, minimalIMPLContent)

	agentReviewsJSON := `[
		{
			"agent_id": "A",
			"verdict": "PASS",
			"issues": []
		},
		{
			"agent_id": "B",
			"verdict": "ISSUES",
			"issues": [
				{
					"check": "symbol_accuracy",
					"severity": "error",
					"description": "Function Foo does not exist",
					"file": "pkg/foo/foo.go",
					"symbol": "Foo"
				}
			]
		}
	]`

	var stdout bytes.Buffer

	cmd := newSetCriticReviewCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		implPath,
		"--verdict", "ISSUES",
		"--summary", "Found 1 error in agent B",
		"--issue-count", "1",
		"--agent-reviews", agentReviewsJSON,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-critic-review failed: %v", err)
	}

	// Verify the output is valid JSON
	outStr := stdout.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(outStr), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, outStr)
	}

	if result["verdict"] != "ISSUES" {
		t.Errorf("output verdict: expected ISSUES, got %v", result["verdict"])
	}

	if result["saved"] != true {
		t.Errorf("output saved: expected true, got %v", result["saved"])
	}

	// Load the IMPL doc and verify the critic_report was written correctly
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to load IMPL after set-critic-review: %v", err)
	}

	criticResult := protocol.GetCriticReview(context.Background(), manifest)
	if criticResult == nil {
		t.Fatal("expected critic_report to be set, got nil")
	}

	if criticResult.Verdict != "ISSUES" {
		t.Errorf("verdict: expected ISSUES, got %q", criticResult.Verdict)
	}

	if criticResult.Summary != "Found 1 error in agent B" {
		t.Errorf("summary: expected %q, got %q", "Found 1 error in agent B", criticResult.Summary)
	}

	if criticResult.IssueCount != 1 {
		t.Errorf("issue_count: expected 1, got %d", criticResult.IssueCount)
	}

	// Verify agent reviews were written
	if len(criticResult.AgentReviews) != 2 {
		t.Errorf("expected 2 agent reviews, got %d", len(criticResult.AgentReviews))
	}

	reviewA, ok := criticResult.AgentReviews["A"]
	if !ok {
		t.Error("expected agent review for A, not found")
	} else if reviewA.Verdict != "PASS" {
		t.Errorf("agent A verdict: expected PASS, got %q", reviewA.Verdict)
	}

	reviewB, ok := criticResult.AgentReviews["B"]
	if !ok {
		t.Error("expected agent review for B, not found")
	} else {
		if reviewB.Verdict != "ISSUES" {
			t.Errorf("agent B verdict: expected ISSUES, got %q", reviewB.Verdict)
		}
		if len(reviewB.Issues) != 1 {
			t.Errorf("agent B issues: expected 1, got %d", len(reviewB.Issues))
		} else {
			issue := reviewB.Issues[0]
			if issue.Check != "symbol_accuracy" {
				t.Errorf("issue check: expected symbol_accuracy, got %q", issue.Check)
			}
			if issue.Severity != "error" {
				t.Errorf("issue severity: expected error, got %q", issue.Severity)
			}
			if issue.Symbol != "Foo" {
				t.Errorf("issue symbol: expected Foo, got %q", issue.Symbol)
			}
		}
	}
}

// TestSetCriticReviewCmd_InvalidJSON verifies that invalid --agent-reviews JSON
// returns an appropriate error.
func TestSetCriticReviewCmd_InvalidJSON(t *testing.T) {
	implPath := writeTestIMPL(t, minimalIMPLContent)

	cmd := newSetCriticReviewCmd()
	cmd.SetArgs([]string{
		implPath,
		"--verdict", "PASS",
		"--summary", "All good",
		"--agent-reviews", "this is not valid json {{{",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "invalid --agent-reviews JSON") {
		t.Errorf("expected 'invalid --agent-reviews JSON' error, got: %v", err)
	}
}

// TestSetCriticReviewCmd_InvalidVerdict verifies that invalid --verdict values
// return an appropriate error.
func TestSetCriticReviewCmd_InvalidVerdict(t *testing.T) {
	implPath := writeTestIMPL(t, minimalIMPLContent)

	cmd := newSetCriticReviewCmd()
	cmd.SetArgs([]string{
		implPath,
		"--verdict", "MAYBE",
		"--summary", "Not sure",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid verdict, got nil")
	}

	if !strings.Contains(err.Error(), "--verdict must be PASS or ISSUES") {
		t.Errorf("expected verdict validation error, got: %v", err)
	}
}

// TestSetCriticReviewCmd_EmptyAgentReviews verifies that omitting --agent-reviews
// produces an empty reviews map (not an error).
func TestSetCriticReviewCmd_EmptyAgentReviews(t *testing.T) {
	implPath := writeTestIMPL(t, minimalIMPLContent)

	var stdout bytes.Buffer

	cmd := newSetCriticReviewCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		implPath,
		"--verdict", "PASS",
		"--summary", "No issues found",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-critic-review failed: %v", err)
	}

	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to load IMPL: %v", err)
	}

	result := protocol.GetCriticReview(context.Background(), manifest)
	if result == nil {
		t.Fatal("expected critic_report to be set, got nil")
	}

	if result.Verdict != "PASS" {
		t.Errorf("verdict: expected PASS, got %q", result.Verdict)
	}

	if result.AgentReviews == nil {
		t.Error("expected AgentReviews map to be initialized (not nil)")
	}
}

// TestCollectRepoPaths verifies that repo path extraction from manifest works correctly.
func TestCollectRepoPaths(t *testing.T) {
	tests := []struct {
		name     string
		manifest *protocol.IMPLManifest
		want     []string
	}{
		{
			name: "single repository field",
			manifest: &protocol.IMPLManifest{
				Repository: "/usr/src/myrepo",
			},
			want: []string{"/usr/src/myrepo"},
		},
		{
			name: "multiple repositories field",
			manifest: &protocol.IMPLManifest{
				Repositories: []string{"/usr/src/repo1", "/usr/src/repo2"},
			},
			want: []string{"/usr/src/repo1", "/usr/src/repo2"},
		},
		{
			name: "both fields deduplicates",
			manifest: &protocol.IMPLManifest{
				Repository:   "/usr/src/repo1",
				Repositories: []string{"/usr/src/repo1", "/usr/src/repo2"},
			},
			want: []string{"/usr/src/repo1", "/usr/src/repo2"},
		},
		{
			name:     "empty manifest returns nil",
			manifest: &protocol.IMPLManifest{},
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collectRepoPaths(tc.manifest)
			if len(got) != len(tc.want) {
				t.Errorf("len: expected %d, got %d (want=%v, got=%v)", len(tc.want), len(got), tc.want, got)
				return
			}
			for i, p := range tc.want {
				if got[i] != p {
					t.Errorf("paths[%d]: expected %q, got %q", i, p, got[i])
				}
			}
		})
	}
}

// TestInferRepoRoot verifies that repo root inference from IMPL path works.
func TestInferRepoRoot(t *testing.T) {
	tests := []struct {
		name     string
		implPath string
		want     string
	}{
		{
			name:     "standard docs/IMPL path",
			implPath: "/Users/user/code/myrepo/docs/IMPL/IMPL-feature.yaml",
			want:     "/Users/user/code/myrepo",
		},
		{
			name:     "path without docs/IMPL returns empty",
			implPath: "/tmp/IMPL-feature.yaml",
			want:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inferRepoRoot(tc.implPath)
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// TestRunCriticCmd_BackendAgentToolRequiresAbsolutePath verifies that
// --backend agent-tool rejects relative IMPL paths before attempting
// to build the critic prompt.
func TestRunCriticCmd_BackendAgentToolRequiresAbsolutePath(t *testing.T) {
	cmd := newRunCriticCmd()
	cmd.SetArgs([]string{"relative/path/IMPL-foo.yaml", "--backend", "agent-tool"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for relative impl-path with --backend agent-tool")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("expected 'absolute' in error, got: %v", err)
	}
}

// TestRunCriticCmd_BackendAgentToolNonExistentIMPL verifies that
// --backend agent-tool rejects a non-existent IMPL path.
func TestRunCriticCmd_BackendAgentToolNonExistentIMPL(t *testing.T) {
	cmd := newRunCriticCmd()
	cmd.SetArgs([]string{"/nonexistent/path/IMPL-missing.yaml", "--backend", "agent-tool"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent IMPL path with --backend agent-tool")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' in error, got: %v", err)
	}
}
