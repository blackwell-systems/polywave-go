package protocol

import (
	"testing"
)

// makeMinimalValidManifest creates a minimal valid manifest with one agent and no scaffolds.
func makeMinimalValidManifest() *IMPLManifest {
	return &IMPLManifest{
		Title:       "Test IMPL",
		FeatureSlug: "test-impl",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "implement test feature"},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "pkg/test/file.go", Agent: "A", Wave: 1, Action: "new"},
		},
	}
}

// TestPreWaveGate_ValidManifest tests that a valid minimal manifest passes all checks.
func TestPreWaveGate_ValidManifest(t *testing.T) {
	m := makeMinimalValidManifest()

	result := PreWaveGate(m)

	if !result.Ready {
		t.Error("expected Ready=true for valid manifest with no scaffolds, no critic report, <3 agents")
	}

	if len(result.Checks) != 4 {
		t.Fatalf("expected 4 checks, got %d", len(result.Checks))
	}

	for _, check := range result.Checks {
		if check.Status == "fail" {
			t.Errorf("check %q unexpected fail: %s", check.Name, check.Message)
		}
	}

	// Specifically verify critic_review is pass (not warn, since <3 agents)
	checkMap := checksToMap(result.Checks)
	if checkMap["critic_review"] != "pass" {
		t.Errorf("expected critic_review=pass for <3 agents, got %q", checkMap["critic_review"])
	}
}

// TestPreWaveGate_ThreeAgentsNoCriticReport tests that 3+ agents without critic report produces warn.
func TestPreWaveGate_ThreeAgentsNoCriticReport(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Test IMPL",
		FeatureSlug: "test-impl",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "implement feature A"},
					{ID: "B", Task: "implement feature B"},
					{ID: "C", Task: "implement feature C"},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "pkg/test/a.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/test/b.go", Agent: "B", Wave: 1, Action: "new"},
			{File: "pkg/test/c.go", Agent: "C", Wave: 1, Action: "new"},
		},
	}

	result := PreWaveGate(m)

	// E37 enforcement: missing critic review now blocks when threshold met
	if result.Ready {
		t.Error("expected Ready=false for >=3 agents without critic review (E37 enforcement)")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["critic_review"] != "fail" {
		t.Errorf("expected critic_review=fail for >=3 agents without critic, got %q", checkMap["critic_review"])
	}
}

