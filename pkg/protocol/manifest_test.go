package protocol

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// TestManifestLoadSave tests Load/Save roundtrip with real YAML.
func TestManifestLoadSave(t *testing.T) {
	// Create a test manifest
	original := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		FileOwnership: []FileOwnership{
			{File: "pkg/test.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/test2.go", Agent: "B", Wave: 1, Action: "modify"},
		},
		InterfaceContracts: []InterfaceContract{
			{
				Name:        "TestInterface",
				Description: "Test interface contract",
				Definition:  "type TestInterface interface { Test() error }",
				Location:    "pkg/test.go",
			},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement test", Files: []string{"pkg/test.go"}},
					{ID: "B", Task: "Implement test2", Files: []string{"pkg/test2.go"}, Dependencies: []string{"A"}},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C", Task: "Implement test3", Files: []string{"pkg/test3.go"}},
				},
			},
		},
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "go build ./...", Required: true},
				{Type: "test", Command: "go test ./...", Required: true},
			},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "pkg/types.go", ImportPath: "github.com/test/pkg", Status: "committed", Commit: "abc123"},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				Branch:       "wave1-agent-A",
				Commit:       "def456",
				FilesCreated: []string{"pkg/test.go"},
				TestsAdded:   []string{"TestExample"},
				Verification: "PASS",
			},
		},
		PreMortem: &PreMortem{
			OverallRisk: "medium",
			Rows: []PreMortemRow{
				{Scenario: "Test scenario", Likelihood: "medium", Impact: "high", Mitigation: "Test mitigation"},
			},
		},
		KnownIssues: []KnownIssue{
			{Description: "Test issue", Status: "open", Workaround: "Test workaround"},
		},
	}

	// Create temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-manifest.yaml")

	// Save
	if err := Save(original, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Compare key fields
	if loaded.Title != original.Title {
		t.Errorf("Title mismatch: got %q, want %q", loaded.Title, original.Title)
	}
	if loaded.FeatureSlug != original.FeatureSlug {
		t.Errorf("FeatureSlug mismatch: got %q, want %q", loaded.FeatureSlug, original.FeatureSlug)
	}
	if loaded.Verdict != original.Verdict {
		t.Errorf("Verdict mismatch: got %q, want %q", loaded.Verdict, original.Verdict)
	}
	if len(loaded.Waves) != len(original.Waves) {
		t.Errorf("Waves length mismatch: got %d, want %d", len(loaded.Waves), len(original.Waves))
	}
	if len(loaded.FileOwnership) != len(original.FileOwnership) {
		t.Errorf("FileOwnership length mismatch: got %d, want %d", len(loaded.FileOwnership), len(original.FileOwnership))
	}
	if len(loaded.CompletionReports) != len(original.CompletionReports) {
		t.Errorf("CompletionReports length mismatch: got %d, want %d", len(loaded.CompletionReports), len(original.CompletionReports))
	}

	// Verify completion report contents
	if report, ok := loaded.CompletionReports["A"]; ok {
		if report.Status != "complete" {
			t.Errorf("CompletionReport[A].Status = %q, want %q", report.Status, "complete")
		}
		if report.Commit != "def456" {
			t.Errorf("CompletionReport[A].Commit = %q, want %q", report.Commit, "def456")
		}
	} else {
		t.Error("CompletionReport[A] not found")
	}
}

// TestManifestLoadInvalidFile tests Load with non-existent file.
func TestManifestLoadInvalidFile(t *testing.T) {
	_, err := Load("/nonexistent/path/manifest.yaml")
	if err == nil {
		t.Error("Load should fail with non-existent file")
	}
}

// TestManifestLoadInvalidYAML tests Load with invalid YAML.
func TestManifestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	if err := os.WriteFile(path, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load should fail with invalid YAML")
	}
}

