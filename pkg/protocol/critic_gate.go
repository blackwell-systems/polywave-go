package protocol

// CriticGatePasses implements E37 critic gate enforcement logic.
// Returns true when the wave should proceed, false when it should be blocked.
//
// Decision logic:
// - PASS verdict → always proceed
// - ISSUES verdict with errors → always block
// - ISSUES verdict with warnings only → proceed in auto mode, block in manual mode
// - No critic report → block (safety default)
// - Unknown verdict → block (safety default)
//
// Parameters:
//   - m: The IMPL manifest containing the critic report
//   - autoMode: If true, warnings-only issues pass. If false, surface to user.
func CriticGatePasses(m *IMPLManifest, autoMode bool) bool {
	if m.CriticReport == nil {
		return false // No critic report found — block
	}
	if m.CriticReport.Verdict == CriticVerdictPass {
		return true
	}
	if m.CriticReport.Verdict != CriticVerdictIssues {
		return false // Unknown verdict — block
	}
	// Verdict is ISSUES — check severity of all issues
	hasError := false
	for _, review := range m.CriticReport.AgentReviews {
		for _, issue := range review.Issues {
			if issue.Severity == CriticSeverityError {
				hasError = true
				break
			}
		}
		if hasError {
			break
		}
	}
	if hasError {
		return false // ISSUES with errors — block
	}
	// ISSUES with warnings only
	if autoMode {
		return true // Auto mode — proceed
	}
	return false // Manual mode — surface to user
}