// TestPreWaveGate_CriticReportPass tests that a passing critic report sets critic_review=pass.
func TestPreWaveGate_CriticReportPass(t *testing.T) {
	m := makeMinimalValidManifest()
	m.CriticReport = &CriticData{
		Verdict:    CriticVerdictPass,
		IssueCount: 0,
	}

	result := PreWaveGate(m)

	if !result.Ready {
		t.Error("expected Ready=true for critic report with PASS verdict")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["critic_review"] != "pass" {
		t.Errorf("expected critic_review=pass, got %q", checkMap["critic_review"])
	}
}

// TestPreWaveGate_CriticReportIssues tests that a failing critic report sets critic_review=fail.
func TestPreWaveGate_CriticReportIssues(t *testing.T) {
	m := makeMinimalValidManifest()
	m.CriticReport = &CriticData{
		Verdict:    CriticVerdictIssues,
		IssueCount: 3,
	}

	result := PreWaveGate(m)

	if result.Ready {
		t.Error("expected Ready=false for critic report with ISSUES verdict")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["critic_review"] != "fail" {
		t.Errorf("expected critic_review=fail, got %q", checkMap["critic_review"])
	}
}

// TestPreWaveGate_UncommittedScaffolds tests that uncommitted scaffolds cause scaffolds=fail.
func TestPreWaveGate_UncommittedScaffolds(t *testing.T) {
	m := makeMinimalValidManifest()
	m.Scaffolds = []ScaffoldFile{
		{FilePath: "pkg/shared/types.go", Status: "pending"},
	}

	result := PreWaveGate(m)

	if result.Ready {
		t.Error("expected Ready=false for manifest with uncommitted scaffolds")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["scaffolds"] != "fail" {
		t.Errorf("expected scaffolds=fail, got %q", checkMap["scaffolds"])
	}
}

// TestPreWaveGate_StateComplete tests that state=COMPLETE causes state=fail.
func TestPreWaveGate_StateComplete(t *testing.T) {
	m := makeMinimalValidManifest()
	m.State = StateComplete

	result := PreWaveGate(m)

	if result.Ready {
		t.Error("expected Ready=false for manifest with state=COMPLETE")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["state"] != "fail" {
		t.Errorf("expected state=fail for COMPLETE, got %q", checkMap["state"])
	}
}

// TestPreWaveGate_StateBlocked tests that state=BLOCKED causes state=fail.
func TestPreWaveGate_StateBlocked(t *testing.T) {
	m := makeMinimalValidManifest()
	m.State = StateBlocked

	result := PreWaveGate(m)

	if result.Ready {
		t.Error("expected Ready=false for manifest with state=BLOCKED")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["state"] != "fail" {
		t.Errorf("expected state=fail for BLOCKED, got %q", checkMap["state"])
	}
}

// TestPreWaveGate_ValidationErrors tests that validation errors cause validation=fail.
func TestPreWaveGate_ValidationErrors(t *testing.T) {
	// Duplicate file ownership in same wave (I1 violation)
	m := &IMPLManifest{
		Title:       "Test IMPL",
		FeatureSlug: "test-impl",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "implement feature A"},
					{ID: "B", Task: "implement feature B"},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "pkg/test/file.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/test/file.go", Agent: "B", Wave: 1, Action: "new"}, // duplicate
		},
	}

	result := PreWaveGate(m)

	if result.Ready {
		t.Error("expected Ready=false for manifest with validation errors")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["validation"] != "fail" {
		t.Errorf("expected validation=fail for I1 violation, got %q", checkMap["validation"])
	}
}

// TestPreWaveGate_MultiRepoNoCriticReport tests that multi-repo without critic report blocks (E37 enforcement).
func TestPreWaveGate_MultiRepoNoCriticReport(t *testing.T) {
	m := makeMinimalValidManifest()
	m.Repositories = []string{"/repo/a", "/repo/b"}

	result := PreWaveGate(m)

	// E37: missing critic review now blocks
	if result.Ready {
		t.Error("expected Ready=false for multi-repo without critic review (E37 enforcement)")
	}

	checkMap := checksToMap(result.Checks)
	if checkMap["critic_review"] != "fail" {
		t.Errorf("expected critic_review=fail for multi-repo without critic, got %q", checkMap["critic_review"])
	}
}

// TestPreWaveGate_CheckNames verifies all four expected checks are present.
func TestPreWaveGate_CheckNames(t *testing.T) {
	m := makeMinimalValidManifest()
	result := PreWaveGate(m)

	expectedNames := []string{"validation", "critic_review", "scaffolds", "state"}
	checkMap := checksToMap(result.Checks)

	for _, name := range expectedNames {
		if _, ok := checkMap[name]; !ok {
			t.Errorf("missing expected check %q", name)
		}
	}
}

// TestPreWaveGate_WavePendingStateAllowed tests that state=WAVE_PENDING is acceptable.
func TestPreWaveGate_WavePendingStateAllowed(t *testing.T) {
	m := makeMinimalValidManifest()
	m.State = StateWavePending

	result := PreWaveGate(m)

	checkMap := checksToMap(result.Checks)
	if checkMap["state"] == "fail" {
		t.Errorf("expected state=pass for WAVE_PENDING, got fail")
	}
}

// checksToMap converts a slice of checks to a map[name]status for easy lookup.
func checksToMap(checks []PreWaveGateCheck) map[string]string {
	m := make(map[string]string, len(checks))
	for _, c := range checks {
		m[c.Name] = c.Status
	}
	return m
}
