package protocol

import (
	"testing"
)

// TestValidateI1DisjointOwnership_Valid tests that disjoint ownership passes validation.
func TestValidateI1DisjointOwnership_Valid(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file2.go", Agent: "B", Wave: 1},
			{File: "file1.go", Agent: "C", Wave: 2}, // Same file, different wave - OK
		},
	}

	errs := validateI1DisjointOwnership(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI1DisjointOwnership_Violation tests that duplicate ownership in same wave is caught.
func TestValidateI1DisjointOwnership_Violation(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file1.go", Agent: "B", Wave: 1}, // Same file, same wave - violation
		},
	}

	errs := validateI1DisjointOwnership(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I1_VIOLATION" {
		t.Errorf("Expected I1_VIOLATION, got %s", errs[0].Code)
	}
	if errs[0].Field != "file_ownership" {
		t.Errorf("Expected field 'file_ownership', got %s", errs[0].Field)
	}
}

// TestValidateI2AgentDependencies_Valid tests valid cross-wave dependencies.
func TestValidateI2AgentDependencies_Valid(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
					{ID: "B", Task: "task B"},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C", Task: "task C", Dependencies: []string{"A"}},
					{ID: "D", Task: "task D", Dependencies: []string{"A", "B"}},
				},
			},
		},
	}

	errs := validateI2AgentDependencies(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI2AgentDependencies_MissingDep tests unknown dependency detection.
func TestValidateI2AgentDependencies_MissingDep(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Dependencies: []string{"Z"}}, // Z doesn't exist
				},
			},
		},
	}

	errs := validateI2AgentDependencies(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I2_MISSING_DEP" {
		t.Errorf("Expected I2_MISSING_DEP, got %s", errs[0].Code)
	}
}

// TestValidateI2AgentDependencies_SameWave tests that same-wave dependencies are rejected.
func TestValidateI2AgentDependencies_SameWave(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
					{ID: "B", Task: "task B", Dependencies: []string{"A"}}, // Same wave
				},
			},
		},
	}

	errs := validateI2AgentDependencies(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I2_WAVE_ORDER" {
		t.Errorf("Expected I2_WAVE_ORDER, got %s", errs[0].Code)
	}
}

// TestValidateI2AgentDependencies_FutureWave tests that future-wave dependencies are rejected.
func TestValidateI2AgentDependencies_FutureWave(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Dependencies: []string{"B"}}, // B is in future wave
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "B", Task: "task B"},
				},
			},
		},
	}

	errs := validateI2AgentDependencies(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I2_WAVE_ORDER" {
		t.Errorf("Expected I2_WAVE_ORDER, got %s", errs[0].Code)
	}
}

// TestValidateI2AgentDependencies_FileOwnership tests dependency validation in FileOwnership.
func TestValidateI2AgentDependencies_FileOwnership(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "task A"}}},
			{Number: 2, Agents: []Agent{{ID: "B", Task: "task B"}}},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "B", Wave: 2, DependsOn: []string{"A:file0.go"}}, // Valid
			{File: "file2.go", Agent: "B", Wave: 2, DependsOn: []string{"Z"}},         // Unknown agent
		},
	}

	errs := validateI2AgentDependencies(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I2_MISSING_DEP" {
		t.Errorf("Expected I2_MISSING_DEP, got %s", errs[0].Code)
	}
}

// TestValidateI3WaveOrdering_Valid tests sequential wave numbering.
func TestValidateI3WaveOrdering_Valid(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "task A"}}},
			{Number: 2, Agents: []Agent{{ID: "B", Task: "task B"}}},
			{Number: 3, Agents: []Agent{{ID: "C", Task: "task C"}}},
		},
	}

	errs := validateI3WaveOrdering(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI3WaveOrdering_SkippedWave tests that skipped wave numbers are caught.
func TestValidateI3WaveOrdering_SkippedWave(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "task A"}}},
			{Number: 3, Agents: []Agent{{ID: "B", Task: "task B"}}}, // Skipped wave 2
		},
	}

	errs := validateI3WaveOrdering(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I3_WAVE_ORDER" {
		t.Errorf("Expected I3_WAVE_ORDER, got %s", errs[0].Code)
	}
}

// TestValidateI3WaveOrdering_EmptyManifest tests that empty manifests are valid.
func TestValidateI3WaveOrdering_EmptyManifest(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{},
	}

	errs := validateI3WaveOrdering(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty manifest, got %d: %v", len(errs), errs)
	}
}

