package protocol

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// helper to build a minimal valid manifest for cross-field tests.
func crossFieldManifest() *IMPLManifest {
	return &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "do stuff", Files: []string{"pkg/a.go"}},
					{ID: "B", Task: "do other stuff", Files: []string{"pkg/b.go"}},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "pkg/a.go", Agent: "A", Wave: 1},
			{File: "pkg/b.go", Agent: "B", Wave: 1},
		},
	}
}

func TestCrossField_Valid(t *testing.T) {
	m := crossFieldManifest()
	errs := validateCrossFieldConsistency(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid manifest, got %d: %v", len(errs), errs)
	}
}

func TestCrossField_OrphanAgentInOwnership(t *testing.T) {
	m := crossFieldManifest()
	// Add an ownership entry for agent "Z" which is not in any wave.
	m.FileOwnership = append(m.FileOwnership, FileOwnership{
		File: "pkg/z.go", Agent: "Z", Wave: 1,
	})

	errs := validateCrossFieldConsistency(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeCrossField && e.Field == "file_ownership[2].agent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V038_CROSS_FIELD error for orphan agent Z in file_ownership, got: %v", errs)
	}
}

func TestCrossField_InvalidWaveInOwnership(t *testing.T) {
	m := crossFieldManifest()
	// Add an ownership entry referencing wave 99 which doesn't exist.
	m.FileOwnership = append(m.FileOwnership, FileOwnership{
		File: "pkg/x.go", Agent: "A", Wave: 99,
	})

	errs := validateCrossFieldConsistency(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeCrossField && e.Field == "file_ownership[2].wave" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V038_CROSS_FIELD error for invalid wave 99, got: %v", errs)
	}
}

func TestCrossField_NotSuitableWithWaves(t *testing.T) {
	m := crossFieldManifest()
	m.Verdict = "NOT_SUITABLE"
	// Keep waves and file_ownership populated -- should trigger warnings.

	errs := validateCrossFieldConsistency(m)
	if len(errs) == 0 {
		t.Fatal("expected warnings for NOT_SUITABLE with populated waves/ownership, got none")
	}

	// Should have warnings for waves, file_ownership (and possibly interface_contracts).
	wavesWarning := false
	ownershipWarning := false
	for _, e := range errs {
		if e.Code == result.CodeCrossField && e.Field == "waves" {
			wavesWarning = true
		}
		if e.Code == result.CodeCrossField && e.Field == "file_ownership" {
			ownershipWarning = true
		}
	}
	if !wavesWarning {
		t.Error("expected V038_CROSS_FIELD warning for waves when verdict is NOT_SUITABLE")
	}
	if !ownershipWarning {
		t.Error("expected V038_CROSS_FIELD warning for file_ownership when verdict is NOT_SUITABLE")
	}
}

func TestCrossField_NotSuitableClean(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Rejected Feature",
		FeatureSlug: "rejected",
		Verdict:     "NOT_SUITABLE",
		// Empty waves, file_ownership, interface_contracts.
	}

	errs := validateCrossFieldConsistency(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for clean NOT_SUITABLE manifest, got %d: %v", len(errs), errs)
	}
}

func TestCrossField_CompletionReportUnknownAgent(t *testing.T) {
	m := crossFieldManifest()
	m.CompletionReports = map[string]CompletionReport{
		"A": {Status: "complete", Commit: "abc123"},
		"X": {Status: "complete", Commit: "def456"}, // X is not in any wave
	}

	errs := validateCrossFieldConsistency(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeCrossField && e.Field == "completion_reports[X]" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V038_CROSS_FIELD error for unknown agent X in completion_reports, got: %v", errs)
	}

	// Agent A should not trigger an error.
	for _, e := range errs {
		if e.Field == "completion_reports[A]" {
			t.Errorf("agent A is valid and should not trigger an error, but got: %v", e)
		}
	}
}

func TestCrossField_AgentFileNotInOwnership(t *testing.T) {
	m := crossFieldManifest()
	// Add a file to agent A that is NOT in file_ownership with matching agent+wave.
	m.Waves[0].Agents[0].Files = append(m.Waves[0].Agents[0].Files, "pkg/extra.go")

	errs := validateCrossFieldConsistency(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeCrossField && e.Field == "waves[0].agents[A].files" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V038_CROSS_FIELD error for agent file not in ownership, got: %v", errs)
	}
}

func TestCrossField_EmptyManifest(t *testing.T) {
	// Manifest with valid required fields but no waves/ownership/reports.
	m := &IMPLManifest{
		Title:       "Empty Feature",
		FeatureSlug: "empty",
		Verdict:     "SUITABLE",
	}

	errs := validateCrossFieldConsistency(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty manifest, got %d: %v", len(errs), errs)
	}
}