// TestManifestCurrentWave tests CurrentWave logic.
func TestManifestCurrentWave(t *testing.T) {
	tests := []struct {
		name              string
		waves             []Wave
		completionReports map[string]CompletionReport
		wantWaveNumber    *int // nil means no current wave
	}{
		{
			name: "no waves",
			waves: []Wave{},
			completionReports: nil,
			wantWaveNumber: nil,
		},
		{
			name: "first wave incomplete",
			waves: []Wave{
				{Number: 1, Agents: []Agent{{ID: "A", Task: "Test"}}},
				{Number: 2, Agents: []Agent{{ID: "B", Task: "Test"}}},
			},
			completionReports: nil,
			wantWaveNumber: intPtr(1),
		},
		{
			name: "first wave complete, second incomplete",
			waves: []Wave{
				{Number: 1, Agents: []Agent{{ID: "A", Task: "Test"}}},
				{Number: 2, Agents: []Agent{{ID: "B", Task: "Test"}}},
			},
			completionReports: map[string]CompletionReport{
				"A": {Status: "complete"},
			},
			wantWaveNumber: intPtr(2),
		},
		{
			name: "all waves complete",
			waves: []Wave{
				{Number: 1, Agents: []Agent{{ID: "A", Task: "Test"}}},
				{Number: 2, Agents: []Agent{{ID: "B", Task: "Test"}}},
			},
			completionReports: map[string]CompletionReport{
				"A": {Status: "complete"},
				"B": {Status: "complete"},
			},
			wantWaveNumber: nil,
		},
		{
			name: "partial status is incomplete",
			waves: []Wave{
				{Number: 1, Agents: []Agent{{ID: "A", Task: "Test"}}},
			},
			completionReports: map[string]CompletionReport{
				"A": {Status: "partial"},
			},
			wantWaveNumber: intPtr(1),
		},
		{
			name: "blocked status is incomplete",
			waves: []Wave{
				{Number: 1, Agents: []Agent{{ID: "A", Task: "Test"}}},
			},
			completionReports: map[string]CompletionReport{
				"A": {Status: "blocked"},
			},
			wantWaveNumber: intPtr(1),
		},
		{
			name: "multi-agent wave partially complete",
			waves: []Wave{
				{Number: 1, Agents: []Agent{
					{ID: "A", Task: "Test"},
					{ID: "B", Task: "Test"},
					{ID: "C", Task: "Test"},
				}},
			},
			completionReports: map[string]CompletionReport{
				"A": {Status: "complete"},
				"C": {Status: "complete"},
			},
			wantWaveNumber: intPtr(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &IMPLManifest{
				Waves:             tt.waves,
				CompletionReports: tt.completionReports,
			}

			wave := CurrentWave(manifest)

			if tt.wantWaveNumber == nil {
				if wave != nil {
					t.Errorf("CurrentWave() = wave %d, want nil", wave.Number)
				}
			} else {
				if wave == nil {
					t.Errorf("CurrentWave() = nil, want wave %d", *tt.wantWaveNumber)
				} else if wave.Number != *tt.wantWaveNumber {
					t.Errorf("CurrentWave() = wave %d, want wave %d", wave.Number, *tt.wantWaveNumber)
				}
			}
		})
	}
}

// TestManifestSetCompletionReport tests SetCompletionReport validation.
func TestManifestSetCompletionReport(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Test A"},
				{ID: "B", Task: "Test B"},
			}},
			{Number: 2, Agents: []Agent{
				{ID: "C", Task: "Test C"},
			}},
		},
		CompletionReports: make(map[string]CompletionReport),
	}

	// Test valid agent
	report := CompletionReport{
		Status: "complete",
		Branch: "wave1-agent-A",
		Commit: "abc123",
	}

	err := SetCompletionReport(manifest, "A", report)
	if err != nil {
		t.Errorf("SetCompletionReport(A) failed: %v", err)
	}

	// Verify report was stored
	if stored, ok := manifest.CompletionReports["A"]; !ok {
		t.Error("Report for agent A not stored")
	} else if stored.Status != "complete" {
		t.Errorf("Stored report status = %q, want %q", stored.Status, "complete")
	}

	// Test invalid agent
	err = SetCompletionReport(manifest, "Z", report)
	if err == nil {
		t.Error("SetCompletionReport(Z) should fail with unknown agent")
	}
	if err != nil && !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("SetCompletionReport(Z) error = %v, want ErrAgentNotFound", err)
	}

	// Test empty agent ID
	err = SetCompletionReport(manifest, "", report)
	if err == nil {
		t.Error("SetCompletionReport('') should fail with empty agent ID")
	}

	// Test update existing report
	updatedReport := CompletionReport{
		Status: "partial",
		Branch: "wave1-agent-A",
		Commit: "def456",
	}

	err = SetCompletionReport(manifest, "A", updatedReport)
	if err != nil {
		t.Errorf("SetCompletionReport(A) update failed: %v", err)
	}

	if stored := manifest.CompletionReports["A"]; stored.Status != "partial" {
		t.Errorf("Updated report status = %q, want %q", stored.Status, "partial")
	}
}

