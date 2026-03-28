package protocol

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPostMergeChecklistUnmarshal(t *testing.T) {
	yamlData := `
groups:
  - title: "Build Verification"
    items:
      - description: "Full build passes"
        command: "go build ./..."
`
	var pmc PostMergeChecklist
	if err := yaml.Unmarshal([]byte(yamlData), &pmc); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(pmc.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(pmc.Groups))
	}
	if pmc.Groups[0].Title != "Build Verification" {
		t.Errorf("Expected group title 'Build Verification', got %q", pmc.Groups[0].Title)
	}
}

func TestKnownIssueTitleField(t *testing.T) {
	yamlData := `
title: "Flaky test"
description: "Test fails intermittently"
status: "Pre-existing"
`
	var ki KnownIssue
	if err := yaml.Unmarshal([]byte(yamlData), &ki); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if ki.Title != "Flaky test" {
		t.Errorf("Expected title 'Flaky test', got %q", ki.Title)
	}
}

func TestIMPLManifestPostMergeChecklistField(t *testing.T) {
	yamlData := `
title: "test"
feature_slug: "test"
post_merge_checklist:
  groups:
    - title: "Build"
      items:
        - description: "Full build"
          command: "go build"
`
	var manifest IMPLManifest
	if err := yaml.Unmarshal([]byte(yamlData), &manifest); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if manifest.PostMergeChecklist == nil {
		t.Fatal("Expected PostMergeChecklist to be populated")
	}
	if len(manifest.PostMergeChecklist.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(manifest.PostMergeChecklist.Groups))
	}
}

func TestKnownIssueTitleFieldInManifest(t *testing.T) {
	yamlData := `
title: "test"
feature_slug: "test"
known_issues:
  - title: "Known issue 1"
    description: "Description 1"
    status: "Pre-existing"
  - title: "Known issue 2"
    description: "Description 2"
`
	var manifest IMPLManifest
	if err := yaml.Unmarshal([]byte(yamlData), &manifest); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(manifest.KnownIssues) != 2 {
		t.Fatalf("Expected 2 known issues, got %d", len(manifest.KnownIssues))
	}
	if manifest.KnownIssues[0].Title != "Known issue 1" {
		t.Errorf("Expected title 'Known issue 1', got %q", manifest.KnownIssues[0].Title)
	}
	if manifest.KnownIssues[1].Title != "Known issue 2" {
		t.Errorf("Expected title 'Known issue 2', got %q", manifest.KnownIssues[1].Title)
	}
}

