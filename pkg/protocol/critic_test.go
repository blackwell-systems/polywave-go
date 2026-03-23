package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

func TestIsNewFile(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/new.go", Agent: "A", Action: "new"},
			{File: "pkg/existing.go", Agent: "B", Action: "modify"},
		},
	}

	if !IsNewFile(manifest, "pkg/new.go") {
		t.Error("expected pkg/new.go to be marked as new")
	}

	if IsNewFile(manifest, "pkg/existing.go") {
		t.Error("expected pkg/existing.go to NOT be marked as new")
	}

	if IsNewFile(manifest, "pkg/missing.go") {
		t.Error("expected missing file to NOT be marked as new")
	}
}

func TestGetAgentNewFiles(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/new1.go", Agent: "A", Action: "new"},
			{File: "pkg/new2.go", Agent: "A", Action: "new"},
			{File: "pkg/existing.go", Agent: "A", Action: "modify"},
			{File: "pkg/other.go", Agent: "B", Action: "new"},
		},
	}

	newFiles := GetAgentNewFiles(manifest, "A")
	if len(newFiles) != 2 {
		t.Errorf("expected 2 new files for agent A, got %d", len(newFiles))
	}
}

// TestCriticResult_IsAlias verifies that CriticResult is a backward-compatible
// alias for CriticData so existing callers compile without changes.
func TestCriticResult_IsAlias(t *testing.T) {
	// If CriticResult is an alias for CriticData, assignment must be direct (no conversion).
	var d CriticData
	d.Verdict = CriticVerdictPass
	var r CriticResult = d // alias: no conversion needed
	if r.Verdict != CriticVerdictPass {
		t.Errorf("CriticResult.Verdict = %q, want %q", r.Verdict, CriticVerdictPass)
	}
}

// TestWriteCriticReviewResult_Success verifies that WriteCriticReviewResult persists
// the CriticData and returns a SUCCESS result.
func TestWriteCriticReviewResult_Success(t *testing.T) {
	path := buildTestManifestWithCritic(t)
	data := sampleCriticResult()

	res := WriteCriticReviewResult(path, data)
	if !res.IsSuccess() {
		t.Fatalf("WriteCriticReviewResult() Code = %q, want SUCCESS; errors = %v", res.Code, res.Errors)
	}

	got := res.GetData()
	if got.Verdict != data.Verdict {
		t.Errorf("Verdict = %q, want %q", got.Verdict, data.Verdict)
	}
	if got.IssueCount != data.IssueCount {
		t.Errorf("IssueCount = %d, want %d", got.IssueCount, data.IssueCount)
	}
}

// TestWriteCriticReviewResult_FileNotFound verifies that WriteCriticReviewResult
// returns a FATAL result when the IMPL path does not exist.
func TestWriteCriticReviewResult_FileNotFound(t *testing.T) {
	data := sampleCriticResult()
	res := WriteCriticReviewResult("/nonexistent/path/to/impl.yaml", data)
	if !res.IsFatal() {
		t.Errorf("WriteCriticReviewResult() Code = %q, want FATAL", res.Code)
	}
	if len(res.Errors) == 0 {
		t.Error("WriteCriticReviewResult() Errors empty, want at least one StructuredError")
	}
}

// TestGetCriticReviewResult_Success verifies that GetCriticReviewResult returns
// a SUCCESS result when a critic review is present in the manifest.
func TestGetCriticReviewResult_Success(t *testing.T) {
	path := buildTestManifestWithCritic(t)
	data := sampleCriticResult()

	if err := WriteCriticReview(path, data); err != nil {
		t.Fatalf("WriteCriticReview() error = %v", err)
	}

	manifest, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	res := GetCriticReviewResult(manifest)
	if !res.IsSuccess() {
		t.Fatalf("GetCriticReviewResult() Code = %q, want SUCCESS; errors = %v", res.Code, res.Errors)
	}

	got := res.GetData()
	if got.Verdict != data.Verdict {
		t.Errorf("Verdict = %q, want %q", got.Verdict, data.Verdict)
	}
	if got.Summary != data.Summary {
		t.Errorf("Summary = %q, want %q", got.Summary, data.Summary)
	}
	if got.IssueCount != data.IssueCount {
		t.Errorf("IssueCount = %d, want %d", got.IssueCount, data.IssueCount)
	}
}