// TestValidateI4RequiredFields_Valid tests that all required fields pass validation.
func TestValidateI4RequiredFields_Valid(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
	}

	errs := validateI4RequiredFields(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI4RequiredFields_MissingTitle tests that missing title is caught.
func TestValidateI4RequiredFields_MissingTitle(t *testing.T) {
	m := &IMPLManifest{
		Title:       "",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
	}

	errs := validateI4RequiredFields(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I4_MISSING_FIELD" {
		t.Errorf("Expected I4_MISSING_FIELD, got %s", errs[0].Code)
	}
	if errs[0].Field != "title" {
		t.Errorf("Expected field 'title', got %s", errs[0].Field)
	}
}

// TestValidateI4RequiredFields_MultipleErrors tests that multiple missing fields are reported.
func TestValidateI4RequiredFields_MultipleErrors(t *testing.T) {
	m := &IMPLManifest{
		Title:       "",
		FeatureSlug: "",
		Verdict:     "",
	}

	errs := validateI4RequiredFields(m)
	if len(errs) != 3 {
		t.Errorf("Expected 3 errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI4RequiredFields_InvalidVerdict tests that invalid verdict values are caught.
func TestValidateI4RequiredFields_InvalidVerdict(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "MAYBE",
	}

	errs := validateI4RequiredFields(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I4_INVALID_VALUE" {
		t.Errorf("Expected I4_INVALID_VALUE, got %s", errs[0].Code)
	}
}

// TestValidateI4RequiredFields_AllVerdictValues tests all valid verdict values.
func TestValidateI4RequiredFields_AllVerdictValues(t *testing.T) {
	validVerdicts := []string{"SUITABLE", "NOT_SUITABLE", "SUITABLE_WITH_CAVEATS"}

	for _, verdict := range validVerdicts {
		m := &IMPLManifest{
			Title:       "Test Feature",
			FeatureSlug: "test-feature",
			Verdict:     verdict,
		}

		errs := validateI4RequiredFields(m)
		if len(errs) != 0 {
			t.Errorf("Expected no errors for verdict %q, got %d: %v", verdict, len(errs), errs)
		}
	}
}

// TestValidateI5FileOwnershipComplete_Valid tests that all agent files are in ownership table.
func TestValidateI5FileOwnershipComplete_Valid(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"file1.go", "file2.go"}},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file2.go", Agent: "A", Wave: 1},
		},
	}

	errs := validateI5FileOwnershipComplete(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI5FileOwnershipComplete_OrphanFile tests that missing ownership entries are caught.
func TestValidateI5FileOwnershipComplete_OrphanFile(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"file1.go", "file2.go"}},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			// file2.go is missing from ownership table
		},
	}

	errs := validateI5FileOwnershipComplete(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I5_ORPHAN_FILE" {
		t.Errorf("Expected I5_ORPHAN_FILE, got %s", errs[0].Code)
	}
}

// TestValidateI6NoCycles_Valid tests that acyclic dependency graphs pass validation.
func TestValidateI6NoCycles_Valid(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "task A"}}},
			{Number: 2, Agents: []Agent{{ID: "B", Task: "task B", Dependencies: []string{"A"}}}},
			{Number: 3, Agents: []Agent{{ID: "C", Task: "task C", Dependencies: []string{"A", "B"}}}},
		},
	}

	errs := validateI6NoCycles(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateI6NoCycles_SimpleCycle tests that simple cycles are detected.
func TestValidateI6NoCycles_SimpleCycle(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Dependencies: []string{"B"}},
					{ID: "B", Task: "task B", Dependencies: []string{"A"}}, // Cycle: A -> B -> A
				},
			},
		},
	}

	errs := validateI6NoCycles(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I6_CYCLE" {
		t.Errorf("Expected I6_CYCLE, got %s", errs[0].Code)
	}
}

// TestValidateI6NoCycles_ComplexCycle tests that longer cycles are detected.
func TestValidateI6NoCycles_ComplexCycle(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Dependencies: []string{"B"}},
					{ID: "B", Task: "task B", Dependencies: []string{"C"}},
					{ID: "C", Task: "task C", Dependencies: []string{"A"}}, // Cycle: A -> B -> C -> A
				},
			},
		},
	}

	errs := validateI6NoCycles(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I6_CYCLE" {
		t.Errorf("Expected I6_CYCLE, got %s", errs[0].Code)
	}
}

