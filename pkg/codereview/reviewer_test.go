package codereview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// mockAnthropicResponse builds a minimal Anthropic Messages API response JSON
// containing a single text block.
func mockAnthropicResponse(text string) []byte {
	resp := map[string]interface{}{
		"id":            "msg_test",
		"type":          "message",
		"role":          "assistant",
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"model":         "claude-haiku-4-5",
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
		"usage": map[string]interface{}{
			"input_tokens":  100,
			"output_tokens": 50,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

// TestAllDimensionsConstant verifies AllDimensions has exactly 5 entries.
func TestAllDimensionsConstant(t *testing.T) {
	if len(AllDimensions) != 5 {
		t.Errorf("AllDimensions: expected 5 entries, got %d", len(AllDimensions))
	}
	expected := []string{
		DimCodeQuality,
		DimNamingClarity,
		DimComplexity,
		DimPatternAdherence,
		DimSecurity,
	}
	for i, dim := range expected {
		if AllDimensions[i] != dim {
			t.Errorf("AllDimensions[%d]: expected %q, got %q", i, dim, AllDimensions[i])
		}
	}
}

// TestReviewOptsDefaults verifies that default model and threshold are applied
// when ReviewOpts has zero values.
func TestReviewOptsDefaults(t *testing.T) {
	// We test the defaults indirectly by observing that ReviewDiff with an
	// empty diff (which short-circuits before applying defaults) still uses
	// defaults in the returned result.
	// For the model default check, we directly call ReviewDiff with an empty diff
	// and verify the model field in the result.
	res := ReviewDiff(context.Background(), "", ReviewOpts{})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal: %v", res.Errors)
	}
	got := res.GetData()
	if got.Model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, got.Model)
	}

	// Also verify threshold default is applied (70) by checking a non-empty diff
	// path would use 70 — we test this via the constant.
	if defaultThreshold != 70 {
		t.Errorf("expected defaultThreshold 70, got %d", defaultThreshold)
	}
}

// TestReviewDiffEmptyDiff verifies that an empty diff returns Passed=true, Overall=100, Skipped=true.
func TestReviewDiffEmptyDiff(t *testing.T) {
	res := ReviewDiff(context.Background(), "", ReviewOpts{})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal: %v", res.Errors)
	}
	got := res.GetData()
	if !got.Passed {
		t.Error("expected Passed=true for empty diff")
	}
	if got.Overall != 100 {
		t.Errorf("expected Overall=100 for empty diff, got %d", got.Overall)
	}
	if !got.Skipped {
		t.Error("expected Skipped=true for empty diff")
	}
	if got.DiffBytes != 0 {
		t.Errorf("expected DiffBytes=0 for empty diff, got %d", got.DiffBytes)
	}
}

// TestRunCodeReviewDisabled verifies that a disabled config returns a successful
// result with Skipped=true.
func TestRunCodeReviewDisabled(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled: false,
	}
	res := RunCodeReview(context.Background(), t.TempDir(), cfg)
	if res.IsFatal() {
		t.Fatalf("expected success, got fatal: %v", res.Errors)
	}
	got := res.GetData()
	if !got.Skipped {
		t.Errorf("expected Skipped=true for disabled config")
	}
}