// TestManifestYAMLUnmarshalOptionalFields tests YAML unmarshal with optional fields.
func TestManifestYAMLUnmarshalOptionalFields(t *testing.T) {
	yamlData := `
title: Minimal Manifest
feature_slug: minimal-feature
verdict: SUITABLE
test_command: go test
lint_command: go vet
waves:
  - number: 1
    agents:
      - id: A
        task: Test
        files:
          - pkg/test.go
`

	var manifest IMPLManifest
	err := yaml.Unmarshal([]byte(yamlData), &manifest)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if manifest.Title != "Minimal Manifest" {
		t.Errorf("Title = %q, want %q", manifest.Title, "Minimal Manifest")
	}
	if manifest.QualityGates != nil {
		t.Error("QualityGates should be nil for minimal manifest")
	}
	if manifest.PreMortem != nil {
		t.Error("PreMortem should be nil for minimal manifest")
	}
	if len(manifest.Scaffolds) != 0 {
		t.Error("Scaffolds should be empty for minimal manifest")
	}
}

// TestManifestJSONMarshal tests JSON marshal for web UI compatibility.
func TestManifestJSONMarshal(t *testing.T) {
	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Test", Files: []string{"pkg/test.go"}},
			}},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: "abc123"},
		},
	}

	jsonData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Verify it can be unmarshaled
	var decoded IMPLManifest
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if decoded.Title != manifest.Title {
		t.Errorf("JSON roundtrip: Title = %q, want %q", decoded.Title, manifest.Title)
	}
	if len(decoded.Waves) != len(manifest.Waves) {
		t.Errorf("JSON roundtrip: Waves length = %d, want %d", len(decoded.Waves), len(manifest.Waves))
	}
	if decoded.CompletionReports["A"].Status != "complete" {
		t.Errorf("JSON roundtrip: CompletionReports[A].Status = %q, want %q", decoded.CompletionReports["A"].Status, "complete")
	}
}