// TestValidateI6NoCycles_NoDependencies tests that agents with no dependencies are valid.
func TestValidateI6NoCycles_NoDependencies(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
					{ID: "B", Task: "task B"},
					{ID: "C", Task: "task C"},
				},
			},
		},
	}

	errs := validateI6NoCycles(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidate_CompleteManifest tests validation of a complete valid manifest.
func TestValidate_CompleteManifest(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"file1.go"}},
					{ID: "B", Task: "task B", Files: []string{"file2.go"}},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C", Task: "task C", Files: []string{"file3.go"}, Dependencies: []string{"A"}},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file2.go", Agent: "B", Wave: 1},
			{File: "file3.go", Agent: "C", Wave: 2, DependsOn: []string{"A"}},
		},
	}

	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid manifest, got %d: %v", len(errs), errs)
	}
}

// TestValidate_MultipleErrors tests that all errors are reported together.
func TestValidate_MultipleErrors(t *testing.T) {
	m := &IMPLManifest{
		Title:       "", // Missing title (I4)
		FeatureSlug: "",
		Verdict:     "",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"file1.go"}},
				},
			},
			{
				Number: 3, // Skipped wave 2 (I3)
				Agents: []Agent{
					{ID: "B", Task: "task B", Files: []string{"file2.go"}, Dependencies: []string{"Z"}}, // Unknown dep (I2)
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file1.go", Agent: "B", Wave: 1}, // Duplicate ownership (I1)
			// file2.go missing from ownership (I5)
		},
	}

	errs := Validate(m)
	// Should have errors from I1, I2, I3, I4, I5
	if len(errs) < 5 {
		t.Errorf("Expected at least 5 errors, got %d: %v", len(errs), errs)
	}

	// Check that we have errors from different invariants
	codes := make(map[string]bool)
	for _, err := range errs {
		codes[err.Code] = true
	}

	expectedCodes := []string{"I1_VIOLATION", "I2_MISSING_DEP", "I3_WAVE_ORDER", "I4_MISSING_FIELD", "I5_ORPHAN_FILE"}
	for _, code := range expectedCodes {
		if !codes[code] {
			t.Errorf("Expected error code %s, but it was not found in errors: %v", code, errs)
		}
	}
}

// TestValidate_EmptyManifest tests that an empty manifest is handled gracefully.
func TestValidate_EmptyManifest(t *testing.T) {
	m := &IMPLManifest{}

	errs := Validate(m)
	// Should have I4 errors for missing required fields
	if len(errs) == 0 {
		t.Error("Expected errors for empty manifest (missing required fields)")
	}

	foundI4 := false
	for _, err := range errs {
		if err.Code == "I4_MISSING_FIELD" {
			foundI4 = true
			break
		}
	}
	if !foundI4 {
		t.Error("Expected I4_MISSING_FIELD error for empty manifest")
	}
}

// TestValidate_SingleWave tests validation of a single-wave manifest.
func TestValidate_SingleWave(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Single Wave Feature",
		FeatureSlug: "single-wave",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"file1.go"}},
					{ID: "B", Task: "task B", Files: []string{"file2.go"}},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file2.go", Agent: "B", Wave: 1},
		},
	}

	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid single-wave manifest, got %d: %v", len(errs), errs)
	}
}

// TestValidate_CrossRepoOwnership tests validation with cross-repo file ownership.
func TestValidate_CrossRepoOwnership(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Cross-Repo Feature",
		FeatureSlug: "cross-repo",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"repo1/file1.go"}},
					{ID: "B", Task: "task B", Files: []string{"repo2/file2.go"}},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "repo1/file1.go", Agent: "A", Wave: 1, Repo: "repo1"},
			{File: "repo2/file2.go", Agent: "B", Wave: 1, Repo: "repo2"},
		},
	}

	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid cross-repo manifest, got %d: %v", len(errs), errs)
	}
}

// TestValidateI5CommitBeforeReport_Valid tests that reports with valid commits pass validation.
func TestValidateI5CommitBeforeReport_Valid(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: "abc123def456"},
			"B": {Status: "complete", Commit: "789012345678"},
		},
	}

	errs := validateI5CommitBeforeReport(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid commits, got %d: %v", len(errs), errs)
	}
}