// TestReviewDiffWithMockAPI verifies that ReviewDiff correctly calls the Anthropic
// API and parses the response when using a mock HTTP server.
func TestReviewDiffWithMockAPI(t *testing.T) {
	// Build the expected JSON response that the mock LLM would return.
	reviewJSON := `{
		"dimensions": [
			{"name": "code_quality", "score": 80, "rationale": "Clean and readable."},
			{"name": "naming_clarity", "score": 75, "rationale": "Names are clear."},
			{"name": "complexity", "score": 85, "rationale": "Low complexity."},
			{"name": "pattern_adherence", "score": 90, "rationale": "Follows patterns."},
			{"name": "security", "score": 70, "rationale": "No issues found."}
		],
		"overall": 80,
		"summary": "Good quality code with minor improvements possible."
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockAnthropicResponse(reviewJSON))
	}))
	defer srv.Close()

	// Set ANTHROPIC_API_KEY (mock server doesn't validate it).
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	res := ReviewDiff(context.Background(), "diff --git a/foo.go b/foo.go\n+func Foo() {}", ReviewOpts{
		Model:     "claude-haiku-4-5",
		Threshold: 70,
		BaseURL:   srv.URL,
	})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal: %v", res.Errors)
	}
	got := res.GetData()
	if got.Overall != 80 {
		t.Errorf("expected Overall=80, got %d", got.Overall)
	}
	if !got.Passed {
		t.Error("expected Passed=true (80 >= 70)")
	}
	if len(got.Dimensions) != 5 {
		t.Errorf("expected 5 dimensions, got %d", len(got.Dimensions))
	}
	if got.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if got.Model != "claude-haiku-4-5" {
		t.Errorf("expected model claude-haiku-4-5, got %q", got.Model)
	}
}

// TestReviewDiffThresholdFail verifies that a score below threshold produces Passed=false.
func TestReviewDiffThresholdFail(t *testing.T) {
	reviewJSON := `{
		"dimensions": [
			{"name": "code_quality", "score": 40, "rationale": "Poor quality."}
		],
		"overall": 40,
		"summary": "Code needs significant improvement."
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockAnthropicResponse(reviewJSON))
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	res := ReviewDiff(context.Background(), "some diff content", ReviewOpts{
		Threshold: 70,
		BaseURL:   srv.URL,
	})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal: %v", res.Errors)
	}
	got := res.GetData()
	if got.Passed {
		t.Error("expected Passed=false (40 < 70)")
	}
}

// TestReviewDiffNoAPIKey verifies that missing ANTHROPIC_API_KEY returns a fatal result.
func TestReviewDiffNoAPIKey(t *testing.T) {
	// Unset the API key for this test.
	orig := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("ANTHROPIC_API_KEY", orig)
		}
	}()

	res := ReviewDiff(context.Background(), "some diff", ReviewOpts{})
	if !res.IsFatal() {
		t.Fatal("expected fatal result when ANTHROPIC_API_KEY is not set")
	}
}

// TestReviewDiffTruncation verifies that diffs exceeding 50000 bytes are truncated
// and the Truncated flag is set in the result.
func TestReviewDiffTruncation(t *testing.T) {
	// Generate a diff that exceeds maxDiffBytes.
	largeDiff := make([]byte, maxDiffBytes+100)
	for i := range largeDiff {
		largeDiff[i] = 'x'
	}

	reviewJSON := `{
		"dimensions": [
			{"name": "code_quality", "score": 75, "rationale": "OK."}
		],
		"overall": 75,
		"summary": "Truncated diff reviewed."
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockAnthropicResponse(reviewJSON))
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	res := ReviewDiff(context.Background(), string(largeDiff), ReviewOpts{
		BaseURL: srv.URL,
	})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal: %v", res.Errors)
	}
	got := res.GetData()
	if !got.Truncated {
		t.Error("expected Truncated=true for large diff")
	}
	if got.DiffBytes != maxDiffBytes+100 {
		t.Errorf("expected DiffBytes=%d, got %d", maxDiffBytes+100, got.DiffBytes)
	}
}

// TestValidateReviewResponseFailure tests that an empty dimensions list in the
// LLM response triggers a fatal result (Bug 3 validation).
func TestValidateReviewResponseFailure(t *testing.T) {
	// Empty dimensions should trigger validation failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockAnthropicResponse(`{"dimensions":[],"overall":80,"summary":"ok"}`))
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	res := ReviewDiff(context.Background(), "some diff", ReviewOpts{
		BaseURL: srv.URL,
	})
	if !res.IsFatal() {
		t.Error("expected fatal result for empty dimensions in LLM response")
	}
}

// TestReviewDiffEmptyDiffScore tests the Bug 4 semantic fix: empty diff returns
// Overall=100 and Skipped=true.
func TestReviewDiffEmptyDiffScore(t *testing.T) {
	res := ReviewDiff(context.Background(), "", ReviewOpts{})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal: %v", res.Errors)
	}
	got := res.GetData()
	if got.Overall != 100 {
		t.Errorf("empty diff: expected Overall=100, got %d", got.Overall)
	}
	if !got.Skipped {
		t.Error("empty diff: expected Skipped=true")
	}
}