// TestManifestLoadInitializesNilMaps tests that Load initializes nil maps.
func TestManifestLoadInitializesNilMaps(t *testing.T) {
	yamlData := `
title: Test
feature_slug: test
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: Test
        files: []
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(path, []byte(yamlData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	manifest, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if manifest.CompletionReports == nil {
		t.Error("Load should initialize CompletionReports map")
	}
}

// intPtr returns a pointer to an int (helper for test cases).
func intPtr(i int) *int {
	return &i
}

// TestValidateSM02_WaveExecutingToMerging_AllComplete tests legal transition when all agents complete.
func TestValidateSM02_WaveExecutingToMerging_AllComplete(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Test A"},
				{ID: "B", Task: "Test B"},
			}},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: "abc123"},
			"B": {Status: "complete", Commit: "def456"},
		},
	}

	errs := ValidateSM02TransitionGuards(StateWaveExecuting, StateWaveMerging, m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors when all agents complete, got %d: %v", len(errs), errs)
	}
}

// TestValidateSM02_WaveExecutingToMerging_Incomplete tests blocked transition when agent incomplete.
func TestValidateSM02_WaveExecutingToMerging_Incomplete(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Test A"},
				{ID: "B", Task: "Test B"},
			}},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", Commit: "abc123"},
			// B is missing
		},
	}

	errs := ValidateSM02TransitionGuards(StateWaveExecuting, StateWaveMerging, m)
	if len(errs) == 0 {
		t.Error("Expected error when agent B incomplete")
	}
	if len(errs) > 0 && errs[0].Code != result.CodeStateTransitionInvalid {
		t.Errorf("Expected SM02_INVALID_TRANSITION, got %s", errs[0].Code)
	}
}

// TestValidateSM02_IllegalTransition tests that illegal transitions are blocked.
func TestValidateSM02_IllegalTransition(t *testing.T) {
	m := &IMPLManifest{}

	errs := ValidateSM02TransitionGuards(StateScoutPending, StateWaveMerging, m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for illegal transition, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeStateTransitionInvalid {
		t.Errorf("Expected SM02_INVALID_TRANSITION, got %s", errs[0].Code)
	}
}

// TestValidateSM02_BlockedToAny tests that BLOCKED can transition to any state.
func TestValidateSM02_BlockedToAny(t *testing.T) {
	m := &IMPLManifest{}

	targetStates := []ProtocolState{
		StateScoutPending,
		StateScoutValidating,
		StateReviewed,
		StateScaffoldPending,
		StateWavePending,
		StateWaveExecuting,
		StateWaveMerging,
		StateWaveVerified,
		StateComplete,
		StateNotSuitable,
	}

	for _, targetState := range targetStates {
		errs := ValidateSM02TransitionGuards(StateBlocked, targetState, m)
		if len(errs) != 0 {
			t.Errorf("Expected no errors for BLOCKED -> %s, got %d: %v", targetState, len(errs), errs)
		}
	}
}

// TestValidateSM02_MergingToVerified_RequiresCompleted tests merge state guard.
func TestValidateSM02_MergingToVerified_RequiresCompleted(t *testing.T) {
	// Should fail without completed merge state
	m := &IMPLManifest{MergeState: MergeStateInProgress}
	errs := ValidateSM02TransitionGuards(StateWaveMerging, StateWaveVerified, m)
	if len(errs) == 0 {
		t.Error("Expected error when merge_state is not 'completed'")
	}

	// Should pass with completed merge state
	m.MergeState = MergeStateCompleted
	errs = ValidateSM02TransitionGuards(StateWaveMerging, StateWaveVerified, m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors when merge_state is 'completed', got %d: %v", len(errs), errs)
	}
}

// TestTransitionTo_ValidTransition tests successful state transition.
func TestTransitionTo_ValidTransition(t *testing.T) {
	m := &IMPLManifest{
		State: StateScoutPending,
	}

	errs := TransitionTo(m, StateScoutValidating)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid transition, got %d: %v", len(errs), errs)
	}

	if m.State != StateScoutValidating {
		t.Errorf("State = %q, want %q", m.State, StateScoutValidating)
	}
}

// TestTransitionTo_InvalidTransition tests that invalid transitions return errors and don't change state.
func TestTransitionTo_InvalidTransition(t *testing.T) {
	m := &IMPLManifest{
		State: StateScoutPending,
	}

	errs := TransitionTo(m, StateWaveMerging)
	if len(errs) == 0 {
		t.Error("Expected error for invalid transition SCOUT_PENDING -> WAVE_MERGING")
	}

	// State should remain unchanged
	if m.State != StateScoutPending {
		t.Errorf("State = %q, want %q (should be unchanged after failed transition)", m.State, StateScoutPending)
	}

	if len(errs) > 0 && errs[0].Code != result.CodeStateTransitionInvalid {
		t.Errorf("Expected SM02_INVALID_TRANSITION error code, got %s", errs[0].Code)
	}
}

// TestTransitionTo_DefaultState tests that empty state defaults to SCOUT_PENDING.
func TestTransitionTo_DefaultState(t *testing.T) {
	m := &IMPLManifest{
		State: "", // empty state
	}

	// Should allow transition from default SCOUT_PENDING to SCOUT_VALIDATING
	errs := TransitionTo(m, StateScoutValidating)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for transition from default state, got %d: %v", len(errs), errs)
	}

	if m.State != StateScoutValidating {
		t.Errorf("State = %q, want %q", m.State, StateScoutValidating)
	}
}

// TestTransitionTo_BlockedEscape tests that BLOCKED can transition to any state.
func TestTransitionTo_BlockedEscape(t *testing.T) {
	targetStates := []ProtocolState{
		StateScoutPending,
		StateScoutValidating,
		StateReviewed,
		StateScaffoldPending,
		StateWavePending,
		StateWaveExecuting,
		StateWaveMerging,
		StateWaveVerified,
		StateComplete,
		StateNotSuitable,
	}

	for _, targetState := range targetStates {
		m := &IMPLManifest{
			State: StateBlocked,
		}

		errs := TransitionTo(m, targetState)
		if len(errs) != 0 {
			t.Errorf("Expected no errors for BLOCKED -> %s, got %d: %v", targetState, len(errs), errs)
		}

		if m.State != targetState {
			t.Errorf("State = %q, want %q", m.State, targetState)
		}
	}
}

// TestTransitionTo_Idempotent tests that transitioning to current state is valid.
func TestTransitionTo_Idempotent(t *testing.T) {
	m := &IMPLManifest{
		State: StateWaveExecuting,
	}

	errs := TransitionTo(m, StateWaveExecuting)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for idempotent transition, got %d: %v", len(errs), errs)
	}

	if m.State != StateWaveExecuting {
		t.Errorf("State = %q, want %q", m.State, StateWaveExecuting)
	}
}
