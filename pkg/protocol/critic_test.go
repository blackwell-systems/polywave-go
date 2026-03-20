package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// buildTestManifestWithCritic creates a minimal valid manifest YAML for testing critic round-trips.
// The manifest includes one wave with one agent to satisfy the Load validation.
func buildTestManifestWithCritic(t *testing.T) string {
	t.Helper()
	yamlData := `title: Test Feature
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
waves:
  - number: 1
    agents:
      - id: A
        task: Implement something
        files:
          - pkg/foo.go
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-manifest.yaml")
	if err := os.WriteFile(path, []byte(yamlData), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}
	return path
}

// sampleCriticResult returns a fully populated CriticResult for use in tests.
func sampleCriticResult() CriticResult {
	return CriticResult{
		Verdict: CriticVerdictIssues,
		AgentReviews: map[string]AgentCriticReview{
			"A": {
				AgentID: "A",
				Verdict: CriticVerdictIssues,
				Issues: []CriticIssue{
					{
						Check:       "file_existence",
						Severity:    CriticSeverityError,
						Description: "file pkg/foo.go was not created",
						File:        "pkg/foo.go",
					},
					{
						Check:       "symbol_accuracy",
						Severity:    CriticSeverityWarning,
						Description: "function Foo has wrong signature",
						File:        "pkg/foo.go",
						Symbol:      "Foo",
					},
				},
			},
			"B": {
				AgentID: "B",
				Verdict: CriticVerdictPass,
			},
		},
		Summary:    "Agent A has 2 issues; agent B passed.",
		ReviewedAt: "2026-03-19T12:00:00Z",
		IssueCount: 2,
	}
}

// TestWriteCriticReview_WritesAndReads verifies that WriteCriticReview persists the
// CriticResult and GetCriticReview returns the same data after a Load.
func TestWriteCriticReview_WritesAndReads(t *testing.T) {
	path := buildTestManifestWithCritic(t)
	result := sampleCriticResult()

	if err := WriteCriticReview(path, result); err != nil {
		t.Fatalf("WriteCriticReview() error = %v", err)
	}

	manifest, err := Load(path)
	if err != nil {
		t.Fatalf("Load() after WriteCriticReview error = %v", err)
	}

	got := GetCriticReview(manifest)
	if got == nil {
		t.Fatal("GetCriticReview() returned nil, want non-nil")
	}

	if got.Verdict != result.Verdict {
		t.Errorf("Verdict = %q, want %q", got.Verdict, result.Verdict)
	}
	if got.Summary != result.Summary {
		t.Errorf("Summary = %q, want %q", got.Summary, result.Summary)
	}
	if got.ReviewedAt != result.ReviewedAt {
		t.Errorf("ReviewedAt = %q, want %q", got.ReviewedAt, result.ReviewedAt)
	}
	if got.IssueCount != result.IssueCount {
		t.Errorf("IssueCount = %d, want %d", got.IssueCount, result.IssueCount)
	}
	if len(got.AgentReviews) != len(result.AgentReviews) {
		t.Errorf("AgentReviews length = %d, want %d", len(got.AgentReviews), len(result.AgentReviews))
	}

	// Verify agent A review round-trips correctly
	aReview, ok := got.AgentReviews["A"]
	if !ok {
		t.Fatal("AgentReviews[A] missing")
	}
	if aReview.AgentID != "A" {
		t.Errorf("AgentReviews[A].AgentID = %q, want %q", aReview.AgentID, "A")
	}
	if aReview.Verdict != CriticVerdictIssues {
		t.Errorf("AgentReviews[A].Verdict = %q, want %q", aReview.Verdict, CriticVerdictIssues)
	}
	if len(aReview.Issues) != 2 {
		t.Fatalf("AgentReviews[A].Issues length = %d, want 2", len(aReview.Issues))
	}
	if aReview.Issues[0].Check != "file_existence" {
		t.Errorf("Issues[0].Check = %q, want %q", aReview.Issues[0].Check, "file_existence")
	}
	if aReview.Issues[0].Severity != CriticSeverityError {
		t.Errorf("Issues[0].Severity = %q, want %q", aReview.Issues[0].Severity, CriticSeverityError)
	}
	if aReview.Issues[0].File != "pkg/foo.go" {
		t.Errorf("Issues[0].File = %q, want %q", aReview.Issues[0].File, "pkg/foo.go")
	}
	if aReview.Issues[1].Symbol != "Foo" {
		t.Errorf("Issues[1].Symbol = %q, want %q", aReview.Issues[1].Symbol, "Foo")
	}

	// Verify agent B review (PASS, no issues)
	bReview, ok := got.AgentReviews["B"]
	if !ok {
		t.Fatal("AgentReviews[B] missing")
	}
	if bReview.Verdict != CriticVerdictPass {
		t.Errorf("AgentReviews[B].Verdict = %q, want %q", bReview.Verdict, CriticVerdictPass)
	}
	if len(bReview.Issues) != 0 {
		t.Errorf("AgentReviews[B].Issues length = %d, want 0 (PASS verdict)", len(bReview.Issues))
	}
}

// TestWriteCriticReview_FileNotFound verifies that a nonexistent path returns an error.
func TestWriteCriticReview_FileNotFound(t *testing.T) {
	result := sampleCriticResult()
	err := WriteCriticReview("/nonexistent/path/to/impl.yaml", result)
	if err == nil {
		t.Error("WriteCriticReview() with nonexistent path should return error, got nil")
	}
}

// TestGetCriticReview_Nil verifies that GetCriticReview returns nil when no
// critic_report has been written to the manifest.
func TestGetCriticReview_Nil(t *testing.T) {
	manifest := &IMPLManifest{}
	got := GetCriticReview(manifest)
	if got != nil {
		t.Errorf("GetCriticReview(manifest without critic_report) = %v, want nil", got)
	}
}

// TestCriticResult_YAMLRoundtrip verifies that CriticResult marshals and unmarshals
// through YAML with all fields preserved, including nested AgentReviews map.
func TestCriticResult_YAMLRoundtrip(t *testing.T) {
	original := sampleCriticResult()

	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded CriticResult
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.Verdict != original.Verdict {
		t.Errorf("Verdict = %q, want %q", decoded.Verdict, original.Verdict)
	}
	if decoded.Summary != original.Summary {
		t.Errorf("Summary = %q, want %q", decoded.Summary, original.Summary)
	}
	if decoded.ReviewedAt != original.ReviewedAt {
		t.Errorf("ReviewedAt = %q, want %q", decoded.ReviewedAt, original.ReviewedAt)
	}
	if decoded.IssueCount != original.IssueCount {
		t.Errorf("IssueCount = %d, want %d", decoded.IssueCount, original.IssueCount)
	}
	if len(decoded.AgentReviews) != len(original.AgentReviews) {
		t.Errorf("AgentReviews length = %d, want %d", len(decoded.AgentReviews), len(original.AgentReviews))
	}

	// Verify nested map keys survive round-trip
	for agentID, origReview := range original.AgentReviews {
		gotReview, ok := decoded.AgentReviews[agentID]
		if !ok {
			t.Errorf("AgentReviews[%s] missing after YAML round-trip", agentID)
			continue
		}
		if gotReview.AgentID != origReview.AgentID {
			t.Errorf("AgentReviews[%s].AgentID = %q, want %q", agentID, gotReview.AgentID, origReview.AgentID)
		}
		if gotReview.Verdict != origReview.Verdict {
			t.Errorf("AgentReviews[%s].Verdict = %q, want %q", agentID, gotReview.Verdict, origReview.Verdict)
		}
		if len(gotReview.Issues) != len(origReview.Issues) {
			t.Errorf("AgentReviews[%s].Issues length = %d, want %d", agentID, len(gotReview.Issues), len(origReview.Issues))
		}
	}

	// Verify CriticIssue fields on agent A
	aReview := decoded.AgentReviews["A"]
	if len(aReview.Issues) >= 1 {
		issue := aReview.Issues[0]
		if issue.Check != "file_existence" {
			t.Errorf("Issues[0].Check = %q, want file_existence", issue.Check)
		}
		if issue.Severity != CriticSeverityError {
			t.Errorf("Issues[0].Severity = %q, want %q", issue.Severity, CriticSeverityError)
		}
		if issue.Description != "file pkg/foo.go was not created" {
			t.Errorf("Issues[0].Description = %q", issue.Description)
		}
		if issue.File != "pkg/foo.go" {
			t.Errorf("Issues[0].File = %q, want pkg/foo.go", issue.File)
		}
	}
	if len(aReview.Issues) >= 2 {
		issue := aReview.Issues[1]
		if issue.Symbol != "Foo" {
			t.Errorf("Issues[1].Symbol = %q, want Foo", issue.Symbol)
		}
		if issue.Severity != CriticSeverityWarning {
			t.Errorf("Issues[1].Severity = %q, want %q", issue.Severity, CriticSeverityWarning)
		}
	}
}
