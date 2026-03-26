package retry

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// RetryAttempt is the unified per-attempt state carrier.
// It replaces both the old RetryAttempt and RetryResult types.
// It preserves identical JSON field names for sawtools build-retry-context binary contract.
type RetryAttempt struct {
	AttemptNumber  int        `json:"attempt_number"`
	AgentID        string     `json:"agent_id"`
	ErrorClass     ErrorClass `json:"error_class"`
	ErrorExcerpt   string     `json:"error_excerpt"`
	GateResults    []string   `json:"gate_results"`
	SuggestedFixes []string   `json:"suggested_fixes"`
	PriorNotes     string     `json:"prior_notes"`
	PromptText     string     `json:"prompt_text"`
	FailureType    string     `json:"failure_type,omitempty"`
	// Fields merged from former RetryResult:
	GatePassed bool   `json:"gate_passed"`
	GateOutput string `json:"gate_output"`
	RetryIMPL  string `json:"retry_impl,omitempty"`
	FinalState string `json:"final_state"` // "passed" | "retrying" | "blocked"
}

const maxExcerptLen = 2000

// BuildRetryAttempt reads the agent's completion report from the manifest at
// manifestPath, classifies the error, and returns a populated *RetryAttempt.
//
// Returns an error when:
//   - the manifest cannot be loaded
//   - the agent has no completion report in the manifest
func BuildRetryAttempt(manifestPath string, agentID string, attemptNum int) (*RetryAttempt, error) {
	m, err := protocol.Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("retry: failed to load manifest: %w", err)
	}

	report, ok := m.CompletionReports[agentID]
	if !ok {
		return nil, fmt.Errorf("retry: no completion report found for agent %q", agentID)
	}

	// Combine Notes and Verification as the error output source.
	rawOutput := report.Notes + "\n" + report.Verification

	// Truncate to 2000 chars max.
	excerpt := rawOutput
	if len(excerpt) > maxExcerptLen {
		excerpt = excerpt[:maxExcerptLen]
	}

	class := ClassifyError(rawOutput)
	fixes := SuggestFixes(class)

	// Collect gate result summaries.
	var gateResults []string
	if m.QualityGates != nil {
		for _, gate := range m.QualityGates.Gates {
			status := "unknown"
			gateResults = append(gateResults, fmt.Sprintf("%s: %s", gate.Type, status))
		}
	}

	// Derive GatePassed and GateOutput from the completion report.
	gatePassed := report.Status == "complete" || report.Status == ""
	gateOutput := report.Verification

	promptText := BuildPromptText(RetryAttempt{
		AttemptNumber:  attemptNum,
		ErrorClass:     class,
		ErrorExcerpt:   excerpt,
		SuggestedFixes: fixes,
		PriorNotes:     report.Notes,
		FailureType:    report.FailureType,
	})

	ra := &RetryAttempt{
		AttemptNumber:  attemptNum,
		AgentID:        agentID,
		ErrorClass:     class,
		ErrorExcerpt:   excerpt,
		GateResults:    gateResults,
		SuggestedFixes: fixes,
		PriorNotes:     report.Notes,
		PromptText:     promptText,
		FailureType:    report.FailureType,
		GatePassed:     gatePassed,
		GateOutput:     gateOutput,
	}
	return ra, nil
}

// BuildPromptText formats a markdown retry prompt for the agent.
// When FailureType is "fixable" and PriorNotes is non-empty, a "## Fix Required"
// section is prepended to guide the agent to apply the identified fix first.
func BuildPromptText(attempt RetryAttempt) string {
	var sb strings.Builder

	// For fixable failures, surface the specific fix prominently at the top.
	if attempt.FailureType == "fixable" && attempt.PriorNotes != "" {
		sb.WriteString("## Fix Required\n\n")
		sb.WriteString("The previous attempt identified a specific fix needed:\n\n")
		sb.WriteString(attempt.PriorNotes)
		sb.WriteString("\n\nApply this fix before proceeding with your task.\n\n")
	}

	fmt.Fprintf(&sb, "## Retry Context (Attempt %d)\n\n", attempt.AttemptNumber)
	fmt.Fprintf(&sb, "### Error Classification: %s\n\n", string(attempt.ErrorClass))

	sb.WriteString("### Error Output\n\n")
	sb.WriteString("```\n")
	sb.WriteString(attempt.ErrorExcerpt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("### Suggested Fixes\n\n")
	for _, fix := range attempt.SuggestedFixes {
		fmt.Fprintf(&sb, "- %s\n", fix)
	}
	sb.WriteString("\n")

	sb.WriteString("### Prior Attempt Notes\n\n")
	sb.WriteString(attempt.PriorNotes)
	sb.WriteString("\n")

	return sb.String()
}
