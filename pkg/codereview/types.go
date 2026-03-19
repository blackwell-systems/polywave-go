// Package codereview provides AI-powered code review using the Anthropic Messages API.
// It scores a git diff across up to five standard dimensions (code quality, naming
// clarity, complexity, pattern adherence, security) and returns a structured
// ReviewResult that callers can use to gate wave progression.
package codereview

// Standard dimension name constants. Use these as values in ReviewOpts.Dimensions.
const (
	DimCodeQuality      = "code_quality"
	DimNamingClarity    = "naming_clarity"
	DimComplexity       = "complexity"
	DimPatternAdherence = "pattern_adherence"
	DimSecurity         = "security"
)

// AllDimensions lists the 5 standard review dimensions in their canonical order.
var AllDimensions = []string{
	DimCodeQuality, DimNamingClarity, DimComplexity,
	DimPatternAdherence, DimSecurity,
}

// DimensionScore holds the LLM-assigned score and rationale for a single dimension.
type DimensionScore struct {
	Name      string `json:"name"`
	Score     int    `json:"score"`     // 0-100
	Rationale string `json:"rationale"` // 1-2 sentence explanation
}

// ReviewResult is the structured output from ReviewDiff.
// Dimensions contains one entry per scored dimension.
// Overall is the weighted average (0-100).
// Passed is true when Overall >= threshold.
// Summary is a 1-3 sentence plain-text summary for display in the UI.
// Truncated is true when the input diff was truncated to 50 000 bytes.
type ReviewResult struct {
	Dimensions []DimensionScore `json:"dimensions"`
	Overall    int              `json:"overall"`
	Passed     bool             `json:"passed"`
	Summary    string           `json:"summary"`
	Model      string           `json:"model"`
	DiffBytes  int              `json:"diff_bytes"`
	Truncated  bool             `json:"truncated,omitempty"`
}

// ReviewOpts configures a single call to ReviewDiff.
// Model defaults to "claude-haiku-4-5" when empty.
// Threshold is the minimum overall score (0-100) for a passing review; default 70.
// Dimensions lists which of the 5 standard dimensions to score; empty means all 5.
type ReviewOpts struct {
	Model      string   // Anthropic model ID; defaults to "claude-haiku-4-5"
	Threshold  int      // Minimum overall score to pass (0-100); default 70
	Dimensions []string // Subset of standard dimensions; empty = all 5
}

// CodeReviewConfig mirrors the saw.config.json "code_review" section.
// Enabled gates whether any review runs.
// Blocking controls whether a failing review stops progression to the next wave.
// Model and Threshold override defaults when non-zero/non-empty.
type CodeReviewConfig struct {
	Enabled    bool     `json:"enabled"`
	Blocking   bool     `json:"blocking"`
	Model      string   `json:"model"`      // defaults to "claude-haiku-4-5"
	Threshold  int      `json:"threshold"`  // defaults to 70
	Dimensions []string `json:"dimensions"` // empty = all 5
}