// TestValidateI5CommitBeforeReport_Uncommitted tests that "uncommitted" is rejected.
func TestValidateI5CommitBeforeReport_Uncommitted(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: "uncommitted"},
		},
	}

	errs := validateI5CommitBeforeReport(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "I5_UNCOMMITTED" {
		t.Errorf("Expected I5_UNCOMMITTED, got %s", errs[0].Code)
	}
}

// TestValidateI5CommitBeforeReport_Empty tests that empty commit is rejected.
func TestValidateI5CommitBeforeReport_Empty(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: ""},
			"B": {Status: "complete", Commit: "   "},
		},
	}

	errs := validateI5CommitBeforeReport(m)
	if len(errs) != 2 {
		t.Fatalf("Expected 2 errors, got %d: %v", len(errs), errs)
	}
	for _, err := range errs {
		if err.Code != "I5_UNCOMMITTED" {
			t.Errorf("Expected I5_UNCOMMITTED, got %s", err.Code)
		}
	}
}

// TestValidateE9MergeState_Valid tests that all valid merge states pass.
func TestValidateE9MergeState_Valid(t *testing.T) {
	validStates := []MergeState{
		MergeStateIdle,
		MergeStateInProgress,
		MergeStateCompleted,
		MergeStateFailed,
	}

	for _, state := range validStates {
		m := &IMPLManifest{MergeState: state}
		errs := validateE9MergeState(m)
		if len(errs) != 0 {
			t.Errorf("Expected no errors for merge_state=%q, got %d: %v", state, len(errs), errs)
		}
	}
}

// TestValidateE9MergeState_Invalid tests that invalid merge states are rejected.
func TestValidateE9MergeState_Invalid(t *testing.T) {
	m := &IMPLManifest{MergeState: "merging"}

	errs := validateE9MergeState(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E9_INVALID_MERGE_STATE" {
		t.Errorf("Expected E9_INVALID_MERGE_STATE, got %s", errs[0].Code)
	}
}

// TestValidateE9MergeState_Empty tests that empty merge state is valid (backward compat).
func TestValidateE9MergeState_Empty(t *testing.T) {
	m := &IMPLManifest{MergeState: ""}

	errs := validateE9MergeState(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty merge_state, got %d: %v", len(errs), errs)
	}
}

// TestValidateSM01StateValid_AllStates tests that all valid protocol states pass.
func TestValidateSM01StateValid_AllStates(t *testing.T) {
	validStates := []ProtocolState{
		StateScoutPending,
		StateScoutValidating,
		StateReviewed,
		StateScaffoldPending,
		StateWavePending,
		StateWaveExecuting,
		StateWaveMerging,
		StateWaveVerified,
		StateBlocked,
		StateComplete,
		StateNotSuitable,
	}

	for _, state := range validStates {
		m := &IMPLManifest{State: state}
		errs := validateSM01StateValid(m)
		if len(errs) != 0 {
			t.Errorf("Expected no errors for state=%q, got %d: %v", state, len(errs), errs)
		}
	}
}

// TestValidateSM01StateValid_Invalid tests that invalid protocol states are rejected.
func TestValidateSM01StateValid_Invalid(t *testing.T) {
	m := &IMPLManifest{State: "RUNNING"}

	errs := validateSM01StateValid(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "SM01_INVALID_STATE" {
		t.Errorf("Expected SM01_INVALID_STATE, got %s", errs[0].Code)
	}
}

// TestValidateSM01StateValid_Empty tests that empty state is valid (backward compat).
func TestValidateSM01StateValid_Empty(t *testing.T) {
	m := &IMPLManifest{State: ""}

	errs := validateSM01StateValid(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty state, got %d: %v", len(errs), errs)
	}
}

// TestValidateAgentIDs_Valid tests that all valid agent ID formats pass validation.
func TestValidateAgentIDs_Valid(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
					{ID: "B", Task: "task B"},
					{ID: "C2", Task: "task C2"},
					{ID: "D9", Task: "task D9"},
				},
			},
		},
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file2.go", Agent: "B", Wave: 1},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: "abc123"},
			"B": {Status: "complete", Commit: "def456"},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid agent IDs, got %d: %v", len(errs), errs)
	}
}

