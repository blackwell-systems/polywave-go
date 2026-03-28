package protocol

import "testing"

func TestCriticGatePasses_PassVerdict(t *testing.T) {
	manifest := &IMPLManifest{
		CriticReport: &CriticData{
			Verdict: CriticVerdictPass,
		},
	}

	// PASS verdict should pass in both auto and manual modes
	if !CriticGatePasses(manifest, true) {
		t.Error("Expected true for PASS verdict in auto mode")
	}
	if !CriticGatePasses(manifest, false) {
		t.Error("Expected true for PASS verdict in manual mode")
	}
}

func TestCriticGatePasses_IssuesWithErrorsBlocks(t *testing.T) {
	manifest := &IMPLManifest{
		CriticReport: &CriticData{
			Verdict: CriticVerdictIssues,
			AgentReviews: map[string]AgentCriticReview{
				"A": {
					AgentID: "A",
					Verdict: CriticVerdictIssues,
					Issues: []CriticIssue{
						{
							Check:       "file_existence",
							Severity:    CriticSeverityError,
							Description: "File does not exist",
						},
					},
				},
			},
		},
	}

	// ISSUES with errors should block in both auto and manual modes
	if CriticGatePasses(manifest, true) {
		t.Error("Expected false for ISSUES with errors in auto mode")
	}
	if CriticGatePasses(manifest, false) {
		t.Error("Expected false for ISSUES with errors in manual mode")
	}
}

func TestCriticGatePasses_IssuesWithWarningsOnlyAutoMode(t *testing.T) {
	manifest := &IMPLManifest{
		CriticReport: &CriticData{
			Verdict: CriticVerdictIssues,
			AgentReviews: map[string]AgentCriticReview{
				"A": {
					AgentID: "A",
					Verdict: CriticVerdictIssues,
					Issues: []CriticIssue{
						{
							Check:       "symbol_accuracy",
							Severity:    CriticSeverityWarning,
							Description: "Symbol not found (but file is new)",
						},
					},
				},
			},
		},
	}

	// ISSUES with warnings only should pass in auto mode
	if !CriticGatePasses(manifest, true) {
		t.Error("Expected true for ISSUES with warnings only in auto mode")
	}
}

func TestCriticGatePasses_IssuesWithWarningsOnlyManualMode(t *testing.T) {
	manifest := &IMPLManifest{
		CriticReport: &CriticData{
			Verdict: CriticVerdictIssues,
			AgentReviews: map[string]AgentCriticReview{
				"A": {
					AgentID: "A",
					Verdict: CriticVerdictIssues,
					Issues: []CriticIssue{
						{
							Check:       "symbol_accuracy",
							Severity:    CriticSeverityWarning,
							Description: "Symbol not found (but file is new)",
						},
					},
				},
			},
		},
	}

	// ISSUES with warnings only should block in manual mode
	if CriticGatePasses(manifest, false) {
		t.Error("Expected false for ISSUES with warnings only in manual mode")
	}
}

func TestCriticGatePasses_NoCriticReport(t *testing.T) {
	manifest := &IMPLManifest{
		CriticReport: nil,
	}

	// No critic report should block in both modes
	if CriticGatePasses(manifest, true) {
		t.Error("Expected false when CriticReport is nil in auto mode")
	}
	if CriticGatePasses(manifest, false) {
		t.Error("Expected false when CriticReport is nil in manual mode")
	}
}

func TestCriticGatePasses_MixedSeverity(t *testing.T) {
	manifest := &IMPLManifest{
		CriticReport: &CriticData{
			Verdict: CriticVerdictIssues,
			AgentReviews: map[string]AgentCriticReview{
				"A": {
					AgentID: "A",
					Verdict: CriticVerdictIssues,
					Issues: []CriticIssue{
						{
							Check:       "symbol_accuracy",
							Severity:    CriticSeverityWarning,
							Description: "Minor issue 1",
						},
						{
							Check:       "symbol_accuracy",
							Severity:    CriticSeverityWarning,
							Description: "Minor issue 2",
						},
					},
				},
				"B": {
					AgentID: "B",
					Verdict: CriticVerdictIssues,
					Issues: []CriticIssue{
						{
							Check:       "file_existence",
							Severity:    CriticSeverityError,
							Description: "Critical issue",
						},
					},
				},
			},
		},
	}

	// ISSUES with mixed severity (2 warnings + 1 error) should block in both modes
	if CriticGatePasses(manifest, true) {
		t.Error("Expected false for ISSUES with mixed severity in auto mode")
	}
	if CriticGatePasses(manifest, false) {
		t.Error("Expected false for ISSUES with mixed severity in manual mode")
	}
}
