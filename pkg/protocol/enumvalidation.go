package protocol

import "fmt"

// ValidateCompletionStatuses checks that all completion report statuses are valid enum values.
// Valid statuses: "complete", "partial", "blocked".
func ValidateCompletionStatuses(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	validStatuses := map[string]bool{
		"complete": true,
		"partial":  true,
		"blocked":  true,
	}

	for agentID, report := range m.CompletionReports {
		if !validStatuses[report.Status] {
			errs = append(errs, ValidationError{
				Code:    "DC02_INVALID_STATUS",
				Message: fmt.Sprintf("agent %s completion report has invalid status %q — must be one of: complete, partial, blocked", agentID, report.Status),
				Field:   fmt.Sprintf("completion_reports[%s].status", agentID),
			})
		}
	}

	return errs
}

// ValidateFailureTypes checks that all non-empty failure_type fields are valid enum values.
// Valid types: "transient", "fixable", "needs_replan", "escalate", "timeout".
// Empty/omitted values are valid (backward compatibility — status=complete doesn't require failure_type).
func ValidateFailureTypes(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	validTypes := map[string]bool{
		"transient":    true,
		"fixable":      true,
		"needs_replan": true,
		"escalate":     true,
		"timeout":      true,
	}

	for agentID, report := range m.CompletionReports {
		// Empty is valid (omitted when status is complete)
		if report.FailureType == "" {
			continue
		}

		if !validTypes[report.FailureType] {
			errs = append(errs, ValidationError{
				Code:    "DC03_INVALID_FAILURE_TYPE",
				Message: fmt.Sprintf("agent %s completion report has invalid failure_type %q — must be one of: transient, fixable, needs_replan, escalate, timeout", agentID, report.FailureType),
				Field:   fmt.Sprintf("completion_reports[%s].failure_type", agentID),
			})
		}
	}

	return errs
}

// ValidatePreMortemRisk checks that the pre-mortem overall_risk field is a valid enum value.
// Valid values: "low", "medium", "high".
// Empty/omitted values are valid (backward compatibility — pre-mortem may not exist).
func ValidatePreMortemRisk(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// Nil pre-mortem is valid
	if m.PreMortem == nil {
		return errs
	}

	// Empty overall_risk is valid
	if m.PreMortem.OverallRisk == "" {
		return errs
	}

	validRisks := map[string]bool{
		"low":    true,
		"medium": true,
		"high":   true,
	}

	if !validRisks[m.PreMortem.OverallRisk] {
		errs = append(errs, ValidationError{
			Code:    "DC06_INVALID_RISK",
			Message: fmt.Sprintf("pre_mortem overall_risk has invalid value %q — must be one of: low, medium, high", m.PreMortem.OverallRisk),
			Field:   "pre_mortem.overall_risk",
		})
	}

	return errs
}