// TestValidateAgentIDs_InvalidLowercase tests that lowercase agent IDs are rejected.
func TestValidateAgentIDs_InvalidLowercase(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "a", Task: "task a"},
				},
			},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
}

// TestValidateAgentIDs_InvalidMultiChar tests that multi-character agent IDs are rejected.
func TestValidateAgentIDs_InvalidMultiChar(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "AB", Task: "task AB"},
				},
			},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
}

// TestValidateAgentIDs_InvalidDigit1 tests that agent IDs with digit 1 are rejected.
func TestValidateAgentIDs_InvalidDigit1(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A1", Task: "task A1"},
				},
			},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
}

// TestValidateAgentIDs_InvalidDigit0 tests that agent IDs with digit 0 are rejected.
func TestValidateAgentIDs_InvalidDigit0(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A0", Task: "task A0"},
				},
			},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
}

// TestValidateAgentIDs_Empty tests that empty agent IDs are rejected.
func TestValidateAgentIDs_Empty(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "", Task: "task"},
				},
			},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
}

// TestValidateAgentIDs_FileOwnership tests that invalid agent IDs in FileOwnership are caught.
func TestValidateAgentIDs_FileOwnership(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "agent-1", Wave: 1},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
	if !testContains(errs[0].Field, "file_ownership") {
		t.Errorf("Expected field to contain 'file_ownership', got %s", errs[0].Field)
	}
}

// TestValidateAgentIDs_CompletionReports tests that invalid agent IDs in CompletionReports are caught.
func TestValidateAgentIDs_CompletionReports(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"agent-X": {Status: "complete", Commit: "abc123"},
		},
	}

	errs := validateAgentIDs(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC04_INVALID_AGENT_ID" {
		t.Errorf("Expected DC04_INVALID_AGENT_ID, got %s", errs[0].Code)
	}
	if !testContains(errs[0].Field, "completion_reports") {
		t.Errorf("Expected field to contain 'completion_reports', got %s", errs[0].Field)
	}
}

// TestValidateGateTypes_Valid tests that all valid gate types pass validation.
func TestValidateGateTypes_Valid(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "go build", Required: true},
				{Type: "lint", Command: "golint", Required: true},
				{Type: "test", Command: "go test", Required: true},
				{Type: "typecheck", Command: "go vet", Required: false},
				{Type: "format", Command: "gofmt -l .", Required: false},
				{Type: "custom", Command: "./custom.sh", Required: false},
			},
		},
	}

	errs := validateGateTypes(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid gate types, got %d: %v", len(errs), errs)
	}
}

// TestValidateGateTypes_FormatIsValid tests that "format" is now a valid gate type.
func TestValidateGateTypes_FormatIsValid(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "format", Command: "gofmt -l .", Required: false},
			},
		},
	}

	errs := validateGateTypes(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for format gate type, got %d: %v", len(errs), errs)
	}
}

// TestFixGateTypes_FormatNotRewrittenToCustom tests that "format" is now a valid gate type
// and FixGateTypes does NOT rewrite it to "custom".
func TestFixGateTypes_FormatNotRewrittenToCustom(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "format", Command: "gofmt -l .", Required: false},
			},
		},
	}

	fixed := FixGateTypes(m)
	if fixed != 0 {
		t.Errorf("Expected 0 fixes (format is valid), got %d", fixed)
	}
	if m.QualityGates.Gates[0].Type != "format" {
		t.Errorf("Expected gate type to remain 'format', got %q", m.QualityGates.Gates[0].Type)
	}
}

// TestValidateGateTypes_Invalid tests that invalid gate types are rejected.
func TestValidateGateTypes_Invalid(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "compile", Command: "make", Required: true},
			},
		},
	}

	errs := validateGateTypes(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC07_INVALID_GATE_TYPE" {
		t.Errorf("Expected DC07_INVALID_GATE_TYPE, got %s", errs[0].Code)
	}
}

// TestValidateGateTypes_NoGates tests that nil QualityGates returns empty.
func TestValidateGateTypes_NoGates(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: nil,
	}

	errs := validateGateTypes(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors when QualityGates is nil, got %d: %v", len(errs), errs)
	}
}

// TestValidateGateTypes_EmptyType tests that empty gate type returns error.
func TestValidateGateTypes_EmptyType(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "", Command: "make", Required: true},
			},
		},
	}

	errs := validateGateTypes(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "DC07_INVALID_GATE_TYPE" {
		t.Errorf("Expected DC07_INVALID_GATE_TYPE, got %s", errs[0].Code)
	}
}

