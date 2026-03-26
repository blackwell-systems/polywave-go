// Package protocol provides critic validation types and helpers.
//
// Critic Validation Rules (for future critic agent implementations):
//
// 1. File Existence Checks:
//    - If file is in file_ownership with action:new, skip "file does not exist" error
//    - If file is in file_ownership with action:modify, enforce existence check
//    - If file is NOT in file_ownership (external), enforce existence check
//
// 2. Symbol/Function Existence Checks:
//    - If symbol is defined in a file with action:new, skip "symbol does not exist" error
//    - If symbol is in existing file (action:modify or external), enforce existence check
//    - Use GetAgentNewFiles() to determine which symbols agent will create
//
// 3. Cross-File References:
//    - If reference is to external file (not in file_ownership), enforce existence
//    - If reference is to same-wave agent's file, defer (coordination, not verification)
//
// Use IsNewFile() and IsSymbolInNewFile() helpers to implement these rules.
package protocol

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Verdict values for CriticData and AgentCriticReview.
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

// CriticData is the structured output of a critic-agent review run.
// Written to the IMPL doc by WriteCriticReview after the critic agent completes.
type CriticData struct {
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
//
// Deprecated: prefer WriteCriticReviewResult which returns result.Result[CriticData].
func WriteCriticReview(implPath string, data CriticData) error {
	manifest, err := Load(implPath)
	if err != nil {
		return err
	}

	manifest.CriticReport = &data

	return Save(manifest, implPath)
}

// WriteCriticReviewResult writes the critic result to the IMPL manifest at implPath.
// It loads the existing manifest, sets the CriticReport field, and saves.
// Returns a result.Result[CriticData] indicating success or failure.
func WriteCriticReviewResult(implPath string, data CriticData) result.Result[CriticData] {
	manifest, err := Load(implPath)
	if err != nil {
		return result.NewFailure[CriticData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  err.Error(),
				Severity: "fatal",
			},
		})
	}

	manifest.CriticReport = &data

	if err := Save(manifest, implPath); err != nil {
		return result.NewFailure[CriticData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  err.Error(),
				Severity: "fatal",
			},
		})
	}

	return result.NewSuccess(data)
}

// GetCriticReview returns the critic review from a loaded manifest, or nil
// if no review has been written. Does not load from disk; caller must load
// the manifest first using protocol.Load.
//
// Deprecated: prefer GetCriticReviewResult which returns result.Result[CriticData].
func GetCriticReview(manifest *IMPLManifest) *CriticData {
	return manifest.CriticReport
}

// GetCriticReviewResult returns the critic review from a loaded manifest wrapped in
// a result.Result[CriticData]. Returns a FATAL result if no review has been
// written. Does not load from disk; caller must load the manifest first using
// protocol.Load.
func GetCriticReviewResult(manifest *IMPLManifest) result.Result[CriticData] {
	if manifest.CriticReport == nil {
		return result.NewFailure[CriticData]([]result.SAWError{
			{
				Code:     result.CodeCompletionReportMissing,
				Message:  "no critic review found in manifest",
				Severity: "fatal",
			},
		})
	}
	return result.NewSuccess(*manifest.CriticReport)
}

// IsNewFile returns true if the file is marked action:new in file_ownership.
// Used by critic validation to skip existence checks for files that will be created.
func IsNewFile(manifest *IMPLManifest, filePath string) bool {
	for _, fo := range manifest.FileOwnership {
		if fo.File == filePath {
			return fo.Action == "new"
		}
	}
	return false
}

// IsSymbolInNewFile returns true if the symbol's defining file is marked action:new.
// Used by critic validation to skip existence checks for symbols that will be created.
func IsSymbolInNewFile(manifest *IMPLManifest, symbolFile string) bool {
	return IsNewFile(manifest, symbolFile)
}

// GetAgentNewFiles returns all files owned by an agent marked action:new.
// Used by critic validation to understand which files/symbols agent will create.
func GetAgentNewFiles(manifest *IMPLManifest, agentID string) []string {
	var newFiles []string
	for _, fo := range manifest.FileOwnership {
		if fo.Agent == agentID && fo.Action == "new" {
			newFiles = append(newFiles, fo.File)
		}
	}
	return newFiles
}
