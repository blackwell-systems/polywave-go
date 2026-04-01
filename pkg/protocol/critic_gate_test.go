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

func TestE37Required_BelowThreshold(t *testing.T) {
	// 2 agents in wave 1, no repo tags → expect false
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}},
			},
		},
	}
	if E37Required(m) {
		t.Error("Expected false: 2 agents in wave 1 is below threshold")
	}
}

func TestE37Required_ThreeAgents(t *testing.T) {
	// 3 agents in wave 1, no repo tags → expect true
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}, {ID: "C"}},
			},
		},
	}
	if !E37Required(m) {
		t.Error("Expected true: 3 agents in wave 1 meets threshold")
	}
}

func TestE37Required_TwoAgents(t *testing.T) {
	// Exactly 2 agents in wave 1 → expect false (2 is below threshold)
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}},
			},
		},
	}
	if E37Required(m) {
		t.Error("Expected false: 2 agents in wave 1 is below threshold (boundary)")
	}
}

func TestE37Required_MultiRepo_Repositories(t *testing.T) {
	// 2 entries in m.Repositories, 2 agents in wave 1 → expect true (multi-repo trigger)
	m := &IMPLManifest{
		Repositories: []string{"/repo/a", "/repo/b"},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}},
			},
		},
	}
	if !E37Required(m) {
		t.Error("Expected true: 2 repos in Repositories triggers multi-repo condition")
	}
}

func TestE37Required_MultiRepo_FileOwnership(t *testing.T) {
	// file_ownership entries carrying 2 distinct repo: values, 2 agents → expect true
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/a/file.go", Agent: "A", Wave: 1, Repo: "repo-alpha"},
			{File: "pkg/b/file.go", Agent: "B", Wave: 1, Repo: "repo-beta"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}},
			},
		},
	}
	if !E37Required(m) {
		t.Error("Expected true: 2 distinct repo values in FileOwnership triggers multi-repo condition")
	}
}

func TestE37Required_SingleRepo(t *testing.T) {
	// All file_ownership entries carrying the same repo: "my-repo", 2 agents → expect false
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/a/file.go", Agent: "A", Wave: 1, Repo: "my-repo"},
			{File: "pkg/b/file.go", Agent: "B", Wave: 1, Repo: "my-repo"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}},
			},
		},
	}
	if E37Required(m) {
		t.Error("Expected false: single repo value in FileOwnership, only 2 agents")
	}
}

func TestE37Required_NoWaves(t *testing.T) {
	// No waves → expect false (0 agents < 3)
	m := &IMPLManifest{}
	if E37Required(m) {
		t.Error("Expected false: no waves means 0 agents in wave 1")
	}
}