// TestFixGateTypes_FixesInvalid tests that invalid gate types are rewritten to "custom".
func TestFixGateTypes_FixesInvalid(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "go build", Required: true},
				{Type: "build-vite", Command: "npx vite build", Required: true},
				{Type: "compile", Command: "make", Required: false},
			},
		},
	}

	fixed := FixGateTypes(m)
	if fixed != 2 {
		t.Errorf("Expected 2 fixes, got %d", fixed)
	}
	if m.QualityGates.Gates[0].Type != "build" {
		t.Errorf("Expected gate 0 to remain 'build', got %q", m.QualityGates.Gates[0].Type)
	}
	if m.QualityGates.Gates[1].Type != "custom" {
		t.Errorf("Expected gate 1 to be fixed to 'custom', got %q", m.QualityGates.Gates[1].Type)
	}
	if m.QualityGates.Gates[2].Type != "custom" {
		t.Errorf("Expected gate 2 to be fixed to 'custom', got %q", m.QualityGates.Gates[2].Type)
	}

	// Validate should now pass for gate types
	errs := validateGateTypes(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors after fix, got %d: %v", len(errs), errs)
	}
}

// TestFixGateTypes_NilGates tests that nil QualityGates returns 0 fixes.
func TestFixGateTypes_NilGates(t *testing.T) {
	m := &IMPLManifest{QualityGates: nil}
	fixed := FixGateTypes(m)
	if fixed != 0 {
		t.Errorf("Expected 0 fixes for nil gates, got %d", fixed)
	}
}

// TestFixGateTypes_AllValid tests that valid gates are not modified.
func TestFixGateTypes_AllValid(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{
			Gates: []QualityGate{
				{Type: "build", Command: "go build"},
				{Type: "custom", Command: "./run.sh"},
			},
		},
	}
	fixed := FixGateTypes(m)
	if fixed != 0 {
		t.Errorf("Expected 0 fixes for all-valid gates, got %d", fixed)
	}
}

// TestValidateMultiRepoConsistency_AllExplicit tests that all entries having repo: passes.
func TestValidateMultiRepoConsistency_AllExplicit(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1, Repo: "repo-go"},
			{File: "file2.go", Agent: "B", Wave: 1, Repo: "repo-protocol"},
		},
	}
	errs := validateMultiRepoConsistency(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateMultiRepoConsistency_AllImplicit tests that no entries having repo: passes (single repo).
func TestValidateMultiRepoConsistency_AllImplicit(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "file1.go", Agent: "A", Wave: 1},
			{File: "file2.go", Agent: "B", Wave: 1},
		},
	}
	errs := validateMultiRepoConsistency(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for all-implicit repo, got %d: %v", len(errs), errs)
	}
}

// TestValidateMultiRepoConsistency_Mixed tests that mixed repo tags are caught.
func TestValidateMultiRepoConsistency_Mixed(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/engine/foo.go", Agent: "A", Wave: 1},           // implicit
			{File: "protocol/bar.md", Agent: "B", Wave: 2, Repo: "scout-and-wave"}, // explicit
		},
	}
	errs := validateMultiRepoConsistency(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "MR01_INCONSISTENT_REPO" {
		t.Errorf("Expected MR01_INCONSISTENT_REPO, got %s", errs[0].Code)
	}
}

// TestValidateMultiRepoConsistency_Empty tests that empty file ownership is fine.
func TestValidateMultiRepoConsistency_Empty(t *testing.T) {
	m := &IMPLManifest{}
	errs := validateMultiRepoConsistency(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty ownership, got %d: %v", len(errs), errs)
	}
}

// TestValidate_E16Enhancements_DuplicateKey tests that a manifest with a duplicate YAML key
// causes ValidateDuplicateKeys to return E16_DUPLICATE_KEY.
func TestValidate_E16Enhancements_DuplicateKey(t *testing.T) {
	rawYAML := []byte(`title: Test Feature
feature_slug: test-feature
state: WAVE_PENDING
state: COMPLETE
verdict: SUITABLE
`)
	errs := ValidateDuplicateKeys(rawYAML)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E16_DUPLICATE_KEY" {
		t.Errorf("Expected E16_DUPLICATE_KEY, got %s", errs[0].Code)
	}
	if errs[0].Field != "state" {
		t.Errorf("Expected field 'state', got %s", errs[0].Field)
	}
}