func TestWave_AgentLaunchOrderSerialization(t *testing.T) {
	tests := []struct {
		name        string
		wave        Wave
		expectField bool
	}{
		{
			name: "with populated AgentLaunchOrder",
			wave: Wave{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
					{ID: "B", Task: "task B"},
				},
				AgentLaunchOrder: []string{"B", "A"},
			},
			expectField: true,
		},
		{
			name: "with nil AgentLaunchOrder",
			wave: Wave{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
				},
				AgentLaunchOrder: nil,
			},
			expectField: false,
		},
		{
			name: "with empty slice AgentLaunchOrder",
			wave: Wave{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A"},
				},
				AgentLaunchOrder: []string{},
			},
			expectField: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			yamlBytes, err := yaml.Marshal(&tt.wave)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			yamlStr := string(yamlBytes)

			// Check if agent_launch_order field is present in YAML
			hasField := containsString(yamlStr, "agent_launch_order")
			if hasField != tt.expectField {
				t.Errorf("Expected field presence %v, got %v. YAML:\n%s", tt.expectField, hasField, yamlStr)
			}

			// Unmarshal back and verify round-trip
			var roundTrip Wave
			if err := yaml.Unmarshal(yamlBytes, &roundTrip); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if roundTrip.Number != tt.wave.Number {
				t.Errorf("Number mismatch: expected %d, got %d", tt.wave.Number, roundTrip.Number)
			}
			if len(roundTrip.Agents) != len(tt.wave.Agents) {
				t.Errorf("Agents length mismatch: expected %d, got %d", len(tt.wave.Agents), len(roundTrip.Agents))
			}

			// Verify AgentLaunchOrder round-trip
			if len(roundTrip.AgentLaunchOrder) != len(tt.wave.AgentLaunchOrder) {
				t.Errorf("AgentLaunchOrder length mismatch: expected %d, got %d", len(tt.wave.AgentLaunchOrder), len(roundTrip.AgentLaunchOrder))
			}
			for i, id := range tt.wave.AgentLaunchOrder {
				if i >= len(roundTrip.AgentLaunchOrder) {
					break
				}
				if roundTrip.AgentLaunchOrder[i] != id {
					t.Errorf("AgentLaunchOrder[%d] mismatch: expected %q, got %q", i, id, roundTrip.AgentLaunchOrder[i])
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestQualityGate_PhaseAndParallelGroup_YAMLUnmarshal tests YAML unmarshaling with Phase and ParallelGroup fields present.
func TestQualityGate_PhaseAndParallelGroup_YAMLUnmarshal(t *testing.T) {
	yamlData := `
type: test
command: go test ./...
required: true
phase: PRE_VALIDATION
parallel_group: group1
`
	var gate QualityGate
	if err := yaml.Unmarshal([]byte(yamlData), &gate); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if gate.Type != "test" {
		t.Errorf("Expected type 'test', got %q", gate.Type)
	}
	if gate.Phase != GatePhasePre {
		t.Errorf("Expected phase PRE_VALIDATION, got %q", gate.Phase)
	}
	if gate.ParallelGroup != "group1" {
		t.Errorf("Expected parallel_group 'group1', got %q", gate.ParallelGroup)
	}
}

// TestQualityGate_BackwardCompatibility tests YAML unmarshaling with fields absent (backward compat).
func TestQualityGate_BackwardCompatibility(t *testing.T) {
	yamlData := `
type: build
command: go build ./...
required: true
`
	var gate QualityGate
	if err := yaml.Unmarshal([]byte(yamlData), &gate); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if gate.Type != "build" {
		t.Errorf("Expected type 'build', got %q", gate.Type)
	}
	// Phase should be empty (zero value) when not present in YAML
	if gate.Phase != "" {
		t.Errorf("Expected empty phase for backward compat, got %q", gate.Phase)
	}
	// ParallelGroup should be empty (zero value) when not present in YAML
	if gate.ParallelGroup != "" {
		t.Errorf("Expected empty parallel_group for backward compat, got %q", gate.ParallelGroup)
	}
}

// TestQualityGate_JSONSerialization tests JSON serialization includes new fields when present.
func TestQualityGate_JSONSerialization(t *testing.T) {
	gate := QualityGate{
		Type:          "lint",
		Command:       "golangci-lint run",
		Required:      true,
		Phase:         GatePhaseMain,
		ParallelGroup: "checks",
	}

	jsonBytes, err := json.Marshal(&gate)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify that new fields are present in JSON output
	if !containsString(jsonStr, "phase") {
		t.Errorf("Expected 'phase' field in JSON output, got: %s", jsonStr)
	}
	if !containsString(jsonStr, "VALIDATION") {
		t.Errorf("Expected 'VALIDATION' value in JSON output, got: %s", jsonStr)
	}
	if !containsString(jsonStr, "parallel_group") {
		t.Errorf("Expected 'parallel_group' field in JSON output, got: %s", jsonStr)
	}
	if !containsString(jsonStr, "checks") {
		t.Errorf("Expected 'checks' value in JSON output, got: %s", jsonStr)
	}

	// Verify round-trip
	var roundTrip QualityGate
	if err := json.Unmarshal(jsonBytes, &roundTrip); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if roundTrip.Phase != GatePhaseMain {
		t.Errorf("Round-trip phase mismatch: expected %q, got %q", GatePhaseMain, roundTrip.Phase)
	}
	if roundTrip.ParallelGroup != "checks" {
		t.Errorf("Round-trip parallel_group mismatch: expected 'checks', got %q", roundTrip.ParallelGroup)
	}
}

// TestGatePhase_Constants tests that GatePhase constants have expected string values.
func TestGatePhase_Constants(t *testing.T) {
	tests := []struct {
		constant GatePhase
		expected string
	}{
		{GatePhasePre, "PRE_VALIDATION"},
		{GatePhaseMain, "VALIDATION"},
		{GatePhasePost, "POST_VALIDATION"},
	}

	for _, tt := range tests {
		t.Run(string(tt.constant), func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(tt.constant))
			}
		})
	}
}