// TestGetCriticReviewResult_NoCriticReport verifies that GetCriticReviewResult
// returns a FATAL result when no critic_report is present in the manifest.
func TestGetCriticReviewResult_NoCriticReport(t *testing.T) {
	manifest := &IMPLManifest{}
	res := GetCriticReviewResult(manifest)
	if !res.IsFatal() {
		t.Errorf("GetCriticReviewResult() Code = %q, want FATAL", res.Code)
	}
	if len(res.Errors) == 0 {
		t.Error("GetCriticReviewResult() Errors empty, want at least one StructuredError")
	}
}

// TestGetCriticReviewResult_ErrorCode verifies that the StructuredError returned
// for a missing critic report has the expected error code.
func TestGetCriticReviewResult_ErrorCode(t *testing.T) {
	manifest := &IMPLManifest{}
	res := GetCriticReviewResult(manifest)
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if res.Errors[0].Code != "E003" {
		t.Errorf("Errors[0].Code = %q, want E003", res.Errors[0].Code)
	}
	if res.Errors[0].Severity != "fatal" {
		t.Errorf("Errors[0].Severity = %q, want fatal", res.Errors[0].Severity)
	}
}

// TestWriteCriticReviewResult_DataRoundtrip verifies that data written via
// WriteCriticReviewResult can be retrieved via GetCriticReviewResult with all
// fields preserved.
func TestWriteCriticReviewResult_DataRoundtrip(t *testing.T) {
	path := buildTestManifestWithCritic(t)
	original := sampleCriticResult()

	writeRes := WriteCriticReviewResult(path, original)
	if !writeRes.IsSuccess() {
		t.Fatalf("WriteCriticReviewResult() failed: %v", writeRes.Errors)
	}

	manifest, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	readRes := GetCriticReviewResult(manifest)
	if !readRes.IsSuccess() {
		t.Fatalf("GetCriticReviewResult() failed: %v", readRes.Errors)
	}

	got := readRes.GetData()
	if got.Verdict != original.Verdict {
		t.Errorf("Verdict = %q, want %q", got.Verdict, original.Verdict)
	}
	if got.ReviewedAt != original.ReviewedAt {
		t.Errorf("ReviewedAt = %q, want %q", got.ReviewedAt, original.ReviewedAt)
	}
	if got.IssueCount != original.IssueCount {
		t.Errorf("IssueCount = %d, want %d", got.IssueCount, original.IssueCount)
	}
	if len(got.AgentReviews) != len(original.AgentReviews) {
		t.Errorf("AgentReviews length = %d, want %d", len(got.AgentReviews), len(original.AgentReviews))
	}
}

// TestCriticWriteResult_ResultInterface verifies that WriteCriticReviewResult and
// GetCriticReviewResult conform to the result.Result[T] contract including
// HasErrors, IsPartial, and IsFatal helpers.
func TestCriticWriteResult_ResultInterface(t *testing.T) {
	path := buildTestManifestWithCritic(t)
	data := sampleCriticResult()

	res := WriteCriticReviewResult(path, data)

	// SUCCESS result must not report fatal, partial, or errors
	if res.IsFatal() {
		t.Error("SUCCESS result.IsFatal() = true, want false")
	}
	if res.IsPartial() {
		t.Error("SUCCESS result.IsPartial() = true, want false")
	}
	if res.HasErrors() {
		t.Errorf("SUCCESS result.HasErrors() = true, errors = %v", res.Errors)
	}

	// Confirm result package type is used correctly
	_ = result.NewSuccess(data)
}