// TestValidate_E16Enhancements_InvalidAction tests that a file_ownership entry with
// action="update" causes ValidateActionEnums to return E16_INVALID_ACTION.
func TestValidate_E16Enhancements_InvalidAction(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "update"},
		},
	}
	errs := ValidateActionEnums(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E16_INVALID_ACTION" {
		t.Errorf("Expected E16_INVALID_ACTION, got %s", errs[0].Code)
	}
}

// TestValidate_E16Enhancements_MissingChecklist tests that a new handler without a
// post_merge_checklist causes ValidateIntegrationChecklist to return E16_MISSING_CHECKLIST.
func TestValidate_E16Enhancements_MissingChecklist(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/widget_handler.go", Agent: "A", Wave: 1, Action: "new"},
		},
		PostMergeChecklist: nil,
	}
	errs := ValidateIntegrationChecklist(m, "")
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "E16_MISSING_CHECKLIST" {
		t.Errorf("Expected E16_MISSING_CHECKLIST, got %s", errs[0].Code)
	}
	if errs[0].Field != "post_merge_checklist" {
		t.Errorf("Expected field 'post_merge_checklist', got %s", errs[0].Field)
	}
}

// TestValidate_E16Enhancements_AllPass tests that a fully valid manifest passes all E16 checks.
func TestValidate_E16Enhancements_AllPass(t *testing.T) {
	m := &IMPLManifest{
		Title:       "Valid Feature",
		FeatureSlug: "valid-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/bar.go", Agent: "B", Wave: 1, Action: "modify"},
		},
	}

	actionErrs := ValidateActionEnums(m)
	if len(actionErrs) != 0 {
		t.Errorf("Expected no action errors, got %d: %v", len(actionErrs), actionErrs)
	}

	checklistErrs := ValidateIntegrationChecklist(m, "")
	if len(checklistErrs) != 0 {
		t.Errorf("Expected no checklist errors, got %d: %v", len(checklistErrs), checklistErrs)
	}

	fileExistErrs := ValidateFileExistence(m, "")
	if len(fileExistErrs) != 0 {
		t.Errorf("Expected no file existence errors (repoPath empty), got %d: %v", len(fileExistErrs), fileExistErrs)
	}

	dupKeyErrs := ValidateDuplicateKeys([]byte("title: Valid Feature\nfeature_slug: valid-feature\nverdict: SUITABLE\n"))
	if len(dupKeyErrs) != 0 {
		t.Errorf("Expected no duplicate key errors, got %d: %v", len(dupKeyErrs), dupKeyErrs)
	}
}

// testContains is a helper function to check if a string contains a substring.
func testContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || testContainsMiddle(s, substr)))
}

func testContainsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestFeatureSlugKebabValidation tests that feature_slug must be kebab-case.
func TestFeatureSlugKebabValidation(t *testing.T) {
	validSlugs := []string{
		"my-feature",
		"tool-journaling",
		"abc123",
		"a",
	}
	for _, slug := range validSlugs {
		t.Run("valid/"+slug, func(t *testing.T) {
			m := &IMPLManifest{
				Title:       "Test",
				FeatureSlug: slug,
				Verdict:     "SUITABLE",
			}
			errs := validateI4RequiredFields(m)
			for _, e := range errs {
				if e.Code == "I4_INVALID_FORMAT" {
					t.Errorf("valid slug %q should not produce I4_INVALID_FORMAT, got: %v", slug, e.Message)
				}
			}
		})
	}

	invalidSlugs := []string{
		"MyFeature",
		"my_feature",
		"-starts-with-hyphen",
		"ends-with-hyphen-",
	}
	for _, slug := range invalidSlugs {
		t.Run("invalid/"+slug, func(t *testing.T) {
			m := &IMPLManifest{
				Title:       "Test",
				FeatureSlug: slug,
				Verdict:     "SUITABLE",
			}
			errs := validateI4RequiredFields(m)
			found := false
			for _, e := range errs {
				if e.Code == "I4_INVALID_FORMAT" && e.Field == "feature_slug" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("invalid slug %q should produce I4_INVALID_FORMAT error, got: %v", slug, errs)
			}
		})
	}
}
