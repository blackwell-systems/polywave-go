package protocol

// Verdict values for CriticResult and AgentCriticReview.
const (
	// CriticVerdictPass indicates no blocking issues were found.
	CriticVerdictPass = "PASS"
	// CriticVerdictIssues indicates one or more issues were found.
	CriticVerdictIssues = "ISSUES"
	// CriticVerdictSkipped indicates the operator explicitly skipped critic review.
	CriticVerdictSkipped = "SKIPPED"
)

// Severity values for CriticIssue.
const (
	// CriticSeverityError indicates a blocking issue that must be resolved before the wave proceeds.
	CriticSeverityError = "error"
	// CriticSeverityWarning indicates an advisory issue that does not block the wave.
	CriticSeverityWarning = "warning"
)

// CriticResult is the structured output of a critic-agent review run.
// Written to the IMPL doc by WriteCriticReview after the critic agent completes.
type CriticResult struct {
	// Verdict is the overall decision: "PASS" or "ISSUES"
	Verdict string `yaml:"verdict" json:"verdict"`
	// AgentReviews contains per-agent verdicts indexed by agent ID
	AgentReviews map[string]AgentCriticReview `yaml:"agent_reviews" json:"agent_reviews"`
	// Summary is a human-readable summary of the overall findings
	Summary string `yaml:"summary" json:"summary"`
	// ReviewedAt is an ISO8601 timestamp of when the review completed
	ReviewedAt string `yaml:"reviewed_at" json:"reviewed_at"`
	// IssueCount is the total number of issues found across all agents
	IssueCount int `yaml:"issue_count" json:"issue_count"`
}

// AgentCriticReview is the per-agent verdict from the critic.
type AgentCriticReview struct {
	// AgentID is the agent identifier (e.g. "A", "B2")
	AgentID string `yaml:"agent_id" json:"agent_id"`
	// Verdict is "PASS" or "ISSUES"
	Verdict string `yaml:"verdict" json:"verdict"`
	// Issues lists specific problems found, empty if Verdict is PASS
	Issues []CriticIssue `yaml:"issues,omitempty" json:"issues,omitempty"`
}

// CriticIssue describes a single problem found by the critic.
type CriticIssue struct {
	// Check is the verification category (e.g. "file_existence", "symbol_accuracy")
	Check string `yaml:"check" json:"check"`
	// Severity is "error" (blocks wave) or "warning" (advisory)
	Severity string `yaml:"severity" json:"severity"`
	// Description is a human-readable explanation of the problem
	Description string `yaml:"description" json:"description"`
	// File is the specific file referenced in the brief, if applicable
	File string `yaml:"file,omitempty" json:"file,omitempty"`
	// Symbol is the specific function/type name that failed verification, if applicable
	Symbol string `yaml:"symbol,omitempty" json:"symbol,omitempty"`
}

// WriteCriticReview writes the critic result to the IMPL manifest at implPath.
// It loads the existing manifest, sets the CriticReport field, and saves.
// Returns an error if the manifest cannot be loaded or saved.
func WriteCriticReview(implPath string, result CriticResult) error {
	manifest, err := Load(implPath)
	if err != nil {
		return err
	}

	manifest.CriticReport = &result

	return Save(manifest, implPath)
}

// GetCriticReview returns the critic review from a loaded manifest, or nil
// if no review has been written. Does not load from disk; caller must load
// the manifest first using protocol.Load.
func GetCriticReview(manifest *IMPLManifest) *CriticResult {
	return manifest.CriticReport
}
